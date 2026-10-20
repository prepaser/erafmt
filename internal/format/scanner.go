package format

import (
	"bytes"
	"unicode"
)

type scanStatement struct {
	first, last, prefix int
	head                []byte
	optional, marked    bool
	continuation        *continuationInput
}

func (p *formatter) collectContinuation(first int) (*continuationInput, bool) {
	last := first
	var joined bytes.Buffer
	closed := false
	for last = first + 1; last < len(p.lines); last++ {
		if p.aborted {
			return nil, false
		}
		inner := p.src[p.lines[last].start:p.lines[last].bodyEnd]
		trimmed := bytes.TrimLeftFunc(inner, unicode.IsSpace)
		if bytes.Equal(bytes.TrimSpace(inner), []byte("}")) {
			closed = true
			break
		}
		if continuationContentLimitExceeded(last - first - 1) {
			p.resourceProblem(first, p.lines[first].indentEnd, "continuation source exceeds the formatter resource limit")
			return nil, false
		}
		if len(trimmed) != 0 && trimmed[0] == '}' {
			p.structuralProblem(last, "characters after continuation end")
		}
		if bytes.Equal(trimmed, []byte("{")) {
			p.structuralProblem(last, "nested continuation")
		}
		if !appendWithinLimit(joined.Len(), len(inner), maxFormattedOutputBytes) {
			p.resourceProblem(first, p.lines[first].indentEnd, "continuation source exceeds the formatter resource limit")
			return nil, false
		}
		joined.Write(inner)
		p.parseSegments = append(p.parseSegments, continuationSegment{logicalStart: joined.Len() - len(inner), logicalEnd: joined.Len(), line: last, sourceStart: p.lines[last].start, sourceEnd: p.lines[last].bodyEnd, indent: p.lines[last].indentEnd - p.lines[last].start})
		if p.continuationSeparator == nil {
			if !appendWithinLimit(joined.Len(), 1, maxFormattedOutputBytes) {
				p.resourceProblem(first, p.lines[first].indentEnd, "continuation source exceeds the formatter resource limit")
				return nil, false
			}
			joined.WriteByte(' ')
		} else {
			if !appendWithinLimit(joined.Len(), len(*p.continuationSeparator), maxFormattedOutputBytes) {
				p.resourceProblem(first, p.lines[first].indentEnd, "continuation source exceeds the formatter resource limit")
				return nil, false
			}
			joined.WriteString(*p.continuationSeparator)
		}
	}
	if !closed {
		p.structuralProblem(first, "unterminated continuation")
		p.preserveFile = true
		return nil, false
	}
	return &continuationInput{first: first, last: last, logical: joined.Bytes(), segments: p.parseSegments}, true
}

func (p *formatter) scanFunction(statement scanStatement) bool {
	first, last, prefix := statement.first, statement.last, statement.prefix
	head, marked := statement.head, statement.marked
	input := statement.continuation
	continued := input != nil
	l := p.lines[first]
	if p.conditional() || p.optional || bytes.Contains(head, []byte("[[")) {
		p.preserveFile = true
		p.problem(first, l.start+prefix, "unsupported_structure", "function boundary depends on preprocessing or aliases", Unsupported)
		return false
	}
	p.syntaxSession = p.newExpressionSession(head)
	p.syntaxBase = l.start + prefix
	validation := validateHeaderWithSession(head, p.syntaxSession)
	if p.aborted {
		return false
	}
	if validation.status != headerValid {
		severity := Error
		if validation.status == headerUnsupported {
			severity = Unsupported
		}
		p.preserveFile = true
		p.problem(first, l.start+prefix, "invalid_header", validation.message, severity)
		return false
	}
	p.finishFunction(first)
	p.protectFunction(first)
	p.functionEdit = len(p.edits)
	p.functionStart = first
	p.preserved = false
	p.state = state{declarations: true}
	p.resetFunctionAttributes()
	p.functionName = functionHeaderName(head, p.allowFullwidthSpace)
	p.eventFunctionKnown, p.eventFunctionExact = eventFunctionContext(p.functionName)
	p.systemFunctionKnown = systemFunctionName(p.functionName)
	p.dollarLabels = nil
	p.labelsUnknown = false
	p.base = 1
	if p.kind == ERH {
		p.structuralProblem(first, "function definition in ERH")
		return true
	}
	p.indentSpan(first, last, 0, FunctionIndent)
	if !continued && !marked {
		p.headers = append(p.headers, first)
		if !p.headerArgumentsValid(first, head, l.start+prefix) {
			p.truncateEdits(p.functionEdit)
			p.syntaxOpaque(first)
			return true
		}
		if p.rules[LabelMarkerSpacing] && len(head) > 1 && p.inputSpaceIn(head, 1) {
			p.syntaxLabel(first, head, l.start+prefix)
		}
		p.syntax(first, head, prefix)
	} else if continued && !marked {
		if !p.continuationHeaderArgumentsValid(input) {
			p.truncateEdits(p.functionEdit)
			for row := first; row <= last; row++ {
				p.syntaxOpaque(row)
			}
			return true
		}
		if p.continuationSeparator != nil && !p.optional && !p.conditional() && !p.preserved && !p.preserveFile && !bytesHasAlias(head) && !continuationUnsafe(head) {
			p.headers = append(p.headers, first)
		}
		p.continuationSyntax(input)
	}
	return true
}

func (p *formatter) scanDeclaration(statement scanStatement) {
	first, last, prefix := statement.first, statement.last, statement.prefix
	head, optional, marked := statement.head, statement.optional, statement.marked
	input := statement.continuation
	continued := input != nil
	l := p.lines[first]
	if p.conditional() || optional || marked || bytesHasAlias(head) || continued && !p.continuationDeclarationSafe(head) {
		p.sharpStateUnknown = true
	}
	if p.kind == ERH && !bytes.Contains(head, []byte("[[")) && erhUnsupportedDeclarationLine(head, p.allowFullwidthSpace) {
		p.preserveFile = true
		p.problem(first, l.start+prefix, "unsupported_declaration", "ERH declaration is unsupported: "+declarationName(head), Unsupported)
		return
	}
	if p.kind == ERB && (p.base == 0 || !p.state.declarations) {
		p.problem(first, l.start+prefix, "unsupported_declaration", "ERB declarations must follow a function header before its body", Unsupported)
		return
	}
	p.indentSpan(first, last, p.base, DeclarationIndent)
	if !continued && !marked {
		p.auditSharpDeclaration(first, head, l.start+prefix, optional, marked, continued)
		p.syntax(first, head, prefix)
	} else if continued && !marked {
		p.continuationSyntax(input)
	}
}

func (p *formatter) scanLabel(statement scanStatement) {
	first, last, prefix := statement.first, statement.last, statement.prefix
	head, optional, marked := statement.head, statement.optional, statement.marked
	input := statement.continuation
	continued := input != nil
	l := p.lines[first]
	p.state.declarations = false
	if p.state.sif != 0 {
		p.structuralProblem(first, "SIF cannot target a label")
		return
	}
	if len(p.state.frames) > 0 {
		top := p.state.frames[len(p.state.frames)-1].kind
		if top == "DATA" || top == "DATALIST" || top == "FUNCLIST" {
			p.structuralProblem(first, "label is not allowed in "+top)
			return
		}
	}
	p.indentSpan(first, last, p.base+p.state.depth(), LabelIndent)
	if !continued {
		p.auditDollarLabel(first, head, l.start+prefix, optional || marked)
	} else if p.continuationSeparator == nil || optional || marked || p.conditional() || p.preserved || p.preserveFile || continuationUnsafe(head) {
		p.labelsUnknown = true
	}
	if !continued && !marked {
		p.syntaxLabel(first, head, l.start+prefix)
	} else if continued && !marked {
		p.continuationSyntax(input)
	}
}

func (p *formatter) scanCommand(statement scanStatement) (int, bool) {
	first, last, prefix := statement.first, statement.last, statement.prefix
	head, optional, marked := statement.head, statement.optional, statement.marked
	input := statement.continuation
	continued := input != nil
	l := p.lines[first]
	command, wordEnd := commandOfWithSpace(head, p.allowFullwidthSpace)
	if continued && p.continuationSeparator == nil && p.continuationNilCommandIdentityUnknown(input, command) {
		p.problem(first, l.start+prefix, "unsupported_structure", "continued command identity depends on the separator", Unsupported)
		return last, true
	}
	if continued && !p.recordContinuationOrigin(input) {
		return last, false
	}
	if (reservedInstructions[command] || reservedBuiltinMethod(command)) && !commandUnavailable(command, p.dialect) && string(head[:wordEnd]) != command {
		if start, _, ok := p.assignmentBoundary(head); ok {
			if _, variable := p.variableLHS(head[:start]); variable {
				p.problem(first, l.start+prefix, "unsupported_structure", "instruction identity depends on case-sensitivity configuration", Unsupported)
				return last, true
			}
		}
	}
	if command == "" {
		if end := bytes.Index(head, []byte("　")); end > 0 {
			name, wordEnd := commandOfWithSpace(head[:end], p.allowFullwidthSpace)
			if wordEnd == end && (reservedInstructions[name] || reservedBuiltinMethod(name)) {
				row, offset := first, l.start+prefix+end
				if continued {
					if position, ok := input.position(len(input.logical) - len(head) + end); ok {
						row, offset = position.line, position.offset
					}
				}
				p.problem(row, offset, "unsupported_structure", "instruction separator depends on runtime configuration", Unsupported)
				return last, true
			}
		}
		firstWord := head
		if end := bytes.IndexAny(firstWord, " \t"); end >= 0 {
			firstWord = firstWord[:end]
		}
		if bytes.Contains(firstWord, []byte("[[")) {
			p.problem(first, l.start+prefix, "unsupported_structure", "instruction identity depends on aliases", Unsupported)
			return last, true
		}
	}
	if optional && !continued && p.optionalSIFPair(first, head, prefix) {
		return last + 1, true
	}
	before := p.state
	if optional {
		before = before.clone()
	}
	if p.structureLimitExceeded(command) {
		p.statementResourceProblem(first, l.start+prefix, "structural nesting exceeds the formatter resource limit")
		return last, false
	}
	proof := continuationProof{}
	if continued {
		proof = p.proveContinuation(input)
	}
	trusted := !p.preserved && !p.preserveFile && !optional && !p.conditional() && !marked && !continued && !bytes.Contains(head, []byte("[["))
	logicalTrusted := trusted || proof.certain && !p.conditional()
	metadataTrusted := trusted || proof.certain || !optional && !marked && !continued && !bytes.Contains(head, []byte("[[")) && p.conditional()
	warn := func(code, message string) {
		if continued {
			p.continuationWarning(proof, code, message)
			return
		}
		p.warning(first, l.start+prefix, code, message)
	}
	if logicalTrusted && before.branchAfterFinal(command) {
		warn("branch_after_final", command+" follows a final branch")
	}
	if logicalTrusted && command == "REPEAT" && before.hasTrustedRepeat() {
		warn("nested_repeat", "nested REPEAT may create an infinite loop")
	}
	if logicalTrusted && command == "REPEAT" {
		if continued {
			p.auditRepeatCountContinuation(proof)
		} else {
			p.auditRepeatCount(first, head, wordEnd, l.start+prefix)
		}
	}
	emptyData := before.emptyData(command)
	caseMismatch := trusted && command == "CASE" && p.caseTypeMismatch(before, head, wordEnd, l.start+prefix)
	if command == "ENDLIST" {
		p.state.finishData(command, metadataTrusted)
	}
	depth, message := p.state.instruction(command, first+1)
	if message != "" {
		p.structuralProblem(first, message)
		return last, true
	}
	if optional && !before.equal(p.state) {
		p.problem(first, l.start+prefix, "unsupported_structure", "conditional marker changes block or SIF state", Unsupported)
		return last, true
	}
	if command == "SELECTCASE" && !optional && !marked && !continued && !bytes.Contains(head, []byte("[[")) {
		p.state.setSelectorType(p.provenSelectorType(head, wordEnd, l.start+prefix))
	}
	if caseMismatch {
		p.syntaxProblem(first, l.start+prefix+wordEnd, "CASE has an incompatible type")
		p.syntaxOpaque(first)
	}
	p.state.markDataOpener(command, metadataTrusted)
	p.state.markData(command, metadataTrusted)
	if logicalTrusted && emptyData {
		warn("empty_data", command+" closes an empty data block")
	}
	if command == "REPEAT" && len(p.state.frames) > 0 {
		repeatTrusted := logicalTrusted
		if !repeatTrusted && p.conditional() && !optional && !marked && !continued && !bytes.Contains(head, []byte("[[")) {
			repeatTrusted = p.provenRepeatPositive(head, wordEnd, l.start+prefix)
		}
		if !repeatTrusted && p.conditional() && proof.certain {
			repeatTrusted = p.provenRepeatPositiveContinuation(proof)
		}
		p.state.frames[len(p.state.frames)-1].repeatTrusted = repeatTrusted
	}
	if (command == "BREAK" || command == "CONTINUE") && !optional && !p.conditional() && !p.state.hasLoop() {
		p.warning(first, l.start+prefix, "invalid_loop_control", command+" is outside a loop")
	}
	p.state.declarations = false
	rule := InstructionIndent
	if before.sif != 0 {
		rule = SIFIndent
	}
	p.indentSpan(first, last, p.base+depth, rule)
	if !continued && !marked {
		diagnosticStart := len(p.diagnostics)
		p.syntax(first, head, prefix)
		if p.opaque[first] && command != "DATA" {
			if command == "DATAFORM" || openers[command] == "DATA" {
				p.state.markCurrentDataUncertain()
			}
		}
		if len(p.diagnostics) == diagnosticStart && !bytes.Contains(head, []byte("[[")) && p.knownPayloadBody(command) {
			p.bodyStarts[first] = true
		}
	} else if continued && !marked {
		p.continuationSyntaxWithProof(first, last, proof)
	}
	return last, true
}
