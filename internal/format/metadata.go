package format

const (
	maxMetadataBytes  = 32 << 20
	lineMetadataBytes = 128
	editMetadataBytes = 128
)

type metadataBudget struct{ used int }

func (p *formatter) reserveMetadata(i, offset, size int) bool {
	if p.aborted {
		return false
	}
	if p.metadata == nil {
		p.metadata = &metadataBudget{}
	}
	if !appendWithinLimit(p.metadata.used, size, maxMetadataBytes) {
		p.resourceProblem(i, offset, "formatter metadata exceeds the formatter resource limit")
		return false
	}
	p.metadata.used += size
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
	if origin, ok := p.statementOrigins[i]; ok {
		return origin.line, origin.offset
	}
	return i, offset
}

func (p *formatter) diagnostic(i, offset int, severity Severity, code, message string) {
	i, offset = p.statementOrigin(i, offset)
	if !p.reserveMetadata(i, offset, 128+len(code)+len(message)) {
		return
	}
	col := p.columnAt(i, offset)
	if !p.aborted {
		p.diagnostics = append(p.diagnostics, Diagnostic{Line: i + 1, Column: col, Severity: severity, Code: code, Message: message})
	}
}

func sourceLineCount(src []byte, limit int) (int, bool) {
	count := 0
	for pos := 0; pos < len(src); {
		if count == limit {
			return count, false
		}
		count++
		for pos < len(src) && src[pos] != '\r' && src[pos] != '\n' {
			pos++
		}
		if pos < len(src) {
			cr := src[pos] == '\r'
			pos++
			if cr && pos < len(src) && src[pos] == '\n' {
				pos++
			}
		}
	}
	return count, true
}
