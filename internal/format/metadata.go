package format

import (
	"github.com/prepaser/erafmt/internal/erabasic"
)

const editMetadataBytes = 128

func (p *formatter) reserveMetadata(i, offset, size int) bool {
	if p.Aborted {
		return false
	}
	if p.Metadata == nil {
		p.Metadata = erabasic.NewMetadataBudget(0)
	}
	if !p.Metadata.Reserve(size) {
		p.resourceProblem(i, offset, "formatter metadata exceeds the formatter resource limit")
		return false
	}
	return true
}

func (p *formatter) appendEdit(e edit) bool {
	if !p.reserveMetadata(e.change.Line-1, e.start, editMetadataBytes) {
		return false
	}
	p.appendReservedEdit(e)
	return true
}

func (p *formatter) statementOrigin(i, offset int) (int, int) {
	return p.DocumentSession.StatementSourceOrigin(i, offset)
}

func (p *formatter) diagnostic(i, offset int, severity erabasic.Severity, code, message string) {
	i, offset = p.statementOrigin(i, offset)
	if !p.reserveMetadata(i, offset, 128+len(code)+len(message)) {
		return
	}
	col := p.columnAt(i, offset)
	if !p.Aborted {
		p.diagnostics = append(p.diagnostics, Diagnostic{Line: i + 1, Column: col, Severity: Severity(severity), Code: code, Message: message})
	}
}

func (p *formatter) syntaxResource(offset int, message string) {
	if len(p.ParseSegments) != 0 {
		segment, local := erabasic.ContinuationDiagnosticSegment(p.ParseBase+offset, p.ParseSegments)
		if segment >= 0 {
			s := p.ParseSegments[segment]
			p.resourceProblem(s.Line, min(s.SourceEnd, max(s.SourceStart, s.SourceStart+local)), message)
			return
		}
	}
	p.resourceProblem(p.ParseLine, p.ParseBase+offset, message)
}
