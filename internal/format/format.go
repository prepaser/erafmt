package format

import (
	"bytes"
	"cmp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/prepaser/erafmt/internal/erabasic"
)

const maxFormattedOutputBytes = 64 << 20

type formatter struct {
	erabasic.DocumentSession
	edits               []edit
	diagnostics         []Diagnostic
	functionEdit        int
	statementEdit       int
	statementDiagnostic int
	rules               ruleSet
	deleted             []RuleID
	eol                 EOL
	editBytes           uint64
	editBytesEdits      int
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
	count, fits := erabasic.SourceLineCount(src, erabasic.MaxMetadataBytes/erabasic.LineMetadataBytes)
	if !fits {
		r.Diagnostics = []Diagnostic{{Line: count + 1, Column: 1, Severity: Error, Code: "resource_limit", Message: "source line metadata exceeds the formatter resource limit"}}
		return r
	}
	ls := erabasic.MakeLinesWithSpace(src, options.AllowFullwidthSpace, count)
	p := formatter{
		DocumentSession: erabasic.DocumentSession{
			Metadata:              erabasic.NewMetadataBudget(count * erabasic.LineMetadataBytes),
			Src:                   src,
			Lines:                 ls,
			Kind:                  erabasic.Kind(kind),
			Opaque:                make([]bool, len(ls)),
			BodyStarts:            make([]bool, len(ls)),
			CodeEnds:              make(map[int]int),
			PpLines:               make(map[int]int),
			Dialect:               erabasic.Dialect(options.Dialect),
			AllowFullwidthSpace:   options.AllowFullwidthSpace,
			ContinuationSeparator: options.ContinuationSeparator,
			FunctionContext:       erabasic.FunctionContext{PrivateDeclarations: make(map[string]bool)},
		},
		rules: rules,
		eol:   options.EOL,
	}
	p.run()
	r.Diagnostics = p.diagnostics
	for _, d := range p.diagnostics {
		if d.Severity == Error {
			return r
		}
	}
	if p.PreserveFile {
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
	for i := 0; i < len(p.Lines); i++ {
		if p.Aborted {
			return
		}
		p.SyntaxSession = nil
		p.statementEdit = len(p.edits)
		p.statementDiagnostic = len(p.diagnostics)
		l := p.Lines[i]
		body := p.Src[l.Start:l.BodyEnd]
		head, prefix, optional, marked := p.headOf(body)
		p.ParseLine = i
		p.ParseBase = l.Start + prefix
		p.ParseSegments = nil
		p.Optional = optional
		if erabasic.ClassifySourceHead(head) == erabasic.SourceHeadEmpty {
			if p.skipped() {
				p.Opaque[i] = true
			}
			continue
		}
		if p.Kind == erabasic.ERH && erabasic.ClassifySourceHead(head) == erabasic.SourceHeadDirective {
			p.PreserveFile = true
			p.problem(i, l.Start+prefix, "unsupported_structure", "ERH preprocessor directives are unsupported", erabasic.Unsupported)
			return
		}
		if erabasic.ClassifySourceHead(head) == erabasic.SourceHeadDirective {
			p.LabelsUnknown = true
			if bytes.Contains(head, []byte("[[")) {
				p.PreserveFile = true
				p.problem(i, l.Start+prefix, "unsupported_structure", "preprocessor structure depends on aliases", erabasic.Unsupported)
				return
			}
			if optional {
				p.PreserveFile = true
				p.problem(i, l.Start+prefix, "unsupported_structure", "conditional preprocessor marker", erabasic.Unsupported)
				return
			}
			wasSkipped := p.skipped()
			p.directive(i, head)
			end := bytes.IndexByte(head, ']')
			var first []byte
			fieldCount := 0
			if end >= 0 {
				first, _, fieldCount = erabasic.PreprocessorFields(head, end, p.AllowFullwidthSpace)
			}
			if !wasSkipped || fieldCount != 0 && bytes.Equal(first, []byte("SKIPEND")) {
				if end >= 0 {
					p.PpLines[i] = l.Start + prefix + end
				}
			} else {
				p.Opaque[i] = true
			}
			continue
		}
		if p.skipped() {
			p.Opaque[i] = true
			continue
		}
		if erabasic.ClassifySourceHead(head) == erabasic.SourceHeadComment {
			if erabasic.CommentIdentityUnknown(head) {
				p.problem(i, l.Start+prefix, "unsupported_structure", "comment identity depends on aliases", erabasic.Unsupported)
				continue
			}
			if !p.Preserved {
				p.indent(i, p.Base+p.State.Depth(), CommentIndent)
			}
			continue
		}
		first, last := i, i
		continued := false
		var input *erabasic.ContinuationInput
		if erabasic.ContinuationOpening(body, p.AllowFullwidthSpace) {
			continued = true
			var ok bool
			input, ok = p.collectContinuation(first)
			if !ok {
				return
			}
			last = input.Last
			for inner := first + 1; inner < last; inner++ {
				p.Opaque[inner] = true
			}
			head, _, optional, marked = p.headOf(input.Logical)
			p.ParseBase = len(input.Logical) - len(head)
			p.Optional = optional
			i = last
			prefix = 0
			if p.ContinuationSeparator == nil && !p.Preserved && !p.PreserveFile {
				if unknown, preserveFile := p.continuationNilDispatchUnknown(input); unknown {
					p.problem(first, l.Start, "unsupported_structure", "continued dispatch depends on the separator", erabasic.Unsupported)
					if preserveFile {
						p.PreserveFile = true
						return
					}
					continue
				}
			}
			if p.ContinuationSeparator == nil && len(head) > 0 && head[0] == '@' && !bytes.Contains(head, []byte("[[")) && !p.continuationNilHeaderEquivalent(input) {
				if p.Aborted {
					return
				}
				p.PreserveFile = true
				p.problem(first, l.Start, "unsupported_structure", "continued function header depends on the separator", erabasic.Unsupported)
				return
			}
			if len(head) == 0 || erabasic.ClassifySourceHead(head) == erabasic.SourceHeadComment {
				if !p.Preserved {
					p.indentSpan(first, last, p.Base+p.State.Depth(), ContinuationIndent)
				}
				p.continuationSyntax(input)
				continue
			}
			if erabasic.ClassifySourceHead(head) == erabasic.SourceHeadDirective {
				if p.ContinuationSeparator == nil && !p.continuationNilPreprocessorEquivalent(input) {
					p.PreserveFile = true
					p.problem(first, l.Start, "unsupported_structure", "continued preprocessor directive depends on the separator", erabasic.Unsupported)
					return
				}
				if !p.recordContinuationOrigin(input) {
					return
				}
				if p.Kind == erabasic.ERH {
					p.PreserveFile = true
					p.problem(first, l.IndentEnd, "unsupported_structure", "ERH preprocessor directives are unsupported", erabasic.Unsupported)
					return
				}
				if bytes.Contains(head, []byte("[[")) {
					p.PreserveFile = true
					p.problem(first, l.IndentEnd, "unsupported_structure", "preprocessor structure depends on aliases", erabasic.Unsupported)
					return
				}
				if optional {
					p.PreserveFile = true
					p.problem(first, l.IndentEnd, "unsupported_structure", "conditional preprocessor marker", erabasic.Unsupported)
					return
				}
				p.directive(first, head)
				p.continuationSyntax(input)
				continue
			}
		}
		if erabasic.SourceStructureUnknown(head) {
			p.problem(first, l.Start+prefix, "unsupported_structure", "source structure depends on runtime configuration or aliases", erabasic.Unsupported)
			continue
		}
		if continued && (p.Kind == erabasic.ERH || head[0] == '@' || head[0] == '#' || head[0] == '$' || head[0] == '{' || head[0] == '}') && !p.recordContinuationOrigin(input) {
			return
		}
		statement := erabasic.ScanStatement{First: first, Last: last, Prefix: prefix, Head: head, Optional: optional, Marked: marked, Continuation: input}
		if erabasic.ClassifySourceHead(head) == erabasic.SourceHeadFunction {
			if !p.scanFunction(statement) {
				return
			}
			continue
		}
		if p.Preserved || p.PreserveFile {
			continue
		}
		if erabasic.ClassifySourceHead(head) == erabasic.SourceHeadDeclaration {
			p.scanDeclaration(statement)
			continue
		}
		if p.Kind == erabasic.ERH {
			p.structuralProblem(first, "ERH statements must start with #")
			continue
		}
		if erabasic.ClassifySourceHead(head) == erabasic.SourceHeadLabel {
			p.scanLabel(statement)
			continue
		}
		if erabasic.ClassifySourceHead(head) == erabasic.SourceHeadDelimiter {
			p.structuralProblem(first, "continuation delimiter must occupy its own line")
			continue
		}
		next, ok := p.scanCommand(statement)
		if !ok {
			return
		}
		i = next
	}
	if len(p.Pp) > 0 {
		p.Optional = false
		p.unsupportedPreprocessor(p.Pp[0].Line-1, "invalid_preprocessor", "unterminated preprocessor region")
	}
	p.Optional = false
	p.finishFunction(len(p.Lines))
	p.protectFunction(len(p.Lines))
}

func (p *formatter) knownPayloadBody(command string) bool {
	return p.DocumentSession.PayloadBodyKnown(command)
}

func (p *formatter) protectFunction(next int) {
	p.DocumentSession.ProtectFunctionSource(next)
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
	l := p.Lines[i]
	if depth < 0 || depth > erabasic.MaxStructureDepth {
		p.resourceProblem(i, l.Start, "structural indentation exceeds the formatter resource limit")
		return
	}
	if depth == 0 && bytes.HasPrefix(p.Src[l.IndentEnd:l.BodyEnd], []byte("\ufeff")) {
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
	if l.IndentEnd-l.Start == indentWidth && allSpaces(p.Src[l.Start:l.IndentEnd]) {
		return
	}
	if !p.reserveReplacementBytes(l.Start, l.IndentEnd, indentWidth) {
		return
	}
	text := bytes.Repeat([]byte{' '}, indentWidth)
	p.addEditUnchecked(i, l.Start, l.IndentEnd, text, rule)
}

func (p *formatter) statementResourceProblem(i, offset int, message string) {
	i, offset = p.statementOrigin(i, offset)
	p.resourceProblem(i, offset, message)
}

func (p *formatter) resourceProblem(i, offset int, message string) {
	if p.Aborted {
		return
	}
	p.Aborted = true
	position := p.DocumentSession.ResourceLocation(i, offset)
	p.diagnostics = append(p.diagnostics, Diagnostic{Line: position.Line, Column: position.Column, Severity: Error, Code: "resource_limit", Message: message})
}

func (p *formatter) structureLimitExceeded(command string) bool {
	return p.DocumentSession.SourceStructureLimitExceeded(command)
}

func (p *formatter) problem(i, offset int, code, message string, severity erabasic.Severity) {
	if severity == erabasic.Unsupported {
		if p.Preserved {
			return
		}
		p.truncateEdits(p.functionEdit)
		p.Preserved = true
	}
	p.diagnostic(i, offset, severity, code, message)
}

func (p *formatter) warning(i, offset int, code, message string) {
	p.diagnostic(i, offset, erabasic.Warning, code, message)
}

func (p *formatter) structuralProblem(i int, message string) {
	severity := erabasic.Error
	if p.conditional() || p.Optional {
		severity = erabasic.Unsupported
	}
	p.problem(i, p.Lines[i].IndentEnd, "invalid_structure", message, severity)
}

func (p *formatter) unsupportedPreprocessor(i int, code, message string) {
	p.PreserveFile = true
	p.problem(i, p.Lines[i].IndentEnd, code, message, erabasic.Unsupported)
}

func (p *formatter) finishFunction(next int) {
	for _, issue := range p.DocumentSession.FunctionEndIssues(next) {
		p.problem(issue.Line, issue.Offset, issue.Code, issue.Message, issue.Severity)
	}
}

func (p *formatter) conditional() bool {
	return p.preprocessorConditional()
}

func (p *formatter) skipped() bool {
	return p.preprocessorSkipped()
}

func (p *formatter) directive(i int, head []byte) {
	p.analyzePreprocessorDirective(i, head)
}
