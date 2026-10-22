package format

import (
	"bytes"

	"github.com/prepaser/erafmt/internal/erabasic"
)

func (p *formatter) layout() {
	p.layoutWithEOL(p.effectiveEOL())
}

func (p *formatter) layoutWithEOL(eol []byte) {
	if !p.PreserveFile {
		info := p.layoutInfo()
		p.layoutPreprocessors()
		p.layoutComments(info)
		p.layoutLineEndings(eol)
		p.layoutBlankLines(info.boundary)
		p.layoutDeclarationBodies(eol)
		p.layoutFileBoundaryBlankLines()
		p.layoutFunctions(eol, info)
		p.layoutDeletions()
	} else {
		p.layoutLineEndings(eol)
	}
	p.layoutFinalNewline(eol)
}

func (p *formatter) effectiveEOL() []byte {
	switch p.eol {
	case LF:
		return []byte{'\n'}
	case CRLF:
		return []byte{'\r', '\n'}
	}
	if p.rules[LineEndingNormalization] {
		return []byte{'\n'}
	}
	last := len(p.Lines) - 1
	if !p.PreserveFile && p.rules[FileBoundaryBlankLines] {
		for last >= 0 && p.ordinaryBlank(last) {
			last--
		}
	}
	for i := last; i >= 0; i-- {
		l := p.Lines[i]
		if l.BodyEnd == l.End {
			continue
		}
		return append([]byte(nil), p.Src[l.BodyEnd:l.End]...)
	}
	return []byte{'\n'}
}

func (p *formatter) lineOpaque(i int) bool {
	return i >= 0 && i < len(p.Opaque) && p.Opaque[i]
}

func (p *formatter) ordinaryBlank(i int) bool {
	if i < 0 || i >= len(p.Lines) || p.lineOpaque(i) {
		return false
	}
	l := p.Lines[i]
	return len(erabasic.TrimInput(erabasic.LBody(p.Src, l), p.AllowFullwidthSpace)) == 0
}

func (p *formatter) ordinaryComment(i int) (int, bool) {
	if i < 0 || i >= len(p.Lines) || p.lineOpaque(i) {
		return 0, false
	}
	l := p.Lines[i]
	body := erabasic.LBody(p.Src, l)
	semi := l.IndentEnd - l.Start
	if semi >= len(body) || body[semi] != ';' {
		return 0, false
	}
	rest := body[semi+1:]
	trimmed := bytes.TrimLeft(rest, " \t")
	if bytes.HasPrefix(trimmed, []byte("!;")) || bytes.HasPrefix(trimmed, []byte("#;")) || p.Dialect == erabasic.EMEE && bytes.HasPrefix(trimmed, []byte("^;")) {
		return 0, false
	}
	if p.Dialect != erabasic.EMEE && bytes.HasPrefix(trimmed, []byte("^;")) {
		return l.Start + semi, true
	}
	if bytes.HasPrefix(trimmed, []byte(";")) || bytes.Contains(trimmed, []byte("[[")) {
		return 0, false
	}
	if decorativeComment(trimmed) {
		return 0, false
	}
	return l.Start + semi, true
}

func (p *formatter) isOrdinaryComment(i int) bool {
	_, ok := p.ordinaryComment(i)
	return ok
}

func decorativeComment(text []byte) bool {
	text = trimDecorativeCommentSpace(text)
	if len(text) == 0 {
		return false
	}
	if text[0] == '[' || text[0] == '@' || text[0] == '#' || text[0] == '!' || text[0] == '^' {
		return true
	}
	for _, r := range string(text) {
		if r >= 0x2500 && r <= 0x257f {
			return true
		}
	}
	if bytes.ContainsRune(text, '|') {
		return true
	}
	decorative := func(b byte) bool {
		switch b {
		case '-', '=', '~', '*', '+', '#', '|', '/', '\\':
			return true
		default:
			return false
		}
	}
	if !decorative(text[0]) {
		return false
	}
	n := 0
	for n < len(text) && decorative(text[n]) {
		n++
	}
	return n >= 3
}

func trimDecorativeCommentSpace(text []byte) []byte {
	text = bytes.TrimLeft(text, " \t")
	for bytes.HasPrefix(text, []byte("　")) {
		rest := text[len([]byte("　")):]
		if bytes.HasPrefix(rest, []byte("!;")) || bytes.HasPrefix(rest, []byte("#;")) || bytes.HasPrefix(rest, []byte("^;")) {
			return text
		}
		text = bytes.TrimLeft(rest, " \t")
	}
	return text
}

func (p *formatter) layoutPreprocessors() {
	if !p.rules[PreprocessorIndent] && !p.rules[PreprocessorFieldSpacing] {
		return
	}
	for i := range p.Lines {
		close, ok := p.PpLines[i]
		if !ok {
			continue
		}
		if p.lineOpaque(i) {
			continue
		}
		l := p.Lines[i]
		if p.rules[PreprocessorIndent] {
			p.addEdit(i, l.Start, l.IndentEnd, nil, PreprocessorIndent)
		}
		if p.rules[PreprocessorFieldSpacing] {
			p.layoutPreprocessorFields(i, close)
		}
	}
}

func (p *formatter) layoutPreprocessorFields(i, close int) {
	l := p.Lines[i]
	open := l.IndentEnd
	if open >= l.BodyEnd || p.Src[open] != '[' {
		return
	}
	if close >= len(p.Src) || p.Src[close] != ']' {
		if close > 0 && close-1 < len(p.Src) && p.Src[close-1] == ']' {
			close--
		} else {
			return
		}
	}
	if close <= open+1 || close > l.BodyEnd {
		return
	}
	inner := p.Src[open+1 : close]
	left := erabasic.TrimInputLeft(inner, 0, len(inner), p.AllowFullwidthSpace)
	right := erabasic.TrimInputRight(inner, left, len(inner), p.AllowFullwidthSpace)
	if left == right {
		p.addEdit(i, open+1, close, nil, PreprocessorFieldSpacing)
		return
	}
	nameEnd := left
	for nameEnd < right {
		size := erabasic.InputSpaceSize(inner, nameEnd, p.AllowFullwidthSpace)
		if size != 0 {
			break
		}
		nameEnd++
	}
	operand := nameEnd
	operand = erabasic.TrimInputLeft(inner, operand, right, p.AllowFullwidthSpace)
	var text []byte
	text = append(text, inner[left:nameEnd]...)
	if operand < right {
		text = append(text, ' ')
		text = append(text, inner[operand:right]...)
	}
	p.addEdit(i, open+1, close, text, PreprocessorFieldSpacing)
}

func (p *formatter) layoutComments(info layoutInfo) {
	for i := range p.Lines {
		if p.lineOpaque(i) {
			continue
		}
		semi, ok := p.ordinaryComment(i)
		if !ok {
			p.layoutInlineComment(i)
			p.layoutCodeTrailing(i)
			continue
		}
		l := p.Lines[i]
		if info.protectedComments[i] {
			continue
		}
		if p.Dialect != erabasic.EMEE && bytes.HasPrefix(bytes.TrimLeft(p.Src[semi+1:l.BodyEnd], " \t"), []byte("^;")) {
			continue
		}
		if p.rules[CommentPrefixSpacing] {
			content := semi + 1
			for content < l.BodyEnd && erabasic.HalfSpace(p.Src[content]) {
				content++
			}
			if content < l.BodyEnd {
				p.addEdit(i, semi+1, content, []byte{' '}, CommentPrefixSpacing)
			}
		}
		if p.rules[CommentTrailingWhitespace] {
			end := l.BodyEnd
			for end > semi+1 && erabasic.HalfSpace(p.Src[end-1]) {
				end--
			}
			p.addEdit(i, end, l.BodyEnd, nil, CommentTrailingWhitespace)
		}
	}
}

func (p *formatter) layoutCodeTrailing(i int) {
	if !p.rules[CodeTrailingWhitespace] || i < 0 || i >= len(p.Lines) || p.lineOpaque(i) {
		return
	}
	end, ok := p.CodeEnds[i]
	if !ok || end < 0 || end > p.Lines[i].BodyEnd {
		return
	}
	l := p.Lines[i]
	semi := erabasic.TrimInputLeft(p.Src, end, l.BodyEnd, p.AllowFullwidthSpace)
	if semi < l.BodyEnd && p.Src[semi] == ';' {
		return
	}
	last := erabasic.TrimInputRight(p.Src, end, l.BodyEnd, p.AllowFullwidthSpace)
	p.addEdit(i, last, l.BodyEnd, nil, CodeTrailingWhitespace)
}

func (p *formatter) layoutInlineComment(i int) {
	if (!p.rules[InlineCommentSpacing] && !p.rules[CommentPrefixSpacing] && !p.rules[CommentTrailingWhitespace]) || i < 0 || i >= len(p.Lines) || p.lineOpaque(i) {
		return
	}
	end, ok := p.CodeEnds[i]
	if !ok || end < 0 || end >= len(p.Src) {
		return
	}
	l := p.Lines[i]
	if end < l.Start || end > l.BodyEnd {
		return
	}
	semi := erabasic.TrimInputLeft(p.Src, end, l.BodyEnd, p.AllowFullwidthSpace)
	if semi >= l.BodyEnd || p.Src[semi] != ';' || semi == l.IndentEnd {
		return
	}
	commentText := bytes.TrimLeft(p.Src[semi+1:l.BodyEnd], " \t")
	if erabasic.ExecutableMarker(p.Src[semi:l.BodyEnd], 0) || bytes.Contains(commentText, []byte("[[")) || bytes.HasPrefix(commentText, []byte(";;")) || decorativeComment(commentText) {
		return
	}
	if p.rules[InlineCommentSpacing] {
		p.addEdit(i, end, semi, []byte{' '}, InlineCommentSpacing)
	}
	if p.rules[CommentPrefixSpacing] {
		content := semi + 1
		for content < l.BodyEnd && erabasic.HalfSpace(p.Src[content]) {
			content++
		}
		if content < l.BodyEnd {
			p.addEdit(i, semi+1, content, []byte{' '}, CommentPrefixSpacing)
		}
	}
	if p.rules[CommentTrailingWhitespace] {
		last := l.BodyEnd
		for last > semi && erabasic.HalfSpace(p.Src[last-1]) {
			last--
		}
		p.addEdit(i, last, l.BodyEnd, nil, CommentTrailingWhitespace)
	}
}

type layoutInfo struct {
	attached          map[int]int
	boundary          map[int]bool
	protectedComments map[int]bool
}

func (p *formatter) layoutInfo() layoutInfo {
	info := layoutInfo{attached: make(map[int]int), boundary: make(map[int]bool), protectedComments: make(map[int]bool)}
	for i := 0; i < len(p.Lines); {
		if !p.commentForAttachment(i) {
			i++
			continue
		}
		j := i
		protected := false
		for j < len(p.Lines) && p.commentForAttachment(j) {
			protected = protected || !p.isOrdinaryComment(j)
			j++
		}
		if protected {
			for k := i; k < j; k++ {
				info.protectedComments[k] = true
			}
		}
		i = j
	}
	headers := p.Headers
	for n, h := range headers {
		start := h
		for start > 0 && p.commentForAttachment(start-1) {
			start--
		}
		info.attached[h] = start
		if n == 0 {
			continue
		}
		j := start - 1
		for j > headers[n-1] && p.ordinaryBlank(j) {
			j--
		}
		for k := j + 1; k < start; k++ {
			info.boundary[k] = true
		}
	}
	return info
}

func (p *formatter) layoutBlankLines(boundary map[int]bool) {
	if !p.rules[BlankLineWhitespace] && !p.rules[BlankLineRuns] {
		return
	}
	for i := 0; i < len(p.Lines); {
		if !p.ordinaryBlank(i) {
			i++
			continue
		}
		j := i + 1
		for j < len(p.Lines) && p.ordinaryBlank(j) {
			j++
		}
		if p.rules[BlankLineWhitespace] {
			for k := i; k < j; k++ {
				l := p.Lines[k]
				if !p.rules[LineEndingNormalization] && l.Start > 0 && p.Src[l.Start-1] == '\r' && l.BodyEnd < l.End && p.Src[l.BodyEnd] == '\n' {
					continue
				}
				p.addEdit(k, l.Start, l.BodyEnd, nil, BlankLineWhitespace)
			}
		}
		boundaryRun := false
		for k := i; k < j; k++ {
			boundaryRun = boundaryRun || boundary[k]
		}
		if p.rules[BlankLineRuns] && j-i > 1 && !boundaryRun {
			p.deleteLayoutLines(i+1, j, BlankLineRuns)
		}
		i = j
	}
}

func (p *formatter) layoutFileBoundaryBlankLines() {
	if !p.rules[FileBoundaryBlankLines] || len(p.Lines) == 0 {
		return
	}
	start := 0
	for start < len(p.Lines) && p.ordinaryBlank(start) {
		start++
	}
	if start > 0 && (start == len(p.Lines) || !bytes.HasPrefix(p.Src[p.Lines[start].Start:], []byte("\ufeff"))) {
		p.deleteLayoutLines(0, start, FileBoundaryBlankLines)
	}
	end := len(p.Lines) - 1
	for end >= start && p.ordinaryBlank(end) {
		end--
	}
	if end+1 < len(p.Lines) {
		p.deleteLayoutLines(end+1, len(p.Lines), FileBoundaryBlankLines)
	}
}

func (p *formatter) layoutDeclarationBodies(eol []byte) {
	if !p.rules[DeclarationBodySeparation] || p.PreserveFile || p.Kind != erabasic.ERB || len(p.Headers) == 0 {
		return
	}
	headers := p.Headers
	for n, header := range headers {
		if p.lineOpaque(header) {
			continue
		}
		head, _, _, _ := p.headOf(erabasic.LBody(p.Src, p.Lines[header]))
		if !p.declarationRegionHeaderValid(head) {
			continue
		}
		limit := len(p.Lines)
		if n+1 < len(headers) {
			limit = headers[n+1]
		}
		body, ok := p.declarationRegionBodyStart(header+1, limit)
		if !ok {
			continue
		}
		offset := p.Lines[body].Start
		p.insertLayout(offset, p.eolBeforeLine(body, eol), body, DeclarationBodySeparation)
	}
}

func (p *formatter) layoutFunctions(eol []byte, info layoutInfo) {
	if !p.rules[FunctionSeparation] || len(p.Headers) < 2 {
		p.layoutAttachedCommentsOnly(info)
		return
	}
	headers := p.Headers
	for n := 1; n < len(headers); n++ {
		h := headers[n]
		if p.lineOpaque(h) || p.lineOpaque(headers[n-1]) {
			continue
		}
		start := info.attached[h]
		j := start - 1
		for j > headers[n-1] && p.ordinaryBlank(j) {
			j--
		}
		runStart, runEnd := j+1, start
		count := runEnd - runStart
		if count == 0 {
			offset := erabasic.ILineStart(p.Lines, start)
			p.insertLayout(offset, p.eolBeforeLine(start, eol), start, FunctionSeparation)
			continue
		}
		if count > 1 {
			p.deleteLayoutLines(runStart+1, runEnd, FunctionSeparation)
		}
	}
	p.layoutAttachedCommentsOnly(info)
}

func (p *formatter) eolBeforeLine(i int, fallback []byte) []byte {
	if p.rules[LineEndingNormalization] {
		return fallback
	}
	if i > 0 && i < len(p.Lines) {
		prev := p.Lines[i-1]
		if prev.BodyEnd != prev.End {
			terminator := p.Src[prev.BodyEnd:prev.End]
			if p.eol == LF && bytes.Equal(terminator, []byte{'\r'}) {
				return []byte{'\n', '\n'}
			}
			if p.eol == DefaultEOL {
				return terminator
			}
		}
	}
	return fallback
}

func (p *formatter) commentForAttachment(i int) bool {
	if i < 0 || i >= len(p.Lines) || p.lineOpaque(i) {
		return false
	}
	return p.rawCommentForAttachment(i)
}

func (p *formatter) rawCommentForAttachment(i int) bool {
	if i < 0 || i >= len(p.Lines) {
		return false
	}
	l := p.Lines[i]
	body := erabasic.LBody(p.Src, l)
	semi := l.IndentEnd - l.Start
	if semi >= len(body) || body[semi] != ';' {
		return false
	}
	text := bytes.TrimLeft(body[semi+1:], " \t")
	return !bytes.HasPrefix(text, []byte("!;")) && !bytes.HasPrefix(text, []byte("#;")) && !bytes.HasPrefix(text, []byte("^;"))
}

func (p *formatter) layoutAttachedCommentsOnly(info layoutInfo) {
	if !p.rules[CommentIndent] {
		return
	}
	attached := make([]bool, len(p.Lines))
	for _, h := range p.Headers {
		start := info.attached[h]
		for i := start; i < h; i++ {
			if !p.commentForAttachment(i) {
				continue
			}
			attached[i] = true
		}
	}
	p.filterEdits(0, func(e edit) bool {
		return e.change.Rule == CommentIndent && attached[e.change.Line-1]
	})
	for i, attach := range attached {
		if attach {
			l := p.Lines[i]
			p.addEdit(i, l.Start, l.IndentEnd, nil, CommentIndent)
		}
	}
}

func (p *formatter) removeLineRule(i int, rule RuleID) {
	line := i + 1
	start := min(p.statementEdit, len(p.edits))
	p.filterEdits(start, func(e edit) bool {
		return e.change.Rule == rule && e.change.Line == line
	})
}

func (p *formatter) insertLayout(offset int, text []byte, line int, rule RuleID) {
	p.insertLayoutAt(offset, text, line, 1, rule)
}

func (p *formatter) insertLayoutAt(offset int, text []byte, line, col int, rule RuleID) {
	if !p.rules[rule] || len(text) == 0 {
		return
	}
	if !p.reserveReplacementBytes(offset, offset, len(text)) {
		return
	}
	if !p.reserveMetadata(line, offset, editMetadataBytes) {
		return
	}
	p.appendReservedEdit(edit{start: offset, end: offset, text: append([]byte(nil), text...), change: Change{Rule: rule, Line: line + 1, Column: col}})
}

func (p *formatter) deleteLayoutLines(first, last int, rule RuleID) {
	if !p.rules[rule] || last <= first {
		return
	}
	if p.deleted == nil {
		p.deleted = make([]RuleID, len(p.Lines))
	}
	for i := first; i < last; i++ {
		p.deleted[i] = rule
	}
}

func (p *formatter) layoutDeletions() {
	if p.deleted == nil {
		return
	}
	var deletions []edit
	for i := 0; i < len(p.deleted); {
		rule := p.deleted[i]
		if rule == "" {
			i++
			continue
		}
		j := i + 1
		for j < len(p.deleted) && p.deleted[j] == rule {
			j++
		}
		if !p.reserveMetadata(i, p.Lines[i].Start, editMetadataBytes) {
			return
		}
		deletions = append(deletions, edit{start: p.Lines[i].Start, end: p.Lines[j-1].End, change: Change{Rule: rule, Line: i + 1, Column: 1}})
		i = j
	}
	sortEdits(p.edits)
	index := 0
	p.filterEdits(0, func(e edit) bool {
		for index < len(deletions) && deletions[index].end <= e.start {
			index++
		}
		return index < len(deletions) && e.start >= deletions[index].start && e.end <= deletions[index].end
	})
	p.edits = append(p.edits, deletions...)
	p.recalculateEditBytes()
}

func (p *formatter) layoutLineEndings(eol []byte) {
	if !p.rules[LineEndingNormalization] || len(eol) == 0 {
		return
	}
	for i, l := range p.Lines {
		if l.BodyEnd == l.End {
			continue
		}
		if bytes.Equal(p.Src[l.BodyEnd:l.End], eol) {
			continue
		}
		p.addEdit(i, l.BodyEnd, l.End, eol, LineEndingNormalization)
	}
}

func (p *formatter) layoutFinalNewline(eol []byte) {
	if !p.rules[FinalNewline] || len(p.Src) == 0 || len(p.Lines) == 0 || len(eol) == 0 {
		return
	}
	last := len(p.Lines) - 1
	if p.Lines[last].BodyEnd != p.Lines[last].End {
		return
	}
	sortEdits(p.edits)
	if !validSortedEdits(p.edits, len(p.Src)) {
		return
	}
	if len(p.edits) == 0 && len(p.Src) > maxFormattedOutputBytes {
		lastByte := p.Src[len(p.Src)-1]
		if lastByte != '\n' && lastByte != '\r' {
			p.resourceProblem(last, p.Lines[last].BodyEnd, "formatted output exceeds the formatter resource limit")
		}
		return
	}
	if size, ok := projectedOutputSizeSorted(p.Src, p.edits); !ok || size > maxFormattedOutputBytes {
		p.resourceProblem(last, p.Lines[last].BodyEnd, "formatted output exceeds the formatter resource limit")
		return
	}
	summary := summarizeSortedEdits(p.Src, p.edits)
	if summary.empty || summary.last == '\n' || summary.last == '\r' {
		return
	}
	if p.eol == DefaultEOL && !p.rules[LineEndingNormalization] {
		switch summary.lastEOL {
		case outputEOLCR:
			eol = []byte{'\r'}
		case outputEOLCRLF:
			eol = []byte{'\r', '\n'}
		default:
			eol = []byte{'\n'}
		}
	}
	p.insertLayoutAt(len(p.Src), eol, last, erabasic.Column(p.Src, p.Lines[last], len(p.Src)), FinalNewline)
}
