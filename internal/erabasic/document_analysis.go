package erabasic

import (
	"bytes"
	"fmt"
	"unicode"
)

type sourceHeadKind uint8

const (
	SourceHeadEmpty sourceHeadKind = iota
	SourceHeadComment
	SourceHeadDirective
	SourceHeadFunction
	SourceHeadDeclaration
	SourceHeadLabel
	SourceHeadDelimiter
	sourceHeadCommand
)

func ClassifySourceHead(head []byte) sourceHeadKind {
	if len(head) == 0 {
		return SourceHeadEmpty
	}
	switch head[0] {
	case ';':
		return SourceHeadComment
	case '[':
		if !bytes.HasPrefix(head, []byte("[[")) {
			return SourceHeadDirective
		}
	case '@':
		return SourceHeadFunction
	case '#':
		return SourceHeadDeclaration
	case '$':
		return SourceHeadLabel
	case '{', '}':
		return SourceHeadDelimiter
	}
	return sourceHeadCommand
}

func ContinuationOpening(body []byte, allowFullwidth bool) bool {
	return bytes.Equal(bytes.TrimRightFunc(TrimInput(body, allowFullwidth), unicode.IsSpace), []byte("{"))
}

func CommentIdentityUnknown(head []byte) bool {
	if len(head) == 0 || head[0] != ';' {
		return false
	}
	tail := head[1:]
	return bytes.HasPrefix(tail, []byte("[[")) || bytes.HasPrefix(tail, []byte("![[")) || bytes.HasPrefix(tail, []byte("#[[")) || bytes.HasPrefix(tail, []byte("^[["))
}

func SourceStructureUnknown(head []byte) bool {
	return bytes.HasPrefix(head, []byte("　")) || bytes.HasPrefix(head, []byte("[["))
}

type sourceIssue struct {
	Line     int
	Offset   int
	Severity Severity
	Code     string
	Message  string
}

func (s *DocumentSession) FunctionEndIssues(next int) []sourceIssue {
	if s.Preserved {
		return nil
	}
	severity := Error
	if s.Optional || s.Conditional() {
		severity = Unsupported
	}
	var issues []sourceIssue
	if s.State.Sif != 0 {
		row := s.State.SifOrigin - 1
		if row < 0 || row >= len(s.Lines) {
			row = next - 1
			if row < 0 {
				row = 0
			}
			if row >= len(s.Lines) {
				row = len(s.Lines) - 1
			}
		}
		issues = append(issues, sourceIssue{row, s.Lines[row].IndentEnd, severity, "invalid_structure", "SIF has no following instruction"})
	}
	if len(s.State.Frames) > 0 {
		frame := s.State.Frames[len(s.State.Frames)-1]
		row := frame.Line - 1
		issues = append(issues, sourceIssue{row, s.Lines[row].IndentEnd, severity, "invalid_structure", fmt.Sprintf("unclosed %s before line %d", frame.Kind, next+1)})
	}
	return issues
}

func (s *DocumentSession) SourceStructureLimitExceeded(command string) bool {
	depth := s.State.Depth()
	if depth > MaxStructureDepth {
		return true
	}
	if openers[command] != "" {
		return depth+1 > MaxStructureDepth
	}
	if (command == "CASE" || command == "CASEELSE") && len(s.State.Frames) != 0 {
		top := s.State.Frames[len(s.State.Frames)-1]
		if top.Kind == "SELECTCASE" && !top.Branch {
			return depth+1 > MaxStructureDepth
		}
	}
	return false
}

func (s *DocumentSession) PayloadBodyKnown(command string) bool {
	if CommandUnavailable(command, s.Dialect) {
		return false
	}
	if spec, ok := registeredCommandSpecs[command]; ok {
		return registeredDialect(spec.Dialects, s.Dialect) && (spec.Mode == RegisteredRaw || spec.Mode == RegisteredForm || spec.Mode == RegisteredPrintV || spec.Mode == RegisteredFormAny || spec.Mode == RegisteredInputS)
	}
	return RawCommand(command) || PrintVCommand(command) || PrintFormCommand(command) || CallCommand(command)
}

func (s *DocumentSession) ProtectFunctionSource(next int) {
	if s.Preserved {
		for i := s.FunctionStart; i < next; i++ {
			s.Opaque[i] = true
		}
	}
}
