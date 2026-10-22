package erabasic

import (
	"bytes"
	"unicode"
)

type scanIssueSeverity uint8

const (
	ScanIssueWarning scanIssueSeverity = iota
	scanIssueError
	ScanIssueUnsupported
	ScanIssueResource
)

type ScanStatement struct {
	First, Last, Prefix int
	Head                []byte
	Optional, Marked    bool
	Continuation        *ContinuationInput
}

type ScanIssue struct {
	Line     int
	Offset   int
	Code     string
	Message  string
	Severity scanIssueSeverity
}

type ScanContinuationFacts struct{ Certain bool }
type functionScanAnalysis struct {
	Issue     *ScanIssue
	validated bool
}

type declarationScanAnalysis struct{ Issue *ScanIssue }
type labelScanAnalysis struct{ Issue *ScanIssue }
type commandScanAnalysis struct {
	Command          string
	WordEnd          int
	Early            bool
	Before           State
	LogicalTrusted   bool
	MetadataTrusted  bool
	EmptyData        bool
	CaseMismatch     bool
	Depth            int
	RepeatAudit      bool
	RepeatNeedsProof bool
}

type ScanContext struct {
	Session    *DocumentSession
	Structural *StructuralContext
	Issue      func(ScanIssue)
}

func (c *ScanContext) report(issue ScanIssue) {
	if c.Issue != nil {
		c.Issue(issue)
	}
	if issue.Severity == ScanIssueResource {
		c.Session.Aborted = true
	}
}

func (c *ScanContext) conditional() bool {
	for _, frame := range c.Session.Pp {
		if frame.Kind == "IF" {
			return true
		}
	}
	return false
}

func (c *ScanContext) CollectContinuation(first int) (*ContinuationInput, bool) {
	last := first
	var joined bytes.Buffer
	closed := false
	for last = first + 1; last < len(c.Session.Lines); last++ {
		if c.Session.Aborted {
			return nil, false
		}
		inner := c.Session.Src[c.Session.Lines[last].Start:c.Session.Lines[last].BodyEnd]
		trimmed := bytes.TrimLeftFunc(inner, unicode.IsSpace)
		if bytes.Equal(bytes.TrimSpace(inner), []byte("}")) {
			closed = true
			break
		}
		if continuationContentLimitExceeded(last - first - 1) {
			c.report(ScanIssue{Line: first, Offset: c.Session.Lines[first].IndentEnd, Code: "resource_limit", Message: "continuation source exceeds the formatter resource limit", Severity: ScanIssueResource})
			return nil, false
		}
		if !appendWithinLimit(joined.Len(), len(inner), MaxSourceBufferBytes) {
			c.report(ScanIssue{Line: first, Offset: c.Session.Lines[first].IndentEnd, Code: "resource_limit", Message: "continuation source exceeds the formatter resource limit", Severity: ScanIssueResource})
			return nil, false
		}
		joined.Write(inner)
		c.Session.ParseSegments = append(c.Session.ParseSegments, ContinuationSegment{
			LogicalStart: joined.Len() - len(inner),
			LogicalEnd:   joined.Len(),
			Line:         last,
			SourceStart:  c.Session.Lines[last].Start,
			SourceEnd:    c.Session.Lines[last].BodyEnd,
			Indent:       c.Session.Lines[last].IndentEnd - c.Session.Lines[last].Start,
		})
		if len(trimmed) != 0 && trimmed[0] == '}' {
			c.report(ScanIssue{Line: last, Offset: c.Session.Lines[last].IndentEnd, Code: "invalid_structure", Message: "characters after continuation end", Severity: scanIssueError})
		}
		if bytes.Equal(trimmed, []byte("{")) {
			c.report(ScanIssue{Line: last, Offset: c.Session.Lines[last].IndentEnd, Code: "invalid_structure", Message: "nested continuation", Severity: scanIssueError})
		}
		if c.Session.ContinuationSeparator == nil {
			if !appendWithinLimit(joined.Len(), 1, MaxSourceBufferBytes) {
				c.report(ScanIssue{Line: first, Offset: c.Session.Lines[first].IndentEnd, Code: "resource_limit", Message: "continuation source exceeds the formatter resource limit", Severity: ScanIssueResource})
				return nil, false
			}
			joined.WriteByte(' ')
		} else {
			if !appendWithinLimit(joined.Len(), len(*c.Session.ContinuationSeparator), MaxSourceBufferBytes) {
				c.report(ScanIssue{Line: first, Offset: c.Session.Lines[first].IndentEnd, Code: "resource_limit", Message: "continuation source exceeds the formatter resource limit", Severity: ScanIssueResource})
				return nil, false
			}
			joined.WriteString(*c.Session.ContinuationSeparator)
		}
	}
	if !closed {
		c.report(ScanIssue{Line: first, Offset: c.Session.Lines[first].IndentEnd, Code: "invalid_structure", Message: "unterminated continuation", Severity: ScanIssueUnsupported})
		return nil, false
	}
	return &ContinuationInput{First: first, Last: last, Logical: joined.Bytes(), Segments: c.Session.ParseSegments}, true
}

func (c *ScanContext) FunctionBoundary(first, offset int, head []byte, session *ExpressionSession) functionScanAnalysis {
	if c.conditional() || c.Session.Optional || bytes.Contains(head, []byte("[[")) {
		return functionScanAnalysis{Issue: &ScanIssue{Line: first, Offset: offset, Code: "unsupported_structure", Message: "function boundary depends on preprocessing or aliases", Severity: ScanIssueUnsupported}}
	}
	validation := validateHeaderWithSession(head, session)
	if validation.Status != headerValid {
		severity := scanIssueError
		if validation.Status == headerUnsupported {
			severity = ScanIssueUnsupported
		}
		return functionScanAnalysis{Issue: &ScanIssue{Line: first, Offset: offset, Code: "invalid_header", Message: validation.Message, Severity: severity}}
	}
	return functionScanAnalysis{validated: true}
}

func (c *ScanContext) ResetFunction(first int, head []byte) {
	c.Session.FunctionStart = first
	c.Session.State = State{Declarations: true}
	c.Structural.ResetFunctionAttributes()
	c.Session.FunctionName = functionHeaderName(head, c.Session.AllowFullwidthSpace)
	c.Session.EventFunctionKnown, c.Session.EventFunctionExact = EventFunctionContext(c.Session.FunctionName)
	c.Session.SystemFunctionKnown = systemFunctionName(c.Session.FunctionName)
	c.Session.DollarLabels = nil
	c.Session.LabelsUnknown = false
	c.Session.Base = 1
}

func (c *ScanContext) Declaration(first, offset int, head []byte, optional, marked, continued bool) declarationScanAnalysis {
	if c.conditional() || optional || marked || BytesHasAlias(head) || continued && !ContinuationDeclarationSafe(c.Session.ContinuationSeparator != nil, optional, c.conditional(), head) {
		c.Session.SharpStateUnknown = true
	}
	if c.Session.Kind == ERH && !bytes.Contains(head, []byte("[[")) && erhUnsupportedDeclarationLine(head, c.Session.AllowFullwidthSpace) {
		return declarationScanAnalysis{Issue: &ScanIssue{Line: first, Offset: offset, Code: "unsupported_declaration", Message: "ERH declaration is unsupported: " + declarationName(head), Severity: ScanIssueUnsupported}}
	}
	if c.Session.Kind == ERB && (c.Session.Base == 0 || !c.Session.State.Declarations) {
		return declarationScanAnalysis{Issue: &ScanIssue{Line: first, Offset: offset, Code: "unsupported_declaration", Message: "ERB declarations must follow a function header before its body", Severity: ScanIssueUnsupported}}
	}
	return declarationScanAnalysis{}
}

func (c *ScanContext) Label(first int) labelScanAnalysis {
	c.Session.State.Declarations = false
	if c.Session.State.Sif != 0 {
		return labelScanAnalysis{Issue: &ScanIssue{Line: first, Offset: c.Session.Lines[first].IndentEnd, Code: "invalid_structure", Message: "SIF cannot target a label", Severity: scanIssueError}}
	}
	if len(c.Session.State.Frames) != 0 {
		top := c.Session.State.Frames[len(c.Session.State.Frames)-1].Kind
		if top == "DATA" || top == "DATALIST" || top == "FUNCLIST" {
			return labelScanAnalysis{Issue: &ScanIssue{Line: first, Offset: c.Session.Lines[first].IndentEnd, Code: "invalid_structure", Message: "label is not allowed in " + top, Severity: scanIssueError}}
		}
	}
	return labelScanAnalysis{}
}

func (c *ScanContext) CommandBegin(first, offset int, head []byte, optional, marked, continued bool, facts ScanContinuationFacts) commandScanAnalysis {
	command, wordEnd := CommandOfWithSpace(head, c.Session.AllowFullwidthSpace)
	result := commandScanAnalysis{Command: command, WordEnd: wordEnd}
	if command == "" {
		if end := bytes.Index(head, []byte("　")); end > 0 {
			name, endWord := CommandOfWithSpace(head[:end], c.Session.AllowFullwidthSpace)
			if endWord == end && (ReservedInstruction(name) || ReservedBuiltinMethod(name)) {
				result.Early = true
				c.report(ScanIssue{Line: first, Offset: offset + end, Code: "unsupported_structure", Message: "instruction separator depends on runtime configuration", Severity: ScanIssueUnsupported})
				return result
			}
		}
		firstWord := head
		if end := bytes.IndexAny(firstWord, " \t"); end >= 0 {
			firstWord = firstWord[:end]
		}
		if bytes.Contains(firstWord, []byte("[[")) {
			result.Early = true
			c.report(ScanIssue{Line: first, Offset: offset, Code: "unsupported_structure", Message: "instruction identity depends on aliases", Severity: ScanIssueUnsupported})
			return result
		}
	}
	result.Before = c.Session.State
	if optional {
		result.Before = result.Before.Clone()
	}
	trusted := !c.Session.Preserved && !c.Session.PreserveFile && !optional && !c.conditional() && !marked && !continued && !bytes.Contains(head, []byte("[["))
	result.LogicalTrusted = trusted || facts.Certain && !c.conditional()
	result.MetadataTrusted = trusted || facts.Certain || !optional && !marked && !continued && !bytes.Contains(head, []byte("[[")) && c.conditional()
	if result.LogicalTrusted && result.Before.branchAfterFinal(command) {
		c.report(ScanIssue{Line: first, Offset: offset, Code: "branch_after_final", Message: command + " follows a final branch", Severity: ScanIssueWarning})
	}
	if result.LogicalTrusted && command == "REPEAT" && result.Before.hasTrustedRepeat() {
		c.report(ScanIssue{Line: first, Offset: offset, Code: "nested_repeat", Message: "nested REPEAT may create an infinite loop", Severity: ScanIssueWarning})
	}
	if result.LogicalTrusted && command == "REPEAT" {
		result.RepeatAudit = true
	}
	result.EmptyData = result.Before.emptyData(command)
	result.RepeatNeedsProof = command == "REPEAT" && !result.LogicalTrusted && c.conditional() && facts.Certain
	return result
}

func (c *ScanContext) CommandFacts(result *commandScanAnalysis, head []byte, optional, marked, continued bool, offset int) {
	result.CaseMismatch = !c.Session.Preserved && !c.Session.PreserveFile && !optional && !c.conditional() && !marked && !continued && !bytes.Contains(head, []byte("[[")) && result.Command == "CASE" && c.Structural.CaseTypeMismatch(result.Before, head, result.WordEnd, offset)
}

func (c *ScanContext) CommandTransition(result *commandScanAnalysis, first int, optional, marked, continued bool, head []byte, offset int) {
	if result.Early {
		return
	}
	if result.Command == "ENDLIST" {
		c.Session.State.finishData(result.Command, result.MetadataTrusted)
	}
	depth, message := c.Session.State.Instruction(result.Command, first+1)
	if message != "" {
		result.Early = true
		severity := scanIssueError
		if c.conditional() || optional {
			severity = ScanIssueUnsupported
		}
		c.report(ScanIssue{Line: first, Offset: c.Session.Lines[first].IndentEnd, Code: "invalid_structure", Message: message, Severity: severity})
		return
	}
	if optional && !result.Before.equal(c.Session.State) {
		result.Early = true
		c.report(ScanIssue{Line: first, Offset: offset, Code: "unsupported_structure", Message: "conditional marker changes block or SIF state", Severity: ScanIssueUnsupported})
		return
	}
	if result.Command == "SELECTCASE" && !optional && !marked && !continued && !bytes.Contains(head, []byte("[[")) {
		c.Session.State.SetSelectorType(c.Structural.ProvenSelectorType(head, result.WordEnd, offset))
	}
	result.Depth = depth
}

func (c *ScanContext) CommandFinishBeforeRepeat(result *commandScanAnalysis, first, offset int) {
	if result.Early {
		return
	}
	c.Session.State.markDataOpener(result.Command, result.MetadataTrusted)
	c.Session.State.markData(result.Command, result.MetadataTrusted)
	if result.EmptyData && result.LogicalTrusted {
		c.report(ScanIssue{Line: first, Offset: offset, Code: "empty_data", Message: result.Command + " closes an empty data block", Severity: ScanIssueWarning})
	}
}

func (c *ScanContext) CommandFinishAfterRepeat(result *commandScanAnalysis, first, offset int, head []byte, optional, marked, continued bool, facts ScanContinuationFacts, repeatPositive bool) {
	if result.Early {
		return
	}
	if result.Command == "REPEAT" && len(c.Session.State.Frames) > 0 {
		trusted := result.LogicalTrusted
		if !trusted && c.conditional() && !optional && !marked && !continued && !bytes.Contains(head, []byte("[[")) {
			trusted = c.Structural.ProvenRepeatPositive(head, result.WordEnd, offset)
		}
		if !trusted && c.conditional() && facts.Certain {
			trusted = repeatPositive
		}
		c.Session.State.Frames[len(c.Session.State.Frames)-1].RepeatTrusted = trusted
	}
	if (result.Command == "BREAK" || result.Command == "CONTINUE") && !optional && !c.conditional() && !c.Session.State.hasLoop() {
		c.report(ScanIssue{Line: first, Offset: offset, Code: "invalid_loop_control", Message: result.Command + " is outside a loop", Severity: ScanIssueWarning})
	}
	c.Session.State.Declarations = false
}
