package erabasic

import (
	"sort"
	"unicode/utf8"
)

type PrintVAnalysis struct {
	Dialect             Dialect
	AllowFullwidthSpace bool
	ReserveMetadata     func(int, int) bool
	Resource            func(int, string)
	NewSession          func([]byte) *ExpressionSession
}

type PrintVProjection struct {
	Src      []byte
	Segments []printVProjectionSegment
}

type printVProjectionSegment struct {
	ProjectedStart int
	ProjectedEnd   int
	OriginalStart  int
	OriginalEnd    int
}

const projectionSegmentMetadataBytes = 32

func (p PrintVProjection) canMergeSegment(segment printVProjectionSegment) bool {
	if len(p.Segments) == 0 {
		return false
	}
	last := p.Segments[len(p.Segments)-1]
	return last.ProjectedEnd == segment.ProjectedStart && last.OriginalEnd == segment.OriginalStart && last.ProjectedEnd-last.ProjectedStart == last.OriginalEnd-last.OriginalStart && segment.ProjectedEnd-segment.ProjectedStart == segment.OriginalEnd-segment.OriginalStart
}

func (p *PrintVProjection) appendSegment(segment printVProjectionSegment) {
	if segment.ProjectedStart == segment.ProjectedEnd {
		return
	}
	if p.canMergeSegment(segment) {
		last := &p.Segments[len(p.Segments)-1]
		last.ProjectedEnd = segment.ProjectedEnd
		last.OriginalEnd = segment.OriginalEnd
		return
	}
	p.Segments = append(p.Segments, segment)
}

func (p PrintVProjection) OriginalOffset(offset int) int {
	if len(p.Segments) == 0 {
		return min(len(p.Src), max(0, offset))
	}
	if offset < 0 {
		return p.Segments[0].OriginalStart
	}
	if offset >= len(p.Src) {
		return p.Segments[len(p.Segments)-1].OriginalEnd
	}
	index := sort.Search(len(p.Segments), func(index int) bool {
		return p.Segments[index].ProjectedEnd >= offset
	})
	if index >= len(p.Segments) {
		return p.Segments[len(p.Segments)-1].OriginalEnd
	}
	segment := p.Segments[index]
	if offset <= segment.ProjectedStart {
		return segment.OriginalStart
	}
	if offset >= segment.ProjectedEnd {
		return segment.OriginalEnd
	}
	if segment.ProjectedEnd-segment.ProjectedStart == segment.OriginalEnd-segment.OriginalStart {
		return segment.OriginalStart + offset - segment.ProjectedStart
	}
	return segment.OriginalStart
}

func (p *PrintVProjection) truncate(end int) {
	if end < 0 {
		end = 0
	}
	if end >= len(p.Src) {
		return
	}
	originalEnd := p.OriginalOffset(end)
	p.Src = p.Src[:end]
	for len(p.Segments) != 0 && p.Segments[len(p.Segments)-1].ProjectedStart >= end {
		p.Segments = p.Segments[:len(p.Segments)-1]
	}
	if len(p.Segments) != 0 {
		last := &p.Segments[len(p.Segments)-1]
		last.ProjectedEnd = end
		last.OriginalEnd = originalEnd
	}
}

func MapPrintVError(err error, projection PrintVProjection) error {
	e, ok := err.(*ExprError)
	if !ok {
		return err
	}
	shifted := *e
	shifted.Offset = projection.OriginalOffset(e.Offset)
	return &shifted
}

type printVProjectionResult struct {
	Projection PrintVProjection
	CodeEnd    int
	RawEnd     bool
	Ok         bool
	Problem    *statementIssue
}

func (a *PrintVAnalysis) Project(src []byte) printVProjectionResult {
	projection := PrintVProjection{Src: src}
	result := printVProjectionResult{Projection: projection}
	codeEnd := len(src)
	rawEnd := false
	codeEndSet := false
	projectedBytes := 0
	rawAtoms := 0
	materialized := false
	unsupportedFullwidth := false
	tooLarge := func(offset int) bool {
		if a.Resource != nil {
			a.Resource(offset, "expression work exceeds the formatter resource limit")
		}
		return false
	}
	reserveProjectionSegment := func(segment printVProjectionSegment) bool {
		if segment.ProjectedStart == segment.ProjectedEnd || projection.canMergeSegment(segment) {
			return true
		}
		if a.ReserveMetadata == nil {
			return true
		}
		return a.ReserveMetadata(segment.OriginalStart, projectionSegmentMetadataBytes)
	}
	ensure := func(pos int) bool {
		if materialized {
			return true
		}
		if projectedBytes > MaxSyntaxBytes || pos > MaxSyntaxBytes {
			return tooLarge(pos)
		}
		var prefixSegment printVProjectionSegment
		if pos != 0 {
			prefixSegment = printVProjectionSegment{ProjectedEnd: pos, OriginalEnd: pos}
			if !reserveProjectionSegment(prefixSegment) {
				return false
			}
		}
		materialized = true
		projection.Src = make([]byte, 0, min(projectedBytes+16, MaxSyntaxBytes))
		projection.Src = append(projection.Src, src[:pos]...)
		if pos != 0 {
			projection.appendSegment(prefixSegment)
		}
		return true
	}
	appendMapped := func(text []byte, from, to int) bool {
		start := len(projection.Src)
		segment := printVProjectionSegment{ProjectedStart: start, ProjectedEnd: start + len(text), OriginalStart: from, OriginalEnd: to}
		if !reserveProjectionSegment(segment) {
			return false
		}
		projection.Src = append(projection.Src, text...)
		projection.appendSegment(segment)
		return true
	}
	appendOriginal := func(from, to int) bool {
		if to < from || projectedBytes > MaxSyntaxBytes-to+from {
			return tooLarge(from)
		}
		projectedBytes += to - from
		if materialized {
			start := len(projection.Src)
			segment := printVProjectionSegment{ProjectedStart: start, ProjectedEnd: start + to - from, OriginalStart: from, OriginalEnd: to}
			if !reserveProjectionSegment(segment) {
				return false
			}
			projection.Src = append(projection.Src, src[from:to]...)
			if to != from {
				projection.appendSegment(segment)
			}
		}
		return true
	}
	appendSpace := func(from, to int) bool {
		size := min(to-from, 2)
		if projectedBytes > MaxSyntaxBytes-size {
			return tooLarge(from)
		}
		projectedBytes += size
		if materialized {
			start := len(projection.Src)
			first := printVProjectionSegment{ProjectedStart: start, ProjectedEnd: start + 1, OriginalStart: from, OriginalEnd: to}
			if size == 2 {
				first = printVProjectionSegment{ProjectedStart: start, ProjectedEnd: start + 1, OriginalStart: from, OriginalEnd: from}
			}
			if !reserveProjectionSegment(first) {
				return false
			}
			projection.Src = append(projection.Src, ' ')
			if size == 1 {
				projection.appendSegment(first)
			} else {
				projection.appendSegment(first)
				projection.Src = append(projection.Src, ' ')
				second := printVProjectionSegment{ProjectedStart: start + 1, ProjectedEnd: start + 2, OriginalStart: from, OriginalEnd: to}
				if !reserveProjectionSegment(second) {
					return false
				}
				projection.appendSegment(second)
			}
		}
		return true
	}
	for pos := 0; pos < len(src); {
		if next, atom := SyntaxAtomEnd(src, pos, len(src)); atom {
			if next < 0 {
				result.Problem = &statementIssue{Offset: pos, Message: "unterminated PRINTV expression atom"}
				return result
			}
			if !materialized && next-pos > MaxSyntaxBytes-projectedBytes {
				tooLarge(pos)
				return result
			}
			if materialized {
				if !appendOriginal(pos, next) {
					return result
				}
			} else {
				projectedBytes += next - pos
			}
			pos = next
			continue
		}
		if src[pos] == ';' {
			if MandatoryInlineMarker(src, pos) {
				if !materialized && !ensure(pos) {
					return result
				}
				if !appendOriginal(pos, pos+3) {
					return result
				}
				pos += 3
				continue
			}
			if materialized {
				codeEnd = len(projection.Src)
			} else {
				codeEnd = pos
			}
			if unsupportedFullwidth {
				rawEnd = true
			}
			codeEndSet = true
			if pos+3 <= len(src) && ExecutableMarker(src, pos) {
				if !materialized {
					projection.Src = src[:pos+3]
				} else if !appendOriginal(pos, pos+3) {
					return result
				}
			} else if !materialized {
				projection.Src = src[:pos]
			}
			break
		}
		if src[pos] == '\'' {
			next := pos + 1
			for next < len(src) && src[next] != ',' && src[next] != 0 {
				if src[next] == '\\' {
					if next+1 == len(src) || src[next+1] == 0 {
						result.Problem = &statementIssue{Offset: next, Message: "missing character after PRINTV escape"}
						return result
					}
					next = escapedEnd(src, next)
				} else {
					next++
				}
			}
			if !materialized && unsupportedFullwidth {
				result.Projection, result.CodeEnd, result.RawEnd, result.Ok = projection, len(src), true, true
				return result
			}
			if !materialized && !ensure(pos) {
				return result
			}
			if projectedBytes == MaxSyntaxBytes || rawAtoms == MaxExpressionTokens {
				tooLarge(pos)
				return result
			}
			rawAtoms++
			projectedBytes++
			if !appendMapped([]byte{'X'}, pos, next) {
				return result
			}
			rawEnd = next == len(src)
			if next < len(src) && src[next] == 0 {
				codeEnd, rawEnd = len(projection.Src), true
				codeEndSet = true
				break
			}
			pos = next
			continue
		}
		if r, size := utf8.DecodeRune(src[pos:]); r == '\u3000' {
			if !a.AllowFullwidthSpace {
				unsupportedFullwidth = true
				if materialized && !appendOriginal(pos, pos+size) {
					return result
				}
				pos += size
				continue
			}
			next := pos + size
			for next < len(src) {
				r, size = utf8.DecodeRune(src[next:])
				if r != '\u3000' && r != ' ' && r != '\t' {
					break
				}
				next += size
			}
			if !materialized && !ensure(pos) {
				return result
			}
			if !appendSpace(pos, next) {
				return result
			}
			pos = next
			continue
		}
		if src[pos] == ' ' || src[pos] == '\t' {
			next := pos + 1
			for next < len(src) && (src[next] == ' ' || src[next] == '\t') {
				next++
			}
			if next-pos > 1 {
				if !materialized && !ensure(pos) {
					return result
				}
				if !appendSpace(pos, next) {
					return result
				}
				pos = next
				continue
			}
		}
		if !materialized {
			if projectedBytes == MaxSyntaxBytes {
				tooLarge(pos)
				return result
			}
			projectedBytes++
		} else if !appendOriginal(pos, pos+1) {
			return result
		}
		pos++
	}
	if unsupportedFullwidth && !materialized {
		codeEnd, rawEnd = len(src), true
		result.Projection, result.CodeEnd, result.RawEnd, result.Ok = projection, codeEnd, rawEnd, true
		return result
	}
	if !codeEndSet {
		codeEnd = len(projection.Src)
	}
	result.Projection, result.CodeEnd, result.RawEnd, result.Ok = projection, codeEnd, rawEnd, true
	return result
}

type printVValidation struct {
	Projection PrintVProjection
	CodeEnd    int
	RawEnd     bool
	Session    *ExpressionSession
	Excluded   bool
	ErrorBase  int
	Err        error
	Problem    *statementIssue
}

func (a *PrintVAnalysis) Validate(projection PrintVProjection, codeEnd int, rawEnd bool) printVValidation {
	result := printVValidation{Projection: projection, CodeEnd: codeEnd, RawEnd: rawEnd}
	newSession := func() *ExpressionSession {
		if a.NewSession != nil {
			return a.NewSession(result.Projection.Src)
		}
		s := NewExpressionSessionWithSpace(result.Projection.Src, a.AllowFullwidthSpace)
		s.BuiltinTypes = true
		s.Dialect = a.Dialect
		return s
	}
	active, _, err := expressionBodyEndWithSession(newSession(), ExpressionSourceOriginal, result.Projection.Src, 0)
	if err != nil {
		result.Err = err
		return result
	}
	if active < len(result.Projection.Src) && result.Projection.Src[active] == 0 {
		result.Projection.truncate(active)
		result.CodeEnd, result.RawEnd = active, true
	}
	result.Session = newSession()
	if ExpressionExcludedWithSession(result.Session, result.Projection.Src) {
		result.Excluded = true
		return result
	}
	if TrimInputLeft(result.Projection.Src, 0, result.CodeEnd, a.AllowFullwidthSpace) >= TrimInputRight(result.Projection.Src, 0, result.CodeEnd, a.AllowFullwidthSpace) {
		return result
	}
	spans := SplitTopLevel(result.Projection.Src, 0, result.CodeEnd, ',', result.Session)
	for index, span := range spans {
		if TrimInputLeft(result.Projection.Src, span.Start, span.End, a.AllowFullwidthSpace) < TrimInputRight(result.Projection.Src, span.Start, span.End, a.AllowFullwidthSpace) {
			continue
		}
		if index == len(spans)-1 && index > 0 {
			continue
		}
		result.Problem = &statementIssue{Offset: span.Start, Message: "PRINTV contains an empty expression"}
		return result
	}
	macroPrefix := false
	for _, span := range spans {
		left := TrimInputLeft(result.Projection.Src, span.Start, span.End, a.AllowFullwidthSpace)
		right := TrimInputRight(result.Projection.Src, left, span.End, a.AllowFullwidthSpace)
		if left >= right {
			continue
		}
		tokens, err := result.Session.LexRange(ExpressionSourceOriginal, left, right, left)
		if err != nil {
			macroPrefix = true
			continue
		}
		if ExpressionNeedsDeferral(tokens, a.Dialect) {
			macroPrefix = true
			continue
		}
		if !macroPrefix {
			if err := ProvenExpressionShape(tokens, a.Dialect, false); err != nil {
				result.ErrorBase = 0
				result.Err = err
				return result
			}
		}
		if !macroPrefix && BuiltinScalarIndexesMissing(tokens, a.Dialect) {
			result.ErrorBase = left
			result.Err = &ExprError{Msg: "scalar variable requires indexes"}
			return result
		}
	}
	if bodyEnd, unsupported, err := expressionBodyEndWithSession(result.Session, ExpressionSourceOriginal, result.Projection.Src, 0); err == nil && !unsupported {
		if tokens, err := result.Session.LexRange(ExpressionSourceOriginal, 0, bodyEnd, 0); err == nil {
			if err := ProvenExpressionShape(tokens, a.Dialect, false); err != nil {
				result.Err = err
				return result
			}
		}
	}
	return result
}
