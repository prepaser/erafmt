package format

import "bytes"

func (p *formatter) addEdit(i, start, end int, text []byte, rule RuleID) {
	if p.aborted {
		return
	}
	if p.rules[rule] && !bytes.Equal(text, p.src[start:end]) {
		if !p.reserveReplacementBytes(start, end, len(text)) {
			return
		}
		p.addEditUnchecked(i, start, end, text, rule)
	}
}

func (p *formatter) addEditUnchecked(i, start, end int, text []byte, rule RuleID) {
	if !p.reserveMetadata(i, start, editMetadataBytes) {
		return
	}
	col := p.columnAt(i, start)
	if p.aborted {
		return
	}
	p.appendReservedEdit(edit{start, end, text, Change{Rule: rule, Line: i + 1, Column: col}})
}

func (p *formatter) reserveReplacementBytes(start, end, replacement int) bool {
	if p.aborted {
		return false
	}
	if len(p.edits) != p.editBytesEdits {
		p.recalculateEditBytes()
	}
	if start < 0 || end < start || replacement < 0 || end > len(p.src) {
		return false
	}
	if p.editBytes > maxFormattedOutputBytes || uint64(replacement) > maxFormattedOutputBytes-p.editBytes {
		p.resourceProblem(0, 0, "formatting work exceeds the formatter resource limit")
		return false
	}
	p.editBytes += uint64(replacement)
	return true
}

func (p *formatter) recalculateEditBytes() {
	var total uint64
	for _, e := range p.edits {
		if uint64(len(e.text)) > maxFormattedOutputBytes-total {
			total = maxFormattedOutputBytes + 1
			break
		}
		total += uint64(len(e.text))
	}
	p.editBytes = total
	p.editBytesEdits = len(p.edits)
}

func (p *formatter) truncateEdits(mark int) {
	if p.editBytesEdits == len(p.edits) {
		for _, e := range p.edits[mark:] {
			p.editBytes -= uint64(len(e.text))
		}
		p.editBytesEdits = mark
	}
	clear(p.edits[mark:])
	p.edits = p.edits[:mark]
}

func (p *formatter) filterEdits(start int, discard func(edit) bool) {
	cached := p.editBytesEdits == len(p.edits)
	removed := uint64(0)
	kept := p.edits[:start]
	for _, e := range p.edits[start:] {
		if discard(e) {
			removed += uint64(len(e.text))
			continue
		}
		kept = append(kept, e)
	}
	clear(p.edits[len(kept):])
	p.edits = kept
	if cached {
		p.editBytes -= removed
		p.editBytesEdits = len(p.edits)
	}
}

func (p *formatter) appendReservedEdit(e edit) {
	p.edits = append(p.edits, e)
	p.editBytesEdits = len(p.edits)
}
