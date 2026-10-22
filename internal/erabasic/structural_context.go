package erabasic

import "strings"

// StructuralContext contains language facts used while scanning a function.
type structuralIssueSeverity uint8

const (
	structuralWarning structuralIssueSeverity = iota
	StructuralUnsupported
)

type StructuralIssue struct {
	Line     int
	Offset   int
	Code     string
	Message  string
	Severity structuralIssueSeverity
	Opaque   bool
}

type StructuralContext struct {
	Source              []byte
	Lines               []Line
	Kind                Kind
	Dialect             Dialect
	AllowFullwidthSpace bool
	ExpressionEnabled   bool
	Function            *FunctionContext
	FunctionHeader      []byte
	Base                int
	Conditional         bool
	InContinuationProxy bool
	PendingDeclaration  string
	PendingLabel        string
	LexRange            func(start, end int) ([]ExprToken, error)
	NewSession          func(src []byte) *ExpressionSession
	Issue               func(StructuralIssue)
}

func (c *StructuralContext) report(line, offset int, code, message string, severity structuralIssueSeverity, opaque bool) {
	if c.Issue != nil {
		c.Issue(StructuralIssue{Line: line, Offset: offset, Code: code, Message: message, Severity: severity, Opaque: opaque})
	}
}

func (c *StructuralContext) session(src []byte) *ExpressionSession {
	if c.NewSession != nil {
		return c.NewSession(src)
	}
	s := NewExpressionSessionWithSpace(src, c.AllowFullwidthSpace)
	s.BuiltinTypes = true
	s.Dialect = c.Dialect
	return s
}

func (c *StructuralContext) lex(start, end int) ([]ExprToken, error) {
	if c.LexRange != nil {
		return c.LexRange(start, end)
	}
	s := c.session(c.Source)
	return s.LexRange(ExpressionSourceOriginal, start, end, start)
}

func (c *StructuralContext) split(start, end int, separator byte) []SyntaxSpan {
	return SplitTopLevel(c.Source, start, end, separator, c.session(c.Source))
}

func (c *StructuralContext) instructionBody(head []byte, wordEnd int) ([]byte, int, int, int, bool) {
	return InstructionBodyWithSpace(head, wordEnd, c.AllowFullwidthSpace, c.session(head))
}

func (c *StructuralContext) eventFunctionStatus() (bool, bool) {
	if c.Kind != ERB || c.Function == nil {
		return false, false
	}
	if c.Function.FunctionName != "" {
		return c.Function.EventFunctionKnown, c.Function.EventFunctionExact
	}
	if len(c.FunctionHeader) == 0 {
		return false, false
	}
	name := functionHeaderName(c.FunctionHeader, c.AllowFullwidthSpace)
	return EventFunctionContext(name)
}

func (c *StructuralContext) systemFunctionHeader() bool {
	if c.Kind != ERB || c.Function == nil {
		return false
	}
	if c.Function.FunctionName != "" {
		return c.Function.SystemFunctionKnown
	}
	if c.InContinuationProxy || len(c.FunctionHeader) == 0 {
		return false
	}
	name := functionHeaderName(c.FunctionHeader, c.AllowFullwidthSpace)
	if name == "" || name != strings.ToUpper(name) {
		return false
	}
	return systemFunctionName(name)
}
