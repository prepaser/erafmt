package format

import (
	"bytes"
	"sort"
	"unicode/utf8"
)

type continuationSegment struct {
	logicalStart int
	logicalEnd   int
	line         int
	sourceStart  int
	sourceEnd    int
	indent       int
}

type continuationInput struct {
	first, last int
	logical     []byte
	segments    []continuationSegment
}

func (input *continuationInput) position(offset int) (sourcePosition, bool) {
	index, local := continuationDiagnosticSegment(offset, input.segments)
	if index < 0 {
		return sourcePosition{}, false
	}
	segment := input.segments[index]
	return sourcePosition{segment.line, min(segment.sourceEnd, max(segment.sourceStart, segment.sourceStart+local))}, true
}

func (p *formatter) recordContinuationOrigin(input *continuationInput) bool {
	_, prefix, _, _ := p.headOf(input.logical)
	position, ok := input.position(prefix)
	if !ok {
		return true
	}
	if !p.reserveMetadata(position.line, position.offset, 32) {
		return false
	}
	if p.statementOrigins == nil {
		p.statementOrigins = make(map[int]sourcePosition)
	}
	p.statementOrigins[input.first] = position
	return true
}

func (p *formatter) continuationNilPhysicalHead(input *continuationInput) (head []byte, optional, marked bool) {
	if input == nil || len(input.segments) == 0 {
		return nil, false, false
	}
	segment := input.segments[0]
	if segment.sourceStart < 0 || segment.sourceEnd < segment.sourceStart || segment.sourceEnd > len(p.src) {
		return nil, false, false
	}
	head, _, optional, marked = p.headOf(p.src[segment.sourceStart:segment.sourceEnd])
	return head, optional, marked
}

func (p *formatter) continuationNilCommandIdentityUnknown(input *continuationInput, command string) bool {
	head, optional, marked := p.continuationNilPhysicalHead(input)
	if optional || marked {
		return true
	}
	if len(head) == 0 || head[0] == ';' {
		return command != ""
	}
	physicalCommand, physicalWordEnd := p.commandOf(head)
	if physicalCommand == "" {
		return command != ""
	}
	if physicalWordEnd == len(head) {
		return true
	}
	return command != physicalCommand
}

func (p *formatter) continuationNilHeaderEquivalent(input *continuationInput) bool {
	head, optional, marked := p.continuationNilPhysicalHead(input)
	if optional || marked || len(head) == 0 || head[0] != '@' {
		return false
	}
	session := p.newExpressionSession(head)
	validation := validateHeaderWithSession(head, session)
	if p.aborted {
		return false
	}
	complete, comment := continuationPhysicalHeaderBoundary(head, session)
	if validation.status != headerValid {
		return complete
	}
	return comment
}

func (p *formatter) continuationNilPreprocessorEquivalent(input *continuationInput) bool {
	head, optional, marked := p.continuationNilPhysicalHead(input)
	if optional || marked || len(head) == 0 || head[0] != '[' {
		return false
	}
	end := bytes.IndexByte(head, ']')
	if end < 0 {
		return false
	}
	first, _, count := preprocessorFields(head, end, p.allowFullwidthSpace)
	return count != 0 && len(first) != 0
}

func (p *formatter) continuationNilDispatchUnknown(input *continuationInput) (unknown, preserveFile bool) {
	head, optional, marked := p.continuationNilPhysicalHead(input)
	if optional || marked {
		return true, true
	}
	logicalHead, _, logicalOptional, logicalMarked := p.headOf(input.logical)
	if logicalOptional || logicalMarked {
		return true, len(logicalHead) > 0 && (logicalHead[0] == '@' || logicalHead[0] == '[')
	}
	if len(head) != 0 {
		if continuationNilIncompleteMarker(head) {
			return true, false
		}
		return false, false
	}
	if len(logicalHead) == 0 {
		return len(input.segments) != 0, len(input.segments) != 0
	}
	return true, true
}

func continuationNilIncompleteMarker(head []byte) bool {
	if len(head) == 1 {
		return head[0] == ';'
	}
	return len(head) == 2 && head[0] == ';' && (head[1] == '!' || head[1] == '#' || head[1] == '^')
}

func continuationPhysicalHeaderBoundary(head []byte, session *expressionSession) (complete, comment bool) {
	if len(head) < 2 || head[0] != '@' {
		return false, false
	}
	start := trimInputLeftWithSession(head, 1, len(head), session)
	nameEnd, ok := labelIdentifierEndWithSession(head, start, session)
	if !ok {
		return start < len(head) && head[start] >= '0' && head[start] <= '9', false
	}
	pos := skipHeaderSpaceWithSession(head, nameEnd, session)
	if pos >= len(head) || head[pos] == ';' {
		return pos < len(head), pos < len(head)
	}
	var result headerValidation
	switch head[pos] {
	case '(':
		var close int
		close, result = scanHeaderDelimiterWithSession(head, pos, session)
		if result.status == headerValid {
			return true, continuationHeaderCommentSealed(head, close+1, session)
		}
	case '[':
		_, result = scanHeaderDelimiterWithSession(head, pos, session)
	case ',':
		end, result := scanHeaderCodeEndWithSession(head, pos+1, session)
		if result.status != headerValid {
			return continuationHeaderDelimiterUnrepairable(head, pos+1, session), false
		}
		if end < len(head) && head[end] == ';' {
			return true, true
		}
		return false, false
	default:
		return true, false
	}
	if result.status != headerValid {
		return continuationHeaderDelimiterUnrepairable(head, pos, session), false
	}
	return true, false
}

func continuationHeaderCommentSealed(src []byte, start int, session *expressionSession) bool {
	pos := skipHeaderSpaceWithSession(src, start, session)
	if pos >= len(src) || src[pos] != ';' {
		return false
	}
	for executableMarker(src, pos) {
		pos += 3
		pos = skipHeaderSpaceWithSession(src, pos, session)
		if pos >= len(src) || src[pos] != ';' {
			return false
		}
	}
	return true
}

func continuationHeaderDelimiterUnrepairable(src []byte, start int, session *expressionSession) bool {
	stack := make([]byte, 0, 2)
	i := start
	if start < len(src) && (src[start] == '(' || src[start] == '[') {
		stack = append(stack, headerCloseFor(src[start]))
		i++
	}
	for i < len(src) {
		next, atom, result := scanHeaderAtomWithSession(src, i, len(src), session)
		if result.status != headerValid {
			return false
		}
		if atom {
			i = next
			continue
		}
		if size := inputSpaceSize(src, i, session.allowFullwidthSpace); size > 0 {
			i += size
			continue
		}
		if headerFullwidthAtWithSession(src, i, session) {
			return false
		}
		switch src[i] {
		case ';':
			return len(stack) != 0
		case '(', '[':
			stack = append(stack, headerCloseFor(src[i]))
		case ')', ']':
			if len(stack) == 0 || stack[len(stack)-1] != src[i] {
				return true
			}
			stack = stack[:len(stack)-1]
		}
		i++
	}
	return false
}

type continuationProof struct {
	logical   []byte
	segments  []continuationSegment
	head      []byte
	prefix    int
	wordEnd   int
	proxy     *formatter
	mapped    []edit
	mappingOK bool
	valid     bool
	certain   bool
}

func (p *formatter) proveContinuation(input *continuationInput) continuationProof {
	first, last := input.first, input.last
	proof := continuationProof{}
	if p.continuationSeparator == nil || p.preserved || p.preserveFile || first < 0 || last <= first+1 || last > len(p.lines) {
		return proof
	}
	proof.logical, proof.segments = input.logical, input.segments
	var optional, marked bool
	proof.head, proof.prefix, optional, marked = p.headOf(proof.logical)
	if len(proof.head) == 0 || optional || marked || continuationUnsafe(proof.head) {
		return proof
	}
	proof.proxy = p.continuationProxy(proof.logical)
	continuationProxySyntax(proof.proxy)
	if proof.proxy.aborted {
		proof.valid = true
		return proof
	}
	proof.valid = validEdits(proof.proxy.edits, len(proof.logical))
	if !proof.valid {
		return proof
	}
	proof.mapped = continuationMappedEdits(proof.proxy.edits, proof.segments)
	proof.mappingOK = true
	if proof.mapped != nil {
		verified, diagnostics := p.continuationMappingVerified(proof.logical, proof.proxy.diagnostics, proof.proxy.edits, proof.mapped)
		if !verified {
			continuationAppendDiagnostics(p, diagnostics, proof.segments, proof.logical)
			proof.mappingOK = false
			return proof
		}
	}
	_, proof.wordEnd = p.commandOf(proof.head)
	proof.certain = len(proof.proxy.opaque) != 0 && !proof.proxy.opaque[0] && len(proof.proxy.diagnostics) == 0
	return proof
}

func (p *formatter) continuationWarning(proof continuationProof, code, message string) {
	diagnostic := Diagnostic{Severity: Warning, Code: code, Message: message, Column: utf8.RuneCount(proof.logical[:proof.prefix]) + 1}
	continuationAppendDiagnostics(p, []Diagnostic{diagnostic}, proof.segments, proof.logical)
}

func (p *formatter) auditRepeatCountContinuation(proof continuationProof) {
	proxy := *proof.proxy
	proxy.diagnostics = nil
	proxy.edits = nil
	proxy.auditRepeatCount(0, proof.head, proof.wordEnd, proof.prefix)
	continuationAppendDiagnostics(p, proxy.diagnostics, proof.segments, proof.logical)
}

func (p *formatter) provenRepeatPositiveContinuation(proof continuationProof) bool {
	proxy := *proof.proxy
	return proxy.provenRepeatPositive(proof.head, proof.wordEnd, proof.prefix)
}

func (p *formatter) continuationHeaderArgumentsValid(input *continuationInput) bool {
	if !p.expressionParseEnabled() {
		return true
	}
	logical, segments := input.logical, input.segments
	if continuationUnsafe(logical) {
		return true
	}
	head, prefix, _, marked := p.headOf(logical)
	if marked {
		return true
	}
	if p.continuationSeparator == nil {
		physicalHead, optional, physicalMarked := p.continuationNilPhysicalHead(input)
		if optional || physicalMarked || len(physicalHead) == 0 || physicalHead[0] != '@' || !p.continuationNilHeaderEquivalent(input) {
			return true
		}
		physicalPrefix := 0
		if input != nil && len(input.segments) != 0 {
			segment := input.segments[0]
			if segment.sourceStart >= 0 && segment.sourceEnd >= segment.sourceStart && segment.sourceEnd <= len(p.src) {
				physicalPrefix = len(p.src[segment.sourceStart:segment.sourceEnd]) - len(physicalHead)
			}
		}
		head, prefix = physicalHead, physicalPrefix
	}
	session := p.newExpressionSession(head)
	start, end, ok := headerArgumentRangeWithSession(head, session)
	if !ok {
		return true
	}
	valid := true
	for _, diagnostic := range headerArgumentDiagnosticsWithSession(head, start, end, session, p.dialect) {
		mapped := Diagnostic{Severity: diagnostic.severity, Code: diagnostic.code, Message: diagnostic.message, Column: utf8.RuneCount(logical[:prefix+diagnostic.offset]) + 1}
		continuationAppendDiagnostics(p, []Diagnostic{mapped}, segments, logical)
		if diagnostic.severity != Warning {
			valid = false
		}
	}
	return valid
}

func (p *formatter) continuationSyntax(input *continuationInput) {
	p.continuationSyntaxWithProof(input.first, input.last, p.proveContinuation(input))
}

func (p *formatter) continuationSyntaxWithProof(first, last int, proof continuationProof) {
	sharp := continuationPhysicalSharp(p, first, last)
	if p.continuationSeparator == nil || p.preserved || p.preserveFile || first < 0 || last <= first+1 || last > len(p.lines) {
		if sharp {
			p.sharpStateUnknown = true
		}
		return
	}

	logical, segments := proof.logical, proof.segments
	head := proof.head
	sharp = len(head) > 0 && head[0] == '#'
	if len(logical) == 0 || len(segments) == 0 || continuationUnsafe(logical) {
		if sharp {
			p.sharpStateUnknown = true
		}
		return
	}

	proxy := proof.proxy
	if proxy == nil || !proof.valid {
		if sharp {
			p.sharpStateUnknown = true
		}
		return
	}
	if proxy.preserveFile {
		p.preserveFile = true
	}
	if proxy.preserved {
		p.truncateEdits(p.functionEdit)
		p.preserved = true
	}

	if len(proxy.diagnostics) == 0 {
		head, prefix, _, marked := proxy.headOf(logical)
		command, wordEnd := proxy.commandOf(head)
		if !marked && command == "SELECTCASE" {
			p.state.setSelectorType(proxy.provenSelectorType(head, wordEnd, prefix))
		}
	}

	mapped := proof.mapped
	mappingOK := proof.mappingOK
	if !mappingOK {
		mapped = nil
		if sharp {
			p.sharpStateUnknown = true
		}
	}
	if len(mapped) != 0 && !p.preserved && !p.preserveFile {
		continuationAddSortedMappedEdits(p, mapped, segments)
	}
	p.commitContinuationContext(proof)
	continuationAppendDiagnostics(p, proxy.diagnostics, segments, logical)
}

func continuationUnsafe(src []byte) bool {
	return bytes.Contains(src, []byte("[[")) || bytes.Contains(src, []byte(";!;")) || bytes.Contains(src, []byte(";#;"))
}

func (p *formatter) continuationProxy(src []byte) *formatter {
	if p.metadata == nil {
		p.metadata = &metadataBudget{}
	}
	count, fits := sourceLineCount(src, maxMetadataBytes/lineMetadataBytes)
	q := &formatter{src: src, metadata: p.metadata}
	if !fits || !q.reserveMetadata(0, 0, count*lineMetadataBytes) {
		q.resourceProblem(0, 0, "continuation metadata exceeds the formatter resource limit")
		return q
	}
	lines := makeLinesWithSpace(src, p.allowFullwidthSpace, count)
	*q = formatter{
		metadata:            p.metadata,
		src:                 src,
		lines:               lines,
		kind:                p.kind,
		state:               p.state.clone(),
		base:                p.base,
		rules:               p.rules,
		opaque:              make([]bool, len(lines)),
		bodyStarts:          make([]bool, len(lines)),
		codeEnds:            make(map[int]int),
		ppLines:             make(map[int]int),
		eol:                 p.eol,
		dialect:             p.dialect,
		allowFullwidthSpace: p.allowFullwidthSpace,
		inContinuationProxy: true,
		optional:            p.optional || p.conditional(),
		functionContext:     p.continuationContext(),
	}
	return q
}

func continuationProxySyntax(p *formatter) {
	if len(p.lines) != 1 {
		return
	}
	body := lBody(p.src, p.lines[0])
	head, prefix, _, marked := p.headOf(body)
	p.parseBase = prefix
	if len(head) == 0 || marked || continuationUnsafe(head) {
		return
	}
	if head[0] == '#' {
		p.auditSharpDeclaration(0, head, p.lines[0].start+prefix, p.optional, false, false)
	}
	switch head[0] {
	case ';':
		p.layoutComments(layoutInfo{})
		return
	case '[':
		end := bytes.IndexByte(head, ']')
		if end < 0 || !bytes.HasPrefix(head, []byte("[")) || bytes.HasPrefix(head, []byte("[[")) {
			return
		}
		p.ppLines[0] = p.lines[0].start + prefix + end
		p.layoutPreprocessorFields(0, p.ppLines[0])
		return
	case '$':
		p.auditDollarLabel(0, head, p.lines[0].start+prefix, p.optional || p.conditional())
		p.syntaxLabel(0, head, p.lines[0].start+prefix)
	case '@':
		if p.rules[LabelMarkerSpacing] && len(head) > 1 && p.inputSpaceIn(head, 1) {
			p.syntaxLabel(0, head, p.lines[0].start+prefix)
		}
		p.syntax(0, head, prefix)
	default:
		command, wordEnd := p.commandOf(head)
		if !p.optional && command == "CASE" && p.caseTypeMismatch(p.state, head, wordEnd, prefix) {
			p.syntaxProblem(0, prefix+wordEnd, "CASE has an incompatible type")
			return
		}
		p.syntax(0, head, prefix)
	}
	if head[0] == '#' && len(p.opaque) != 0 && p.opaque[0] && !continuationWarningOnly(p) {
		p.sharpStateUnknown = true
	}
	p.layoutComments(layoutInfo{})
}

func continuationLabelStateCommitted(p *formatter, src []byte) bool {
	head, _, optional, marked := p.headOf(src)
	if len(head) == 0 || head[0] != '$' || optional || marked || p.optional || p.conditional() || bytesHasAlias(head) {
		return false
	}
	return len(p.opaque) != 0 && !p.opaque[0]
}

func (p *formatter) continuationDeclarationSafe(head []byte) bool {
	if p.continuationSeparator == nil || p.optional || p.conditional() || bytesHasAlias(head) || len(head) == 0 || head[0] != '#' {
		return false
	}
	switch declarationName(head) {
	case "DIM", "DIMS", "FUNCTION", "FUNCTIONS", "LOCALSIZE", "LOCALSSIZE", "SINGLE", "LATER", "PRI", "ONLY":
		return true
	default:
		return false
	}
}

func continuationPhysicalSharp(p *formatter, first, last int) bool {
	if first < 0 || last <= first+1 || last > len(p.lines) {
		return false
	}
	for row := first + 1; row < last; row++ {
		body := bytes.TrimSpace(p.src[p.lines[row].start:p.lines[row].bodyEnd])
		if len(body) == 0 {
			continue
		}
		return body[0] == '#'
	}
	return false
}

func continuationDeclarationCommitted(p *formatter, src []byte) bool {
	head, _, optional, marked := p.headOf(src)
	if len(head) == 0 || head[0] != '#' || optional || marked || p.optional || p.conditional() || bytesHasAlias(head) || p.sharpStateUnknown {
		return false
	}
	if len(p.opaque) == 0 || !p.opaque[0] {
		return true
	}
	return continuationWarningOnly(p)
}

func continuationWarningOnly(p *formatter) bool {
	if len(p.diagnostics) == 0 {
		return false
	}
	for _, diagnostic := range p.diagnostics {
		if diagnostic.Severity != Warning {
			return false
		}
	}
	return true
}

func continuationMappedEdits(edits []edit, segments []continuationSegment) []edit {
	if len(edits) == 0 {
		return nil
	}
	sortEdits(edits)
	mapped := make([]edit, 0, len(edits))
	for _, e := range edits {
		index := continuationEditSegment(e, segments)
		if index < 0 {
			continue
		}
		segment := segments[index]
		if e.start == e.end && e.start == segment.logicalStart {
			continue
		}
		if e.start < segment.logicalStart+segment.indent || e.end > segment.logicalEnd {
			continue
		}
		mapped = append(mapped, e)
	}
	return mapped
}

func continuationEditSegment(e edit, segments []continuationSegment) int {
	if e.start != e.end {
		i := sort.Search(len(segments), func(i int) bool {
			return segments[i].logicalEnd >= e.end
		})
		if i < len(segments) && e.start >= segments[i].logicalStart && e.end <= segments[i].logicalEnd {
			return i
		}
		return -1
	}
	i := sort.Search(len(segments), func(i int) bool {
		return segments[i].logicalStart > e.start
	}) - 1
	if i >= 0 && e.start <= segments[i].logicalEnd {
		return i
	}
	if len(segments) != 0 && e.start == segments[0].logicalStart {
		return 0
	}
	return -1
}

func (p *formatter) continuationMappingVerified(original []byte, originalDiagnostics []Diagnostic, canonicalEdits, mapped []edit) (bool, []Diagnostic) {
	mappedSource := applyEdits(original, mapped)
	want := applyEdits(original, canonicalEdits)
	q := p.continuationProxy(mappedSource)
	continuationProxySyntax(q)
	var resourceDiagnostics []Diagnostic
	for _, diagnostic := range q.diagnostics {
		if diagnostic.Severity == Error && diagnostic.Code == "resource_limit" {
			resourceDiagnostics = append(resourceDiagnostics, diagnostic)
		}
	}
	if !validEdits(q.edits, len(mappedSource)) {
		return false, resourceDiagnostics
	}
	got := applyEdits(mappedSource, q.edits)
	verified := bytes.Equal(want, got) && continuationDiagnosticsEqual(originalDiagnostics, q.diagnostics)
	if verified {
		return true, nil
	}
	return false, resourceDiagnostics
}

func continuationDiagnosticsEqual(a, b []Diagnostic) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Severity != b[i].Severity || a[i].Code != b[i].Code || a[i].Message != b[i].Message {
			return false
		}
	}
	return true
}

func continuationAddSortedMappedEdits(p *formatter, edits []edit, segments []continuationSegment) {
	currentLine := -1
	rowEdits := make([]edit, 0, min(len(edits), 4))
	flush := func() {
		if currentLine >= 0 {
			p.addSyntaxEdits(currentLine, 0, rowEdits)
			rowEdits = rowEdits[:0]
		}
	}
	for _, e := range edits {
		index := continuationEditSegment(e, segments)
		if index < 0 {
			continue
		}
		segment := segments[index]
		e.start += segment.sourceStart - segment.logicalStart
		e.end += segment.sourceStart - segment.logicalStart
		if currentLine != segment.line {
			flush()
			currentLine = segment.line
		}
		rowEdits = append(rowEdits, e)
	}
	flush()
}

func continuationAppendDiagnostics(p *formatter, diagnostics []Diagnostic, segments []continuationSegment, source []byte) {
	offset, column := 0, 1
	for _, diagnostic := range diagnostics {
		wanted := max(1, diagnostic.Column)
		if wanted < column {
			offset, column = 0, 1
		}
		offset += continuationByteOffset(wanted-column+1, source[offset:])
		column = wanted
		segment, local := continuationDiagnosticSegment(offset, segments)
		if segment < 0 {
			continue
		}
		row := segments[segment].line
		physical := segments[segment].sourceStart + local
		if physical > segments[segment].sourceEnd {
			physical = segments[segment].sourceEnd
		}
		if diagnostic.Code == "resource_limit" && diagnostic.Severity == Error {
			p.resourceProblem(row, physical, diagnostic.Message)
			return
		}
		p.diagnostic(row, physical, diagnostic.Severity, diagnostic.Code, diagnostic.Message)
	}
}

func continuationByteOffset(columnNumber int, src []byte) int {
	if columnNumber <= 1 {
		return 0
	}
	remaining := columnNumber - 1
	for offset := 0; offset < len(src); {
		_, size := utf8.DecodeRune(src[offset:])
		if size == 0 {
			break
		}
		if remaining == 0 {
			return offset
		}
		offset += size
		remaining--
	}
	return len(src)
}

func continuationDiagnosticSegment(offset int, segments []continuationSegment) (int, int) {
	index := sort.Search(len(segments), func(i int) bool { return segments[i].logicalEnd > offset })
	if index < len(segments) && offset >= segments[index].logicalStart {
		return index, offset - segments[index].logicalStart
	}
	index = sort.Search(len(segments), func(i int) bool { return segments[i].logicalEnd >= offset })
	if index > 0 && index < len(segments) && offset < segments[index].logicalStart {
		index--
		return index, segments[index].sourceEnd - segments[index].sourceStart
	}
	if index < len(segments) {
		if offset == segments[index].logicalEnd {
			return index, segments[index].sourceEnd - segments[index].sourceStart
		}
		return index, offset - segments[index].logicalStart
	}
	if len(segments) == 0 {
		return -1, 0
	}
	last := segments[len(segments)-1]
	return len(segments) - 1, last.sourceEnd - last.sourceStart
}
