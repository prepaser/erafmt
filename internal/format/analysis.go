package format

import (
	"github.com/prepaser/erafmt/internal/erabasic"
)

func (p *formatter) semanticContext() *erabasic.SemanticContext {
	context := &erabasic.SemanticContext{
		Src:                 p.Src,
		Dialect:             p.Dialect,
		AllowFullwidthSpace: p.AllowFullwidthSpace,
		ParseExpressions:    p.expressionParseEnabled(),
		Session:             p.SyntaxSession,
		Base:                p.SyntaxBase,
	}
	context.OnResource = func(offset int, message string) {
		p.syntaxResource(offset, message)
	}
	context.OnAbsoluteResource = func(offset int, message string) {
		p.resourceProblem(p.ParseLine, offset, message)
	}
	return context
}

func (p *formatter) genericCommandRoleValid(command string, start, end int) (int, string, bool) {
	return p.semanticContext().GenericCommandRoleValid(command, start, end)
}

func commandFieldVariableArgument(p *formatter, left, right int) bool {
	return p.semanticContext().CommandFieldVariableArgument(left, right)
}

func (p *formatter) commandAnalysisContext() *erabasic.SemanticContext {
	context := p.semanticContext()
	context.OnResource = func(offset int, message string) {
		p.syntaxResource(offset, message)
	}
	return context
}

func (p *formatter) scanContext() *erabasic.ScanContext {
	return &erabasic.ScanContext{Session: &p.DocumentSession}
}

func (p *formatter) declarationWordAnalysis(head []byte) erabasic.DeclarationWordAnalysis {
	return erabasic.AnalyzeDeclarationWord(head, p.AllowFullwidthSpace)
}

func (p *formatter) declarationModifierAnalysis(start, end int) erabasic.DeclarationModifierAnalysis {
	return erabasic.AnalyzeDeclarationModifiers(p.Src, start, end, p.AllowFullwidthSpace, erabasic.DeclarationKind(p.Kind))
}

func (p *formatter) labelAnalysis(head []byte, marker byte, start int) erabasic.LabelAnalysis {
	return erabasic.AnalyzeLabel(head, marker, start, p.newExpressionSession(head))
}

func (p *formatter) declarationInitializerSuspicious(line int, stringType bool, modifiers erabasic.DeclarationModifierSet, fieldsStart, fieldsEnd, initStart, initEnd int) bool {
	result := p.semanticContext().DeclarationInitializerAnalysis(stringType, modifiers, fieldsStart, fieldsEnd, initStart, initEnd)
	if result.Issue == nil {
		return false
	}
	p.warning(line, result.Issue.Offset, result.Issue.Code, result.Issue.Message)
	return true
}

func (p *formatter) preprocessorContext() *erabasic.PreprocessorContext {
	semantic := p.semanticContext()
	semantic.OnResource = func(offset int, message string) {
		p.resourceProblem(p.ParseLine, p.ParseBase+offset, message)
	}
	context := &erabasic.PreprocessorContext{
		Semantic:            semantic,
		Src:                 p.Src,
		Lines:               p.Lines,
		Kind:                p.Kind,
		Dialect:             p.Dialect,
		AllowFullwidthSpace: p.AllowFullwidthSpace,
		Base:                p.Base,
		ResourceLine:        p.ParseLine,
		ResourceBase:        p.ParseBase,
		Optional:            p.Optional,
		InContinuationProxy: p.InContinuationProxy,
		State:               &p.State,
		Pp:                  &p.Pp,
		Function:            &p.FunctionContext,
		Opaque:              p.Opaque,
		ConvergenceWork:     &p.ConvergenceWork,
		PpStateUnits:        &p.PpStateUnits,
		PreserveFile:        &p.PreserveFile,
		ProtectLine:         p.syntaxOpaque,
		CommentLine:         p.isOrdinaryComment,
	}
	context.Resource = func(line, offset int, message string) {
		p.resourceProblem(line, offset, message)
	}
	context.StatementResource = func(line, offset int, message string) {
		p.statementResourceProblem(line, offset, message)
	}
	context.Issue = func(issue erabasic.PreprocessorIssue) {
		if issue.Preserve {
			p.PreserveFile = true
		}
		severity := erabasic.Unsupported
		if issue.Severity == erabasic.PreprocessorWarning {
			severity = erabasic.Warning
		}
		p.problem(issue.Line, issue.Offset, issue.Code, issue.Message, severity)
	}
	return context
}

func (p *formatter) analyzePreprocessorDirective(i int, head []byte) {
	p.preprocessorContext().Directive(i, head)
}

func (p *formatter) structuralContext() *erabasic.StructuralContext {
	ctx := &erabasic.StructuralContext{
		Source:              p.Src,
		Lines:               p.Lines,
		Kind:                p.Kind,
		Dialect:             p.Dialect,
		AllowFullwidthSpace: p.AllowFullwidthSpace,
		ExpressionEnabled:   p.expressionParseEnabled(),
		Function:            &p.FunctionContext,
		Base:                p.Base,
		Conditional:         p.conditional(),
		InContinuationProxy: p.InContinuationProxy,
		PendingDeclaration:  p.PendingDeclaration,
		PendingLabel:        p.PendingLabel,
		LexRange:            p.lexSyntaxRange,
		NewSession:          p.newExpressionSession,
	}
	if p.FunctionStart >= 0 && p.FunctionStart < len(p.Lines) {
		head, _, _, _ := p.headOf(erabasic.LBody(p.Src, p.Lines[p.FunctionStart]))
		ctx.FunctionHeader = head
	}
	ctx.Issue = func(issue erabasic.StructuralIssue) {
		if issue.Severity == erabasic.StructuralUnsupported {
			p.problem(issue.Line, issue.Offset, issue.Code, issue.Message, erabasic.Unsupported)
		} else {
			p.warning(issue.Line, issue.Offset, issue.Code, issue.Message)
		}
		if issue.Opaque {
			p.syntaxOpaque(issue.Line)
		}
	}
	return ctx
}

func (p *formatter) syncStructuralContext(ctx *erabasic.StructuralContext) {
	p.PendingDeclaration = ctx.PendingDeclaration
	p.PendingLabel = ctx.PendingLabel
}

func (p *formatter) resetFunctionAttributes() {
	ctx := p.structuralContext()
	ctx.ResetFunctionAttributes()
	p.syncStructuralContext(ctx)
}

func (p *formatter) auditSharpDeclaration(i int, head []byte, base int, optional, marked, continued bool) {
	ctx := p.structuralContext()
	ctx.AuditSharpDeclaration(i, head, base, optional, marked, continued)
	p.syncStructuralContext(ctx)
}

func (p *formatter) registerGlobalDeclaration(i, offset int, tokens []erabasic.ExprToken) bool {
	ctx := p.structuralContext()
	result := ctx.RegisterGlobalDeclaration(i, offset, tokens)
	p.syncStructuralContext(ctx)
	return result
}

func (p *formatter) registerPrivateDeclaration(i, offset int, name string) bool {
	ctx := p.structuralContext()
	result := ctx.RegisterPrivateDeclaration(i, offset, name)
	p.syncStructuralContext(ctx)
	return result
}

func (p *formatter) trustedReturnExpressionType(tokens []erabasic.ExprToken) erabasic.KnownValueType {
	return p.structuralContext().TrustedReturnExpressionType(tokens)
}

func (p *formatter) returnCommandMessage(command string, value erabasic.KnownValueType) string {
	return p.structuralContext().ReturnCommandMessage(command, value)
}

func (p *formatter) auditDollarLabel(i int, head []byte, base int, uncertain bool) {
	ctx := p.structuralContext()
	ctx.AuditDollarLabel(i, head, base, uncertain)
	p.syncStructuralContext(ctx)
}

func (p *formatter) auditRepeatCount(i int, head []byte, wordEnd, base int) {
	p.structuralContext().AuditRepeatCount(i, head, wordEnd, base)
}

func (p *formatter) provenRepeatPositive(head []byte, wordEnd, base int) bool {
	return p.structuralContext().ProvenRepeatPositive(head, wordEnd, base)
}

func (p *formatter) provenSelectorType(head []byte, wordEnd, base int) erabasic.KnownValueType {
	return p.structuralContext().ProvenSelectorType(head, wordEnd, base)
}

func (p *formatter) caseTypeMismatch(before erabasic.State, head []byte, wordEnd, base int) bool {
	return p.structuralContext().CaseTypeMismatch(before, head, wordEnd, base)
}

func (p *formatter) statementAnalysis() *erabasic.StatementAnalysis {
	return &erabasic.StatementAnalysis{
		Source:              p.Src,
		Dialect:             p.Dialect,
		AllowFullwidthSpace: p.AllowFullwidthSpace,
		ExpressionEnabled:   p.expressionParseEnabled(),
		Base:                p.Base,
		Optional:            p.Optional,
		Conditional:         p.conditional(),
		Preserved:           p.Preserved,
		State:               &p.State,
		Semantic:            p.semanticContext(),
	}
}

func (p *formatter) returnHasCommaAssignment(start, end int) bool {
	return p.statementAnalysis().ReturnHasCommaAssignment(start, end)
}

func (p *formatter) assignmentLHS(src []byte) bool {
	return p.statementAnalysis().AssignmentLHS(src)
}

func (p *formatter) variableLHS(src []byte) (string, bool) {
	return p.semanticContext().VariableLHS(p.trimInputIn(src))
}

func (p *formatter) printVProjection(i, start, end int) (erabasic.PrintVProjection, int, bool, bool) {
	analysis := erabasic.PrintVAnalysis{Dialect: p.Dialect, AllowFullwidthSpace: p.AllowFullwidthSpace, ReserveMetadata: func(offset, size int) bool {
		return p.reserveMetadata(i, start+offset, size)
	}, Resource: func(offset int, message string) {
		p.resourceProblem(i, start+offset, message)
	}}
	result := analysis.Project(p.Src[start:end])
	if result.Problem != nil {
		p.syntaxProblem(i, start+result.Problem.Offset, result.Problem.Message)
	}
	return result.Projection, result.CodeEnd, result.RawEnd, result.Ok
}
