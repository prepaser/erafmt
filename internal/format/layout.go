package format

import "bytes"

// layout applies source-layout rules after structural scanning has completed.
// The scanner owns protected lines and trusted spans; layout never reparses
// protected payloads.
func (p *formatter) layout() {
	p.layoutWithEOL(p.effectiveEOL())
}

// layoutWithEOL is used by the core when an explicit output EOL policy is
// selected. F001 and F002 are global rules, so they intentionally ignore
// opaque function and preprocessor regions.
func (p *formatter) layoutWithEOL(eol []byte) {
	if !p.preserveFile {
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
	last := len(p.lines) - 1
	if !p.preserveFile && p.rules[FileBoundaryBlankLines] {
		for last >= 0 && p.ordinaryBlank(last) {
			last--
		}
	}
	for i := last; i >= 0; i-- {
		l := p.lines[i]
		if l.bodyEnd == l.end {
			continue
		}
		return append([]byte(nil), p.src[l.bodyEnd:l.end]...)
	}
	return []byte{'\n'}
}

func (p *formatter) lineOpaque(i int) bool {
	return i >= 0 && i < len(p.opaque) && p.opaque[i]
}

func (p *formatter) ordinaryBlank(i int) bool {
	if i < 0 || i >= len(p.lines) || p.lineOpaque(i) {
		return false
	}
	l := p.lines[i]
	return len(trimInput(lBody(p.src, l), p.allowFullwidthSpace)) == 0
}

func (p *formatter) ordinaryComment(i int) (int, bool) {
	if i < 0 || i >= len(p.lines) || p.lineOpaque(i) {
		return 0, false
	}
	l := p.lines[i]
	body := lBody(p.src, l)
	semi := l.indentEnd - l.start
	if semi >= len(body) || body[semi] != ';' {
		return 0, false
	}
	rest := body[semi+1:]
	trimmed := bytes.TrimLeft(rest, " \t")
	if bytes.HasPrefix(trimmed, []byte("!;")) || bytes.HasPrefix(trimmed, []byte("#;")) || p.dialect == EMEE && bytes.HasPrefix(trimmed, []byte("^;")) {
		return 0, false
	}
	if p.dialect != EMEE && bytes.HasPrefix(trimmed, []byte("^;")) {
		return l.start + semi, true
	}
	if bytes.HasPrefix(trimmed, []byte(";")) || bytes.Contains(trimmed, []byte("[[")) {
		return 0, false
	}
	if decorativeComment(trimmed) {
		return 0, false
	}
	return l.start + semi, true
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
	for i := range p.lines {
		close, ok := p.ppLines[i]
		if !ok {
			continue
		}
		if p.lineOpaque(i) {
			continue
		}
		l := p.lines[i]
		if p.rules[PreprocessorIndent] {
			p.addEdit(i, l.start, l.indentEnd, nil, PreprocessorIndent)
		}
		if p.rules[PreprocessorFieldSpacing] {
			p.layoutPreprocessorFields(i, close)
		}
	}
}

func (p *formatter) layoutPreprocessorFields(i, close int) {
	l := p.lines[i]
	open := l.indentEnd
	if open >= l.bodyEnd || p.src[open] != '[' {
		return
	}
	if close >= len(p.src) || p.src[close] != ']' {
		if close > 0 && close-1 < len(p.src) && p.src[close-1] == ']' {
			close--
		} else {
			return
		}
	}
	if close <= open+1 || close > l.bodyEnd {
		return
	}
	inner := p.src[open+1 : close]
	left := trimInputLeft(inner, 0, len(inner), p.allowFullwidthSpace)
	right := trimInputRight(inner, left, len(inner), p.allowFullwidthSpace)
	if left == right {
		p.addEdit(i, open+1, close, nil, PreprocessorFieldSpacing)
		return
	}
	nameEnd := left
	for nameEnd < right {
		size := inputSpaceSize(inner, nameEnd, p.allowFullwidthSpace)
		if size != 0 {
			break
		}
		nameEnd++
	}
	operand := nameEnd
	operand = trimInputLeft(inner, operand, right, p.allowFullwidthSpace)
	var text []byte
	text = append(text, inner[left:nameEnd]...)
	if operand < right {
		text = append(text, ' ')
		text = append(text, inner[operand:right]...)
	}
	p.addEdit(i, open+1, close, text, PreprocessorFieldSpacing)
}

func (p *formatter) layoutComments(info layoutInfo) {
	for i := range p.lines {
		if p.lineOpaque(i) {
			continue
		}
		semi, ok := p.ordinaryComment(i)
		if !ok {
			p.layoutInlineComment(i)
			p.layoutCodeTrailing(i)
			continue
		}
		l := p.lines[i]
		if info.protectedComments[i] {
			continue
		}
		if p.dialect != EMEE && bytes.HasPrefix(bytes.TrimLeft(p.src[semi+1:l.bodyEnd], " \t"), []byte("^;")) {
			continue
		}
		if p.rules[CommentPrefixSpacing] {
			content := semi + 1
			for content < l.bodyEnd && halfSpace(p.src[content]) {
				content++
			}
			if content < l.bodyEnd {
				p.addEdit(i, semi+1, content, []byte{' '}, CommentPrefixSpacing)
			}
		}
		if p.rules[CommentTrailingWhitespace] {
			end := l.bodyEnd
			for end > semi+1 && halfSpace(p.src[end-1]) {
				end--
			}
			p.addEdit(i, end, l.bodyEnd, nil, CommentTrailingWhitespace)
		}
	}
}

func (p *formatter) layoutCodeTrailing(i int) {
	if !p.rules[CodeTrailingWhitespace] || i < 0 || i >= len(p.lines) || p.lineOpaque(i) {
		return
	}
	end, ok := p.codeEnds[i]
	if !ok || end < 0 || end > p.lines[i].bodyEnd {
		return
	}
	l := p.lines[i]
	semi := trimInputLeft(p.src, end, l.bodyEnd, p.allowFullwidthSpace)
	if semi < l.bodyEnd && p.src[semi] == ';' {
		return
	}
	last := trimInputRight(p.src, end, l.bodyEnd, p.allowFullwidthSpace)
	p.addEdit(i, last, l.bodyEnd, nil, CodeTrailingWhitespace)
}

func (p *formatter) layoutInlineComment(i int) {
	if (!p.rules[InlineCommentSpacing] && !p.rules[CommentPrefixSpacing] && !p.rules[CommentTrailingWhitespace]) || i < 0 || i >= len(p.lines) || p.lineOpaque(i) {
		return
	}
	end, ok := p.codeEnds[i]
	if !ok || end < 0 || end >= len(p.src) {
		return
	}
	l := p.lines[i]
	if end < l.start || end > l.bodyEnd {
		return
	}
	semi := trimInputLeft(p.src, end, l.bodyEnd, p.allowFullwidthSpace)
	if semi >= l.bodyEnd || p.src[semi] != ';' || semi == l.indentEnd {
		return
	}
	commentText := bytes.TrimLeft(p.src[semi+1:l.bodyEnd], " \t")
	if executableMarker(p.src[semi:l.bodyEnd], 0) || bytes.Contains(commentText, []byte("[[")) || bytes.HasPrefix(commentText, []byte(";;")) || decorativeComment(commentText) {
		return
	}
	if p.rules[InlineCommentSpacing] {
		p.addEdit(i, end, semi, []byte{' '}, InlineCommentSpacing)
	}
	if p.rules[CommentPrefixSpacing] {
		content := semi + 1
		for content < l.bodyEnd && halfSpace(p.src[content]) {
			content++
		}
		if content < l.bodyEnd {
			p.addEdit(i, semi+1, content, []byte{' '}, CommentPrefixSpacing)
		}
	}
	if p.rules[CommentTrailingWhitespace] {
		last := l.bodyEnd
		for last > semi && halfSpace(p.src[last-1]) {
			last--
		}
		p.addEdit(i, last, l.bodyEnd, nil, CommentTrailingWhitespace)
	}
}

type layoutInfo struct {
	attached          map[int]int
	boundary          map[int]bool
	protectedComments map[int]bool
}

func (p *formatter) layoutInfo() layoutInfo {
	info := layoutInfo{attached: make(map[int]int), boundary: make(map[int]bool), protectedComments: make(map[int]bool)}
	for i := 0; i < len(p.lines); {
		if !p.commentForAttachment(i) {
			i++
			continue
		}
		j := i
		protected := false
		for j < len(p.lines) && p.commentForAttachment(j) {
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
	headers := p.headers
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
	for i := 0; i < len(p.lines); {
		if !p.ordinaryBlank(i) {
			i++
			continue
		}
		j := i + 1
		for j < len(p.lines) && p.ordinaryBlank(j) {
			j++
		}
		if p.rules[BlankLineWhitespace] {
			for k := i; k < j; k++ {
				l := p.lines[k]
				if !p.rules[LineEndingNormalization] && l.start > 0 && p.src[l.start-1] == '\r' && l.bodyEnd < l.end && p.src[l.bodyEnd] == '\n' {
					continue
				}
				p.addEdit(k, l.start, l.bodyEnd, nil, BlankLineWhitespace)
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
	if !p.rules[FileBoundaryBlankLines] || len(p.lines) == 0 {
		return
	}
	start := 0
	for start < len(p.lines) && p.ordinaryBlank(start) {
		start++
	}
	if start > 0 && (start == len(p.lines) || !bytes.HasPrefix(p.src[p.lines[start].start:], []byte("\ufeff"))) {
		p.deleteLayoutLines(0, start, FileBoundaryBlankLines)
	}

	end := len(p.lines) - 1
	for end >= start && p.ordinaryBlank(end) {
		end--
	}
	if end+1 < len(p.lines) {
		p.deleteLayoutLines(end+1, len(p.lines), FileBoundaryBlankLines)
	}
}

func (p *formatter) layoutDeclarationBodies(eol []byte) {
	if !p.rules[DeclarationBodySeparation] || p.preserveFile || p.kind != ERB || len(p.headers) == 0 {
		return
	}
	headers := p.headers
	for n, header := range headers {
		if p.lineOpaque(header) {
			continue
		}
		head, _, _, _ := p.headOf(lBody(p.src, p.lines[header]))
		nameStart := trimInputLeft(head, 1, len(head), p.allowFullwidthSpace)
		nameEnd, validName := labelIdentifierEndWithSession(head, nameStart, p.newExpressionSession(head))
		if !validName || !labelTailWithSpace(head, '@', nameEnd, p.allowFullwidthSpace) {
			continue
		}
		limit := len(p.lines)
		if n+1 < len(headers) {
			limit = headers[n+1]
		}
		body, ok := p.declarationBodyStart(header+1, limit)
		if !ok {
			continue
		}
		offset := p.lines[body].start
		p.insertLayout(offset, p.eolBeforeLine(body, eol), body, DeclarationBodySeparation)
	}
}

func (p *formatter) declarationBodyStart(start, limit int) (int, bool) {
	sawDim := false
	sawDeclaration := false
	pendingComment := false
	for i := start; i < limit; i++ {
		if p.lineOpaque(i) && !p.bodyStarts[i] {
			return 0, false
		}
		if p.ordinaryBlank(i) {
			return 0, false
		}
		if p.isOrdinaryComment(i) {
			if !sawDeclaration {
				continue
			}
			pendingComment = true
			continue
		}
		if p.declarationPrologueLine(i, &sawDim) {
			sawDeclaration = true
			pendingComment = false
			continue
		}
		if !sawDim || pendingComment || !p.provenDeclarationBody(i) {
			return 0, false
		}
		body := lBody(p.src, p.lines[i])
		head, _, optional, marked := p.headOf(body)
		if optional || marked || len(head) == 0 {
			return 0, false
		}
		return i, true
	}
	return 0, false
}

func (p *formatter) declarationPrologueLine(i int, sawDim *bool) bool {
	if i < 0 || i >= len(p.lines) || p.lineOpaque(i) {
		return false
	}
	head, _, optional, marked := p.headOf(lBody(p.src, p.lines[i]))
	if optional || marked || len(head) < 2 || head[0] != '#' {
		return false
	}
	name := declarationName(head)
	if _, ok := p.codeEnds[i]; !ok {
		return false
	}
	switch name {
	case "DIM", "DIMS":
		wordEnd := declarationWordEnd(head)
		if wordEnd >= len(head) || len(trimInput(head[wordEnd:], p.allowFullwidthSpace)) == 0 {
			return false
		}
		*sawDim = true
		return true
	case "FUNCTION", "FUNCTIONS":
		return bareSharpFunction(head, name, p.allowFullwidthSpace)
	case "LOCALSIZE", "LOCALSSIZE":
		wordEnd := declarationWordEnd(head)
		return wordEnd < len(head) && len(trimInput(head[wordEnd:], p.allowFullwidthSpace)) > 0
	default:
		return false
	}
}

func (p *formatter) provenDeclarationBody(i int) bool {
	if i >= 0 && i < len(p.bodyStarts) && p.bodyStarts[i] {
		return true
	}
	if i < 0 || i >= len(p.lines) || p.lineOpaque(i) {
		return false
	}
	if _, ok := p.codeEnds[i]; ok {
		return true
	}
	head, _, _, _ := p.headOf(lBody(p.src, p.lines[i]))
	start, _, ok := p.assignmentBoundary(head)
	return ok && p.assignmentLHS(head[:start])
}

func (p *formatter) layoutFunctions(eol []byte, info layoutInfo) {
	if !p.rules[FunctionSeparation] || len(p.headers) < 2 {
		p.layoutAttachedCommentsOnly(info)
		return
	}
	headers := p.headers
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
			offset := iLineStart(p.lines, start)
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
	if i > 0 && i < len(p.lines) {
		prev := p.lines[i-1]
		if prev.bodyEnd != prev.end {
			terminator := p.src[prev.bodyEnd:prev.end]
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
	if i < 0 || i >= len(p.lines) || p.lineOpaque(i) {
		return false
	}
	return p.rawCommentForAttachment(i)
}

func (p *formatter) rawCommentForAttachment(i int) bool {
	if i < 0 || i >= len(p.lines) {
		return false
	}
	l := p.lines[i]
	body := lBody(p.src, l)
	semi := l.indentEnd - l.start
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
	attached := make([]bool, len(p.lines))
	for _, h := range p.headers {
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
			l := p.lines[i]
			p.addEdit(i, l.start, l.indentEnd, nil, CommentIndent)
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
		p.deleted = make([]RuleID, len(p.lines))
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
		if !p.reserveMetadata(i, p.lines[i].start, editMetadataBytes) {
			return
		}
		deletions = append(deletions, edit{start: p.lines[i].start, end: p.lines[j-1].end, change: Change{Rule: rule, Line: i + 1, Column: 1}})
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
	for i, l := range p.lines {
		if l.bodyEnd == l.end {
			continue
		}
		if bytes.Equal(p.src[l.bodyEnd:l.end], eol) {
			continue
		}
		p.addEdit(i, l.bodyEnd, l.end, eol, LineEndingNormalization)
	}
}

func (p *formatter) layoutFinalNewline(eol []byte) {
	if !p.rules[FinalNewline] || len(p.src) == 0 || len(p.lines) == 0 || len(eol) == 0 {
		return
	}
	last := len(p.lines) - 1
	if p.lines[last].bodyEnd != p.lines[last].end {
		return
	}
	sortEdits(p.edits)
	if !validSortedEdits(p.edits, len(p.src)) {
		return
	}
	if len(p.edits) == 0 && len(p.src) > maxFormattedOutputBytes {
		lastByte := p.src[len(p.src)-1]
		if lastByte != '\n' && lastByte != '\r' {
			p.resourceProblem(last, p.lines[last].bodyEnd, "formatted output exceeds the formatter resource limit")
		}
		return
	}
	if size, ok := projectedOutputSizeSorted(p.src, p.edits); !ok || size > maxFormattedOutputBytes {
		p.resourceProblem(last, p.lines[last].bodyEnd, "formatted output exceeds the formatter resource limit")
		return
	}
	summary := summarizeSortedEdits(p.src, p.edits)
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
	p.insertLayoutAt(len(p.src), eol, last, column(p.src, p.lines[last], len(p.src)), FinalNewline)
}

func lBody(src []byte, l line) []byte { return src[l.start:l.bodyEnd] }

func iLineStart(lines []line, i int) int {
	if i < 0 || i >= len(lines) {
		return 0
	}
	return lines[i].start
}
