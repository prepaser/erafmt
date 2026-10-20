package format

import (
	"bytes"
	"cmp"
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	maxStructureDepth       = 512
	maxPreprocessorDepth    = 512
	maxSyntaxBytes          = 1 << 20
	maxFormattedOutputBytes = 64 << 20
	maxContinuationSegments = maxFormattedOutputBytes / 64
)

type ppFrame struct {
	stateUnits int
	kind       string
	line       int
	entry      state
	branches   []state
	hasElse    bool
	malformed  bool
}

type formatter struct {
	functionContext
	parseLine             int
	parseBase             int
	parseSegments         []continuationSegment
	metadata              *metadataBudget
	convergenceWork       int
	ppStateUnits          int
	src                   []byte
	lines                 []line
	kind                  Kind
	state                 state
	pp                    []ppFrame
	edits                 []edit
	diagnostics           []Diagnostic
	base                  int
	functionEdit          int
	statementEdit         int
	statementDiagnostic   int
	preserved             bool
	preserveFile          bool
	optional              bool
	rules                 ruleSet
	opaque                []bool
	headers               []int
	codeEnds              map[int]int
	bodyStarts            []bool
	deleted               []RuleID
	ppLines               map[int]int
	eol                   EOL
	dialect               Dialect
	allowFullwidthSpace   bool
	continuationSeparator *string
	pendingDeclaration    string
	pendingLabel          string
	inContinuationProxy   bool
	syntaxSession         *expressionSession
	syntaxBase            int
	columns               map[int][]columnPoint
	statementOrigins      map[int]sourcePosition
	aborted               bool
	editBytes             uint64
	editBytesEdits        int
}

func Format(src []byte, kind Kind) ([]byte, []Diagnostic) {
	r := FormatWithOptions(src, kind, Options{})
	return r.Output, r.Diagnostics
}

func FormatWithOptions(src []byte, kind Kind, options Options) Result {
	r := Result{Output: src}
	if !utf8.Valid(src) || bytes.HasPrefix(src, []byte{0xef, 0xbb, 0xbf}) {
		r.Diagnostics = []Diagnostic{{Line: 1, Column: 1, Severity: Error, Code: "invalid_input", Message: "formatter requires UTF-8 without a BOM"}}
		return r
	}
	if kind != ERB && kind != ERH {
		r.Diagnostics = []Diagnostic{{Line: 1, Column: 1, Severity: Error, Code: "invalid_kind", Message: "unknown source kind"}}
		return r
	}
	if options.EOL > CRLF {
		r.Diagnostics = []Diagnostic{{Line: 1, Column: 1, Severity: Error, Code: "invalid_eol", Message: "unknown line ending style"}}
		return r
	}
	if options.Dialect > EMEE {
		r.Diagnostics = []Diagnostic{{Line: 1, Column: 1, Severity: Error, Code: "invalid_dialect", Message: "unknown source dialect"}}
		return r
	}
	if sep := options.ContinuationSeparator; sep != nil && (!utf8.ValidString(*sep) || strings.ContainsAny(*sep, "\r\n\x00")) {
		r.Diagnostics = []Diagnostic{{Line: 1, Column: 1, Severity: Error, Code: "invalid_continuation_separator", Message: "continuation separator must be valid UTF-8 without CR, LF, or NUL"}}
		return r
	}
	if sep := options.ContinuationSeparator; sep != nil && len(*sep) > maxFormattedOutputBytes {
		r.Diagnostics = []Diagnostic{{Line: 1, Column: 1, Severity: Error, Code: "resource_limit", Message: "continuation separator exceeds the formatter resource limit"}}
		return r
	}
	rules, err := resolveRules(options)
	if err != nil {
		r.Diagnostics = []Diagnostic{{Line: 1, Column: 1, Severity: Error, Code: "invalid_rules", Message: err.Error()}}
		return r
	}
	count, fits := sourceLineCount(src, maxMetadataBytes/lineMetadataBytes)
	if !fits {
		r.Diagnostics = []Diagnostic{{Line: count + 1, Column: 1, Severity: Error, Code: "resource_limit", Message: "source line metadata exceeds the formatter resource limit"}}
		return r
	}
	ls := makeLinesWithSpace(src, options.AllowFullwidthSpace, count)
	p := formatter{metadata: &metadataBudget{used: count * lineMetadataBytes}, src: src, lines: ls, kind: kind, rules: rules, opaque: make([]bool, len(ls)), bodyStarts: make([]bool, len(ls)), codeEnds: make(map[int]int), ppLines: make(map[int]int), eol: options.EOL, dialect: options.Dialect, allowFullwidthSpace: options.AllowFullwidthSpace, continuationSeparator: options.ContinuationSeparator, functionContext: functionContext{privateDeclarations: make(map[string]bool)}}
	p.run()
	r.Diagnostics = p.diagnostics
	for _, d := range p.diagnostics {
		if d.Severity == Error {
			return r
		}
	}
	if p.preserveFile {
		p.edits = nil
		p.editBytes = 0
		p.editBytesEdits = 0
	}
	p.layout()
	r.Diagnostics = p.diagnostics
	for _, d := range p.diagnostics {
		if d.Severity == Error {
			return r
		}
	}
	sortEdits(p.edits)
	if !validSortedEdits(p.edits, len(src)) {
		r.Diagnostics = append(r.Diagnostics, Diagnostic{Line: 1, Column: 1, Severity: Error, Code: "invalid_edits", Message: "conflicting formatting spans"})
		return r
	}
	size := len(src)
	if len(p.edits) != 0 {
		var ok bool
		size, ok = projectedOutputSizeSorted(src, p.edits)
		if !ok || size > maxFormattedOutputBytes {
			p.resourceProblem(0, 0, "formatted output exceeds the formatter resource limit")
			r.Diagnostics = p.diagnostics
			return r
		}
	}
	r.Output = applySortedEdits(src, p.edits, size)
	if len(p.edits) == 0 || bytes.Equal(r.Output, src) {
		return r
	}
	r.Changes = make([]Change, len(p.edits))
	for i, e := range p.edits {
		r.Changes[i] = e.change
	}
	slices.SortFunc(r.Changes, func(a, b Change) int {
		if n := cmp.Compare(a.Line, b.Line); n != 0 {
			return n
		}
		if n := cmp.Compare(a.Column, b.Column); n != 0 {
			return n
		}
		return cmp.Compare(a.Rule, b.Rule)
	})
	return r
}

func (p *formatter) run() {
	for i := 0; i < len(p.lines); i++ {
		if p.aborted {
			return
		}
		p.syntaxSession = nil
		p.statementEdit = len(p.edits)
		p.statementDiagnostic = len(p.diagnostics)
		l := p.lines[i]
		body := p.src[l.start:l.bodyEnd]
		head, prefix, optional, marked := p.headOf(body)
		p.parseLine = i
		p.parseBase = l.start + prefix
		p.parseSegments = nil
		p.optional = optional
		if len(head) == 0 {
			if p.skipped() {
				p.opaque[i] = true
			}
			continue
		}
		if p.kind == ERH && head[0] == '[' && !bytes.HasPrefix(head, []byte("[[")) {
			p.preserveFile = true
			p.problem(i, l.start+prefix, "unsupported_structure", "ERH preprocessor directives are unsupported", Unsupported)
			return
		}
		if head[0] == '[' && !bytes.HasPrefix(head, []byte("[[")) {
			p.labelsUnknown = true
			if bytes.Contains(head, []byte("[[")) {
				p.preserveFile = true
				p.problem(i, l.start+prefix, "unsupported_structure", "preprocessor structure depends on aliases", Unsupported)
				return
			}
			if optional {
				p.preserveFile = true
				p.problem(i, l.start+prefix, "unsupported_structure", "conditional preprocessor marker", Unsupported)
				return
			}
			wasSkipped := p.skipped()
			p.directive(i, head)
			end := bytes.IndexByte(head, ']')
			var first []byte
			fieldCount := 0
			if end >= 0 {
				first, _, fieldCount = preprocessorFields(head, end, p.allowFullwidthSpace)
			}
			if !wasSkipped || fieldCount != 0 && bytes.Equal(first, []byte("SKIPEND")) {
				if end >= 0 {
					p.ppLines[i] = l.start + prefix + end
				}
			} else {
				p.opaque[i] = true
			}
			continue
		}
		if p.skipped() {
			p.opaque[i] = true
			continue
		}
		if head[0] == ';' {
			if bytes.HasPrefix(head[1:], []byte("[[")) || bytes.HasPrefix(head[1:], []byte("![[")) || bytes.HasPrefix(head[1:], []byte("#[[")) || bytes.HasPrefix(head[1:], []byte("^[[")) {
				p.problem(i, l.start+prefix, "unsupported_structure", "comment identity depends on aliases", Unsupported)
				continue
			}
			if !p.preserved {
				p.indent(i, p.base+p.state.depth(), CommentIndent)
			}
			continue
		}
		first, last := i, i
		continued := false
		var input *continuationInput
		if bytes.Equal(bytes.TrimRightFunc(trimInput(body, p.allowFullwidthSpace), unicode.IsSpace), []byte("{")) {
			continued = true
			var ok bool
			input, ok = p.collectContinuation(first)
			if !ok {
				return
			}
			last = input.last
			for inner := first + 1; inner < last; inner++ {
				p.opaque[inner] = true
			}
			head, _, optional, marked = p.headOf(input.logical)
			p.parseBase = len(input.logical) - len(head)
			p.optional = optional
			i = last
			prefix = 0
			if p.continuationSeparator == nil && !p.preserved && !p.preserveFile {
				if unknown, preserveFile := p.continuationNilDispatchUnknown(input); unknown {
					p.problem(first, l.start, "unsupported_structure", "continued dispatch depends on the separator", Unsupported)
					if preserveFile {
						p.preserveFile = true
						return
					}
					continue
				}
			}
			if p.continuationSeparator == nil && len(head) > 0 && head[0] == '@' && !bytes.Contains(head, []byte("[[")) && !p.continuationNilHeaderEquivalent(input) {
				if p.aborted {
					return
				}
				p.preserveFile = true
				p.problem(first, l.start, "unsupported_structure", "continued function header depends on the separator", Unsupported)
				return
			}
			if len(head) == 0 || head[0] == ';' {
				if !p.preserved {
					p.indentSpan(first, last, p.base+p.state.depth(), ContinuationIndent)
				}
				p.continuationSyntax(input)
				continue
			}
			if head[0] == '[' && !bytes.HasPrefix(head, []byte("[[")) {
				if p.continuationSeparator == nil && !p.continuationNilPreprocessorEquivalent(input) {
					p.preserveFile = true
					p.problem(first, l.start, "unsupported_structure", "continued preprocessor directive depends on the separator", Unsupported)
					return
				}
				if !p.recordContinuationOrigin(input) {
					return
				}
				if p.kind == ERH {
					p.preserveFile = true
					p.problem(first, l.indentEnd, "unsupported_structure", "ERH preprocessor directives are unsupported", Unsupported)
					return
				}
				if bytes.Contains(head, []byte("[[")) {
					p.preserveFile = true
					p.problem(first, l.indentEnd, "unsupported_structure", "preprocessor structure depends on aliases", Unsupported)
					return
				}
				if optional {
					p.preserveFile = true
					p.problem(first, l.indentEnd, "unsupported_structure", "conditional preprocessor marker", Unsupported)
					return
				}
				p.directive(first, head)
				p.continuationSyntax(input)
				continue
			}
		}
		if bytes.HasPrefix(head, []byte("　")) || bytes.HasPrefix(head, []byte("[[")) {
			p.problem(first, l.start+prefix, "unsupported_structure", "source structure depends on runtime configuration or aliases", Unsupported)
			continue
		}
		if continued && (p.kind == ERH || head[0] == '@' || head[0] == '#' || head[0] == '$' || head[0] == '{' || head[0] == '}') && !p.recordContinuationOrigin(input) {
			return
		}
		statement := scanStatement{first: first, last: last, prefix: prefix, head: head, optional: optional, marked: marked, continuation: input}
		if head[0] == '@' {
			if !p.scanFunction(statement) {
				return
			}
			continue
		}
		if p.preserved || p.preserveFile {
			continue
		}
		if head[0] == '#' {
			p.scanDeclaration(statement)
			continue
		}
		if p.kind == ERH {
			p.structuralProblem(first, "ERH statements must start with #")
			continue
		}
		if head[0] == '$' {
			p.scanLabel(statement)
			continue
		}
		if head[0] == '{' || head[0] == '}' {
			p.structuralProblem(first, "continuation delimiter must occupy its own line")
			continue
		}
		next, ok := p.scanCommand(statement)
		if !ok {
			return
		}
		i = next
	}
	if len(p.pp) > 0 {
		p.optional = false
		p.unsupportedPreprocessor(p.pp[0].line-1, "invalid_preprocessor", "unterminated preprocessor region")
	}
	p.optional = false
	p.finishFunction(len(p.lines))
	p.protectFunction(len(p.lines))
}

func erhUnsupportedDeclaration(name string) bool {
	switch name {
	case "DEFINE", "DIM", "DIMS":
		return false
	default:
		return true
	}
}

func erhUnsupportedDeclarationLine(head []byte, allowFullwidthSpace bool) bool {
	name := declarationName(head)
	if erhUnsupportedDeclaration(name) {
		return true
	}
	if name != "DIM" && name != "DIMS" {
		return false
	}
	return declarationHasRefModifier(head, declarationWordEnd(head), len(head), allowFullwidthSpace)
}

func (p *formatter) knownPayloadBody(command string) bool {
	if commandUnavailable(command, p.dialect) {
		return false
	}
	if spec, ok := registeredCommandSpecs[command]; ok {
		return registeredDialect(spec.dialects, p.dialect) && (spec.mode == registeredRaw || spec.mode == registeredForm || spec.mode == registeredPrintV || spec.mode == registeredFormAny || spec.mode == registeredInputS)
	}
	return rawCommand(command) || printVCommand(command) || printFormCommand(command) || callCommand(command)
}

func (p *formatter) protectFunction(next int) {
	if p.preserved {
		for i := p.functionStart; i < next; i++ {
			p.opaque[i] = true
		}
	}
}

func (p *formatter) indentSpan(first, last, depth int, rule RuleID) {
	if last != first {
		rule = ContinuationIndent
	}
	p.indent(first, depth, rule)
	if last != first {
		p.indent(last, depth, rule)
	}
}

func (p *formatter) indent(i, depth int, rule RuleID) {
	if !p.rules[rule] {
		return
	}
	l := p.lines[i]
	if depth < 0 || depth > maxStructureDepth {
		p.resourceProblem(i, l.start, "structural indentation exceeds the formatter resource limit")
		return
	}
	if depth == 0 && bytes.HasPrefix(p.src[l.indentEnd:l.bodyEnd], []byte("\ufeff")) {
		if i == 0 {
			return
		}
		if p.rules[FileBoundaryBlankLines] {
			row := i - 1
			for row >= 0 && p.ordinaryBlank(row) {
				row--
			}
			if row < 0 {
				return
			}
		}
	}
	indentWidth := depth * 4
	if l.indentEnd-l.start == indentWidth && allSpaces(p.src[l.start:l.indentEnd]) {
		return
	}
	if !p.reserveReplacementBytes(l.start, l.indentEnd, indentWidth) {
		return
	}
	text := bytes.Repeat([]byte{' '}, indentWidth)
	p.addEditUnchecked(i, l.start, l.indentEnd, text, rule)
}

func (p *formatter) statementResourceProblem(i, offset int, message string) {
	i, offset = p.statementOrigin(i, offset)
	p.resourceProblem(i, offset, message)
}

func (p *formatter) resourceProblem(i, offset int, message string) {
	if p.aborted {
		return
	}
	p.aborted = true
	if len(p.lines) == 0 {
		p.diagnostics = append(p.diagnostics, Diagnostic{Line: 1, Column: 1, Severity: Error, Code: "resource_limit", Message: message})
		return
	}
	if i < 0 || i >= len(p.lines) {
		i = 0
	}
	if offset < p.lines[i].start || offset > p.lines[i].end {
		offset = p.lines[i].start
	}
	p.diagnostics = append(p.diagnostics, Diagnostic{Line: i + 1, Column: utf8.RuneCount(p.src[p.lines[i].start:offset]) + 1, Severity: Error, Code: "resource_limit", Message: message})
}

func (p *formatter) structureLimitExceeded(command string) bool {
	depth := p.state.depth()
	if depth > maxStructureDepth {
		return true
	}
	if kind := openers[command]; kind != "" {
		return depth+1 > maxStructureDepth
	}
	if (command == "CASE" || command == "CASEELSE") && len(p.state.frames) != 0 {
		top := p.state.frames[len(p.state.frames)-1]
		if top.kind == "SELECTCASE" && !top.branch {
			return depth+1 > maxStructureDepth
		}
	}
	return false
}

func appendWithinLimit(current, added, limit int) bool {
	return current >= 0 && added >= 0 && current <= limit && added <= limit-current
}

func continuationContentLimitExceeded(rows int) bool {
	return rows >= maxContinuationSegments
}

func (p *formatter) problem(i, offset int, code, message string, severity Severity) {
	if severity == Unsupported {
		if p.preserved {
			return
		}
		p.truncateEdits(p.functionEdit)
		p.preserved = true
	}
	p.diagnostic(i, offset, severity, code, message)
}

func (p *formatter) warning(i, offset int, code, message string) {
	p.diagnostic(i, offset, Warning, code, message)
}

func (p *formatter) structuralProblem(i int, message string) {
	severity := Error
	if p.conditional() || p.optional {
		severity = Unsupported
	}
	p.problem(i, p.lines[i].indentEnd, "invalid_structure", message, severity)
}

func (p *formatter) unsupportedPreprocessor(i int, code, message string) {
	p.preserveFile = true
	p.problem(i, p.lines[i].indentEnd, code, message, Unsupported)
}

func (p *formatter) finishFunction(next int) {
	if p.preserved {
		return
	}
	if p.state.sif != 0 {
		line := p.state.sifOrigin - 1
		if line < 0 || line >= len(p.lines) {
			line = next - 1
			if line < 0 {
				line = 0
			}
			if line >= len(p.lines) {
				line = len(p.lines) - 1
			}
		}
		p.structuralProblem(line, "SIF has no following instruction")
	}
	if len(p.state.frames) > 0 {
		f := p.state.frames[len(p.state.frames)-1]
		p.structuralProblem(f.line-1, fmt.Sprintf("unclosed %s before line %d", f.kind, next+1))
	}
}

func (p *formatter) conditional() bool {
	for _, f := range p.pp {
		if f.kind == "IF" {
			return true
		}
	}
	return false
}

func (p *formatter) skipped() bool {
	for _, f := range p.pp {
		if f.kind == "SKIP" {
			return true
		}
	}
	return false
}

func (p *formatter) directive(i int, head []byte) {
	end := bytes.IndexByte(head, ']')
	if end < 0 {
		p.unsupportedPreprocessor(i, "invalid_preprocessor", "unterminated preprocessor directive")
		return
	}
	first, second, fieldCount := preprocessorFields(head, end, p.allowFullwidthSpace)
	if fieldCount == 0 {
		p.unsupportedPreprocessor(i, "invalid_preprocessor", "empty preprocessor directive")
		return
	}
	name := string(first)
	expectedFields := 1
	if name == "IF" || name == "ELSEIF" {
		expectedFields = 2
	}
	conditionValid := true
	if name == "IF" || name == "ELSEIF" {
		conditionValid = fieldCount == expectedFields && preprocessorConditionToken(second)
	}
	if fieldCount != expectedFields {
		p.preserveFile = true
		p.problem(i, p.lines[i].indentEnd, "unsupported_structure", "preprocessor arguments cannot be resolved", Unsupported)
		if name != "IF" && name != "ELSEIF" {
			return
		}
	}
	if (name == "IF" || name == "ELSEIF") && !conditionValid && fieldCount == expectedFields {
		p.preserveFile = true
		p.problem(i, p.lines[i].indentEnd, "unsupported_structure", "preprocessor arguments cannot be resolved", Unsupported)
	}
	tailStart := trimInputLeft(head, end+1, len(head), p.allowFullwidthSpace)
	tail := head[tailStart:]
	if len(tail) != 0 && (tail[0] != ';' || executableMarker(tail, 0) || bytes.Contains(tail, []byte("[["))) {
		p.preserveFile = true
		p.problem(i, p.lines[i].indentEnd, "unsupported_structure", "preprocessor directive has a non-comment suffix", Unsupported)
	}
	switch name {
	case "SKIPSTART":
		if len(p.pp) >= maxPreprocessorDepth {
			p.statementResourceProblem(i, p.lines[i].indentEnd, "preprocessor nesting exceeds the formatter resource limit")
			return
		}
		if p.skipped() {
			p.unsupportedPreprocessor(i, "invalid_structure", "nested SKIPSTART")
			return
		}
		p.pp = append(p.pp, ppFrame{kind: "SKIP", line: i + 1})
	case "IF", "IF_DEBUG", "IF_NDEBUG":
		if len(p.pp) >= maxPreprocessorDepth {
			p.statementResourceProblem(i, p.lines[i].indentEnd, "preprocessor nesting exceeds the formatter resource limit")
			return
		}
		f := ppFrame{kind: "IF", line: i + 1, malformed: name == "IF" && !conditionValid}
		if !p.retainPreprocessorState(i, &f, p.state) {
			return
		}
		f.entry = p.state.clone()
		p.pp = append(p.pp, f)
	case "ELSE", "ELSEIF":
		if len(p.pp) == 0 || p.pp[len(p.pp)-1].kind != "IF" {
			p.unsupportedPreprocessor(i, "invalid_structure", name+" has no matching preprocessor IF")
			return
		}
		f := &p.pp[len(p.pp)-1]
		if f.hasElse {
			p.unsupportedPreprocessor(i, "invalid_structure", name+" follows preprocessor ELSE")
			return
		}
		if !p.retainPreprocessorState(i, f, p.state) {
			return
		}
		f.branches = append(f.branches, p.state.clone())
		f.hasElse = name == "ELSE"
		f.malformed = f.malformed || name == "ELSEIF" && !conditionValid
		p.state = f.entry.clone()
	case "SKIPEND", "ENDIF":
		expected := "IF"
		if name == "SKIPEND" {
			expected = "SKIP"
		}
		if len(p.pp) == 0 || p.pp[len(p.pp)-1].kind != expected {
			p.unsupportedPreprocessor(i, "invalid_structure", name+" has no matching preprocessor opener")
			return
		}
		f := p.pp[len(p.pp)-1]
		if f.kind == "IF" && !p.retainPreprocessorState(i, &f, p.state) {
			return
		}
		p.ppStateUnits -= f.stateUnits
		p.pp = p.pp[:len(p.pp)-1]
		if f.kind == "SKIP" || p.skipped() {
			return
		}
		if f.malformed {
			p.state = f.entry
			return
		}
		f.branches = append(f.branches, p.state.clone())
		if !f.hasElse {
			f.branches = append(f.branches, f.entry)
		}
		for _, s := range f.branches {
			if !s.equal(f.branches[0]) {
				if p.branchesConverge(i, f.branches) {
					break
				}
				p.preserveFile = true
				p.problem(i, p.lines[i].indentEnd, "unsupported_structure", "preprocessor branches leave different block or SIF states", Unsupported)
				p.state = f.entry
				return
			}
		}
		p.state = mergeBranchState(f.branches)
		if p.state.sif != 0 && !p.state.sifOriginKnown {
			p.state.sifOrigin = i + 1
		}
		for _, branch := range f.branches[1:] {
			p.state.declarations = p.state.declarations && branch.declarations
		}
	default:
		p.preserveFile = p.preserveFile || p.base == 0 || len(p.pp) != 0
		p.problem(i, p.lines[i].indentEnd, "unsupported_structure", "unknown preprocessor directive "+name, Unsupported)
	}
}

func preprocessorFields(head []byte, end int, allowFullwidthSpace bool) (first, second []byte, count int) {
	start := trimInputLeft(head, 1, end, allowFullwidthSpace)
	if start >= end {
		return nil, nil, 0
	}
	pos := start
	for pos < end && inputSpaceSize(head, pos, allowFullwidthSpace) == 0 {
		pos++
	}
	first = head[start:pos]
	count = 1

	start = trimInputLeft(head, pos, end, allowFullwidthSpace)
	if start >= end {
		return first, nil, count
	}
	pos = start
	for pos < end && inputSpaceSize(head, pos, allowFullwidthSpace) == 0 {
		pos++
	}
	second = head[start:pos]
	count = 2

	if trimInputLeft(head, pos, end, allowFullwidthSpace) < end {
		count = 3
	}
	return first, second, count
}

func preprocessorConditionToken(field []byte) bool {
	if len(field) == 0 {
		return false
	}
	for len(field) > 0 {
		r, size := utf8.DecodeRune(field)
		if r == '\u3000' || isDelimiter(r) {
			return false
		}
		field = field[size:]
	}
	return true
}

func (p *formatter) branchesCloseTogether(i int, branches []state) bool {
	if i+1 >= len(p.lines) {
		return false
	}
	l := p.lines[i+1]
	head, _, _, marked := p.headOf(p.src[l.start:l.bodyEnd])
	command, wordEnd := commandOfWithSpace(head, p.allowFullwidthSpace)
	if marked || bytes.Contains(head, []byte("[[")) || string(head[:wordEnd]) != command || command != "ENDIF" && command != "ENDSELECT" {
		return false
	}
	return closedBranchesMatch(command, i+2, branches)
}
