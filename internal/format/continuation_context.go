package format

import (
	"github.com/prepaser/erafmt/internal/erabasic"
)

func (p *formatter) continuationSemanticContext() *erabasic.SemanticContext {
	return &erabasic.SemanticContext{Src: p.Src, Dialect: p.Dialect, AllowFullwidthSpace: p.AllowFullwidthSpace, ParseExpressions: p.expressionParseEnabled(), Session: p.SyntaxSession, Base: p.SyntaxBase, OnResource: p.syntaxResource}
}

func (p *formatter) continuationNilPhysicalHead(input *erabasic.ContinuationInput) (head []byte, optional, marked bool) {
	return erabasic.ContinuationPhysicalHead(p.Src, p.Dialect, p.AllowFullwidthSpace, input)
}

func (p *formatter) continuationNilCommandIdentityUnknown(input *erabasic.ContinuationInput, command string) bool {
	return p.continuationSemanticContext().ContinuationNilCommandIdentityUnknown(input, command)
}

func (p *formatter) continuationNilHeaderEquivalent(input *erabasic.ContinuationInput) bool {
	result := p.continuationSemanticContext().ContinuationNilHeaderEquivalent(input)
	return !p.Aborted && result
}

func (p *formatter) continuationNilPreprocessorEquivalent(input *erabasic.ContinuationInput) bool {
	return p.continuationSemanticContext().ContinuationNilPreprocessorEquivalent(input)
}

func (p *formatter) continuationNilDispatchUnknown(input *erabasic.ContinuationInput) (unknown, preserveFile bool) {
	return p.continuationSemanticContext().ContinuationNilDispatchUnknown(input)
}

func (p *formatter) continuationDeclarationSafe(head []byte) bool {
	return erabasic.ContinuationDeclarationSafe(p.ContinuationSeparator != nil, p.Optional, p.conditional(), head)
}

func continuationLabelStateCommittedFor(p *formatter, src []byte) bool {
	return erabasic.ContinuationLabelStateCommitted(p.Dialect, p.AllowFullwidthSpace, p.Optional, p.conditional(), src) && len(p.Opaque) != 0 && !p.Opaque[0]
}

func continuationDeclarationCommittedFor(p *formatter, src []byte) bool {
	return erabasic.ContinuationDeclarationCommitted(p.Dialect, p.AllowFullwidthSpace, p.Optional, p.conditional(), p.SharpStateUnknown, len(p.Opaque) != 0 && p.Opaque[0], continuationWarningOnly(p), src)
}

func (p *formatter) continuationContext() erabasic.FunctionContext {
	ctx := p.FunctionContext
	ctx.SharpStateUnknown = ctx.SharpStateUnknown || p.Optional || p.conditional()
	return ctx
}

func (p *formatter) commitContinuationContext(proof continuationProof) {
	if proof.proxy == nil {
		return
	}
	candidate := erabasic.ContinuationContextCandidate{Head: proof.head, Function: proof.proxy.FunctionContext, PendingDeclaration: proof.proxy.PendingDeclaration, PendingLabel: proof.proxy.PendingLabel}
	labelAccepted := continuationLabelStateCommittedFor(proof.proxy, proof.logical)
	declarationAccepted := continuationDeclarationCommittedFor(proof.proxy, proof.logical)
	p.FunctionContext.CommitContinuationCandidate(candidate, labelAccepted, declarationAccepted, proof.mappingOK)
}
