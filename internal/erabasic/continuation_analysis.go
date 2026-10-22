package erabasic

import (
	"bytes"
	"sort"
	"unicode/utf8"
)

// ContinuationSegment maps one physical continuation row into the logical
// source used by the language analyzer. Separator bytes are intentionally not
// part of any segment.
type ContinuationSegment struct {
	LogicalStart int
	LogicalEnd   int
	Line         int
	SourceStart  int
	SourceEnd    int
	Indent       int
}

type ContinuationInput struct {
	First, Last int
	Logical     []byte
	Segments    []ContinuationSegment
}

func (input *ContinuationInput) Position(offset int) (SourcePosition, bool) {
	index, local := ContinuationDiagnosticSegment(offset, input.Segments)
	if index < 0 {
		return SourcePosition{}, false
	}
	segment := input.Segments[index]
	return SourcePosition{segment.Line, min(segment.SourceEnd, max(segment.SourceStart, segment.SourceStart+local))}, true
}

func ContinuationPhysicalHead(src []byte, dialect Dialect, allowFullwidthSpace bool, input *ContinuationInput) (head []byte, optional, marked bool) {
	if input == nil || len(input.Segments) == 0 {
		return nil, false, false
	}
	segment := input.Segments[0]
	if segment.SourceStart < 0 || segment.SourceEnd < segment.SourceStart || segment.SourceEnd > len(src) {
		return nil, false, false
	}
	head, _, optional, marked = HeadOfWithSpaceDialect(src[segment.SourceStart:segment.SourceEnd], allowFullwidthSpace, dialect)
	return head, optional, marked
}

func (c *SemanticContext) ContinuationNilCommandIdentityUnknown(input *ContinuationInput, command string) bool {
	head, optional, marked := ContinuationPhysicalHead(c.Src, c.Dialect, c.AllowFullwidthSpace, input)
	if optional || marked {
		return true
	}
	if len(head) == 0 || head[0] == ';' {
		return command != ""
	}
	physicalCommand, physicalWordEnd := CommandOfWithSpace(head, c.AllowFullwidthSpace)
	if physicalCommand == "" {
		return command != ""
	}
	if physicalWordEnd == len(head) {
		return true
	}
	return command != physicalCommand
}

func (c *SemanticContext) ContinuationNilHeaderEquivalent(input *ContinuationInput) bool {
	head, optional, marked := ContinuationPhysicalHead(c.Src, c.Dialect, c.AllowFullwidthSpace, input)
	if optional || marked || len(head) == 0 || head[0] != '@' {
		return false
	}
	session := NewExpressionSessionWithSpace(head, c.AllowFullwidthSpace)
	session.BuiltinTypes = true
	session.Dialect = c.Dialect
	if c.OnResource != nil {
		session.OnResource = c.OnResource
	}
	validation := validateHeaderWithSession(head, session)
	if session.Exhausted != nil {
		return false
	}
	complete, comment := continuationPhysicalHeaderBoundary(head, session)
	if validation.Status != headerValid {
		return complete
	}
	return comment
}

func (c *SemanticContext) ContinuationNilPreprocessorEquivalent(input *ContinuationInput) bool {
	head, optional, marked := ContinuationPhysicalHead(c.Src, c.Dialect, c.AllowFullwidthSpace, input)
	if optional || marked || len(head) == 0 || head[0] != '[' {
		return false
	}
	end := bytes.IndexByte(head, ']')
	if end < 0 {
		return false
	}
	first, _, count := PreprocessorFields(head, end, c.AllowFullwidthSpace)
	return count != 0 && len(first) != 0
}

func (c *SemanticContext) ContinuationNilDispatchUnknown(input *ContinuationInput) (unknown, preserveFile bool) {
	head, optional, marked := ContinuationPhysicalHead(c.Src, c.Dialect, c.AllowFullwidthSpace, input)
	if optional || marked {
		return true, true
	}
	logicalHead, _, logicalOptional, logicalMarked := HeadOfWithSpaceDialect(input.Logical, c.AllowFullwidthSpace, c.Dialect)
	if logicalOptional || logicalMarked {
		return true, len(logicalHead) > 0 && (logicalHead[0] == '@' || logicalHead[0] == '[')
	}
	if len(head) != 0 {
		if continuationNilIncompleteMarker(head) {
			return true, false
		}
		return false, false
	}
	if len(logicalHead) == 0 {
		return len(input.Segments) != 0, len(input.Segments) != 0
	}
	return true, true
}

func continuationNilIncompleteMarker(head []byte) bool {
	if len(head) == 1 {
		return head[0] == ';'
	}
	return len(head) == 2 && head[0] == ';' && (head[1] == '!' || head[1] == '#' || head[1] == '^')
}

func continuationPhysicalHeaderBoundary(head []byte, session *ExpressionSession) (complete, comment bool) {
	if len(head) < 2 || head[0] != '@' {
		return false, false
	}
	start := TrimInputLeftWithSession(head, 1, len(head), session)
	nameEnd, ok := LabelIdentifierEndWithSession(head, start, session)
	if !ok {
		return start < len(head) && head[start] >= '0' && head[start] <= '9', false
	}
	pos := skipHeaderSpaceWithSession(head, nameEnd, session)
	if pos >= len(head) || head[pos] == ';' {
		return pos < len(head), pos < len(head)
	}
	var result headerValidation
	switch head[pos] {
	case '(':
		var close int
		close, result = scanHeaderDelimiterWithSession(head, pos, session)
		if result.Status == headerValid {
			return true, continuationHeaderCommentSealed(head, close+1, session)
		}
	case '[':
		_, result = scanHeaderDelimiterWithSession(head, pos, session)
	case ',':
		end, result := scanHeaderCodeEndWithSession(head, pos+1, session)
		if result.Status != headerValid {
			return continuationHeaderDelimiterUnrepairable(head, pos+1, session), false
		}
		if end < len(head) && head[end] == ';' {
			return true, true
		}
		return false, false
	default:
		return true, false
	}
	if result.Status != headerValid {
		return continuationHeaderDelimiterUnrepairable(head, pos, session), false
	}
	return true, false
}

func continuationHeaderCommentSealed(src []byte, start int, session *ExpressionSession) bool {
	pos := skipHeaderSpaceWithSession(src, start, session)
	if pos >= len(src) || src[pos] != ';' {
		return false
	}
	for ExecutableMarker(src, pos) {
		pos += 3
		pos = skipHeaderSpaceWithSession(src, pos, session)
		if pos >= len(src) || src[pos] != ';' {
			return false
		}
	}
	return true
}

func continuationHeaderDelimiterUnrepairable(src []byte, start int, session *ExpressionSession) bool {
	stack := make([]byte, 0, 2)
	i := start
	if start < len(src) && (src[start] == '(' || src[start] == '[') {
		stack = append(stack, headerCloseFor(src[start]))
		i++
	}
	for i < len(src) {
		next, atom, result := scanHeaderAtomWithSession(src, i, len(src), session)
		if result.Status != headerValid {
			return false
		}
		if atom {
			i = next
			continue
		}
		if size := InputSpaceSize(src, i, session.AllowFullwidthSpace); size > 0 {
			i += size
			continue
		}
		if headerFullwidthAtWithSession(src, i, session) {
			return false
		}
		switch src[i] {
		case ';':
			return len(stack) != 0
		case '(', '[':
			stack = append(stack, headerCloseFor(src[i]))
		case ')', ']':
			if len(stack) == 0 || stack[len(stack)-1] != src[i] {
				return true
			}
			stack = stack[:len(stack)-1]
		}
		i++
	}
	return false
}

func ContinuationLabelStateCommitted(dialect Dialect, allowFullwidthSpace bool, optional, conditional bool, src []byte) bool {
	head, _, _, marked := HeadOfWithSpaceDialect(src, allowFullwidthSpace, dialect)
	return len(head) != 0 && head[0] == '$' && !optional && !marked && !conditional && !BytesHasAlias(head)
}

func ContinuationDeclarationSafe(separatorSet bool, optional, conditional bool, head []byte) bool {
	if !separatorSet || optional || conditional || BytesHasAlias(head) || len(head) == 0 || head[0] != '#' {
		return false
	}
	switch declarationName(head) {
	case "DIM", "DIMS", "FUNCTION", "FUNCTIONS", "LOCALSIZE", "LOCALSSIZE", "SINGLE", "LATER", "PRI", "ONLY":
		return true
	default:
		return false
	}
}

func ContinuationDeclarationCommitted(dialect Dialect, allowFullwidthSpace bool, optional, conditional, sharpStateUnknown, opaque, warningOnly bool, src []byte) bool {
	head, _, _, marked := HeadOfWithSpaceDialect(src, allowFullwidthSpace, dialect)
	if len(head) == 0 || head[0] != '#' || optional || marked || conditional || BytesHasAlias(head) || sharpStateUnknown {
		return false
	}
	return !opaque || warningOnly
}

func ContinuationPhysicalSharp(src []byte, lines []Line, first, last int) bool {
	if first < 0 || last <= first+1 || last > len(lines) {
		return false
	}
	for row := first + 1; row < last; row++ {
		body := bytes.TrimSpace(src[lines[row].Start:lines[row].BodyEnd])
		if len(body) == 0 {
			continue
		}
		return body[0] == '#'
	}
	return false
}

func ContinuationUnsafe(src []byte) bool {
	return bytes.Contains(src, []byte("[[")) || bytes.Contains(src, []byte(";!;")) || bytes.Contains(src, []byte(";#;"))
}

func ContinuationByteOffset(columnNumber int, src []byte) int {
	if columnNumber <= 1 {
		return 0
	}
	remaining := columnNumber - 1
	for offset := 0; offset < len(src); {
		_, size := utf8.DecodeRune(src[offset:])
		if size == 0 {
			break
		}
		if remaining == 0 {
			return offset
		}
		offset += size
		remaining--
	}
	return len(src)
}

func ContinuationDiagnosticSegment(offset int, segments []ContinuationSegment) (int, int) {
	index := sort.Search(len(segments), func(i int) bool {
		return segments[i].LogicalEnd > offset
	})
	if index < len(segments) && offset >= segments[index].LogicalStart {
		return index, offset - segments[index].LogicalStart
	}
	index = sort.Search(len(segments), func(i int) bool {
		return segments[i].LogicalEnd >= offset
	})
	if index > 0 && index < len(segments) && offset < segments[index].LogicalStart {
		index--
		return index, segments[index].SourceEnd - segments[index].SourceStart
	}
	if index < len(segments) {
		if offset == segments[index].LogicalEnd {
			return index, segments[index].SourceEnd - segments[index].SourceStart
		}
		return index, offset - segments[index].LogicalStart
	}
	if len(segments) == 0 {
		return -1, 0
	}
	last := segments[len(segments)-1]
	return len(segments) - 1, last.SourceEnd - last.SourceStart
}
