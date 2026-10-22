package format

import (
	"bytes"
	"sort"
	"unicode/utf8"

	"github.com/prepaser/erafmt/internal/erabasic"
)

func (p *formatter) recordContinuationOrigin(input *erabasic.ContinuationInput) bool {
	_, prefix, _, _ := p.headOf(input.Logical)
	position, ok := input.Position(prefix)
	if !ok {
		return true
	}
	if !p.reserveMetadata(position.Line, position.Offset, 32) {
		return false
	}
	if p.StatementOrigins == nil {
		p.StatementOrigins = make(map[int]erabasic.SourcePosition)
	}
	p.StatementOrigins[input.First] = position
	return true
}

type continuationProof struct {
	logical   []byte
	segments  []erabasic.ContinuationSegment
	head      []byte
	prefix    int
	wordEnd   int
	proxy     *formatter
	mapped    []edit
	mappingOK bool
	valid     bool
	certain   bool
}

func (p *formatter) proveContinuation(input *erabasic.ContinuationInput) continuationProof {
	first, last := input.First, input.Last
	proof := continuationProof{}
	if p.ContinuationSeparator == nil || p.Preserved || p.PreserveFile || first < 0 || last <= first+1 || last > len(p.Lines) {
		return proof
	}
	proof.logical, proof.segments = input.Logical, input.Segments
	var optional, marked bool
	proof.head, proof.prefix, optional, marked = p.headOf(proof.logical)
	if len(proof.head) == 0 || optional || marked || erabasic.ContinuationUnsafe(proof.head) {
		return proof
	}
	proof.proxy = p.continuationProxy(proof.logical)
	continuationProxySyntax(proof.proxy)
	if proof.proxy.Aborted {
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
	proof.certain = len(proof.proxy.Opaque) != 0 && !proof.proxy.Opaque[0] && len(proof.proxy.diagnostics) == 0
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

func (p *formatter) continuationHeaderArgumentsValid(input *erabasic.ContinuationInput) bool {
	if !p.expressionParseEnabled() {
		return true
	}
	logical, segments := input.Logical, input.Segments
	if erabasic.ContinuationUnsafe(logical) {
		return true
	}
	head, prefix, _, marked := p.headOf(logical)
	if marked {
		return true
	}
	if p.ContinuationSeparator == nil {
		physicalHead, optional, physicalMarked := p.continuationNilPhysicalHead(input)
		if optional || physicalMarked || len(physicalHead) == 0 || physicalHead[0] != '@' || !p.continuationNilHeaderEquivalent(input) {
			return true
		}
		physicalPrefix := 0
		if input != nil && len(input.Segments) != 0 {
			segment := input.Segments[0]
			if segment.SourceStart >= 0 && segment.SourceEnd >= segment.SourceStart && segment.SourceEnd <= len(p.Src) {
				physicalPrefix = len(p.Src[segment.SourceStart:segment.SourceEnd]) - len(physicalHead)
			}
		}
		head, prefix = physicalHead, physicalPrefix
	}
	session := p.newExpressionSession(head)
	start, end, ok := erabasic.HeaderArgumentRangeWithSession(head, session)
	if !ok {
		return true
	}
	valid := true
	for _, diagnostic := range erabasic.HeaderArgumentDiagnosticsWithSession(head, start, end, session, p.Dialect) {
		mapped := Diagnostic{Severity: Severity(diagnostic.Severity), Code: diagnostic.Code, Message: diagnostic.Message, Column: utf8.RuneCount(logical[:prefix+diagnostic.Offset]) + 1}
		continuationAppendDiagnostics(p, []Diagnostic{mapped}, segments, logical)
		if diagnostic.Severity != erabasic.Warning {
			valid = false
		}
	}
	return valid
}

func (p *formatter) continuationSyntax(input *erabasic.ContinuationInput) {
	p.continuationSyntaxWithProof(input.First, input.Last, p.proveContinuation(input))
}

func (p *formatter) continuationSyntaxWithProof(first, last int, proof continuationProof) {
	sharp := erabasic.ContinuationPhysicalSharp(p.Src, p.Lines, first, last)
	if p.ContinuationSeparator == nil || p.Preserved || p.PreserveFile || first < 0 || last <= first+1 || last > len(p.Lines) {
		if sharp {
			p.SharpStateUnknown = true
		}
		return
	}
	logical, segments := proof.logical, proof.segments
	head := proof.head
	sharp = len(head) > 0 && head[0] == '#'
	if len(logical) == 0 || len(segments) == 0 || erabasic.ContinuationUnsafe(logical) {
		if sharp {
			p.SharpStateUnknown = true
		}
		return
	}
	proxy := proof.proxy
	if proxy == nil || !proof.valid {
		if sharp {
			p.SharpStateUnknown = true
		}
		return
	}
	if proxy.PreserveFile {
		p.PreserveFile = true
	}
	if proxy.Preserved {
		p.truncateEdits(p.functionEdit)
		p.Preserved = true
	}
	if len(proxy.diagnostics) == 0 {
		head, prefix, _, marked := proxy.headOf(logical)
		command, wordEnd := proxy.commandOf(head)
		if !marked && command == "SELECTCASE" {
			p.State.SetSelectorType(proxy.provenSelectorType(head, wordEnd, prefix))
		}
	}
	mapped := proof.mapped
	mappingOK := proof.mappingOK
	if !mappingOK {
		mapped = nil
		if sharp {
			p.SharpStateUnknown = true
		}
	}
	if len(mapped) != 0 && !p.Preserved && !p.PreserveFile {
		continuationAddSortedMappedEdits(p, mapped, segments)
	}
	p.commitContinuationContext(proof)
	continuationAppendDiagnostics(p, proxy.diagnostics, segments, logical)
}

func (p *formatter) continuationProxy(src []byte) *formatter {
	if p.Metadata == nil {
		p.Metadata = erabasic.NewMetadataBudget(0)
	}
	count, fits := erabasic.SourceLineCount(src, erabasic.MaxMetadataBytes/erabasic.LineMetadataBytes)
	q := &formatter{DocumentSession: erabasic.DocumentSession{Src: src, Metadata: p.Metadata}}
	if !fits || !q.reserveMetadata(0, 0, count*erabasic.LineMetadataBytes) {
		q.resourceProblem(0, 0, "continuation metadata exceeds the formatter resource limit")
		return q
	}
	lines := erabasic.MakeLinesWithSpace(src, p.AllowFullwidthSpace, count)
	*q = formatter{
		DocumentSession: erabasic.DocumentSession{
			Metadata:            p.Metadata,
			Src:                 src,
			Lines:               lines,
			Kind:                p.Kind,
			State:               p.State.Clone(),
			Base:                p.Base,
			Opaque:              make([]bool, len(lines)),
			BodyStarts:          make([]bool, len(lines)),
			CodeEnds:            make(map[int]int),
			PpLines:             make(map[int]int),
			Dialect:             p.Dialect,
			AllowFullwidthSpace: p.AllowFullwidthSpace,
			InContinuationProxy: true,
			Optional:            p.Optional || p.conditional(),
			FunctionContext:     p.continuationContext(),
		},
		rules: p.rules,
		eol:   p.eol,
	}
	return q
}

func continuationProxySyntax(p *formatter) {
	if len(p.Lines) != 1 {
		return
	}
	body := erabasic.LBody(p.Src, p.Lines[0])
	head, prefix, _, marked := p.headOf(body)
	p.ParseBase = prefix
	if len(head) == 0 || marked || erabasic.ContinuationUnsafe(head) {
		return
	}
	if head[0] == '#' {
		p.auditSharpDeclaration(0, head, p.Lines[0].Start+prefix, p.Optional, false, false)
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
		p.PpLines[0] = p.Lines[0].Start + prefix + end
		p.layoutPreprocessorFields(0, p.PpLines[0])
		return
	case '$':
		p.auditDollarLabel(0, head, p.Lines[0].Start+prefix, p.Optional || p.conditional())
		p.syntaxLabel(0, head, p.Lines[0].Start+prefix)
	case '@':
		if p.rules[LabelMarkerSpacing] && len(head) > 1 && p.inputSpaceIn(head, 1) {
			p.syntaxLabel(0, head, p.Lines[0].Start+prefix)
		}
		p.syntax(0, head, prefix)
	default:
		command, wordEnd := p.commandOf(head)
		if !p.Optional && command == "CASE" && p.caseTypeMismatch(p.State, head, wordEnd, prefix) {
			p.syntaxProblem(0, prefix+wordEnd, "CASE has an incompatible type")
			return
		}
		p.syntax(0, head, prefix)
	}
	if head[0] == '#' && len(p.Opaque) != 0 && p.Opaque[0] && !continuationWarningOnly(p) {
		p.SharpStateUnknown = true
	}
	p.layoutComments(layoutInfo{})
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

func continuationMappedEdits(edits []edit, segments []erabasic.ContinuationSegment) []edit {
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
		if e.start == e.end && e.start == segment.LogicalStart {
			continue
		}
		if e.start < segment.LogicalStart+segment.Indent || e.end > segment.LogicalEnd {
			continue
		}
		mapped = append(mapped, e)
	}
	return mapped
}

func continuationEditSegment(e edit, segments []erabasic.ContinuationSegment) int {
	if e.start != e.end {
		i := sort.Search(len(segments), func(i int) bool {
			return segments[i].LogicalEnd >= e.end
		})
		if i < len(segments) && e.start >= segments[i].LogicalStart && e.end <= segments[i].LogicalEnd {
			return i
		}
		return -1
	}
	i := sort.Search(len(segments), func(i int) bool {
		return segments[i].LogicalStart > e.start
	}) - 1
	if i >= 0 && e.start <= segments[i].LogicalEnd {
		return i
	}
	if len(segments) != 0 && e.start == segments[0].LogicalStart {
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

func continuationAddSortedMappedEdits(p *formatter, edits []edit, segments []erabasic.ContinuationSegment) {
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
		e.start += segment.SourceStart - segment.LogicalStart
		e.end += segment.SourceStart - segment.LogicalStart
		if currentLine != segment.Line {
			flush()
			currentLine = segment.Line
		}
		rowEdits = append(rowEdits, e)
	}
	flush()
}

func continuationAppendDiagnostics(p *formatter, diagnostics []Diagnostic, segments []erabasic.ContinuationSegment, source []byte) {
	offset, column := 0, 1
	for _, diagnostic := range diagnostics {
		wanted := max(1, diagnostic.Column)
		if wanted < column {
			offset, column = 0, 1
		}
		offset += erabasic.ContinuationByteOffset(wanted-column+1, source[offset:])
		column = wanted
		segment, local := erabasic.ContinuationDiagnosticSegment(offset, segments)
		if segment < 0 {
			continue
		}
		row := segments[segment].Line
		physical := segments[segment].SourceStart + local
		if physical > segments[segment].SourceEnd {
			physical = segments[segment].SourceEnd
		}
		if diagnostic.Code == "resource_limit" && diagnostic.Severity == Error {
			p.resourceProblem(row, physical, diagnostic.Message)
			return
		}
		p.diagnostic(row, physical, erabasic.Severity(diagnostic.Severity), diagnostic.Code, diagnostic.Message)
	}
}
