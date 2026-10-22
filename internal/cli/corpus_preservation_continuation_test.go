package cli

import (
	"bytes"
	"encoding/binary"
	"strings"

	"github.com/prepaser/erafmt/internal/format"
)

func continuationFingerprint(rows []checkedLine, separator string, dialect format.Dialect, allowFullwidth bool) []byte {
	logical := continuationLogicalSource(rows, separator)

	kind, logicalFingerprint, exact := continuationLogicalFingerprint(logical, dialect, allowFullwidth)
	fingerprintRows := rows
	if separator == "" && (!exact || kind == "void") {
		fingerprintRows = normalizeEmptyJoinTrailingCode(rows, kind, allowFullwidth)
		normalizedLogical := continuationLogicalSource(fingerprintRows, separator)
		if !bytes.Equal(normalizedLogical, logical) {
			logical = normalizedLogical
			kind, logicalFingerprint, exact = continuationLogicalFingerprint(logical, dialect, allowFullwidth)
		}
	}
	if kind == "void" && continuationVoidTrivia(logical, allowFullwidth) {
		exact = false
	}
	boundaryMarker := continuationBoundaryMarker
	if bytes.Contains(logical, []byte{boundaryMarker}) {
		if marker, ok := continuationBoundaryMarkerFor(logical); ok {
			boundaryMarker = marker
		} else {
			exact = true
		}
	}
	protectedRows := make([]bool, len(fingerprintRows))
	for i, row := range fingerprintRows {
		head := trimGrammarSpace(row.body, allowFullwidth)
		markers, _ := stripMarkers(head)
		if len(markers) != 0 || len(head) != 0 && head[0] == ';' && corpusProtectedComment(head) {
			protectedRows[i] = true
		}
	}
	boundaryFingerprint := logicalFingerprint
	if !exact {
		boundaryFingerprint = continuationBoundaryFingerprint(fingerprintRows, separator, kind, boundaryMarker, dialect, allowFullwidth)
	}
	out := appendPreservedField(nil, []byte(kind))
	out = appendPreservedField(out, logicalFingerprint)
	out = appendPreservedField(out, boundaryFingerprint)
	for i, row := range fingerprintRows {
		rowFingerprint := []byte{'i'}
		indent := append([]byte(nil), row.body[:len(row.body)-len(trimGrammarSpace(row.body, allowFullwidth))]...)
		rowFingerprint = appendPreservedField(rowFingerprint, indent)
		if exact || protectedRows[i] || kind == "unknown" {
			rowFingerprint[0] = 'e'
			rowFingerprint = appendPreservedField(rowFingerprint, row.body)
		}
		out = appendPreservedField(out, rowFingerprint)
	}
	return out
}

func continuationLogicalSource(rows []checkedLine, separator string) []byte {
	logical := make([]byte, 0)
	for _, row := range rows {
		logical = append(logical, row.body...)
		logical = append(logical, separator...)
	}
	return logical
}

func normalizeEmptyJoinTrailingCode(rows []checkedLine, kind string, allowFullwidth bool) []checkedLine {
	if kind != "print-v" && kind != "void" && kind != "expression" {
		return rows
	}
	logical := trimGrammarSpace(continuationLogicalSource(rows, ""), allowFullwidth)
	_, commandEnd := firstWord(logical)
	if commandEnd == 0 {
		return rows
	}
	tail := logical[commandEnd:]
	trivia := len(trimGrammarSpaceRight(tail, allowFullwidth)) == 0
	if kind == "print-v" && !trivia {
		trivia = printVTrailingGrammar(tail, allowFullwidth)
	}
	if kind == "void" && !trivia {
		trivia = continuationVoidTrivia(logical, allowFullwidth)
	}
	if !trivia {
		return rows
	}
	last := -1
	for i, row := range rows {
		if len(trimGrammarSpaceBoth(row.body, allowFullwidth)) != 0 {
			last = i
		}
	}
	if last < 0 {
		return rows
	}
	body := rows[last].body
	normalized := trimGrammarSpaceRight(body, allowFullwidth)
	if len(normalized) == len(body) {
		return rows
	}
	copyRows := append([]checkedLine(nil), rows...)
	copyRows[last].body = normalized
	return copyRows
}

func continuationVoidTrivia(src []byte, allowFullwidth bool) bool {
	head := trimGrammarSpace(src, allowFullwidth)
	_, end := firstWord(head)
	if end == 0 {
		return false
	}
	tail := trimGrammarSpace(head[end:], allowFullwidth)
	if len(tail) == 0 {
		return true
	}
	return tail[0] == ';' && mandatoryMarkerSpan(tail, 0) == 0 && !corpusProtectedComment(tail)
}

func trimGrammarSpaceRight(src []byte, allowFullwidth bool) []byte {
	end := len(src)
	for end > 0 {
		switch {
		case src[end-1] == ' ' || src[end-1] == '\t':
			end--
		case allowFullwidth && end >= 3 && bytes.Equal(src[end-3:end], []byte("\u3000")):
			end -= 3
		default:
			return src[:end]
		}
	}
	return src[:end]
}

func continuationBoundaryFingerprint(rows []checkedLine, separator, kind string, marker byte, dialect format.Dialect, allowFullwidth bool) []byte {
	logical := continuationLogicalSource(rows, separator)
	if fingerprint, ok := continuationBoundaryContextFingerprint(rows, separator, kind, logical, dialect, allowFullwidth); ok {
		return fingerprint
	}
	if boundary := continuationNulBoundary(logical, kind, dialect, allowFullwidth); boundary >= 0 {
		return continuationBoundaryFingerprintNul(rows, separator, kind, marker, dialect, allowFullwidth, boundary)
	}
	if fingerprint, ok := continuationArgumentBoundaryFingerprint(rows, separator, kind, dialect, allowFullwidth); ok {
		return fingerprint
	}
	if continuationKindHasForm(kind) && continuationRowsHaveNormalForm(rows, separator, dialect) {
		if kind == "form-call" {
			if fingerprint, ok := continuationFormCallBoundaryFingerprint(rows, separator, dialect, allowFullwidth); ok {
				return fingerprint
			}
		}
		return continuationFormBoundaryFingerprint(rows, separator, kind, dialect, allowFullwidth)
	}
	if kind == "input" {
		return continuationInputBoundaryFingerprint(rows, dialect, allowFullwidth)
	}
	commandRow := -1
	if allowFullwidth {
		for i, row := range rows {
			if len(trimGrammarSpace(row.body, true)) != 0 {
				commandRow = i
				break
			}
		}
	}
	out := make([]byte, 0)
	for i, row := range rows {
		head := trimGrammarSpace(row.body, allowFullwidth)
		if allowFullwidth && i == commandRow {
			head = normalizeFullwidthContinuationCommand(head, kind, dialect)
			if kind == "void" {
				head = normalizeFullwidthVoidCommand(head)
			}
		}
		if kind == "form" || kind == "form-any" {
			head = normalizeFormContinuationRow(head, allowFullwidth)
		} else if allowFullwidth && i != commandRow {
			head = normalizeFullwidthContinuationRow(head, kind, dialect)
		}
		if kind == "comment" && len(head) != 0 && head[0] == ';' {
			out = append(out, continuationCommentFingerprint(head)...)
		} else if kind == "void" && len(head) != 0 && head[0] == ';' {
			out = append(out, continuationCommentFingerprint(head)...)
		} else if (kind == "expression" || kind == "print-v") && continuationRowHasNumericIdentifierBoundary(head, rows, i, separator) && !printVNumericLiteralNeedsExact(head) {
			out = append(out, tokenFingerprintDialect(head, dialect)...)
		} else {
			out = append(out, head...)
		}
		out = append(out, separator...)
		if i+1 < len(rows) {
			out = append(out, marker)
		}
	}
	leading := 0
	for leading < len(out) && out[leading] == marker {
		leading++
	}
	if leading == 0 {
		return continuationFingerprintForKind(out, kind, dialect, allowFullwidth)
	}
	canonical := continuationFingerprintForKind(out[leading:], kind, dialect, allowFullwidth)
	return append(bytes.Repeat([]byte{marker}, leading), canonical...)
}

func continuationBoundaryContextFingerprint(rows []checkedLine, separator, kind string, logical []byte, dialect format.Dialect, allowFullwidth bool) ([]byte, bool) {
	logicalKind, _, exact := continuationLogicalFingerprint(logical, dialect, allowFullwidth)
	if exact {
		return nil, false
	}
	if logicalKind != kind {
		return nil, false
	}
	first := -1
	for i, row := range rows {
		if len(trimGrammarSpace(row.body, allowFullwidth)) != 0 {
			first = i
			break
		}
	}
	if first < 0 {
		return nil, false
	}
	head := trimGrammarSpace(rows[first].body, allowFullwidth)
	word, end := firstWord(head)
	if end == 0 {
		return nil, false
	}
	command := strings.ToUpper(string(word))
	commandGap := func(row []byte) bool {
		head := trimGrammarSpace(row, allowFullwidth)
		_, end := firstWord(head)
		return end != 0 && len(trimGrammarSpace(head[end:], allowFullwidth)) == 0
	}
	switch kind {
	case "print-v":
		if first == 0 || command != "PRINTV" {
			return nil, false
		}
		next := trimGrammarSpace(rows[first].body, allowFullwidth)
		_, end = firstWord(next)
		if end == 0 {
			return nil, false
		}
		next = trimGrammarSpace(next[end:], allowFullwidth)
		if len(next) == 0 || next[0] != '\'' {
			return nil, false
		}
		return continuationContextRowsFingerprint(rows, allowFullwidth, 'P', func(i int, head []byte) []byte {
			if i < first || len(head) == 0 {
				return nil
			}
			if i == first {
				return continuationFingerprintForKind(head, kind, dialect, allowFullwidth)
			}
			return tokenFingerprintDialect(head, dialect)
		}), true
	case "expression":
		if command == "TRYCALL" && separator == "\"" && first+1 < len(rows) && bytes.IndexByte(logical, 0) >= 0 {
			return continuationContextRowsFingerprint(rows, allowFullwidth, 'C', func(_ int, head []byte) []byte {
				if len(head) == 0 {
					return nil
				}
				return tokenFingerprintDialect(head, dialect)
			}), true
		}
		if command == "PRINTFORMS" && separator == "@" && first+1 < len(rows) && bytes.IndexByte(logical, 0) >= 0 && commandGap(rows[first].body) {
			next := trimGrammarSpace(rows[first+1].body, allowFullwidth)
			if len(next) != 0 && next[0] == '"' {
				return continuationContextRowsFingerprint(rows, allowFullwidth, 'F', func(i int, head []byte) []byte {
					if i == first+1 {
						head = append([]byte{'@'}, head...)
						return expressionFingerprintDialect(head, dialect)
					}
					if len(head) == 0 {
						return nil
					}
					return tokenFingerprintDialect(head, dialect)
				}), true
			}
		}
	case "form-call":
		if bytes.IndexByte(logical, 0) >= 0 && command == "CALLFORM" {
			for _, row := range rows {
				if head := trimGrammarSpace(row.body, allowFullwidth); len(head) != 0 && head[0] == 0 {
					return continuationFormCallContextBoundaryFingerprint(rows, first, dialect, allowFullwidth), true
				}
			}
			if fingerprint, ok := continuationFormCallBoundaryFingerprint(rows, separator, dialect, allowFullwidth); ok {
				return fingerprint, true
			}
			return continuationFormCallContextBoundaryFingerprint(rows, first, dialect, allowFullwidth), true
		}
	case "form":
		if separator == "" && first+1 < len(rows) {
			body := rows[first].body
			if len(body) != 0 && body[len(body)-1] == '\\' && (len(body) < 2 || body[len(body)-2] != '\\') {
				next := trimGrammarSpace(rows[first+1].body, allowFullwidth)
				if len(next) != 0 && next[0] == '@' {
					return continuationFormContextBoundaryFingerprint(rows, separator, dialect, allowFullwidth), true
				}
			}
		}
	}
	return nil, false
}

func continuationContextRowsFingerprint(rows []checkedLine, allowFullwidth bool, prefix byte, fingerprint func(int, []byte) []byte) []byte {
	out := []byte{prefix}
	opaque := false
	for i, row := range rows {
		if opaque {
			out = appendPreservedField(out, row.body)
			continue
		}
		head := trimGrammarSpace(row.body, allowFullwidth)
		out = appendPreservedField(out, fingerprint(i, head))
		opaque = bytes.IndexByte(head, 0) >= 0
	}
	return out
}

func continuationFormCallContextBoundaryFingerprint(rows []checkedLine, commandRow int, dialect format.Dialect, allowFullwidth bool) []byte {
	return continuationContextRowsFingerprint(rows, allowFullwidth, 'K', func(i int, head []byte) []byte {
		if len(head) == 0 {
			return nil
		}
		if i == commandRow {
			word, end := firstWord(head)
			tail := head[end:]
			if allowFullwidth {
				tail = normalizeFullwidthCommandGap(tail)
				return joinFingerprint(word, formCallFingerprintFullwidthDialect(tail, dialect))
			}
			return joinFingerprint(word, formCallFingerprintDialect(tail, dialect))
		}
		return tokenFingerprintDialect(head, dialect)
	})
}

func continuationFormContextBoundaryFingerprint(rows []checkedLine, separator string, dialect format.Dialect, allowFullwidth bool) []byte {
	out := []byte{'B'}
	state := continuationFormState{}
	opaque := false
	pendingEscape := false
	for i, row := range rows {
		head := trimGrammarSpace(row.body, allowFullwidth)
		if opaque {
			out = appendPreservedField(out, row.body)
			continue
		}
		code := head
		var suffix []byte
		if boundary := corpusFormNulBoundaryMode(head, allowFullwidth); boundary >= 0 {
			code, suffix = head[:boundary], head[boundary:]
		}
		input := code
		if pendingEscape && separator == "" {
			input = append([]byte{'\\'}, code...)
		}
		normalized, next := normalizeContinuationFormTextState(input, allowFullwidth, state)
		if pendingEscape && separator == "" && len(normalized) != 0 && normalized[0] == '\\' {
			normalized = normalized[1:]
		}
		if suffix != nil {
			normalized = append(normalized, suffix...)
			opaque = true
		}
		out = appendPreservedField(out, normalized)
		state = next
		pendingEscape = separator == "" && continuationTrailingEscape(code)
		if i+1 < len(rows) && !opaque && separator != "" {
			_, state = normalizeContinuationFormTextState([]byte(separator), allowFullwidth, state)
		}
	}
	return out
}

func continuationTrailingEscape(src []byte) bool {
	backslashes := 0
	for i := len(src) - 1; i >= 0 && src[i] == '\\'; i-- {
		backslashes++
	}
	return backslashes%2 != 0
}

func continuationNulBoundary(src []byte, kind string, dialect format.Dialect, allowFullwidth bool) int {
	head := trimGrammarSpace(src, allowFullwidth)
	word, end := firstWord(head)
	if end == 0 {
		switch kind {
		case "assignment", "expression", "increment", "print-v":
			if boundary := corpusExpressionNulBoundaryMode(head, kind == "print-v", allowFullwidth); boundary >= 0 {
				return len(src) - len(head) + boundary
			}
		}
		return -1
	}
	tail := head[end:]
	boundary := -1
	if kind == "form" {
		boundary = corpusFormNulBoundaryMode(tail, allowFullwidth)
	} else {
		boundary = corpusStatementNulBoundary(strings.ToUpper(string(word)), tail, dialect, allowFullwidth)
	}
	if boundary < 0 {
		return -1
	}
	return len(src) - len(head) + end + boundary
}

func continuationBoundaryFingerprintNul(rows []checkedLine, separator, kind string, marker byte, dialect format.Dialect, allowFullwidth bool, boundary int) []byte {
	prefixRows := make([]checkedLine, len(rows))
	position := 0
	found := false
	for i, row := range rows {
		prefixRows[i] = row
		if found {
			prefixRows[i].body = nil
		} else if boundary < position+len(row.body) {
			prefixRows[i].body = row.body[:boundary-position]
			found = true
		}
		position += len(row.body) + len(separator)
	}
	if !found {
		return continuationBoundaryFingerprint(rows, separator, kind, marker, dialect, allowFullwidth)
	}
	out := continuationBoundaryFingerprint(prefixRows, separator, kind, marker, dialect, allowFullwidth)
	out = appendPreservedField(out, []byte{'N'})
	position = 0
	found = false
	for _, row := range rows {
		var suffix []byte
		if found {
			suffix = row.body
		} else if boundary < position+len(row.body) {
			suffix = row.body[boundary-position:]
			found = true
		}
		out = appendPreservedField(out, suffix)
		position += len(row.body) + len(separator)
	}
	return out
}

func continuationRowHasNumericIdentifierBoundary(head []byte, rows []checkedLine, index int, separator string) bool {
	if len(head) == 0 {
		return false
	}
	if printVHasNumericIdentifierBoundary(head) {
		return true
	}
	if continuationRowHasProtectedPayload(head) {
		return false
	}
	nextBodyLen := 0
	if index+1 < len(rows) {
		nextBodyLen = len(rows[index+1].body)
	}
	probe := make([]byte, 0, len(head)+len(separator)+nextBodyLen)
	probe = append(probe, head...)
	probe = append(probe, separator...)
	if index+1 < len(rows) {
		probe = append(probe, rows[index+1].body...)
	}
	return printVNumericIdentifierBoundary(probe, len(head))
}

func continuationRowHasProtectedPayload(head []byte) bool {
	markers, _ := stripMarkers(head)
	return len(markers) != 0 || bytes.Contains(head, []byte("[[")) || bytes.Contains(head, []byte(";!;")) || bytes.Contains(head, []byte(";#;")) || bytes.IndexByte(head, 0) >= 0 || incrementCommentStart(head) < len(head) || strings.ContainsAny(string(head), "'\"`")
}

func continuationKindHasForm(kind string) bool {
	switch kind {
	case "form", "form-any", "form-call", "printforms-literal", "strform-literal", "print-v", "graphics":
		return true
	default:
		return false
	}
}

func continuationRowsHaveNormalForm(rows []checkedLine, separator string, dialect format.Dialect) bool {
	logical := continuationLogicalSource(rows, separator)
	fingerprint := formFingerprintDialect(logical, dialect)
	hasForm := bytes.Contains(fingerprint, []byte("P{")) || bytes.Contains(fingerprint, []byte("P%"))
	return hasForm && !bytes.Contains(fingerprint, []byte("P{R")) && !bytes.Contains(fingerprint, []byte("P%R"))
}

func continuationFormCallBoundaryFingerprint(rows []checkedLine, separator string, dialect format.Dialect, allowFullwidth bool) ([]byte, bool) {
	logical := continuationLogicalSource(rows, separator)
	head := trimGrammarSpace(logical, allowFullwidth)
	_, end := firstWord(head)
	tail := head[end:]
	start := len(tail) - len(trimGrammarSpace(tail, allowFullwidth))
	delimiter := formCallDelimiter(tail, start)
	if delimiter < 0 || tail[delimiter] == ';' {
		return nil, false
	}
	argsStart := len(logical) - len(tail) + delimiter
	out := []byte{'K'}
	state := continuationFormState{}
	offset, commandRow := 0, true
	for i, row := range rows {
		head := trimGrammarSpace(row.body, allowFullwidth)
		split := argsStart - offset - (len(row.body) - len(head))
		split = max(0, min(split, len(head)))
		prefix := head[:split]
		if commandRow && len(prefix) != 0 {
			word, end := firstWord(prefix)
			prefix = append(append([]byte(nil), word...), normalizeFullwidthCommandGap(prefix[end:])...)
			commandRow = false
		}
		var normalized []byte
		normalized, state = normalizeContinuationFormTextState(prefix, allowFullwidth, state)
		out = appendPreservedField(out, normalized)
		if split < len(head) || offset >= argsStart {
			state.code = true
			normalized, state = normalizeContinuationFormTextState(head[split:], allowFullwidth, state)
			out = appendPreservedField(out, tokenFingerprintDialect(normalized, dialect))
		} else {
			out = appendPreservedField(out, nil)
		}
		if i+1 < len(rows) {
			_, state = normalizeContinuationFormTextState([]byte(separator), allowFullwidth, state)
		}
		offset += len(row.body) + len(separator)
	}
	return out, true
}

func continuationFormBoundaryFingerprint(rows []checkedLine, separator, kind string, dialect format.Dialect, allowFullwidth bool) []byte {
	if kind == "graphics" {
		return continuationGraphicsBoundaryFingerprint(rows, separator, dialect, allowFullwidth)
	}
	if kind == "print-v" {
		return continuationPrintVBoundaryFingerprint(rows, separator, dialect, allowFullwidth)
	}
	commandRow := -1
	for i, row := range rows {
		if len(trimGrammarSpace(row.body, allowFullwidth)) != 0 {
			commandRow = i
			break
		}
	}
	out := []byte{'B'}
	formState := continuationFormState{}
	for i, row := range rows {
		head := trimGrammarSpace(row.body, allowFullwidth)
		if i == commandRow {
			word, end := firstWord(head)
			if end > 0 {
				tail := normalizeFullwidthCommandGap(head[end:])
				if kind == "printforms-literal" || kind == "strform-literal" {
					tail = normalizeTrustedCommandGapWithArity(tail, true, false, false)
				}
				if allowFullwidth {
					tail = normalizeFullwidthCommandTailForBoundary(string(word), tail, dialect)
				}
				head = append(append([]byte(nil), word...), tail...)
			}
		}
		var normalized []byte
		normalized, formState = normalizeContinuationFormTextState(head, allowFullwidth, formState)
		out = appendPreservedField(out, normalized)
		if i+1 < len(rows) {
			_, formState = normalizeContinuationFormTextState([]byte(separator), allowFullwidth, formState)
		}
	}
	return out
}

func continuationPrintVBoundaryFingerprint(rows []checkedLine, separator string, dialect format.Dialect, allowFullwidth bool) []byte {
	logical := continuationLogicalSource(rows, separator)
	head := trimGrammarSpace(logical, allowFullwidth)
	word, end := firstWord(head)
	if end == 0 {
		return continuationFormBoundaryFingerprint(rows, separator, "form", dialect, allowFullwidth)
	}
	tail := normalizeFullwidthCommandGap(head[end:])
	if allowFullwidth {
		tail = normalizeFullwidthPrintVTail(tail)
	}
	semantic := joinFingerprint(word, printVFingerprintDialect(tail, allowFullwidth, dialect))
	out := []byte{'V'}
	out = appendPreservedField(out, semantic)
	state := continuationFormState{code: true}
	for i, row := range rows {
		var literals, code []byte
		state = scanContinuationLiterals(row.body, allowFullwidth, state, &literals, &code)
		out = appendPreservedField(out, literals)
		out = appendPreservedField(out, code)
		if i+1 < len(rows) {
			state = scanContinuationLiterals([]byte(separator), allowFullwidth, state, nil, nil)
		}
	}
	return out
}

func continuationGraphicsBoundaryFingerprint(rows []checkedLine, separator string, dialect format.Dialect, allowFullwidth bool) []byte {
	logical := continuationLogicalSource(rows, separator)
	head := trimGrammarSpace(logical, allowFullwidth)
	word, end := firstWord(head)
	if end == 0 {
		return continuationFormBoundaryFingerprint(rows, separator, "form", dialect, allowFullwidth)
	}
	semantic := graphicsFingerprintMode(strings.ToUpper(string(word)), head[end:], dialect, allowFullwidth)
	out := []byte{'G'}
	out = appendPreservedField(out, semantic)
	state := continuationFormState{code: true}
	for i, row := range rows {
		var literals, code []byte
		state = scanContinuationLiterals(row.body, allowFullwidth, state, &literals, &code)
		out = appendPreservedField(out, literals)
		out = appendPreservedField(out, code)
		if i+1 < len(rows) {
			state = scanContinuationLiterals([]byte(separator), allowFullwidth, state, nil, nil)
		}
	}
	return out
}

func scanContinuationLiterals(src []byte, allowFullwidth bool, state continuationFormState, literals, code *[]byte) continuationFormState {
	emitLiteral := func(data []byte) {
		if literals != nil {
			*literals = append(*literals, data...)
		}
	}
	emitCode := func(data []byte) {
		if code == nil {
			return
		}
		for _, b := range data {
			*code = appendPreservedField(*code, []byte{b})
		}
	}
	for i := 0; i < len(src); {
		inside := state.delimiter != 0
		inFormText := !inside && len(state.frames) != 0
		if state.quote != 0 {
			if src[i] == '\\' && i+1 < len(src) {
				emitLiteral(src[i : i+2])
				i += 2
				continue
			}
			emitLiteral(src[i : i+1])
			if src[i] == state.quote {
				emitCode(src[i : i+1])
				state.quote = 0
			}
			i++
			continue
		}
		if src[i] == '@' && i+1 < len(src) && src[i+1] == '"' && (inside || len(state.frames) == 0) {
			state.frames = append(state.frames, state.continuationFormFrame)
			state.continuationFormFrame = continuationFormFrame{}
			emitLiteral(src[i : i+2])
			emitCode(src[i : i+2])
			i += 2
			continue
		}
		if src[i] == '"' && inFormText {
			state.continuationFormFrame = state.frames[len(state.frames)-1]
			state.frames = state.frames[:len(state.frames)-1]
			emitLiteral(src[i : i+1])
			emitCode(src[i : i+1])
			i++
			continue
		}
		if src[i] == '"' && (inside || state.code && len(state.frames) == 0) {
			state.quote = '"'
			emitLiteral(src[i : i+1])
			emitCode(src[i : i+1])
			i++
			continue
		}
		if !inside && !inFormText {
			switch src[i] {
			case ';':
				emitCode(src[i : i+1])
				if corpusProtectedComment(src[i:]) {
					emitLiteral(src[i:])
				}
				return state
			case 0:
				emitCode(src[i : i+1])
				emitLiteral(src[i:])
				return state
			case '\'':
				emitCode(src[i : i+1])
				start := i
				for i < len(src) && src[i] != 0 {
					if src[i] == '\\' && i+1 < len(src) {
						i += 2
						continue
					}
					if src[i] == ',' {
						break
					}
					i++
				}
				emitLiteral(src[start:i])
				continue
			}
		}
		if inside && ((state.delimiter == '{' && src[i] == '}') || (state.delimiter == '%' && src[i] == '%') || (state.delimiter == '?' && state.depth == 0 && src[i] == '?')) {
			emitLiteral(src[i : i+1])
			emitCode(src[i : i+1])
			i++
			state.delimiter = 0
			continue
		}
		if !inside && inFormText && (src[i] == '{' || src[i] == '%') {
			state.delimiter = src[i]
			emitLiteral(src[i : i+1])
			emitCode(src[i : i+1])
			i++
			continue
		}
		if state.delimiter == '?' {
			switch src[i] {
			case '(', '[':
				state.depth++
			case ')', ']':
				if state.depth > 0 {
					state.depth--
				}
			}
		}
		if !inside && !inFormText && src[i] == '\\' && i+1 < len(src) && src[i+1] == '@' {
			state.delimiter, state.depth = '?', 0
			emitLiteral(src[i : i+2])
			emitCode(src[i : i+2])
			i += 2
			continue
		}
		if inside {
			if size := continuationGrammarSpaceSize(src[i:], allowFullwidth); size != 0 {
				i += size
				continue
			}
			emitCode(src[i : i+1])
			i++
			continue
		}
		if inFormText {
			emitLiteral(src[i : i+1])
		} else if size := continuationGrammarSpaceSize(src[i:], allowFullwidth); size == 0 {
			emitCode(src[i : i+1])
		} else {
			i += size
			continue
		}
		i++
	}
	return state
}

func continuationGrammarSpaceSize(src []byte, allowFullwidth bool) int {
	if len(src) == 0 {
		return 0
	}
	if src[0] == ' ' || src[0] == '\t' {
		return 1
	}
	if allowFullwidth && bytes.HasPrefix(src, []byte("\u3000")) {
		return 3
	}
	return 0
}

func normalizeFullwidthCommandTailForBoundary(command string, tail []byte, dialect format.Dialect) []byte {
	switch {
	case isPrintVCommandDialect(command, dialect):
		return normalizeFullwidthPrintVTail(tail)
	case command == "STRFORM" || isPrintFormsCommandDialect(command, dialect):
		return normalizeFullwidthPrintFormsTail(tail)
	case isGraphicsCommand(command):
		return normalizeFullwidthExpressionSpaces(tail)
	default:
		return normalizeFullwidthExpressionSpaces(tail)
	}
}

type continuationFormFrame struct {
	delimiter   byte
	conditional bool
	depth       int
}

type continuationFormState struct {
	continuationFormFrame
	code   bool
	quote  byte
	frames []continuationFormFrame
}

func normalizeContinuationFormTextState(row []byte, allowFullwidth bool, state continuationFormState) ([]byte, continuationFormState) {
	out := make([]byte, 0, len(row))
	pendingSpace := false
	for i := 0; i < len(row); {
		inside := state.delimiter != 0
		if state.quote != 0 {
			if row[i] == '\\' && i+1 < len(row) {
				out = append(out, row[i:i+2]...)
				i += 2
			} else {
				out = append(out, row[i])
				if row[i] == state.quote {
					state.quote = 0
				}
				i++
			}
			pendingSpace = false
			continue
		}
		if row[i] == '@' && i+1 < len(row) && row[i+1] == '"' && (inside || len(state.frames) == 0) {
			state.frames = append(state.frames, state.continuationFormFrame)
			state.continuationFormFrame = continuationFormFrame{}
			out = append(out, row[i:i+2]...)
			i += 2
			pendingSpace = false
			continue
		}
		if row[i] == '"' && !inside && len(state.frames) != 0 {
			last := len(state.frames) - 1
			state.continuationFormFrame = state.frames[last]
			state.frames = state.frames[:last]
			out = append(out, row[i])
			i++
			pendingSpace = false
			continue
		}
		if row[i] == '"' && (inside || state.code && len(state.frames) == 0) || inside && row[i] == '\'' {
			state.quote = row[i]
			out = append(out, row[i])
			i++
			pendingSpace = false
			continue
		}
		if row[i] == '\\' && i+1 < len(row) {
			opening := !inside && row[i+1] == '@' && !state.conditional
			if !inside && row[i+1] == '@' {
				state.conditional = opening
				if opening {
					state.delimiter, state.depth = '?', 0
				}
			}
			out = append(out, row[i:i+2]...)
			i += 2
			if opening {
				for i < len(row) && isContinuationFormSpace(row[i:], allowFullwidth) {
					i += continuationFormSpaceSize(row[i:], allowFullwidth)
				}
			}
			pendingSpace = false
			continue
		}
		if !inside && (row[i] == '{' || row[i] == '%') {
			state.delimiter = row[i]
			out = append(out, row[i])
			i++
			for i < len(row) && isContinuationFormSpace(row[i:], allowFullwidth) {
				i += continuationFormSpaceSize(row[i:], allowFullwidth)
			}
			pendingSpace = false
			continue
		}
		if state.delimiter == '?' {
			switch row[i] {
			case '(', '[':
				state.depth++
			case ')', ']':
				if state.depth > 0 {
					state.depth--
				}
			}
		}
		if inside && (state.delimiter == '{' && row[i] == '}' || state.delimiter == '%' && row[i] == '%' || state.delimiter == '?' && state.depth == 0 && row[i] == '?') {
			for len(out) != 0 && isContinuationFormSpace(out[len(out)-1:], allowFullwidth) {
				out = out[:len(out)-continuationFormSpaceSize(out[len(out)-1:], allowFullwidth)]
			}
			out = append(out, row[i])
			i++
			state.delimiter = 0
			pendingSpace = false
			continue
		}
		if inside && isContinuationFormSpace(row[i:], allowFullwidth) {
			i += continuationFormSpaceSize(row[i:], allowFullwidth)
			continue
		}
		if inside && strings.ContainsRune("+-*/=&|^", rune(row[i])) {
			for len(out) != 0 && isContinuationFormSpace(out[len(out)-1:], allowFullwidth) {
				out = out[:len(out)-continuationFormSpaceSize(out[len(out)-1:], allowFullwidth)]
			}
			unary := len(out) == 0 || out[len(out)-1] == '{' || strings.ContainsRune("+-*/=&|^,", rune(out[len(out)-1]))
			if !unary {
				out = append(out, ' ')
			}
			out = append(out, row[i])
			pendingSpace = !unary
			i++
			continue
		}
		if inside && pendingSpace && (row[i] == '"' || row[i] == '\'' || isFingerprintWord(row[i]) || row[i] >= '0' && row[i] <= '9') {
			out = append(out, ' ')
			pendingSpace = false
		}
		out = append(out, row[i])
		i++
	}
	return out, state
}

func isContinuationFormSpace(src []byte, allowFullwidth bool) bool {
	return len(src) != 0 && (src[0] == ' ' || src[0] == '\t' || allowFullwidth && bytes.HasPrefix(src, []byte("\u3000")))
}

func continuationFormSpaceSize(src []byte, allowFullwidth bool) int {
	if allowFullwidth && bytes.HasPrefix(src, []byte("\u3000")) {
		return 3
	}
	return 1
}

func continuationInputBoundaryFingerprint(rows []checkedLine, dialect format.Dialect, allowFullwidth bool) []byte {
	commandRow := -1
	for i, row := range rows {
		if len(trimGrammarSpace(row.body, allowFullwidth)) != 0 {
			commandRow = i
			break
		}
	}
	out := []byte{'B'}
	for i, row := range rows {
		head := trimGrammarSpace(row.body, allowFullwidth)
		if allowFullwidth && i == commandRow {
			word, end := firstWord(head)
			if end > 0 {
				command := strings.ToUpper(string(word))
				tail := normalizeFullwidthCommandGap(head[end:])
				if isInputCommand(command, dialect) {
					tail = normalizeFullwidthInputTail(command, tail, dialect)
				} else {
					tail = normalizeFullwidthInputExpressionTail(command, tail, dialect)
				}
				head = append(append([]byte(nil), word...), tail...)
			}
		} else if allowFullwidth {
			head = normalizeFullwidthFormPayload(head)
		}
		out = appendPreservedField(out, expressionFingerprint(head))
	}
	return out
}

func normalizeFormContinuationRow(row []byte, allowFullwidth bool) []byte {
	trivia := func(src []byte) bool {
		return src[0] == ' ' || src[0] == '\t' || allowFullwidth && bytes.HasPrefix(src, []byte("\u3000"))
	}
	trimRight := func(src []byte) []byte {
		for len(src) != 0 {
			if src[len(src)-1] == ' ' || src[len(src)-1] == '\t' {
				src = src[:len(src)-1]
				continue
			}
			if allowFullwidth && len(src) >= 3 && bytes.HasSuffix(src, []byte("\u3000")) {
				src = src[:len(src)-3]
				continue
			}
			break
		}
		return src
	}
	out := make([]byte, 0, len(row))
	for i := 0; i < len(row); {
		if row[i] == '"' || row[i] == '\'' {
			end := printVQuotedEnd(row, i)
			out = append(out, row[i:end]...)
			i = end
			continue
		}
		if row[i] == '{' {
			out = append(out, row[i])
			i++
			for i < len(row) && trivia(row[i:]) {
				if row[i] == ' ' || row[i] == '\t' {
					i++
				} else {
					i += 3
				}
			}
			continue
		}
		if row[i] == '}' {
			out = trimRight(out)
			out = append(out, row[i])
			i++
			continue
		}
		out = append(out, row[i])
		i++
	}
	trimmed := trimRight(out)
	if len(trimmed) != 0 {
		last := trimmed[len(trimmed)-1]
		if strings.ContainsRune("+-*/%=&|^", rune(last)) {
			trimmed = trimRight(trimmed[:len(trimmed)-1])
			trimmed = append(trimmed, last)
		}
	}
	return trimmed
}

func normalizeFullwidthContinuationRow(row []byte, kind string, dialect format.Dialect) []byte {
	switch kind {
	case "form", "form-any":
		return normalizeFullwidthFormPayloadDialect(row, dialect)
	case "comment":
		return row
	default:
		return normalizeFullwidthExpressionSpacesDialect(row, dialect)
	}
}

func normalizeFullwidthVoidCommand(head []byte) []byte {
	word, end := firstWord(head)
	if end == 0 {
		return head
	}
	tail := normalizeFullwidthCommandGap(head[end:])
	comment := incrementCommentStart(tail)
	if comment < 0 || comment >= len(tail) {
		return append(append([]byte(nil), word...), tail...)
	}
	code := bytes.TrimRight(tail[:comment], " \t")
	out := append(append([]byte(nil), word...), code...)
	out = append(out, ' ')
	return append(out, continuationCommentFingerprint(tail[comment:])...)
}

func normalizeFullwidthContinuationCommand(head []byte, kind string, dialect format.Dialect) []byte {
	word, end := firstWord(head)
	if end == 0 {
		return head
	}
	command := strings.ToUpper(string(word))
	tail := normalizeFullwidthCommandGap(head[end:])
	switch kind {
	case "print-v":
		tail = normalizeFullwidthPrintVTail(tail)
	case "form":
		tail = normalizeFullwidthFormPayloadDialect(tail, dialect)
	case "form-any":
		tail = normalizeFullwidthStructuredFormTailDialect(tail, 1, dialect)
	case "input":
		if isInputCommand(command, dialect) {
			tail = normalizeFullwidthInputTail(command, tail, dialect)
		} else {
			tail = normalizeFullwidthInputExpressionTail(command, tail, dialect)
		}
	case "expression":
		if _, min, max, ok := corpusStructuredCommand(command, dialect); ok {
			tail = normalizeFullwidthStructuredExpressionTail(command, tail, min, max)
		} else {
			tail = normalizeFullwidthExpressionSpacesDialect(tail, dialect)
		}
	case "graphics", "keyword-value", "variable", "times", "method":
		tail = normalizeFullwidthExpressionSpacesDialect(tail, dialect)
	case "printforms-literal", "strform-literal":
		tail = normalizeFullwidthPrintFormsTailDialect(tail, dialect)
	}
	canonical := append([]byte(nil), word...)
	return append(canonical, tail...)
}

const continuationBoundaryMarker byte = 0x1e

func continuationBoundaryMarkerFor(src []byte) (byte, bool) {
	for marker := byte(1); marker < 0x20; marker++ {
		if marker == '\t' || marker == '\v' || marker == '\f' || marker == '\r' || marker == '\n' {
			continue
		}
		if !bytes.Contains(src, []byte{marker}) {
			return marker, true
		}
	}
	return 0, false
}

func continuationFingerprintForKind(src []byte, kind string, dialect format.Dialect, allowFullwidth bool) []byte {
	if kind == "form" {
		if allowFullwidth {
			return formFingerprintDialect(normalizeFullwidthFormPayloadDialect(src, dialect), dialect)
		}
		return formFingerprintDialect(src, dialect)
	}
	if kind == "form-call" {
		head := trimGrammarSpace(src, allowFullwidth)
		word, end := firstWord(head)
		if end > 0 {
			tail := head[end:]
			if allowFullwidth {
				tail = normalizeFullwidthCommandGap(tail)
				return joinFingerprint(word, formCallFingerprintFullwidthDialect(tail, dialect))
			}
			return joinFingerprint(word, formCallFingerprintDialect(tail, dialect))
		}
	}
	if kind == "form-any" {
		head := trimGrammarSpace(src, allowFullwidth)
		word, end := firstWord(head)
		if end > 0 {
			tail := head[end:]
			if allowFullwidth {
				tail = normalizeFullwidthFormPayloadDialect(trimLeadingGrammarSpace(tail), dialect)
			} else {
				tail = bytes.TrimLeft(tail, " \t")
			}
			return joinFingerprint(word, formFingerprintDialect(tail, dialect))
		}
		return append([]byte(nil), src...)
	}
	if kind == "printforms-literal" {
		head := trimGrammarSpace(src, allowFullwidth)
		word, end := firstWord(head)
		if end > 0 {
			tail := head[end:]
			if allowFullwidth {
				tail = normalizeFullwidthPrintFormsTailDialect(tail, dialect)
			}
			if fingerprint, ok := printFormsLiteralFingerprintDialect(tail, dialect); ok {
				return joinFingerprint(word, fingerprint)
			}
		}
	}
	if kind == "strform-literal" {
		head := trimGrammarSpace(src, allowFullwidth)
		word, end := firstWord(head)
		if end > 0 {
			if fingerprint, ok := directSTRFORMLiteralFingerprintDialect(head[end:], allowFullwidth, dialect); ok {
				return joinFingerprint(word, fingerprint)
			}
		}
	}
	if kind == "print-v" {
		head := trimGrammarSpace(src, allowFullwidth)
		word, end := firstWord(head)
		if end > 0 {
			tail := normalizeFullwidthCommandGap(head[end:])
			if allowFullwidth {
				tail = normalizeFullwidthPrintVTail(tail)
			}
			return joinFingerprint(word, printVFingerprintDialect(tail, allowFullwidth, dialect))
		}
	}
	if kind == "input" {
		head := trimGrammarSpace(src, allowFullwidth)
		word, end := firstWord(head)
		if end > 0 {
			tail := head[end:]
			command := strings.ToUpper(string(word))
			if allowFullwidth {
				tail = normalizeFullwidthCommandGap(tail)
				if isInputCommand(command, dialect) {
					tail = normalizeFullwidthInputTail(command, tail, dialect)
				} else {
					tail = normalizeFullwidthInputExpressionTail(command, tail, dialect)
				}
			}
			if isInputExpressionCommand(command, dialect) {
				return joinFingerprint(word, inputExpressionFingerprint(command, tail, dialect))
			}
			return joinFingerprint(word, inputFingerprint(command, tail, dialect))
		}
	}
	if kind == "comment" {
		return standaloneCommentFingerprint(src)
	}
	if kind == "define" {
		return defineFingerprintWithFullwidth(src, allowFullwidth)
	}
	if kind == "assignment" {
		if fingerprint, ok := assignmentPreservationFingerprint(src, allowFullwidth); ok {
			return fingerprint
		}
	}
	if allowFullwidth {
		src = normalizeFullwidthExpressionSpaces(src)
	}
	return expressionFingerprintDialect(src, dialect)
}

func appendPreservedField(dst, field []byte) []byte {
	var size [4]byte
	binary.BigEndian.PutUint32(size[:], uint32(len(field)))
	dst = append(dst, size[:]...)
	return append(dst, field...)
}

func continuationCommentFingerprint(body []byte) []byte {
	head := bytes.TrimLeft(body, " \t")
	if len(head) == 0 || head[0] != ';' || corpusProtectedComment(head) {
		return append([]byte(nil), body...)
	}
	return standaloneCommentFingerprint(head)
}

func continuationLogicalFingerprint(src []byte, dialect format.Dialect, allowFullwidth bool) (kind string, fingerprint []byte, exact bool) {
	head := trimGrammarSpace(src, allowFullwidth)
	markers, head := stripMarkers(head)
	if len(markers) != 0 || bytes.Contains(src, []byte("[[")) || bytes.Contains(src, []byte(";!;")) || bytes.Contains(src, []byte(";#;")) {
		return "protected", append([]byte(nil), src...), true
	}
	if len(head) == 0 {
		return "unknown", append([]byte(nil), src...), true
	}
	if head[0] == ';' {
		if corpusProtectedComment(head) {
			return "protected", append([]byte(nil), src...), true
		}
		return "comment", standaloneCommentFingerprint(head), false
	}
	if head[0] == '@' {
		if allowFullwidth {
			head = normalizeFullwidthExpressionSpaces(head)
		}
		return "function-header", tokenFingerprint(head), false
	}
	if head[0] == '$' {
		if allowFullwidth {
			head = normalizeFullwidthExpressionSpaces(head)
		}
		return "goto-label", tokenFingerprint(head), false
	}
	if head[0] == '[' {
		fingerprint, trusted := corpusPreprocessorFingerprintMode(head, allowFullwidth)
		return "preprocessor", fingerprint, !trusted
	}
	if head[0] == '#' {
		if isDefineDirective(head) {
			return "define", defineFingerprintWithFullwidth(head, allowFullwidth), true
		}
		if !corpusKnownSharpDeclaration(head) {
			if corpusUnknownSharpNeedsExact(head) {
				return "declaration", trimGrammarSpaceRight(head, allowFullwidth), true
			}
			if allowFullwidth {
				head = normalizeFullwidthExpressionSpaces(head)
			}
			return "declaration", tokenFingerprint(head), false
		}
		if allowFullwidth {
			head = normalizeFullwidthExpressionSpaces(head)
		}
		return "declaration", tokenFingerprint(head), false
	}
	route := statementRouteForHead(head, dialect, allowFullwidth)
	if route.kind == "unknown" || route.end == 0 && route.kind != "assignment" && route.kind != "increment" {
		return "unknown", append([]byte(nil), src...), true
	}
	if route.kind == "print" && isPrintVCommandDialect(route.upper, dialect) {
		return "print-v", route.fingerprint, false
	}
	if route.kind == "form-call" && corpusFormCallTailNeedsExact(head[route.end:], dialect, allowFullwidth) {
		return route.kind, route.fingerprint, true
	}
	return route.kind, route.fingerprint, route.kind == "raw" || route.kind == "data" || route.kind == "print" || route.kind == "void"
}
