package format

import (
	"bytes"

	"github.com/prepaser/erafmt/internal/erabasic"
)

func (p *formatter) emitScanIssue(issue erabasic.ScanIssue) {
	switch issue.Severity {
	case erabasic.ScanIssueWarning:
		p.warning(issue.Line, issue.Offset, issue.Code, issue.Message)
	case erabasic.ScanIssueResource:
		p.resourceProblem(issue.Line, issue.Offset, issue.Message)
	case erabasic.ScanIssueUnsupported:
		p.problem(issue.Line, issue.Offset, issue.Code, issue.Message, erabasic.Unsupported)
	default:
		if issue.Code == "invalid_structure" {
			p.structuralProblem(issue.Line, issue.Message)
		} else {
			p.problem(issue.Line, issue.Offset, issue.Code, issue.Message, erabasic.Error)
		}
	}
}

func (p *formatter) emitCommandScanIssue(issue erabasic.ScanIssue) {
	if issue.Severity == erabasic.ScanIssueResource {
		p.statementResourceProblem(issue.Line, issue.Offset, issue.Message)
	} else {
		p.emitScanIssue(issue)
	}
}

func (p *formatter) collectContinuation(first int) (*erabasic.ContinuationInput, bool) {
	ctx := p.scanContext()
	ctx.Issue = func(issue erabasic.ScanIssue) {
		if issue.Code == "invalid_structure" {
			if issue.Message == "unterminated continuation" {
				p.PreserveFile = true
			}
			p.structuralProblem(issue.Line, issue.Message)
		} else {
			p.emitScanIssue(issue)
		}
	}
	input, ok := ctx.CollectContinuation(first)
	return input, ok
}

func (p *formatter) scanFunction(statement erabasic.ScanStatement) bool {
	first, last, prefix := statement.First, statement.Last, statement.Prefix
	head, marked := statement.Head, statement.Marked
	input := statement.Continuation
	continued := input != nil
	l := p.Lines[first]
	ctx := p.scanContext()
	ctx.Structural = p.structuralContext()
	p.SyntaxSession = p.newExpressionSession(head)
	p.SyntaxBase = l.Start + prefix
	analysis := ctx.FunctionBoundary(first, l.Start+prefix, head, p.SyntaxSession)
	if p.Aborted {
		return false
	}
	if analysis.Issue != nil {
		p.PreserveFile = true
		p.emitScanIssue(*analysis.Issue)
		return false
	}
	p.finishFunction(first)
	p.protectFunction(first)
	p.functionEdit = len(p.edits)
	p.Preserved = false
	ctx.ResetFunction(first, head)
	if p.Kind == erabasic.ERH {
		p.structuralProblem(first, "function definition in ERH")
		return true
	}
	p.indentSpan(first, last, 0, FunctionIndent)
	if !continued && !marked {
		p.Headers = append(p.Headers, first)
		if !p.headerArgumentsValid(first, head, l.Start+prefix) {
			p.truncateEdits(p.functionEdit)
			p.syntaxOpaque(first)
			return true
		}
		if p.rules[LabelMarkerSpacing] && len(head) > 1 && p.inputSpaceIn(head, 1) {
			p.syntaxLabel(first, head, l.Start+prefix)
		}
		p.syntax(first, head, prefix)
	} else if continued && !marked {
		if !p.continuationHeaderArgumentsValid(input) {
			p.truncateEdits(p.functionEdit)
			for row := first; row <= last; row++ {
				p.syntaxOpaque(row)
			}
			return true
		}
		if p.ContinuationSeparator != nil && !p.Optional && !p.conditional() && !p.Preserved && !p.PreserveFile && !erabasic.BytesHasAlias(head) && !erabasic.ContinuationUnsafe(head) {
			p.Headers = append(p.Headers, first)
		}
		p.continuationSyntax(input)
	}
	return true
}

func (p *formatter) scanDeclaration(statement erabasic.ScanStatement) {
	first, last, prefix := statement.First, statement.Last, statement.Prefix
	head, optional, marked := statement.Head, statement.Optional, statement.Marked
	input := statement.Continuation
	continued := input != nil
	l := p.Lines[first]
	ctx := p.scanContext()
	analysis := ctx.Declaration(first, l.Start+prefix, head, optional, marked, continued)
	if analysis.Issue != nil {
		if analysis.Issue.Severity == erabasic.ScanIssueUnsupported {
			p.PreserveFile = p.Kind == erabasic.ERH
		}
		p.emitScanIssue(*analysis.Issue)
		return
	}
	p.indentSpan(first, last, p.Base, DeclarationIndent)
	if !continued && !marked {
		p.auditSharpDeclaration(first, head, l.Start+prefix, optional, marked, continued)
		p.syntax(first, head, prefix)
	} else if continued && !marked {
		p.continuationSyntax(input)
	}
}

func (p *formatter) scanLabel(statement erabasic.ScanStatement) {
	first, last, prefix := statement.First, statement.Last, statement.Prefix
	head, optional, marked := statement.Head, statement.Optional, statement.Marked
	input := statement.Continuation
	continued := input != nil
	l := p.Lines[first]
	ctx := p.scanContext()
	analysis := ctx.Label(first)
	if analysis.Issue != nil {
		p.emitScanIssue(*analysis.Issue)
		return
	}
	p.indentSpan(first, last, p.Base+p.State.Depth(), LabelIndent)
	if !continued {
		p.auditDollarLabel(first, head, l.Start+prefix, optional || marked)
	} else if p.ContinuationSeparator == nil || optional || marked || p.conditional() || p.Preserved || p.PreserveFile || erabasic.ContinuationUnsafe(head) {
		p.LabelsUnknown = true
	}
	if !continued && !marked {
		p.syntaxLabel(first, head, l.Start+prefix)
	} else if continued && !marked {
		p.continuationSyntax(input)
	}
}

func (p *formatter) scanCommand(statement erabasic.ScanStatement, emitIssue func(erabasic.ScanIssue)) (int, bool) {
	first, last, prefix := statement.First, statement.Last, statement.Prefix
	head, optional, marked := statement.Head, statement.Optional, statement.Marked
	input := statement.Continuation
	continued := input != nil
	l := p.Lines[first]
	command, wordEnd := erabasic.CommandOfWithSpace(head, p.AllowFullwidthSpace)
	if continued && p.ContinuationSeparator == nil && p.continuationNilCommandIdentityUnknown(input, command) {
		p.problem(first, l.Start+prefix, "unsupported_structure", "continued command identity depends on the separator", erabasic.Unsupported)
		return last, true
	}
	if continued && !p.recordContinuationOrigin(input) {
		return last, false
	}
	if (erabasic.ReservedInstruction(command) || erabasic.ReservedBuiltinMethod(command)) && !erabasic.CommandUnavailable(command, p.Dialect) && string(head[:wordEnd]) != command {
		if start, _, ok := p.assignmentBoundary(head); ok {
			if _, variable := p.variableLHS(head[:start]); variable {
				p.problem(first, l.Start+prefix, "unsupported_structure", "instruction identity depends on case-sensitivity configuration", erabasic.Unsupported)
				return last, true
			}
		}
	}
	if command == "" {
		if end := bytes.Index(head, []byte("　")); end > 0 {
			name, endWord := erabasic.CommandOfWithSpace(head[:end], p.AllowFullwidthSpace)
			if endWord == end && (erabasic.ReservedInstruction(name) || erabasic.ReservedBuiltinMethod(name)) {
				row, offset := first, l.Start+prefix+end
				if continued {
					if position, ok := input.Position(len(input.Logical) - len(head) + end); ok {
						row, offset = position.Line, position.Offset
					}
				}
				p.problem(row, offset, "unsupported_structure", "instruction separator depends on runtime configuration", erabasic.Unsupported)
				return last, true
			}
		}
	}
	if optional && !continued && p.optionalSIFPair(first, head, prefix) {
		return last + 1, true
	}
	if p.DocumentSession.SourceStructureLimitExceeded(command) {
		p.statementResourceProblem(first, l.Start+prefix, "structural nesting exceeds the formatter resource limit")
		return last, false
	}
	proof := continuationProof{}
	facts := erabasic.ScanContinuationFacts{}
	if continued {
		proof = p.proveContinuation(input)
		facts = erabasic.ScanContinuationFacts{Certain: proof.certain}
	}
	ctx := p.scanContext()
	switch command {
	case "CASE", "SELECTCASE", "REPEAT":
		ctx.Structural = p.structuralContext()
	}
	ctx.Issue = emitIssue
	if continued {
		continuation := proof
		ctx.Issue = func(issue erabasic.ScanIssue) {
			if issue.Severity == erabasic.ScanIssueWarning {
				p.continuationWarning(continuation, issue.Code, issue.Message)
			} else {
				emitIssue(issue)
			}
		}
	}
	analysis := ctx.CommandBegin(first, l.Start+prefix, head, command, wordEnd, optional, marked, continued, facts)
	if analysis.Early {
		return last, true
	}
	if analysis.RepeatAudit {
		if continued {
			p.auditRepeatCountContinuation(proof)
		} else {
			p.auditRepeatCount(first, head, wordEnd, l.Start+prefix)
		}
	}
	ctx.CommandFacts(&analysis, head, optional, marked, continued, l.Start+prefix)
	ctx.CommandTransition(&analysis, first, optional, marked, continued, head, l.Start+prefix)
	if analysis.Early {
		return last, true
	}
	if analysis.CaseMismatch {
		p.syntaxProblem(first, l.Start+prefix+wordEnd, "CASE has an incompatible type")
		p.syntaxOpaque(first)
	}
	ctx.CommandFinishBeforeRepeat(&analysis, first, l.Start+prefix)
	repeatPositive := false
	if analysis.RepeatNeedsProof {
		repeatPositive = p.provenRepeatPositiveContinuation(proof)
	}
	ctx.CommandFinishAfterRepeat(&analysis, first, l.Start+prefix, head, optional, marked, continued, facts, repeatPositive)
	rule := InstructionIndent
	if analysis.Before.Sif != 0 {
		rule = SIFIndent
	}
	p.indentSpan(first, last, p.Base+analysis.Depth, rule)
	if !continued && !marked {
		diagnosticStart := len(p.diagnostics)
		p.syntax(first, head, prefix)
		if p.Opaque[first] && command != "DATA" {
			if command == "DATAFORM" || erabasic.OpenerKind(command) == "DATA" {
				p.State.MarkCurrentDataUncertain()
			}
		}
		if len(p.diagnostics) == diagnosticStart && !bytes.Contains(head, []byte("[[")) && p.knownPayloadBody(command) {
			p.BodyStarts[first] = true
		}
	} else if continued && !marked {
		p.continuationSyntaxWithProof(first, last, proof)
	}
	return last, true
}
