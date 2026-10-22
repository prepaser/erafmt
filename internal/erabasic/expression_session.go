package erabasic

import (
	"bytes"
	"slices"
	"sort"
	"unicode/utf8"
)

type ExpressionSourceID uint8

const (
	ExpressionSourceOriginal ExpressionSourceID = iota
	ExpressionSourceFormatted
)

type expressionRangeKey struct {
	Source ExpressionSourceID
	start  int
	end    int
	mode   FormMode
}

type expressionLexKey struct {
	Source ExpressionSourceID
	start  int
	end    int
}

type expressionScanKey struct {
	Source ExpressionSourceID
	start  int
	mode   FormMode
}

type ExpressionSession struct {
	Formatted            *ExpressionSession
	OnResource           func(int, string)
	Exhausted            *ExprError
	LexTokens            int
	FormDepth            int
	FormFingerprintBytes int
	Sources              [2][]byte
	Utf8                 [2]bool
	Checked              [2]bool
	AllowFullwidthSpace  bool
	SkipFormTypes        bool
	BuiltinTypes         bool
	Dialect              Dialect
	Scans                map[expressionRangeKey]int
	ExplicitScans        map[expressionScanKey]expressionRangeKey
	Lexed                map[expressionLexKey][]ExprToken
	Fingerprints         map[expressionRangeKey][]byte
	AliasStarts          [2][]int
	AliasIndexed         [2]bool
	FormAdvisories       map[formAdvisoryKey]FormAdvisory
}

type formAdvisoryKey struct {
	Source ExpressionSourceID
	offset int
}

type FormAdvisory struct {
	Source  ExpressionSourceID
	Offset  int
	Code    string
	Message string
}

func (s *ExpressionSession) AddFormAdvisory(advisory FormAdvisory) {
	if advisory.Source == ExpressionSourceFormatted && s.Formatted != nil {
		advisory.Source = ExpressionSourceOriginal
		s.Formatted.AddFormAdvisory(advisory)
		return
	}
	if s.FormAdvisories == nil {
		s.FormAdvisories = make(map[formAdvisoryKey]FormAdvisory)
	}
	key := formAdvisoryKey{Source: advisory.Source, offset: advisory.Offset}
	s.FormAdvisories[key] = advisory
}

func (s *ExpressionSession) FormAdvisoriesIn(source ExpressionSourceID, start, end int) []FormAdvisory {
	if source == ExpressionSourceFormatted && s.Formatted != nil {
		advisories := s.Formatted.FormAdvisoriesIn(ExpressionSourceOriginal, start, end)
		for i := range advisories {
			advisories[i].Source = ExpressionSourceFormatted
		}
		return advisories
	}
	if start < 0 {
		start = 0
	}
	if end < start {
		return nil
	}
	out := make([]FormAdvisory, 0)
	for _, advisory := range s.FormAdvisories {
		if advisory.Source == source && advisory.Offset >= start && advisory.Offset < end {
			out = append(out, advisory)
		}
	}
	slices.SortFunc(out, func(a, b FormAdvisory) int {
		return a.Offset - b.Offset
	})
	return out
}

func NewExpressionSession(src []byte) *ExpressionSession {
	return NewExpressionSessionWithSpace(src, false)
}

func NewExpressionSessionWithSpace(src []byte, allowFullwidthSpace bool) *ExpressionSession {
	s := &ExpressionSession{}
	s.Sources[ExpressionSourceOriginal] = src
	s.AllowFullwidthSpace = allowFullwidthSpace
	return s
}

func (s *ExpressionSession) inputSpace(r rune) bool {
	return r == ' ' || r == '\t' || s.AllowFullwidthSpace && r == '\u3000'
}

func (s *ExpressionSession) SetSource(id ExpressionSourceID, src []byte) {
	s.Sources[id] = src
	s.Checked[id] = false
	s.Utf8[id] = false
	s.AliasStarts[id] = nil
	s.AliasIndexed[id] = false
	if id == ExpressionSourceFormatted {
		s.Formatted = NewExpressionSessionWithSpace(src, s.AllowFullwidthSpace)
		s.Formatted.SkipFormTypes = s.SkipFormTypes
		s.Formatted.BuiltinTypes = s.BuiltinTypes
		s.Formatted.Dialect = s.Dialect
		s.Formatted.OnResource = s.OnResource
		return
	}
	s.LexTokens = 0
	s.FormFingerprintBytes = 0
	s.Exhausted = nil
	for key := range s.Scans {
		if key.Source == id {
			delete(s.Scans, key)
		}
	}
	for key := range s.ExplicitScans {
		if key.Source == id {
			delete(s.ExplicitScans, key)
		}
	}
	for key := range s.Lexed {
		if key.Source == id {
			delete(s.Lexed, key)
		}
	}
	for key := range s.Fingerprints {
		if key.Source == id {
			delete(s.Fingerprints, key)
		}
	}
	for key := range s.FormAdvisories {
		if key.Source == id {
			delete(s.FormAdvisories, key)
		}
	}
}

func (s *ExpressionSession) HasAlias(id ExpressionSourceID, start, end int) bool {
	if id == ExpressionSourceFormatted && s.Formatted != nil {
		return s.Formatted.HasAlias(ExpressionSourceOriginal, start, end)
	}
	if start < 0 {
		start = 0
	}
	if end < start || end > len(s.Sources[id]) {
		return false
	}
	if !s.AliasIndexed[id] {
		s.AliasStarts[id] = make([]int, 0)
		src := s.Sources[id]
		for at := bytes.Index(src, []byte("[[")); at >= 0; {
			s.AliasStarts[id] = append(s.AliasStarts[id], at)
			next := at + 1
			rel := bytes.Index(src[next:], []byte("[["))
			if rel < 0 {
				break
			}
			at = next + rel
		}
		s.AliasIndexed[id] = true
	}
	starts := s.AliasStarts[id]
	index := sort.SearchInts(starts, start)
	return index < len(starts) && starts[index]+2 <= end
}

func (s *ExpressionSession) source(id ExpressionSourceID, start, end int) ([]byte, bool) {
	if id == ExpressionSourceFormatted && s.Formatted != nil {
		return s.Formatted.source(ExpressionSourceOriginal, start, end)
	}
	src := s.Sources[id]
	if start < 0 || end < start || end > len(src) {
		return nil, false
	}
	return src[start:end], true
}

func (s *ExpressionSession) ValidUTF8(id ExpressionSourceID) bool {
	if id == ExpressionSourceFormatted && s.Formatted != nil {
		return s.Formatted.ValidUTF8(ExpressionSourceOriginal)
	}
	if !s.Checked[id] {
		s.Utf8[id] = utf8.Valid(s.Sources[id])
		s.Checked[id] = true
	}
	return s.Utf8[id]
}

func (s *ExpressionSession) ScanFormAt(id ExpressionSourceID, start, limit int, mode FormMode) (int, error) {
	if id == ExpressionSourceFormatted && s.Formatted != nil {
		return s.Formatted.ScanFormAt(ExpressionSourceOriginal, start, limit, mode)
	}
	src := s.Sources[id]
	if start < 0 || limit < start || limit > len(src) {
		return 0, &ExprError{Offset: start, Msg: "FORM offset outside source"}
	}
	if s.Scans == nil {
		s.Scans = make(map[expressionRangeKey]int)
	}
	key := expressionRangeKey{Source: id, start: start, end: limit, mode: mode}
	if end, ok := s.Scans[key]; ok {
		return end, nil
	}
	if mode == FormQuoted || mode == FormConditional {
		if cached, ok := s.ExplicitScans[expressionScanKey{Source: id, start: start, mode: mode}]; ok && s.Scans[cached] <= limit {
			return s.Scans[cached], nil
		}
	}
	p := FormAnalyzer{Src: src[start:limit], Session: s, Source: id, Base: start}
	end, err := p.Scan(mode)
	if err != nil {
		return 0, ShiftExpressionError(err, start)
	}
	end += start
	s.Scans[key] = end
	if mode == FormQuoted || mode == FormConditional {
		if s.ExplicitScans == nil {
			s.ExplicitScans = make(map[expressionScanKey]expressionRangeKey)
		}
		s.ExplicitScans[expressionScanKey{Source: id, start: start, mode: mode}] = key
	}
	return end, nil
}

func (s *ExpressionSession) LexRange(id ExpressionSourceID, start, end, base int) ([]ExprToken, error) {
	if id == ExpressionSourceFormatted && s.Formatted != nil {
		return s.Formatted.LexRange(ExpressionSourceOriginal, start, end, base)
	}
	if s.Exhausted != nil {
		return nil, ShiftExpressionError(s.Exhausted, base-start)
	}
	src, ok := s.source(id, start, end)
	if !ok {
		return nil, &ExprError{Offset: base, Msg: "expression range outside source"}
	}
	if !s.ValidUTF8(id) {
		return nil, &ExprError{Offset: base, Msg: "invalid UTF-8"}
	}
	if s.Lexed == nil {
		s.Lexed = make(map[expressionLexKey][]ExprToken)
	}
	key := expressionLexKey{Source: id, start: start, end: end}
	if cached, ok := s.Lexed[key]; ok {
		return cloneExpressionTokens(cached, base), nil
	}
	tokens, err := lexExpressionCore(s, id, src, base, start)
	if err != nil {
		return nil, err
	}
	s.Lexed[key] = cloneExpressionTokens(tokens, -base)
	return tokens, nil
}

func cloneExpressionTokens(tokens []ExprToken, base int) []ExprToken {
	if len(tokens) == 0 {
		return nil
	}
	out := make([]ExprToken, len(tokens))
	copy(out, tokens)
	for i := range out {
		out[i].Off += base
	}
	return out
}

func (s *ExpressionSession) FormFingerprintAt(id ExpressionSourceID, start, end int, mode FormMode) ([]byte, error) {
	if id == ExpressionSourceFormatted && s.Formatted != nil {
		return s.Formatted.FormFingerprintAt(ExpressionSourceOriginal, start, end, mode)
	}
	src, ok := s.source(id, start, end)
	if !ok {
		return nil, &ExprError{Offset: start, Msg: "FORM range outside source"}
	}
	if !s.ValidUTF8(id) {
		return nil, &ExprError{Offset: start, Msg: "invalid UTF-8"}
	}
	if s.HasAlias(id, start, end) {
		return nil, &ExprError{Offset: start, Msg: "FORM contains an unresolved alias"}
	}
	if s.Fingerprints == nil {
		s.Fingerprints = make(map[expressionRangeKey][]byte)
	}
	key := expressionRangeKey{Source: id, start: start, end: end, mode: mode}
	if cached, ok := s.Fingerprints[key]; ok {
		return cached, nil
	}
	p := FormAnalyzer{Src: src, Fingerprint: new(bytes.Buffer), Session: s, Source: id, Base: start}
	localEnd, err := p.Scan(mode)
	if err != nil {
		return nil, ShiftExpressionError(err, start)
	}
	if (mode == FormQuoted || mode == FormConditional) && localEnd != len(src) {
		name := "quoted"
		if mode == FormConditional {
			name = "conditional"
		}
		return nil, &ExprError{Offset: start + localEnd, Msg: "characters after " + name + " FORM"}
	}
	fingerprint := p.Fingerprint.Bytes()
	if err := s.reserveFormFingerprint(start, len(fingerprint)); err != nil {
		return nil, err
	}
	s.Fingerprints[key] = fingerprint
	return fingerprint, nil
}

func ShiftExpressionError(err error, offset int) error {
	if e, ok := err.(*ExprError); ok {
		shifted := *e
		shifted.Offset += offset
		return &shifted
	}
	return err
}
