package format

import (
	"bytes"
	"slices"
	"sort"
	"unicode/utf8"
)

type expressionSourceID uint8

const (
	expressionSourceOriginal expressionSourceID = iota
	expressionSourceFormatted
)

type expressionRangeKey struct {
	source expressionSourceID
	start  int
	end    int
	mode   formMode
}

type expressionLexKey struct {
	source expressionSourceID
	start  int
	end    int
}

type expressionScanKey struct {
	source expressionSourceID
	start  int
	mode   formMode
}

type expressionSession struct {
	formatted            *expressionSession
	onResource           func(int, string)
	exhausted            *exprError
	lexTokens            int
	formDepth            int
	formFingerprintBytes int
	sources              [2][]byte
	utf8                 [2]bool
	checked              [2]bool
	allowFullwidthSpace  bool
	skipFormTypes        bool
	builtinTypes         bool
	dialect              Dialect

	scans          map[expressionRangeKey]int
	explicitScans  map[expressionScanKey]expressionRangeKey
	lexed          map[expressionLexKey][]exprToken
	fingerprints   map[expressionRangeKey][]byte
	aliasStarts    [2][]int
	aliasIndexed   [2]bool
	formAdvisories map[formAdvisoryKey]formAdvisory
}

type formAdvisoryKey struct {
	source expressionSourceID
	offset int
}

type formAdvisory struct {
	source  expressionSourceID
	offset  int
	code    string
	message string
}

func (s *expressionSession) addFormAdvisory(advisory formAdvisory) {
	if advisory.source == expressionSourceFormatted && s.formatted != nil {
		advisory.source = expressionSourceOriginal
		s.formatted.addFormAdvisory(advisory)
		return
	}
	if s.formAdvisories == nil {
		s.formAdvisories = make(map[formAdvisoryKey]formAdvisory)
	}
	key := formAdvisoryKey{source: advisory.source, offset: advisory.offset}
	s.formAdvisories[key] = advisory
}

func (s *expressionSession) formAdvisoriesIn(source expressionSourceID, start, end int) []formAdvisory {
	if source == expressionSourceFormatted && s.formatted != nil {
		advisories := s.formatted.formAdvisoriesIn(expressionSourceOriginal, start, end)
		for i := range advisories {
			advisories[i].source = expressionSourceFormatted
		}
		return advisories
	}
	if start < 0 {
		start = 0
	}
	if end < start {
		return nil
	}
	out := make([]formAdvisory, 0)
	for _, advisory := range s.formAdvisories {
		if advisory.source == source && advisory.offset >= start && advisory.offset < end {
			out = append(out, advisory)
		}
	}
	slices.SortFunc(out, func(a, b formAdvisory) int {
		return a.offset - b.offset
	})
	return out
}

func newExpressionSession(src []byte) *expressionSession {
	return newExpressionSessionWithSpace(src, false)
}

func newExpressionSessionWithSpace(src []byte, allowFullwidthSpace bool) *expressionSession {
	s := &expressionSession{}
	s.sources[expressionSourceOriginal] = src
	s.allowFullwidthSpace = allowFullwidthSpace
	return s
}

func (s *expressionSession) inputSpace(r rune) bool {
	return r == ' ' || r == '\t' || s.allowFullwidthSpace && r == '\u3000'
}

func (s *expressionSession) setSource(id expressionSourceID, src []byte) {
	s.sources[id] = src
	s.checked[id] = false
	s.utf8[id] = false
	s.aliasStarts[id] = nil
	s.aliasIndexed[id] = false
	if id == expressionSourceFormatted {
		s.formatted = newExpressionSessionWithSpace(src, s.allowFullwidthSpace)
		s.formatted.skipFormTypes = s.skipFormTypes
		s.formatted.builtinTypes = s.builtinTypes
		s.formatted.dialect = s.dialect
		s.formatted.onResource = s.onResource
		return
	}
	s.lexTokens = 0
	s.formFingerprintBytes = 0
	s.exhausted = nil
	for key := range s.scans {
		if key.source == id {
			delete(s.scans, key)
		}
	}
	for key := range s.explicitScans {
		if key.source == id {
			delete(s.explicitScans, key)
		}
	}
	for key := range s.lexed {
		if key.source == id {
			delete(s.lexed, key)
		}
	}
	for key := range s.fingerprints {
		if key.source == id {
			delete(s.fingerprints, key)
		}
	}
	for key := range s.formAdvisories {
		if key.source == id {
			delete(s.formAdvisories, key)
		}
	}
}

func (s *expressionSession) hasAlias(id expressionSourceID, start, end int) bool {
	if id == expressionSourceFormatted && s.formatted != nil {
		return s.formatted.hasAlias(expressionSourceOriginal, start, end)
	}
	if start < 0 {
		start = 0
	}
	if end < start || end > len(s.sources[id]) {
		return false
	}
	if !s.aliasIndexed[id] {
		s.aliasStarts[id] = make([]int, 0)
		src := s.sources[id]
		for at := bytes.Index(src, []byte("[[")); at >= 0; {
			s.aliasStarts[id] = append(s.aliasStarts[id], at)
			next := at + 1
			rel := bytes.Index(src[next:], []byte("[["))
			if rel < 0 {
				break
			}
			at = next + rel
		}
		s.aliasIndexed[id] = true
	}
	starts := s.aliasStarts[id]
	index := sort.SearchInts(starts, start)
	return index < len(starts) && starts[index]+2 <= end
}

func (s *expressionSession) source(id expressionSourceID, start, end int) ([]byte, bool) {
	if id == expressionSourceFormatted && s.formatted != nil {
		return s.formatted.source(expressionSourceOriginal, start, end)
	}
	src := s.sources[id]
	if start < 0 || end < start || end > len(src) {
		return nil, false
	}
	return src[start:end], true
}

func (s *expressionSession) validUTF8(id expressionSourceID) bool {
	if id == expressionSourceFormatted && s.formatted != nil {
		return s.formatted.validUTF8(expressionSourceOriginal)
	}
	if !s.checked[id] {
		s.utf8[id] = utf8.Valid(s.sources[id])
		s.checked[id] = true
	}
	return s.utf8[id]
}

func (s *expressionSession) scanFormAt(id expressionSourceID, start, limit int, mode formMode) (int, error) {
	if id == expressionSourceFormatted && s.formatted != nil {
		return s.formatted.scanFormAt(expressionSourceOriginal, start, limit, mode)
	}
	src := s.sources[id]
	if start < 0 || limit < start || limit > len(src) {
		return 0, &exprError{offset: start, msg: "FORM offset outside source"}
	}
	if s.scans == nil {
		s.scans = make(map[expressionRangeKey]int)
	}
	key := expressionRangeKey{source: id, start: start, end: limit, mode: mode}
	if end, ok := s.scans[key]; ok {
		return end, nil
	}
	if mode == formQuoted || mode == formConditional {
		if cached, ok := s.explicitScans[expressionScanKey{source: id, start: start, mode: mode}]; ok && s.scans[cached] <= limit {
			return s.scans[cached], nil
		}
	}
	p := formParser{src: src[start:limit], session: s, source: id, base: start}
	end, err := p.scan(mode)
	if err != nil {
		return 0, shiftExpressionError(err, start)
	}
	end += start
	s.scans[key] = end
	if mode == formQuoted || mode == formConditional {
		if s.explicitScans == nil {
			s.explicitScans = make(map[expressionScanKey]expressionRangeKey)
		}
		s.explicitScans[expressionScanKey{source: id, start: start, mode: mode}] = key
	}
	return end, nil
}

func (s *expressionSession) lexRange(id expressionSourceID, start, end, base int) ([]exprToken, error) {
	if id == expressionSourceFormatted && s.formatted != nil {
		return s.formatted.lexRange(expressionSourceOriginal, start, end, base)
	}
	if s.exhausted != nil {
		return nil, shiftExpressionError(s.exhausted, base-start)
	}
	src, ok := s.source(id, start, end)
	if !ok {
		return nil, &exprError{offset: base, msg: "expression range outside source"}
	}
	if !s.validUTF8(id) {
		return nil, &exprError{offset: base, msg: "invalid UTF-8"}
	}
	if s.lexed == nil {
		s.lexed = make(map[expressionLexKey][]exprToken)
	}
	key := expressionLexKey{source: id, start: start, end: end}
	if cached, ok := s.lexed[key]; ok {
		return cloneExpressionTokens(cached, base), nil
	}
	tokens, err := lexExpressionCore(s, id, src, base, start)
	if err != nil {
		return nil, err
	}
	s.lexed[key] = cloneExpressionTokens(tokens, -base)
	return tokens, nil
}

func cloneExpressionTokens(tokens []exprToken, base int) []exprToken {
	if len(tokens) == 0 {
		return nil
	}
	out := make([]exprToken, len(tokens))
	copy(out, tokens)
	for i := range out {
		out[i].off += base
	}
	return out
}

func (s *expressionSession) formFingerprintAt(id expressionSourceID, start, end int, mode formMode) ([]byte, error) {
	if id == expressionSourceFormatted && s.formatted != nil {
		return s.formatted.formFingerprintAt(expressionSourceOriginal, start, end, mode)
	}
	src, ok := s.source(id, start, end)
	if !ok {
		return nil, &exprError{offset: start, msg: "FORM range outside source"}
	}
	if !s.validUTF8(id) {
		return nil, &exprError{offset: start, msg: "invalid UTF-8"}
	}
	if s.hasAlias(id, start, end) {
		return nil, &exprError{offset: start, msg: "FORM contains an unresolved alias"}
	}
	if s.fingerprints == nil {
		s.fingerprints = make(map[expressionRangeKey][]byte)
	}
	key := expressionRangeKey{source: id, start: start, end: end, mode: mode}
	if cached, ok := s.fingerprints[key]; ok {
		return cached, nil
	}
	p := formParser{src: src, fingerprint: new(bytes.Buffer), session: s, source: id, base: start}
	localEnd, err := p.scan(mode)
	if err != nil {
		return nil, shiftExpressionError(err, start)
	}
	if (mode == formQuoted || mode == formConditional) && localEnd != len(src) {
		name := "quoted"
		if mode == formConditional {
			name = "conditional"
		}
		return nil, &exprError{offset: start + localEnd, msg: "characters after " + name + " FORM"}
	}
	fingerprint := p.fingerprint.Bytes()
	if err := s.reserveFormFingerprint(start, len(fingerprint)); err != nil {
		return nil, err
	}
	s.fingerprints[key] = fingerprint
	return fingerprint, nil
}

func shiftExpressionError(err error, offset int) error {
	if e, ok := err.(*exprError); ok {
		shifted := *e
		shifted.offset += offset
		return &shifted
	}
	return err
}
