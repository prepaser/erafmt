package cli

import (
	"bytes"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/prepaser/erafmt/internal/format"
)

type checkedLine struct {
	body, ending []byte
}

type preservedRecord struct {
	line int
	kind string
	data []byte
}

type sourcePreservationOptions struct {
	dialect               format.Dialect
	continuationSeparator *string
	allowFullwidthSpace   bool
}

type statementRoute struct {
	end             int
	upper           string
	kind            string
	fingerprint     []byte
	unknownFallback bool
}

func checkedLines(src []byte) []checkedLine {
	var lines []checkedLine
	for len(src) != 0 {
		i := bytes.IndexAny(src, "\r\n")
		if i < 0 {
			lines = append(lines, checkedLine{body: src})
			break
		}
		end := i + 1
		if src[i] == '\r' && end < len(src) && src[end] == '\n' {
			end++
		}
		lines = append(lines, checkedLine{src[:i], src[i:end]})
		src = src[end:]
	}
	return lines
}

// checkSourcePreserved compares nonblank semantic records rather than physical
// layout. It permits the formatter's ASCII trivia edits in proven expression,
// declaration, preprocessor, assignment-boundary, and comment regions. Raw
// print/data/form literals, quoted strings, macro bodies, assignment RHS values,
// and continuation interiors remain byte-sensitive. This is a conservative
// lexical contract, not a replacement for the runtime parser.
func checkSourcePreserved(original, output []byte) error {
	return checkSourcePreservedOptions(original, output, sourcePreservationOptions{dialect: format.CommonDialect})
}

func checkSourcePreservedDialect(original, output []byte, dialect format.Dialect) error {
	return checkSourcePreservedOptions(original, output, sourcePreservationOptions{dialect: dialect})
}

func checkSourcePreservedOptions(original, output []byte, options sourcePreservationOptions) error {
	a, b := preservedRecordsOptions(original, options), preservedRecordsOptions(output, options)
	for i, j := 0, 0; i < len(a) && j < len(b); i, j = i+1, j+1 {
		if a[i].kind != b[j].kind || !bytes.Equal(a[i].data, b[j].data) {
			return fmt.Errorf("source payload changed near lines %d and %d (%s/%s)", a[i].line, b[j].line, a[i].kind, b[j].kind)
		}
	}
	if len(a) != len(b) {
		return fmt.Errorf("semantic record count changed (%d to %d)", len(a), len(b))
	}
	return nil
}

func preservedRecordsDialect(src []byte, dialect format.Dialect) []preservedRecord {
	return preservedRecordsOptions(src, sourcePreservationOptions{dialect: dialect})
}

func preservedRecordsOptions(src []byte, options sourcePreservationOptions) []preservedRecord {
	dialect := options.dialect
	lines := checkedLines(src)
	records := make([]preservedRecord, 0, len(lines))
	continuation := false
	continuationStart := 0
	continuationRows := make([]checkedLine, 0)
	for i, line := range lines {
		body := line.body
		head := trimGrammarSpace(body, options.allowFullwidthSpace)
		if continuation {
			if bytes.Equal(bytes.TrimSpace(body), []byte("}")) {
				if options.continuationSeparator != nil && len(continuationRows) != 0 {
					records = append(records, preservedRecord{continuationStart + 1, "continuation", continuationFingerprint(continuationRows, *options.continuationSeparator, dialect, options.allowFullwidthSpace)})
				}
				records = append(records, preservedRecord{i + 1, "continuation-close", append([]byte(nil), trimGrammarSpaceBoth(body, options.allowFullwidthSpace)...)})
				continuation = false
				continuationRows = continuationRows[:0]
				continue
			}
			if options.continuationSeparator != nil {
				continuationRows = append(continuationRows, line)
			} else {
				records = append(records, preservedRecord{i + 1, "continuation-interior", append([]byte(nil), body...)})
			}
			continue
		}
		if len(trimGrammarSpaceBoth(body, options.allowFullwidthSpace)) == 0 {
			continue
		}
		if bytes.Equal(bytes.TrimRightFunc(head, unicode.IsSpace), []byte("{")) {
			records = append(records, preservedRecord{i + 1, "continuation-open", append([]byte(nil), trimGrammarSpaceBoth(body, options.allowFullwidthSpace)...)})
			continuation = true
			continuationStart = i + 1
			continuationRows = continuationRows[:0]
			continue
		}
		rawHead := head
		markers, head := stripMarkers(head)
		if len(markers) != 0 {
			records = append(records, preservedRecord{i + 1, "marked", append([]byte(nil), rawHead...)})
			continue
		}
		if bytes.Contains(head, []byte("[[")) {
			records = append(records, preservedRecord{i + 1, "alias", append([]byte(nil), head...)})
			continue
		}
		if head[0] == ';' {
			fingerprint := standaloneCommentFingerprint(head)
			if corpusProtectedComment(head) {
				fingerprint = append([]byte(nil), head...)
			}
			records = append(records, preservedRecord{i + 1, "comment", fingerprint})
			continue
		}
		if head[0] == '[' {
			fingerprint, trusted := corpusPreprocessorFingerprintMode(head, options.allowFullwidthSpace)
			if !trusted {
				fingerprint = append([]byte(nil), body...)
			}
			records = append(records, preservedRecord{i + 1, "preprocessor", fingerprint})
			continue
		}
		if head[0] == '@' {
			value := head
			if options.allowFullwidthSpace {
				value = normalizeFullwidthExpressionSpaces(value)
			}
			records = append(records, preservedRecord{i + 1, "function-header", tokenFingerprint(value)})
			continue
		}
		if head[0] == '$' {
			value := head
			if options.allowFullwidthSpace {
				value = normalizeFullwidthExpressionSpaces(value)
			}
			records = append(records, preservedRecord{i + 1, "goto-label", tokenFingerprint(value)})
			continue
		}
		if head[0] == '#' {
			value := head
			if isDefineDirective(head) {
				records = append(records, preservedRecord{i + 1, "define", defineFingerprintWithFullwidth(value, options.allowFullwidthSpace)})
			} else if !corpusKnownSharpDeclaration(head) {
				if corpusUnknownSharpNeedsExact(head) {
					value = trimGrammarSpaceRight(value, options.allowFullwidthSpace)
					records = append(records, preservedRecord{i + 1, "declaration", value})
				} else {
					if options.allowFullwidthSpace {
						value = normalizeFullwidthExpressionSpaces(value)
					}
					records = append(records, preservedRecord{i + 1, "declaration", tokenFingerprint(value)})
				}
			} else {
				if options.allowFullwidthSpace {
					value = normalizeFullwidthExpressionSpaces(value)
				}
				records = append(records, preservedRecord{i + 1, "declaration", tokenFingerprint(value)})
			}
			continue
		}
		route := statementRouteForHead(head, dialect, options.allowFullwidthSpace)
		kind, fingerprint := route.kind, route.fingerprint
		if route.unknownFallback {
			kind, fingerprint = "unknown", append([]byte(nil), body...)
		}
		records = append(records, preservedRecord{i + 1, kind, fingerprint})
	}
	if continuation && options.continuationSeparator != nil && len(continuationRows) != 0 {
		records = append(records, preservedRecord{continuationStart + 1, "continuation", continuationFingerprint(continuationRows, *options.continuationSeparator, dialect, options.allowFullwidthSpace)})
	}
	return records
}

func statementRouteForHead(head []byte, dialect format.Dialect, allowFullwidth bool) statementRoute {
	word, end := firstWord(head)
	upper := strings.ToUpper(string(word))
	route := statementRoute{
		end:   end,
		upper: upper,
	}
	knownStatement := end > 0 && isKnownStatementCommandDialect(upper, dialect) && (end == len(head) || head[end] == ' ' || head[end] == '\t' || head[end] == ';' || allowFullwidth && bytes.HasPrefix(head[end:], []byte("\xE3\x80\x80")))
	knownForm := knownStatement && isFormCommandDialect(upper, dialect)
	if !knownForm && (!knownStatement || corpusDialectRestrictedCommand(upper) || isNoArgCommandDialect(upper, dialect)) {
		if fingerprint, ok := assignmentPreservationFingerprint(head, allowFullwidth); ok {
			route.kind, route.fingerprint = "assignment", fingerprint
			return route
		}
	}
	if !knownStatement {
		if fingerprint, ok := incrementFingerprintWithFullwidth(head, allowFullwidth); ok {
			route.kind, route.fingerprint = "increment", fingerprint
			return route
		}
	}
	if end == 0 {
		route.kind, route.unknownFallback = "unknown", true
		return route
	}
	route.kind, route.fingerprint = statementFingerprintDialectWithFullwidth(upper, word, head[end:], dialect, allowFullwidth)
	if route.kind == "" {
		route.kind, route.unknownFallback = "unknown", true
		return route
	}
	if route.kind == "command" && !knownStatement {
		route.unknownFallback = true
	}
	return route
}

func trimGrammarSpace(src []byte, allowFullwidth bool) []byte {
	for {
		src = bytes.TrimLeft(src, " \t")
		if !allowFullwidth || !bytes.HasPrefix(src, []byte("\xE3\x80\x80")) {
			return src
		}
		src = src[3:]
	}
}

func trimGrammarSpaceBoth(src []byte, allowFullwidth bool) []byte {
	src = bytes.Trim(src, " \t")
	if !allowFullwidth {
		return src
	}
	for {
		changed := false
		if bytes.HasPrefix(src, []byte("\xE3\x80\x80")) {
			src = src[3:]
			changed = true
		}
		if bytes.HasSuffix(src, []byte("\xE3\x80\x80")) {
			src = src[:len(src)-3]
			changed = true
		}
		if !changed {
			return src
		}
		src = bytes.Trim(src, " \t")
	}
}

func trimCorpusSpace(src []byte) []byte {
	return bytes.Trim(src, " \t")
}

func corpusPreprocessorFingerprintMode(src []byte, allowFullwidth bool) ([]byte, bool) {
	raw := func() ([]byte, bool) {
		return append([]byte(nil), src...), false
	}
	if len(src) < 2 || src[0] != '[' {
		return raw()
	}
	close := bytes.IndexByte(src, ']')
	if close < 0 {
		return raw()
	}
	first, second, count := corpusPreprocessorFields(src[1:close], allowFullwidth)
	if count == 0 {
		return raw()
	}
	var name string
	expected := 1
	switch {
	case bytes.Equal(first, []byte("IF")):
		name, expected = "IF", 2
	case bytes.Equal(first, []byte("ELSEIF")):
		name, expected = "ELSEIF", 2
	case bytes.Equal(first, []byte("IF_DEBUG")):
		name = "IF_DEBUG"
	case bytes.Equal(first, []byte("IF_NDEBUG")):
		name = "IF_NDEBUG"
	case bytes.Equal(first, []byte("ELSE")):
		name = "ELSE"
	case bytes.Equal(first, []byte("ENDIF")):
		name = "ENDIF"
	case bytes.Equal(first, []byte("SKIPSTART")):
		name = "SKIPSTART"
	case bytes.Equal(first, []byte("SKIPEND")):
		name = "SKIPEND"
	default:
		return raw()
	}
	if name == "IF" || name == "ELSEIF" {
		expected = 2
		if count == expected && !corpusPreprocessorCondition(second) {
			return raw()
		}
	}
	if count != expected {
		return raw()
	}
	tail := trimGrammarSpace(src[close+1:], allowFullwidth)
	if len(tail) != 0 && (tail[0] != ';' || corpusExecutableMarker(tail) || bytes.Contains(tail, []byte("[["))) {
		return raw()
	}
	value := src
	if allowFullwidth {
		tailStart := len(src) - len(tail)
		prefix := normalizeFullwidthExpressionSpaces(src[:tailStart])
		value = make([]byte, 0, len(prefix)+len(tail))
		value = append(value, prefix...)
		value = append(value, tail...)
	}
	return tokenFingerprint(value), true
}

func corpusPreprocessorFields(src []byte, allowFullwidth bool) (first, second []byte, count int) {
	spaceSize := func(pos int) int {
		if src[pos] == ' ' || src[pos] == '\t' {
			return 1
		}
		if allowFullwidth && bytes.HasPrefix(src[pos:], []byte("\xE3\x80\x80")) {
			return 3
		}
		return 0
	}
	pos := 0
	for pos < len(src) {
		size := spaceSize(pos)
		if size == 0 {
			break
		}
		pos += size
	}
	if pos == len(src) {
		return nil, nil, 0
	}
	start := pos
	for pos < len(src) && spaceSize(pos) == 0 {
		pos++
	}
	first, count = src[start:pos], 1
	for pos < len(src) {
		size := spaceSize(pos)
		if size == 0 {
			break
		}
		pos += size
	}
	if pos == len(src) {
		return first, nil, count
	}
	start = pos
	for pos < len(src) && spaceSize(pos) == 0 {
		pos++
	}
	second, count = src[start:pos], 2
	for pos < len(src) {
		size := spaceSize(pos)
		if size == 0 {
			return first, second, 3
		}
		pos += size
	}
	return first, second, count
}

func corpusExecutableMarker(src []byte) bool {
	return len(src) >= 3 && src[0] == ';' && (src[1] == '!' || src[1] == '#') && src[2] == ';'
}

func mandatoryMarkerSpan(src []byte, start int) int {
	rest := trimLeadingGrammarSpace(src[start:])
	if !bytes.HasPrefix(rest, []byte(";!;")) {
		return start
	}
	return len(src) - len(trimLeadingGrammarSpace(rest[3:]))
}

func corpusPreprocessorCondition(src []byte) bool {
	if len(src) == 0 {
		return false
	}
	for len(src) != 0 {
		r, size := utf8.DecodeRune(src)
		if r == '\u3000' || r == ' ' || r == '\t' || strings.ContainsRune(".+-*/%=!<>|&^~?#)}],:({[$\\'\"@;", r) {
			return false
		}
		src = src[size:]
	}
	return true
}

func statementFingerprintDialectWithFullwidth(command string, word, tail []byte, dialect format.Dialect, allowFullwidth bool) (string, []byte) {
	if nul := corpusStatementNulBoundary(command, tail, dialect, allowFullwidth); nul >= 0 {
		kind, prefix := statementPrefixFingerprintDialectWithFullwidth(command, word, tail[:nul], dialect, allowFullwidth)
		return kind, appendPreservedField(appendPreservedField([]byte{'N'}, prefix), tail[nul:])
	}
	return statementPrefixFingerprintDialectWithFullwidth(command, word, tail, dialect, allowFullwidth)
}

func statementPrefixFingerprintDialectWithFullwidth(command string, word, tail []byte, dialect format.Dialect, allowFullwidth bool) (string, []byte) {
	if corpusUnavailableCommandForFingerprint(command, dialect) {
		return "unknown", append(append([]byte(nil), word...), tail...)
	}
	if allowFullwidth {
		tail = normalizeFullwidthCommandGap(tail)
		if isFormCallCommand(command) {
			return "form-call", joinFingerprint(word, formCallFingerprintFullwidthDialect(tail, dialect))
		}
		if isPlainCallCommand(command) {
			return "expression", joinFingerprint(word, callFingerprintModeDialect(tail, true, dialect, false))
		}
		deferredSaveVar := command == "SAVEVAR" && corpusSAVEVARTailMalformedDeferredFallback(tail, dialect, true)
		if corpusStatementUsesExpressionProof(command, dialect) && corpusStatementTailNeedsExact(command, tail, dialect, true) && !commandTailPreservationProof(command, tail, dialect, true) && !deferredSaveVar {
			if isPrintVCommandDialect(command, dialect) && bytes.IndexByte(tail, 0) >= 0 && !printVNumericLiteralNeedsExact(tail) && !printVBoundaryPrefixNeedsExact(tail, dialect) {
				return "print", joinFingerprint(word, printVFingerprintDialect(tail, true, dialect))
			}
			if mandatoryMarkerIndex(tail) >= 0 {
				return "unknown", joinFingerprint(word, expressionFingerprintDialect(normalizeFullwidthExpressionSpacesDialect(tail, dialect), dialect))
			}
			if isPrintVCommandDialect(command, dialect) {
				tail = normalizeUnknownPrintVCommandGap(tail)
			} else {
				tail = normalizeUnknownKnownCommandGap(tail)
			}
			return "unknown", append(append([]byte(nil), word...), tail...)
		}
		kind, _, _, structured := corpusStructuredCommand(command, dialect)
		_, _, _, bounded := corpusCommandArity(command, dialect)
		tail = normalizeTrustedCommandGapWithArity(tail, bounded, structured && kind != "form-any", isGraphicsCommand(command))
		if command == "SAVEVAR" {
			return "expression", joinFingerprint(word, saveVarFingerprint(tail, dialect, true))
		}
		if isInputCommand(command, dialect) {
			normalized := normalizeFullwidthInputTail(command, tail, dialect)
			if fullwidthTailNeedsExact(command, tail, dialect) {
				return "unknown", append(append([]byte(nil), word...), tail...)
			}
			tail = normalized
		} else if isInputExpressionCommand(command, dialect) {
			normalized := normalizeFullwidthInputExpressionTail(command, tail, dialect)
			if fullwidthTailNeedsExact(command, tail, dialect) {
				return "unknown", append(append([]byte(nil), word...), tail...)
			}
			tail = normalized
		} else if kind, min, max, ok := corpusStructuredCommand(command, dialect); ok {
			if kind == "form-any" {
				tail = normalizeFullwidthStructuredFormTail(tail, min)
			} else {
				normalized := normalizeFullwidthStructuredExpressionTail(command, tail, min, max)
				if fullwidthTailNeedsExact(command, tail, dialect) {
					return "unknown", append(append([]byte(nil), word...), tail...)
				}
				tail = normalized
			}
			return kind, joinFingerprint(word, structuredCommandFingerprintDialect(command, tail, kind, min, max, dialect))
		} else if isGraphicsCommand(command) {
			return "graphics", joinFingerprint(word, graphicsFingerprintMode(command, tail, dialect, true))
		} else if command == "SPLIT" || command == "SWAP" {
			tail = normalizeFullwidthExpressionSpaces(tail)
		} else if isPrintVCommandDialect(command, dialect) {
			tail = normalizeFullwidthPrintVTail(tail)
			tail = normalizeFullwidthPrintVPlainTail(tail)
		} else if command == "STRFORM" {
			if fingerprint, ok := directSTRFORMLiteralFingerprintDialect(tail, allowFullwidth, dialect); ok {
				return "strform-literal", joinFingerprint(word, fingerprint)
			}
			if !printFormsTailOpaque(tail) {
				tail = normalizeFullwidthExpressionSpaces(tail)
			}
		} else if isPrintFormsCommandDialect(command, dialect) {
			if !printFormsTailOpaque(tail) {
				tail = normalizeFullwidthPrintFormsTail(tail)
			}
		} else if isFormCommandDialect(command, dialect) {
			tail = normalizeFullwidthFormPayloadDialect(tail, dialect)
		} else if isExpressionCommand(command) || isExpressionLikeCommand(command) || isNoArgCommandDialect(command, dialect) || isMixedExpressionCommand(command) || isArrayCommand(command) || isKeywordCommand(command) || corpusBuiltinMethod(command, dialect) || isExtraVariableCommand(command) || command == "TIMES" || command == "VARSIZE" || command == "DT_COLUMN_OPTIONS" {
			normalized := normalizeFullwidthCommandExpression(command, tail, dialect)
			if fullwidthTailNeedsExact(command, tail, dialect) {
				return "unknown", append(append([]byte(nil), word...), tail...)
			}
			tail = normalized
		}
	}
	return statementFingerprintDialect(command, word, tail, dialect)
}

func normalizeTrustedCommandGap(command string, tail []byte, dialect format.Dialect) []byte {
	_, _, _, structured := corpusStructuredCommand(command, dialect)
	_, _, _, bounded := corpusCommandArity(command, dialect)
	return normalizeTrustedCommandGapWithArity(tail, bounded, structured, isGraphicsCommand(command))
}

func normalizeTrustedCommandGapWithArity(tail []byte, bounded, structured, graphics bool) []byte {
	if !bounded && !structured && !graphics {
		return tail
	}
	body := trimLeadingGrammarSpace(tail)
	if len(body) == 0 || len(body) == len(tail) || body[0] == ';' || body[0] == ',' {
		return tail
	}
	return append([]byte{' '}, body...)
}

func normalizeUnknownKnownCommandGap(tail []byte) []byte {
	body := trimLeadingGrammarSpace(tail)
	if len(body) == len(tail) || len(body) == 0 || body[0] == ';' || body[0] == ',' {
		return tail
	}
	return append([]byte{' '}, body...)
}

func normalizeUnknownPrintVCommandGap(tail []byte) []byte {
	body := trimGrammarSpace(tail, true)
	if len(body) == len(tail) || len(body) == 0 {
		return tail
	}
	out := make([]byte, 0, len(body)+1)
	out = append(out, ' ')
	return append(out, body...)
}

func commandTailPreservationProof(command string, tail []byte, dialect format.Dialect, allowFullwidth bool) bool {
	printV := isPrintVCommandDialect(command, dialect)
	if printV {
		if !printVHasExpression(tail) {
			_, ok := printVPlainTailFingerprintDialect(tail, allowFullwidth, dialect)
			return ok
		}
		if allowFullwidth {
			tail = normalizeFullwidthExpressionSpacesDialect(tail, dialect)
		}
		return bytes.Equal(expressionFingerprintDialect(tail, dialect), tokenFingerprintDialect(tail, dialect)) && printVHasNumericIdentifierBoundary(tail)
	}
	if command != "SAVEVAR" {
		return false
	}
	if allowFullwidth {
		tail = normalizeFullwidthExpressionSpacesDialect(tail, dialect)
	}
	if !bytes.Equal(expressionFingerprintDialect(tail, dialect), tokenFingerprintDialect(tail, dialect)) {
		return false
	}
	return printVHasNumericIdentifierBoundary(tail)
}

func fullwidthTailNeedsExact(command string, tail []byte, dialect format.Dialect) bool {
	if (command == "SETCOLOR" || command == "SETBGCOLOR") && !corpusStatementTailNeedsExact(command, tail, dialect, true) {
		return false
	}
	comment := corpusCommandCommentStart(tail)
	fields := splitFingerprintFields(tail[:comment], topLevelCommas(tail[:comment]))
	if len(fields) > 1 && len(trimGrammarSpaceBoth(fields[len(fields)-1], true)) == 0 {
		allowTrailing := corpusStructuredAllowsTrailing(command) || corpusInputAllowsTrailing(command, dialect)
		if _, _, allowed, ok := corpusCommandArity(command, dialect); ok {
			allowTrailing = allowTrailing || allowed
		}
		if allowTrailing {
			fields = fields[:len(fields)-1]
		}
	}
	for i, field := range fields {
		if corpusCommandFieldType(command, i, dialect) != corpusValueNumeric {
			continue
		}
		candidate := normalizeFullwidthExpressionSpaces(field)
		if !graphicsNumericExpressionValid(trimCorpusSpace(candidate), dialect == format.EMEE) || len(tokenFingerprintCore(trimCorpusSpace(candidate))) == 0 {
			return true
		}
	}
	return false
}

func normalizeFullwidthCommandExpression(command string, tail []byte, dialect format.Dialect) []byte {
	comment := incrementCommentStart(tail)
	if comment < 0 || comment > len(tail) {
		comment = len(tail)
	}
	fields := splitFingerprintFields(tail[:comment], topLevelCommas(tail[:comment]))
	code := tail[:comment]
	if len(trimGrammarSpaceBoth(code, true)) == 0 {
		fields = nil
	}
	trailing := len(fields) > 1 && len(trimGrammarSpaceBoth(fields[len(fields)-1], true)) == 0
	if trailing {
		fields = fields[:len(fields)-1]
	}
	valid := true
	if min, max, allowTrailing, ok := corpusCommandArity(command, dialect); ok {
		if trailing && !allowTrailing || len(fields) < min || len(fields) > max {
			return tail
		}
		for _, field := range fields {
			if len(trimGrammarSpaceBoth(field, true)) == 0 {
				return tail
			}
		}
		if command == "TIMES" && (len(fields) != 2 || !validTimesCoefficientFingerprint(trimGrammarSpaceBoth(fields[1], true))) {
			return tail
		}
		if !corpusCommandFieldTypesValid(command, fields, dialect, true) {
			return tail
		}
	}
	switch command {
	case "SETCOLOR", "SETBGCOLOR":
		valid = len(fields) == 1 || len(fields) == 3
	case "PRINTBUTTON", "PRINTBUTTONC", "PRINTBUTTONLC":
		valid = len(fields) == 2
	case "ARRAYSORT":
		valid = len(fields) >= 1 && len(fields) <= 4
	}
	if !valid {
		return tail
	}
	if _, _, _, bounded := corpusCommandArity(command, dialect); bounded {
		for _, field := range fields {
			if len(trimGrammarSpaceBoth(field, true)) == 0 {
				return tail
			}
		}
	}
	return normalizeFullwidthExpressionSpacesDialect(tail, dialect)
}

func normalizeFullwidthCommandGap(src []byte) []byte {
	if !bytes.HasPrefix(src, []byte("\xE3\x80\x80")) || mandatoryMarkerSpan(src, 0) > 0 {
		return src
	}
	out := make([]byte, 0, len(src))
	out = append(out, ' ')
	return append(out, src[3:]...)
}

func assignmentPreservationFingerprint(src []byte, allowFullwidth bool) ([]byte, bool) {
	index, operator := assignmentOperator(src)
	if index < 0 {
		return nil, false
	}
	left := src[:index]
	if allowFullwidth {
		left = normalizeFullwidthExpressionSpaces(left)
	}
	if !assignmentLHS(left) {
		return nil, false
	}
	if !allowFullwidth {
		return assignmentFingerprint(src, index, operator), true
	}
	right := src[index+operator:]
	if bytes.Equal(src[index:index+operator], []byte("=")) || bytes.Equal(src[index:index+operator], []byte("==")) {
		right = bytes.TrimLeft(right, " \t")
	} else {
		right = normalizeFullwidthExpressionSpaces(right)
	}
	canonical := make([]byte, 0, len(left)+operator+len(right))
	canonical = append(canonical, left...)
	canonical = append(canonical, src[index:index+operator]...)
	canonical = append(canonical, right...)
	return assignmentFingerprint(canonical, len(left), operator), true
}

func incrementFingerprintWithFullwidth(src []byte, allowFullwidth bool) ([]byte, bool) {
	if !allowFullwidth {
		return incrementFingerprint(src)
	}
	comment := incrementCommentStart(src)
	code := normalizeFullwidthExpressionSpaces(src[:comment])
	canonical := append(append([]byte(nil), code...), src[comment:]...)
	return incrementFingerprint(canonical)
}

func trimLeadingGrammarSpace(src []byte) []byte {
	for {
		switch {
		case len(src) != 0 && (src[0] == ' ' || src[0] == '\t'):
			src = src[1:]
		case bytes.HasPrefix(src, []byte("\xE3\x80\x80")):
			src = src[3:]
		default:
			return src
		}
	}
}

func defineFingerprintWithFullwidth(src []byte, allowFullwidth bool) []byte {
	if !allowFullwidth {
		return defineFingerprint(src)
	}
	const keyword = "#DEFINE"
	nameStart := len(keyword)
	for nameStart < len(src) {
		if src[nameStart] == ' ' || src[nameStart] == '\t' {
			nameStart++
			continue
		}
		if bytes.HasPrefix(src[nameStart:], []byte("\xE3\x80\x80")) {
			nameStart += 3
			continue
		}
		break
	}
	nameEnd := nameStart
	for nameEnd < len(src) && isFingerprintWord(src[nameEnd]) {
		if bytes.HasPrefix(src[nameEnd:], []byte("\xE3\x80\x80")) {
			break
		}
		nameEnd++
	}
	if nameStart == len(keyword) || nameEnd == nameStart || nameEnd < len(src) && src[nameEnd] != ' ' && src[nameEnd] != '\t' && !bytes.HasPrefix(src[nameEnd:], []byte("\xE3\x80\x80")) {
		return append([]byte(nil), src...)
	}
	bodyStart := nameEnd
	for bodyStart < len(src) {
		if src[bodyStart] == ' ' || src[bodyStart] == '\t' {
			bodyStart++
			continue
		}
		if bytes.HasPrefix(src[bodyStart:], []byte("\xE3\x80\x80")) {
			bodyStart += 3
			continue
		}
		break
	}
	if bodyStart == len(src) {
		bodyStart = nameEnd
	}
	out := append([]byte(nil), src[:len(keyword)]...)
	out = append(out, 0)
	out = append(out, src[nameStart:nameEnd]...)
	out = append(out, 0)
	return append(out, src[bodyStart:]...)
}

func normalizeFullwidthPrintFormsTail(tail []byte) []byte {
	return normalizeFullwidthPrintFormsTailDialect(tail, format.CommonDialect)
}

func normalizeFullwidthPrintFormsTailDialect(tail []byte, dialect format.Dialect) []byte {
	start := 0
	for start < len(tail) {
		if tail[start] == ' ' || tail[start] == '\t' {
			start++
			continue
		}
		if bytes.HasPrefix(tail[start:], []byte("\xE3\x80\x80")) {
			start += 3
			continue
		}
		break
	}
	if start >= len(tail) || tail[start] != '"' {
		return normalizeFullwidthExpressionSpacesDialect(tail, dialect)
	}
	end := printVQuotedEnd(tail, start)
	if end <= start+1 || end > len(tail) || tail[end-1] != '"' {
		return normalizeFullwidthFormPayloadDialect(tail, dialect)
	}
	out := append(normalizeFullwidthExpressionBodyDialect(tail[:start], dialect), '"')
	out = append(out, normalizeFullwidthFormPayloadDialect(tail[start+1:end-1], dialect)...)
	out = append(out, '"')
	rest := tail[end:]
	gap := 0
	for gap < len(rest) {
		if rest[gap] == ' ' || rest[gap] == '\t' {
			gap++
			continue
		}
		if bytes.HasPrefix(rest[gap:], []byte("\xE3\x80\x80")) {
			gap += 3
			continue
		}
		break
	}
	if gap < len(rest) && rest[gap] == ';' {
		out = append(out, ' ')
		return append(out, rest[gap:]...)
	}
	if gap < len(rest) {
		return normalizeFullwidthExpressionSpacesDialect(tail, dialect)
	}
	return append(out, normalizeFullwidthExpressionSpacesDialect(rest, dialect)...)
}

func normalizeFullwidthExpressionSpaces(src []byte) []byte {
	return normalizeFullwidthExpressionSpacesDialect(src, format.CommonDialect)
}

func normalizeFullwidthExpressionSpacesDialect(src []byte, dialect format.Dialect) []byte {
	if comment := incrementCommentStart(src); comment >= 0 && comment < len(src) {
		out := normalizeFullwidthExpressionBodyDialect(src[:comment], dialect)
		return append(out, src[comment:]...)
	}
	return normalizeFullwidthExpressionBodyDialect(src, dialect)
}

func normalizeFullwidthExpressionBodyDialect(src []byte, dialect format.Dialect) []byte {
	out := make([]byte, 0, len(src))
	for i := 0; i < len(src); {
		if end := mandatoryMarkerSpan(src, i); end > i {
			out = append(out, src[i:end]...)
			i = end
			continue
		}
		if rest := trimLeadingGrammarSpace(src[i:]); len(rest) != len(src)-i {
			end := len(src) - len(rest)
			for i < end {
				if bytes.HasPrefix(src[i:], []byte("\u3000")) {
					out = append(out, ' ')
					i += 3
				} else {
					out = append(out, src[i])
					i++
				}
			}
			continue
		}
		if src[i] == '@' && i+1 < len(src) && src[i+1] == '"' {
			end := quotedFormEnd(src, i)
			if end > i+2 && end <= len(src) {
				out = append(out, normalizeFullwidthFormAtomDialect(src[i:end], dialect)...)
				i = end
				continue
			}
		}
		if src[i] == 0 {
			if !corpusNulEmbeddedInIdentifier(src, i) {
				out = append(out, src[i:]...)
				return out
			}
			out = append(out, src[i])
			i++
			continue
		}
		if src[i] == '\\' && i+1 < len(src) && src[i+1] == '@' {
			_, _, end, ok := formConditional(src, i)
			if ok {
				out = append(out, normalizeFullwidthConditionalDialect(src[i:end+2], dialect)...)
				i = end + 2
				continue
			}
		}
		if src[i] == '[' && i+1 < len(src) && src[i+1] == '[' {
			if end := bytes.Index(src[i+2:], []byte("]]")); end >= 0 {
				end += i + 4
				out = append(out, src[i:end]...)
				i = end
				continue
			}
		}
		if src[i] == '"' || src[i] == '\'' {
			quote := src[i]
			start := i
			i++
			for i < len(src) {
				if src[i] == '\\' && i+1 < len(src) {
					i += 2
					continue
				}
				i++
				if src[i-1] == quote {
					break
				}
			}
			out = append(out, src[start:i]...)
			continue
		}
		out = append(out, src[i])
		i++
	}
	return out
}

func normalizeFullwidthFormAtom(src []byte) []byte {
	return normalizeFullwidthFormAtomDialect(src, format.CommonDialect)
}

func normalizeFullwidthFormAtomDialect(src []byte, dialect format.Dialect) []byte {
	end, complete := quotedFormEndComplete(src, 0)
	if !complete || end != len(src) || end < 3 || bytes.Contains(src[2:end-1], []byte("\\")) {
		return append([]byte(nil), src...)
	}
	out := append([]byte{'@', '"'}, normalizeFullwidthFormPayloadDialect(src[2:end-1], dialect)...)
	return append(out, '"')
}

func normalizeFullwidthConditionalDialect(src []byte, dialect format.Dialect) []byte {
	question, hash, end, ok := formConditional(src, 0)
	if !ok || end+2 != len(src) {
		return append([]byte(nil), src...)
	}
	condition := src[2:question]
	if !formConditionalNeedsExactDialect(src, dialect) {
		condition = normalizeFullwidthExpressionBodyDialect(condition, dialect)
	}
	out := append([]byte{'\\', '@'}, condition...)
	out = append(out, '?')
	if hash >= 0 {
		out = append(out, normalizeFullwidthFormPayloadDialect(src[question+1:hash], dialect)...)
		out = append(out, '#')
		out = append(out, normalizeFullwidthFormPayloadDialect(src[hash+1:end], dialect)...)
	} else {
		out = append(out, normalizeFullwidthFormPayloadDialect(src[question+1:end], dialect)...)
	}
	return append(out, '\\', '@')
}

func printFormsTailOpaque(tail []byte) bool {
	start := 0
	for start < len(tail) && (tail[start] == ' ' || tail[start] == '\t') {
		start++
	}
	if start >= len(tail) || tail[start] != '"' {
		return false
	}
	end := printVQuotedEnd(tail, start)
	if end <= start+1 || end > len(tail) || tail[end-1] != '"' {
		return true
	}
	return bytes.Contains(tail[start+1:end-1], []byte("\\")) || mandatoryMarkerIndex(tail[end:]) >= 0
}

func formConditionalNeedsExactDialect(src []byte, dialect format.Dialect) bool {
	question, _, end, ok := formConditional(src, 0)
	if !ok || end+2 != len(src) {
		return true
	}
	condition := normalizeFullwidthExpressionBodyDialect(src[2:question], dialect)
	return !corpusConditionalConditionValidDialect(condition, dialect)
}

func corpusConditionalConditionValidDialect(src []byte, dialect format.Dialect) bool {
	src = corpusStripOuterParens(src)
	if len(src) == 0 {
		return false
	}
	if corpusExpressionHasKnownStringIndex(src, dialect) && !corpusStandaloneStringIndex(src, dialect) && !corpusExpressionHasDeferredOperand(src, dialect) && !corpusComparisonExpressionValid(src, dialect) && !corpusIndexedStringCallExpressionSafe(src, dialect) && !(corpusConditionHasComparison(src) && !corpusKnownMixedExpressionNeedsExact(src, dialect) && !corpusExpressionHasInvalidStringIndex(src, dialect)) {
		return false
	}
	if corpusKnownMixedExpressionNeedsExact(src, dialect) && !corpusComparisonExpressionValid(src, dialect) {
		return false
	}
	if graphicsNumericExpressionValid(src, false) {
		return corpusProvenValueType(src, dialect) != corpusValueString
	}
	if corpusExpressionHasDeferredOperand(src, dialect) {
		return true
	}
	if graphicsNumericExpressionValid(src, true) && corpusConditionHasComparison(src) {
		return true
	}
	return corpusConditionalMacroShapeDialect(src, dialect)
}

func corpusKnownMixedExpressionNeedsExact(src []byte, dialect format.Dialect) bool {
	src = corpusStripOuterParens(src)
	parts := make([][]byte, 0, 2)
	start := 0
	depth := 0
	for i := 0; i < len(src); i++ {
		switch src[i] {
		case '"', '\'':
			i = printVQuotedEnd(src, i) - 1
		case '@':
			if i+1 < len(src) && src[i+1] == '"' {
				if end := quotedFormEnd(src, i); end > i+2 && end <= len(src) {
					i = end - 1
				}
			}
		case '\\':
			if i+1 < len(src) && src[i+1] == '@' {
				if _, _, end, ok := formConditional(src, i); ok {
					i = end + 1
				}
			}
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
		default:
			if depth != 0 || !strings.ContainsRune("+-*/%&|^<>!=", rune(src[i])) {
				continue
			}
			if i > start && (src[i] == '+' || src[i] == '-') && src[i-1] == src[i] {
				continue
			}
			if i+1 < len(src) && ((src[i] == '<' || src[i] == '>' || src[i] == '!') && (src[i+1] == '=' || src[i+1] == src[i]) || src[i] == '=' && src[i+1] == '=') {
				i++
			}
			parts = append(parts, src[start:i])
			start = i + 1
		}
	}
	if len(parts) == 0 {
		return false
	}
	parts = append(parts, src[start:])
	hasNumeric, hasString := false, false
	for _, part := range parts {
		part = corpusStripOuterParens(part)
		if corpusExpressionHasQuotedFormOperand(part) || bytes.IndexByte(part, 0) >= 0 {
			return false
		}
		switch corpusExpressionValueType(part, dialect) {
		case corpusValueNumeric:
			hasNumeric = true
		case corpusValueString:
			hasString = true
		case corpusValueUnknown:
			return false
		case corpusValueInvalid:
			return true
		}
	}
	return hasNumeric && hasString
}

func corpusComparisonExpressionValid(src []byte, dialect format.Dialect) bool {
	src = corpusStripOuterParens(src)
	if len(src) == 0 {
		return false
	}
	if parts := corpusTopLevelLogicalParts(src); len(parts) > 1 {
		for _, part := range parts {
			if !corpusComparisonExpressionValid(part, dialect) {
				return false
			}
		}
		return true
	}
	if src[0] == '!' && (len(src) == 1 || src[1] != '=') {
		operand := trimCorpusSpace(src[1:])
		return len(operand) != 0 && corpusExpressionValueType(operand, dialect) == corpusValueNumeric
	}
	if parts := corpusTopLevelLogicalParts(src); len(parts) > 1 {
		for _, part := range parts {
			if !corpusComparisonExpressionValid(part, dialect) {
				return false
			}
		}
		return true
	}
	parts := make([][]byte, 0, 2)
	start, depth := 0, 0
	for i := 0; i < len(src); i++ {
		switch src[i] {
		case '"', '\'':
			i = printVQuotedEnd(src, i) - 1
		case '@':
			if i+1 < len(src) && src[i+1] == '"' {
				if end := quotedFormEnd(src, i); end > i+2 && end <= len(src) {
					i = end - 1
				}
			}
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 && i+1 < len(src) && (bytes.Equal(src[i:i+2], []byte("&&")) || bytes.Equal(src[i:i+2], []byte("||"))) {
				parts = append(parts, src[start:i])
				i++
				start = i + 1
			}
		}
	}
	if len(parts) != 0 {
		parts = append(parts, src[start:])
		for _, part := range parts {
			if !corpusComparisonExpressionValid(part, dialect) {
				return false
			}
		}
		return true
	}
	depth = 0
	for i := 0; i < len(src); i++ {
		switch src[i] {
		case '"', '\'':
			i = printVQuotedEnd(src, i) - 1
		case '@':
			if i+1 < len(src) && src[i+1] == '"' {
				if end := quotedFormEnd(src, i); end > i+2 && end <= len(src) {
					i = end - 1
				}
			}
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
		default:
			if depth != 0 || !strings.ContainsRune("<>=!", rune(src[i])) {
				continue
			}
			if i > 0 && strings.ContainsRune("<>=!", rune(src[i-1])) {
				continue
			}
			opStart, opLen := i, 1
			if src[i] == '!' && (i+1 >= len(src) || src[i+1] != '=') || (src[i] == '<' || src[i] == '>') && i+1 < len(src) && src[i+1] == src[i] {
				return false
			}
			if i+1 < len(src) && (src[i+1] == '=' || src[i+1] == src[i] && (src[i] == '<' || src[i] == '>')) {
				opLen = 2
			}
			if opLen == 1 && src[i] == '=' {
				return false
			}
			left := trimCorpusSpace(src[:opStart])
			rightStart := opStart + opLen
			right := trimCorpusSpace(src[rightStart:])
			if len(left) == 0 || len(right) == 0 {
				return false
			}
			lt, rt := corpusExpressionValueType(left, dialect), corpusExpressionValueType(right, dialect)
			if lt == corpusValueInvalid || rt == corpusValueInvalid || lt != corpusValueUnknown && rt != corpusValueUnknown && lt != rt {
				return false
			}
			return true
		}
	}
	return corpusExpressionValueType(src, dialect) == corpusValueNumeric && graphicsNumericExpressionValid(src, dialect == format.EMEE)
}

func corpusComparisonExpressionHasKnownOperands(src []byte, dialect format.Dialect) bool {
	src = corpusStripOuterParens(src)
	if len(src) == 0 {
		return false
	}
	if parts := corpusTopLevelLogicalParts(src); len(parts) > 1 {
		for _, part := range parts {
			if corpusComparisonExpressionHasKnownOperands(part, dialect) {
				return true
			}
		}
		return false
	}
	if src[0] == '!' && (len(src) == 1 || src[1] != '=') {
		operand := trimCorpusSpace(src[1:])
		return len(operand) != 0 && corpusExpressionValueType(operand, dialect) == corpusValueNumeric
	}
	if parts := corpusTopLevelLogicalParts(src); len(parts) > 1 {
		for _, part := range parts {
			if corpusComparisonExpressionHasKnownOperands(part, dialect) {
				return true
			}
		}
		return false
	}
	start, depth := 0, 0
	for i := 0; i < len(src); i++ {
		switch src[i] {
		case '"', '\'':
			i = printVQuotedEnd(src, i) - 1
		case '@':
			if i+1 < len(src) && src[i+1] == '"' {
				if end := quotedFormEnd(src, i); end > i+2 && end <= len(src) {
					i = end - 1
				}
			}
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 && i+1 < len(src) && (bytes.Equal(src[i:i+2], []byte("&&")) || bytes.Equal(src[i:i+2], []byte("||"))) {
				if corpusComparisonExpressionHasKnownOperands(src[start:i], dialect) || corpusComparisonExpressionHasKnownOperands(src[i+2:], dialect) {
					return true
				}
				return false
			}
		}
	}
	depth = 0
	for i := 0; i < len(src); i++ {
		switch src[i] {
		case '"', '\'':
			i = printVQuotedEnd(src, i) - 1
		case '@':
			if i+1 < len(src) && src[i+1] == '"' {
				if end := quotedFormEnd(src, i); end > i+2 && end <= len(src) {
					i = end - 1
				}
			}
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
		default:
			if depth != 0 || !strings.ContainsRune("<>=!", rune(src[i])) {
				continue
			}
			if i > 0 && strings.ContainsRune("<>=!", rune(src[i-1])) {
				continue
			}
			opStart, opLen := i, 1
			if src[i] == '!' && (i+1 >= len(src) || src[i+1] != '=') || (src[i] == '<' || src[i] == '>') && i+1 < len(src) && src[i+1] == src[i] {
				return false
			}
			if i+1 < len(src) && src[i+1] == '=' {
				opLen = 2
			}
			if opLen == 1 && src[i] == '=' {
				return false
			}
			left := trimCorpusSpace(src[:opStart])
			right := trimCorpusSpace(src[opStart+opLen:])
			if len(left) == 0 || len(right) == 0 {
				return false
			}
			lt, rt := corpusExpressionValueType(left, dialect), corpusExpressionValueType(right, dialect)
			return lt != corpusValueUnknown && rt != corpusValueUnknown
		}
	}
	return corpusExpressionValueType(src, dialect) == corpusValueNumeric && graphicsNumericExpressionValid(src, dialect == format.EMEE)
}

func corpusTopLevelLogicalParts(src []byte) [][]byte {
	parts := make([][]byte, 0, 2)
	start, depth := 0, 0
	for i := 0; i < len(src); i++ {
		switch src[i] {
		case '"', '\'':
			i = printVQuotedEnd(src, i) - 1
		case '@':
			if i+1 < len(src) && src[i+1] == '"' {
				if end := quotedFormEnd(src, i); end > i+2 && end <= len(src) {
					i = end - 1
				}
			}
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 && i+1 < len(src) && (src[i : i+2][0] == '&' && src[i : i+2][1] == '&' || src[i : i+2][0] == '|' && src[i : i+2][1] == '|') {
				parts = append(parts, src[start:i])
				i++
				start = i + 1
			}
		}
	}
	if len(parts) == 0 {
		return nil
	}
	return append(parts, src[start:])
}

func corpusPercentMixedExpressionNeedsExact(src []byte, dialect format.Dialect) bool {
	if !corpusKnownMixedExpressionNeedsExact(src, dialect) {
		return false
	}
	if corpusStringConcatenationExpression(src, dialect) {
		return false
	}
	return !corpusPercentStringMultiplicationExpression(src, dialect)
}

func corpusStringConcatenationExpression(src []byte, dialect format.Dialect) bool {
	parts := corpusTopLevelBinaryParts(corpusStripOuterParens(src), '+')
	if len(parts) < 2 {
		return false
	}
	for _, part := range parts {
		if !corpusStringExpressionValue(part, dialect) {
			return false
		}
	}
	return true
}

func corpusStringExpressionValue(src []byte, dialect format.Dialect) bool {
	src = corpusStripOuterParens(src)
	if corpusExpressionValueType(src, dialect) == corpusValueString {
		return true
	}
	if corpusStringConcatenationExpression(src, dialect) {
		return true
	}
	parts := corpusTopLevelBinaryParts(src, '*')
	if len(parts) != 2 {
		return false
	}
	left, right := trimCorpusSpace(parts[0]), trimCorpusSpace(parts[1])
	leftNumeric := corpusExpressionValueType(left, dialect) == corpusValueNumeric
	rightNumeric := corpusExpressionValueType(right, dialect) == corpusValueNumeric
	return (corpusStringExpressionValue(left, dialect) && rightNumeric) || (leftNumeric && corpusStringExpressionValue(right, dialect))
}

func corpusPercentStringMultiplicationExpression(src []byte, dialect format.Dialect) bool {
	parts := corpusTopLevelBinaryParts(corpusStripOuterParens(src), '*')
	if len(parts) != 2 {
		return false
	}
	left, right := trimCorpusSpace(parts[0]), trimCorpusSpace(parts[1])
	leftNumeric := corpusExpressionValueType(left, dialect) == corpusValueNumeric
	rightNumeric := corpusExpressionValueType(right, dialect) == corpusValueNumeric
	return corpusIndexedStringMultiplicationExpression(src, dialect) || (corpusStringExpressionValue(left, dialect) && rightNumeric) || (leftNumeric && corpusStringExpressionValue(right, dialect))
}

func corpusIndexedStringMultiplicationExpression(src []byte, dialect format.Dialect) bool {
	parts := corpusTopLevelBinaryParts(corpusStripOuterParens(src), '*')
	if len(parts) != 2 {
		return false
	}
	left, right := trimCorpusSpace(parts[0]), trimCorpusSpace(parts[1])
	if !corpusLiteralStringIndex(left, dialect) || len(right) == 0 || printVNumericLiteralNeedsExact(src) || !graphicsNumericExpressionValid(right, false) || corpusExpressionHasNumericIdentifierGlue(right) || corpusExpressionLexicallyIncomplete(right) || corpusExpressionHasAssignmentOperator(right) || mandatoryMarkerIndex(right) >= 0 || bytes.IndexByte(right, 0) >= 0 || bytes.Contains(right, []byte("[[")) || corpusExpressionHasNestedNamedCall(right) {
		return false
	}
	if strings.ContainsAny(string(right), "\"',:@") || bytes.HasPrefix(right, []byte("++")) || bytes.HasPrefix(right, []byte("--")) {
		return false
	}
	return bytes.Equal(tokenFingerprintDialect(src, dialect), expressionFingerprintDialect(src, dialect))
}

func corpusIndexedStringFormExpressionSafe(src []byte, dialect format.Dialect) bool {
	src = corpusStripOuterParens(trimCorpusSpace(src))
	if len(src) == 0 || printVNumericLiteralNeedsExact(src) || !corpusExpressionHasKnownStringIndex(src, dialect) || !graphicsNumericExpressionValid(src, true) || corpusExpressionHasAssignmentOperator(src) || corpusExpressionLexicallyIncomplete(src) || corpusExpressionHasAdjacentOperands(src, dialect) || corpusExpressionHasMalformedCall(src, dialect, true) || corpusExpressionHasNestedNamedCall(src) {
		return false
	}
	if bytes.IndexByte(src, 0) >= 0 || bytes.Contains(src, []byte("[[")) || bytes.Contains(src, []byte("@")) || bytes.Contains(src, []byte("'")) || mandatoryMarkerIndex(src) >= 0 {
		return false
	}
	parts := corpusTopLevelBinaryParts(src, '+')
	if len(parts) < 2 {
		return false
	}
	indexed := false
	for _, part := range parts {
		part = trimCorpusSpace(part)
		if corpusLiteralStringIndex(part, dialect) {
			indexed = true
		}
	}
	return indexed && bytes.Equal(tokenFingerprintDialect(src, dialect), expressionFingerprintDialect(src, dialect))
}

func corpusLiteralStringIndex(src []byte, dialect format.Dialect) bool {
	src = corpusStripOuterParens(trimCorpusSpace(src))
	start := 0
	for start < len(src) && isFingerprintWord(src[start]) {
		start++
	}
	if start == 0 || !corpusStringBuiltin(string(src[:start]), dialect) {
		return false
	}
	for start < len(src) && corpusHalfSpace(src[start]) {
		start++
	}
	if start >= len(src) || src[start] != ':' {
		return false
	}
	index := trimCorpusSpace(src[start+1:])
	if len(index) == 0 {
		return false
	}
	for _, b := range index {
		if b < '0' || b > '9' {
			return false
		}
	}
	return true
}

func corpusExpressionValueType(src []byte, dialect format.Dialect) corpusValueType {
	src = trimCorpusSpace(src)
	value := corpusProvenValueType(src, dialect)
	if value != corpusValueUnknown {
		return value
	}
	if corpusStringIndexDeferredValue(src, dialect) {
		return corpusValueString
	}
	return corpusValueUnknown
}

func corpusStringIndexDeferredValue(src []byte, dialect format.Dialect) bool {
	src = corpusStripOuterParens(src)
	start := 0
	for start < len(src) && isFingerprintWord(src[start]) {
		start++
	}
	if start == 0 || !corpusStringBuiltin(string(src[:start]), dialect) {
		return false
	}
	for start < len(src) && corpusHalfSpace(src[start]) {
		start++
	}
	if start >= len(src) || src[start] != ':' {
		return false
	}
	start++
	for start < len(src) && corpusHalfSpace(src[start]) {
		start++
	}
	if start >= len(src) {
		return false
	}
	end := corpusStringIndexEnd(src, start)
	if end != len(src) {
		return false
	}
	index := corpusStripOuterParens(src[start:end])
	if len(index) == 0 {
		return false
	}
	value := corpusProvenValueType(index, dialect)
	return value == corpusValueNumeric
}

func corpusConditionalMacroShapeDialect(src []byte, dialect format.Dialect) bool {
	quoted := byte(0)
	plus := false
	unknown := false
	wordStart := -1
	flushWord := func(end int) {
		if wordStart < 0 {
			return
		}
		word := trimCorpusSpace(src[wordStart:end])
		if len(word) != 0 && corpusProvenValueType(word, dialect) == corpusValueUnknown {
			unknown = true
		}
		wordStart = -1
	}
	for i := 0; i < len(src); i++ {
		if quoted != 0 {
			if src[i] == '\\' {
				i++
			} else if src[i] == quoted {
				quoted = 0
			}
			continue
		}
		if src[i] == '"' || src[i] == '\'' {
			flushWord(i)
			quoted = src[i]
			continue
		}
		if src[i] == '+' {
			flushWord(i)
			plus = true
			continue
		}
		if isFingerprintWord(src[i]) || src[i] >= 0x80 {
			if wordStart < 0 {
				wordStart = i
			}
			continue
		}
		flushWord(i)
	}
	flushWord(len(src))
	if plus {
		return unknown
	}
	fields := bytes.Fields(src)
	if len(fields) < 2 {
		return false
	}
	for _, field := range fields {
		if corpusProvenValueType(field, dialect) != corpusValueUnknown || !corpusPreprocessorCondition(field) {
			return false
		}
	}
	return true
}

func corpusStatementTailNeedsExact(command string, tail []byte, dialect format.Dialect, allowFullwidth bool) bool {
	if !corpusStatementUsesExpressionProof(command, dialect) {
		return false
	}
	if command == "TIMES" && timesOverflowCoefficientSafe(tail, allowFullwidth) {
		return false
	}
	printV := isPrintVCommandDialect(command, dialect)
	if printV {
		var complete bool
		tail, complete = printVExpressionProofSource(tail)
		if !complete {
			return true
		}
	}
	comment := corpusCommandCommentStart(tail)
	if comment < 0 || comment > len(tail) {
		comment = len(tail)
	}
	code := tail[:comment]
	if allowFullwidth {
		code = normalizeFullwidthExpressionSpacesDialect(code, dialect)
	}
	if corpusNulBoundaryExpressionSafe(code, dialect) {
		return false
	}
	if printV {
		if printVNumericLiteralNeedsExact(code) {
			return true
		}
		if corpusExpressionLexicallyIncomplete(code) {
			return true
		}
		if printVBoundaryPrefixNeedsExact(code, dialect) {
			return true
		}
	}
	fields := splitFingerprintFields(code, topLevelCommas(code))
	if len(fields) == 0 {
		return false
	}
	expectedNumeric := command == "SETCOLOR" || command == "SETBGCOLOR"
	deferFields := printV
	deferredField := false
	for index, field := range fields {
		if len(trimCorpusSpace(field)) == 0 {
			continue
		}
		if deferFields {
			if deferredField {
				continue
			}
			if corpusExpressionHasDeferredOperand(trimCorpusSpace(field), dialect) {
				deferredField = true
				continue
			}
		}
		if corpusExpressionNeedsExact(field, dialect, expectedNumeric, corpusCommandAllowsStringMultiplication(command, index, dialect), corpusCommandFieldType(command, index, dialect) == corpusValueString) {
			return true
		}
	}
	return false
}

func corpusStatementUsesExpressionProof(command string, dialect format.Dialect) bool {
	return command == "TIMES" || command == "PRINTVN" || command == "PRINTSN" || isPrintVCommand(command) || isExpressionCommand(command) || isExpressionLikeCommand(command) || isMixedExpressionCommand(command) || isArrayCommand(command) || isKeywordCommand(command) || corpusBuiltinMethod(command, dialect)
}

func corpusCommandAllowsStringMultiplication(command string, index int, dialect format.Dialect) bool {
	if command == "CALL" || command == "PRINTVN" || command == "PRINTSN" || isPrintVCommand(command) {
		return true
	}
	switch corpusCommandFieldType(command, index, dialect) {
	case corpusValueString:
		return true
	case corpusValueNumeric:
		return false
	}
	switch command {
	case "RETURNF", "PRINTS", "PRINTSL", "PRINTSW", "PRINTSK", "PRINTSKL", "PRINTSKW", "PRINTSD", "PRINTSDL", "PRINTSDW", "PRINTSINGLES", "PRINTSINGLEVK", "PRINTSINGLEVD", "PRINTSINGLESK", "PRINTSINGLESD", "PRINTFORMS", "PRINTFORMSL", "PRINTFORMSW", "PRINTFORMSK", "PRINTFORMSKL", "PRINTFORMSKW", "PRINTFORMSD", "PRINTFORMSDL", "PRINTFORMSDW", "PRINTSINGLEFORMS", "PRINTSINGLEFORMSK", "PRINTSINGLEFORMSD":
		return index == 0
	case "SAVETEXT":
		return index == 0 || dialect == format.EMEE && index == 1
	case "LOADTEXT":
		return dialect == format.EMEE && index == 0
	case "PRINTBUTTON", "PRINTBUTTONC", "PRINTBUTTONLC":
		return index <= 1
	case "DT_CELL_SET":
		return index == 0 || index == 2 || index == 3
	case "SETVAR":
		return index <= 1
	case "VARSETEX":
		return index < 2
	case "DT_COLUMN_ADD":
		return index < 3
	case "SUBSTRING", "SUBSTRINGU":
		return index == 0
	case "STRFIND", "STRFINDU":
		return index < 2
	case "REPLACE":
		return index < 3
	case "RETURN", "IF", "ELSEIF", "SIF", "WHILE", "REPEAT", "LOOP", "SELECTCASE", "CASE", "POWER", "GETBIT", "SETBIT", "CLEARBIT", "INVERTBIT", "TIMES", "FOR":
		return false
	default:
		return true
	}
}

func corpusExpressionNeedsExact(src []byte, dialect format.Dialect, expectedNumeric, allowStringMultiplication, expectedString bool) bool {
	src = trimCorpusSpace(src)
	if len(src) == 0 {
		return true
	}
	if src[0] == 0 && !corpusNulEmbeddedInIdentifier(src, 0) {
		return false
	}
	if corpusPostfixGapWithDeferredOperand(src, dialect) {
		return false
	}
	if corpusNulBoundaryExpressionSafe(src, dialect) {
		return false
	}
	if printVHasNumericIdentifierBoundary(src) && !printVNumericLiteralNeedsExact(src) {
		return false
	}
	if corpusIncrementExpressionNeedsExact(src, dialect) {
		return true
	}
	if corpusPrefixIncrementExpressionValid(src, dialect) {
		return false
	}
	deferred := corpusExpressionHasDeferredOperand(src, dialect)
	malformedCall := corpusExpressionHasMalformedCall(src, dialect, allowStringMultiplication)
	if printVNumericLiteralNeedsExact(src) {
		return true
	}
	if corpusExpressionLexicallyIncomplete(src) {
		return true
	}
	if corpusExpressionHasAssignmentOperator(src) && !deferred || malformedCall && !corpusMalformedCallHasDeferredOperand(src, dialect) || corpusExpressionHasKnownStringIndex(src, dialect) && (expectedNumeric || !corpusStandaloneStringIndex(src, dialect)) && !corpusComparisonExpressionValid(src, dialect) && !(corpusIndexedStringCallExpressionSafe(src, dialect) || allowStringMultiplication && (corpusPercentStringMultiplicationExpression(src, dialect) || corpusStringConcatenationExpression(src, dialect))) && !deferred {
		return true
	}
	if corpusOpaqueUnknownMacroShapeDialect(src, dialect) {
		return false
	}
	if expectedNumeric && graphicsStringFieldMode(src, dialect) {
		return true
	}
	if expectedString && corpusExpressionContainsComparisonOperator(src) && corpusComparisonExpressionValid(src, dialect) && corpusComparisonExpressionHasKnownOperands(src, dialect) {
		return true
	}
	if !deferred && corpusKnownMixedExpressionNeedsExact(src, dialect) && !corpusComparisonExpressionValid(src, dialect) && !(corpusIndexedStringCallExpressionSafe(src, dialect) || allowStringMultiplication && (corpusPercentStringMultiplicationExpression(src, dialect) || corpusStringConcatenationExpression(src, dialect))) {
		return true
	}
	if !graphicsNumericExpressionValid(src, true) {
		return !deferred
	}
	if !deferred && corpusMixedQuotedNumericExpressionNeedsExact(src, dialect) {
		return true
	}
	value := corpusProvenValueType(src, dialect)
	return value == corpusValueInvalid && !deferred && !(allowStringMultiplication && corpusPercentStringMultiplicationExpression(src, dialect)) || expectedNumeric && value == corpusValueString
}

func corpusIndexedStringCallExpressionSafe(src []byte, dialect format.Dialect) bool {
	src = corpusStripOuterParens(trimCorpusSpace(src))
	if len(src) == 0 || printVNumericLiteralNeedsExact(src) || !corpusExpressionHasKnownStringIndex(src, dialect) || !corpusExpressionHasNamedCall(src) {
		return false
	}
	if bytes.IndexByte(src, 0) >= 0 || bytes.Contains(src, []byte("[[")) || bytes.Contains(src, []byte("@")) || bytes.Contains(src, []byte("'")) || mandatoryMarkerIndex(src) >= 0 || corpusExpressionHasAssignmentOperator(src) || corpusExpressionLexicallyIncomplete(src) {
		return false
	}
	if !graphicsNumericExpressionValid(src, true) || corpusExpressionHasAdjacentOperands(src, dialect) || corpusExpressionHasMalformedCall(src, dialect, true) {
		return false
	}
	if !bytes.Equal(tokenFingerprintDialect(src, dialect), expressionFingerprintDialect(src, dialect)) {
		return false
	}

	for i := 0; i < len(src); i++ {
		if src[i] != '(' || i == 0 || !isFingerprintWord(src[i-1]) {
			continue
		}
		end := matchingCallDelimiter(src, i, '(', ')')
		if end < 0 {
			return false
		}
		args := src[i+1 : end]
		fields := splitFingerprintFields(args, topLevelCommas(args))
		if len(fields) == 0 {
			return false
		}
		indexed, allNumeric := false, true
		for _, field := range fields {
			field = trimCorpusSpace(field)
			if len(field) == 0 || corpusExpressionLexicallyIncomplete(field) || corpusExpressionHasAssignmentOperator(field) || mandatoryMarkerIndex(field) >= 0 || bytes.IndexByte(field, 0) >= 0 || bytes.Contains(field, []byte("[[")) {
				return false
			}
			if corpusExpressionHasNamedCall(field) {
				return false
			}
			if corpusExpressionHasKnownStringIndex(field, dialect) {
				indexed = true
			}
			if !graphicsNumericExpressionValid(field, false) {
				allNumeric = false
			}
		}
		if indexed || len(fields) >= 3 && allNumeric {
			return true
		}
		i = end
	}
	return false
}

func corpusExpressionHasNamedCall(src []byte) bool {
	for i := 0; i < len(src); {
		switch {
		case src[i] == '\'' || src[i] == '"':
			i = printVQuotedEnd(src, i)
		case src[i] == '@' && i+1 < len(src) && src[i+1] == '"':
			i = quotedFormEnd(src, i)
		case src[i] == '\\' && i+1 < len(src) && src[i+1] == '@':
			if _, _, end, ok := formConditional(src, i); ok {
				i = end + 2
			} else {
				i += 2
			}
		case src[i] == '(' && i > 0 && isFingerprintWord(src[i-1]):
			return true
		default:
			i++
		}
	}
	return false
}

func corpusExpressionHasNestedNamedCall(src []byte) bool {
	for i := 0; i < len(src); {
		if src[i] != '(' || i == 0 || !isFingerprintWord(src[i-1]) {
			i++
			continue
		}
		end := matchingCallDelimiter(src, i, '(', ')')
		if end < 0 {
			return false
		}
		if corpusExpressionHasNamedCall(src[i+1 : end]) {
			return true
		}
		i = end + 1
	}
	return false
}

func corpusExpressionHasNumericIdentifierGlue(src []byte) bool {
	for i := 0; i < len(src); {
		switch {
		case src[i] == '\'' || src[i] == '"':
			i = printVQuotedEnd(src, i)
		case src[i] == '@' && i+1 < len(src) && src[i+1] == '"':
			i = quotedFormEnd(src, i)
		case src[i] == '\\' && i+1 < len(src) && src[i+1] == '@':
			if _, _, end, ok := formConditional(src, i); ok {
				i = end + 2
			} else {
				i += 2
			}
		case src[i] >= '0' && src[i] <= '9' && (i == 0 || !isLHSIdentifierByte(src[i-1])):
			end, ok := printVRuntimeNumberEnd(src, i)
			if !ok || end < len(src) && isLHSIdentifierStart(src[end]) {
				return true
			}
			i = end
		default:
			i++
		}
	}
	return false
}

func corpusNulBoundaryExpressionSafe(src []byte, dialect format.Dialect) bool {
	nul := bytes.IndexByte(src, 0)
	if nul < 0 || corpusNulEmbeddedInIdentifier(src, nul) {
		return false
	}
	prefix := trimCorpusSpace(src[:nul])
	if len(prefix) == 0 || corpusExpressionHasAssignmentOperator(prefix) || !graphicsNumericExpressionValid(prefix, dialect == format.EMEE) {
		return false
	}
	return true
}

func corpusPostfixGapWithDeferredOperand(src []byte, dialect format.Dialect) bool {
	for i := 0; i+2 < len(src); i++ {
		if (src[i] != '+' && src[i] != '-') || src[i+1] != src[i] {
			continue
		}
		j := i + 2
		for j < len(src) && corpusHalfSpace(src[j]) {
			j++
		}
		if j >= len(src) || !(isLHSIdentifierStart(src[j]) || src[j] >= '0' && src[j] <= '9') {
			continue
		}
		if corpusExpressionHasDeferredOperand(src[:i+2], dialect) {
			return true
		}
	}
	return false
}

func corpusExpressionLexicallyIncomplete(src []byte) bool {
	delimiters := make([]byte, 0, 4)
	for i := 0; i < len(src); {
		switch src[i] {
		case 0:
			if !corpusNulEmbeddedInIdentifier(src, i) {
				return len(delimiters) != 0
			}
			i++
		case '"':
			end, complete := printVQuotedEndComplete(src, i)
			if !complete || end <= i || end > len(src) {
				return true
			}
			i = end
		case '\'':
			i = printVAtomEnd(src, i)
		case '@':
			if i+1 < len(src) && src[i+1] == '"' {
				end, complete := quotedFormEndComplete(src, i)
				if !complete || end <= i || end > len(src) {
					return true
				}
				i = end
			} else {
				i++
			}
		case '\\':
			if i+1 < len(src) && src[i+1] == '@' {
				_, _, end, ok := formConditional(src, i)
				if !ok {
					return true
				}
				i = end + 2
			} else if i+1 < len(src) {
				i += 2
			} else {
				i++
			}
		case '(', '[', '{':
			close := byte(')')
			if src[i] == '[' {
				close = ']'
			} else if src[i] == '{' {
				close = '}'
			}
			delimiters = append(delimiters, close)
			i++
		case ')', ']', '}':
			if len(delimiters) == 0 || delimiters[len(delimiters)-1] != src[i] {
				return true
			}
			delimiters = delimiters[:len(delimiters)-1]
			i++
		default:
			i++
		}
	}
	return len(delimiters) != 0
}

func corpusExpressionContainsComparisonOperator(src []byte) bool {
	for i := 0; i < len(src); i++ {
		switch src[i] {
		case '"', '\'':
			i = printVQuotedEnd(src, i) - 1
		case '@':
			if i+1 < len(src) && src[i+1] == '"' {
				if end := quotedFormEnd(src, i); end > i+2 && end <= len(src) {
					i = end - 1
				}
			}
		case '=', '!', '<', '>':
			return true
		}
	}
	return false
}

func corpusPrefixIncrementExpressionValid(src []byte, dialect format.Dialect) bool {
	src = trimCorpusSpace(src)
	if len(src) < 3 || !(bytes.HasPrefix(src, []byte("++")) || bytes.HasPrefix(src, []byte("--"))) {
		return false
	}
	body := trimCorpusSpace(src[2:])
	if len(body) == 0 {
		return false
	}
	target := body
	for _, operator := range []byte{'<', '>', '='} {
		if parts := corpusTopLevelBinaryParts(body, operator); len(parts) == 2 {
			target = trimCorpusSpace(parts[0])
			break
		}
	}
	return corpusExpressionValueType(target, dialect) != corpusValueString && graphicsNumericExpressionValid(body, dialect == format.EMEE)
}

func corpusExpressionHasMalformedCall(src []byte, dialect format.Dialect, allowStringMultiplication bool) bool {
	if mandatoryMarkerIndex(src) >= 0 {
		return false
	}
	for i := 0; i < len(src); i++ {
		switch src[i] {
		case '"', '\'':
			i = printVQuotedEnd(src, i) - 1
		case '@':
			if i+1 < len(src) && src[i+1] == '"' {
				if end := quotedFormEnd(src, i); end > i+2 && end <= len(src) {
					i = end - 1
				}
			}
		case '\\':
			if i+1 < len(src) && src[i+1] == '@' {
				if _, _, end, ok := formConditional(src, i); ok {
					i = end + 1
				}
			}
		case '(':
			end := matchingCallDelimiter(src, i, '(', ')')
			if end < 0 {
				return true
			}
			if corpusCallArgumentsMalformed(src[i+1:end], dialect, allowStringMultiplication) {
				return true
			}
			i = end
		}
	}
	return false
}

func corpusMalformedCallHasDeferredOperand(src []byte, dialect format.Dialect) bool {
	for i := 0; i < len(src); i++ {
		switch src[i] {
		case '"', '\'':
			i = printVQuotedEnd(src, i) - 1
		case '@':
			if i+1 < len(src) && src[i+1] == '"' {
				if end := quotedFormEnd(src, i); end > i+2 && end <= len(src) {
					i = end - 1
				}
			}
		case '(':
			end := matchingCallDelimiter(src, i, '(', ')')
			if end < 0 {
				return false
			}
			for _, field := range splitFingerprintFields(src[i+1:end], topLevelCommas(src[i+1:end])) {
				if corpusExpressionHasDeferredOperand(trimCorpusSpace(field), dialect) {
					return true
				}
			}
			i = end
		}
	}
	return false
}

func corpusCallArgumentsMalformed(src []byte, dialect format.Dialect, allowStringMultiplication bool) bool {
	for _, field := range splitFingerprintFields(src, topLevelCommas(src)) {
		field = trimCorpusSpace(field)
		if len(field) == 0 {
			continue
		}
		if corpusFormCallArgumentNeedsExact(field, dialect, allowStringMultiplication) {
			return true
		}
	}
	return false
}

func corpusFormCallArgumentsNeedExact(src []byte, dialect format.Dialect, allowFullwidth bool) bool {
	if nul := corpusExpressionNulBoundaryMode(src, false, allowFullwidth); nul >= 0 {
		src = src[:nul]
	}
	if comment := corpusCommandCommentStart(src); comment >= 0 && comment < len(src) {
		src = src[:comment]
	}
	if allowFullwidth {
		src = normalizeFullwidthExpressionSpacesDialect(src, dialect)
	}
	return corpusCallArgumentsMalformed(src, dialect, true)
}

func corpusFormCallArgumentNeedsExact(src []byte, dialect format.Dialect, allowStringMultiplication bool) bool {
	if corpusNumericIdentifierBoundarySafe(src) && !corpusExpressionHasKnownStringIndex(src, dialect) {
		return false
	}
	if corpusExpressionHasAssignmentOperator(src) || corpusExpressionEndsWithOperator(src) || corpusExpressionHasAdjacentOperands(src, dialect) || corpusIncrementExpressionNeedsExact(src, dialect) {
		return true
	}
	return corpusKnownMixedExpressionNeedsExact(src, dialect) && !(allowStringMultiplication && (corpusPercentStringMultiplicationExpression(src, dialect) || corpusStringConcatenationExpression(src, dialect)))
}

func corpusExpressionHasAdjacentOperands(src []byte, dialect format.Dialect) bool {
	operand := false
	var operandKind uint8
	operandName := ""
	for i := 0; i < len(src); {
		if corpusHalfSpace(src[i]) {
			i++
			continue
		}
		switch src[i] {
		case '"', '\'':
			if operand && operandKind != 0 {
				return true
			}
			i = printVQuotedEnd(src, i)
			operand = true
			operandKind = 1
			operandName = ""
		case '@':
			if i+1 < len(src) && src[i+1] == '"' {
				if end := quotedFormEnd(src, i); end > i+2 && end <= len(src) {
					if operand && operandKind != 0 {
						return true
					}
					i = end
					operand = true
					operandKind = 1
					operandName = ""
					continue
				}
			}
			operand = false
			operandKind = 0
			operandName = ""
			i++
		case '(', '[', '{':
			if operand && operandKind != 0 && (src[i] != '(' || operandName == "" || !corpusBuiltinMethod(operandName, dialect)) {
				return true
			}
			operand = false
			operandKind = 0
			operandName = ""
			i++
		case ')', ']', '}':
			operand = true
			operandKind = 0
			operandName = ""
			i++
		default:
			if isFingerprintWord(src[i]) || src[i] >= '0' && src[i] <= '9' {
				start := i
				i++
				for i < len(src) && (isFingerprintWord(src[i]) || src[i] >= '0' && src[i] <= '9') {
					i++
				}
				kind := uint8(0)
				if value := corpusExpressionValueType(src[start:i], dialect); value == corpusValueNumeric || value == corpusValueString {
					kind = 1
				}
				if operand && (operandKind != 0 || kind != 0) {
					return true
				}
				operand = true
				operandKind = kind
				operandName = string(src[start:i])
				continue
			}
			operand = false
			operandKind = 0
			operandName = ""
			i++
		}
	}
	return false
}

func corpusExpressionEndsWithOperator(src []byte) bool {
	src = trimCorpusSpace(src)
	if len(src) == 0 {
		return false
	}
	if len(src) >= 2 && (bytes.HasSuffix(src, []byte("++")) || bytes.HasSuffix(src, []byte("--"))) {
		return false
	}
	return strings.ContainsRune("+-*/%&|^=!?<>", rune(src[len(src)-1]))
}

func corpusExpressionHasDeferredOperand(src []byte, dialect format.Dialect) bool {
	for i := 0; i < len(src); {
		switch {
		case src[i] == 0:
			return corpusNulEmbeddedInIdentifier(src, i)
		case src[i] == '"' || src[i] == '\'':
			end := printVQuotedEnd(src, i)
			if end <= i || end > len(src) {
				return false
			}
			i = end
		case src[i] == '@' && i+1 < len(src) && src[i+1] == '"':
			end := quotedFormEnd(src, i)
			if end <= i+2 || end > len(src) {
				return false
			}
			return true
		case src[i] == '\\' && i+1 < len(src) && src[i+1] == '@':
			if _, _, _, ok := formConditional(src, i); ok {
				return true
			}
			i += 2
		case (src[i] > 0 && src[i] < 0x20 || src[i] == 0x7f) && isLHSIdentifierStart(src[i]):
			return true
		case isLHSIdentifierStart(src[i]) || src[i] >= '0' && src[i] <= '9':
			start := i
			i++
			for i < len(src) && isLHSIdentifierByte(src[i]) {
				if src[i] == 0 && !corpusNulEmbeddedInIdentifier(src, i) {
					break
				}
				i++
			}
			name := string(src[start:i])
			if corpusIdentifierTokenHasControl(src[start:i]) {
				return true
			}
			j := i
			for j < len(src) && corpusHalfSpace(src[j]) {
				j++
			}
			if j < len(src) && src[j] == '(' {
				if !corpusBuiltinMethod(name, dialect) {
					return true
				}
			} else if !corpusExpressionIdentifierKnown([]byte(name), dialect) {
				return true
			}
		default:
			i++
		}
	}
	return false
}

func corpusIdentifierTokenHasControl(src []byte) bool {
	for _, b := range src {
		if (b > 0 && b < 0x20 || b == 0x7f) && isLHSIdentifierStart(b) {
			return true
		}
	}
	return false
}

func corpusExpressionIdentifierKnown(src []byte, dialect format.Dialect) bool {
	for _, b := range src {
		if !isFingerprintWord(b) {
			return false
		}
	}
	return corpusProvenValueType(src, dialect) != corpusValueUnknown
}

func corpusExpressionHasKnownStringIndex(src []byte, dialect format.Dialect) bool {
	for i := 0; i < len(src); {
		if src[i] == '"' || src[i] == '\'' {
			end := printVQuotedEnd(src, i)
			if end <= i || end > len(src) {
				return false
			}
			i = end
			continue
		}
		if src[i] == '@' && i+1 < len(src) && src[i+1] == '"' {
			end := quotedFormEnd(src, i)
			if end > i+2 && end <= len(src) {
				i = end
				continue
			}
		}
		if !isFingerprintWord(src[i]) {
			i++
			continue
		}
		start := i
		i++
		for i < len(src) && isFingerprintWord(src[i]) {
			i++
		}
		name := string(src[start:i])
		if !corpusStringBuiltin(name, dialect) {
			continue
		}
		j := i
		for j < len(src) && corpusHalfSpace(src[j]) {
			j++
		}
		if j >= len(src) || src[j] != ':' {
			continue
		}
		j++
		for j < len(src) && corpusHalfSpace(src[j]) {
			j++
		}
		end := corpusStringIndexEnd(src, j)
		if end <= j {
			return true
		}
		value := corpusProvenValueType(corpusStripOuterParens(src[j:end]), dialect)
		if value == corpusValueNumeric || value == corpusValueString || value == corpusValueInvalid {
			if corpusNestedNumericStringIndex(src, start, j, end, dialect) {
				i = end
				continue
			}
			return true
		}
		i = end
	}
	return false
}

func corpusNestedNumericStringIndex(src []byte, nameStart, indexStart, indexEnd int, dialect format.Dialect) bool {
	before := nameStart - 1
	for before >= 0 && corpusHalfSpace(src[before]) {
		before--
	}
	if before < 0 || src[before] != '(' {
		return false
	}
	open := before
	before--
	for before >= 0 && corpusHalfSpace(src[before]) {
		before--
	}
	if before < 0 || src[before] != ':' {
		callEnd := open
		callStart := callEnd
		for callStart > 0 && isFingerprintWord(src[callStart-1]) {
			callStart--
		}
		if open == nameStart-1 && callStart < callEnd {
			index := corpusStripOuterParens(src[indexStart:indexEnd])
			return len(index) != 0 && corpusProvenValueType(index, dialect) != corpusValueString && corpusProvenValueType(index, dialect) != corpusValueInvalid && graphicsNumericExpressionValid(index, dialect == format.EMEE)
		}
		return false
	}
	outerEnd := before
	before--
	if before >= 0 && src[before] == ')' {
		depth := 1
		before--
		for before >= 0 && depth > 0 {
			switch src[before] {
			case ')':
				depth++
			case '(':
				depth--
			}
			before--
		}
	}
	if before >= 0 && src[before] == ':' {
		outerEnd = before
		before--
	}
	for before >= 0 && isFingerprintWord(src[before]) {
		before--
	}
	outerNumeric := false
	for {
		end := outerEnd
		for end > 0 && corpusHalfSpace(src[end-1]) {
			end--
		}
		nameEnd := end
		nameStart := nameEnd
		for nameStart > 0 && isFingerprintWord(src[nameStart-1]) {
			nameStart--
		}
		if nameStart == nameEnd {
			break
		}
		if corpusNameKnown(corpusNumericBuiltinVariableSet, string(src[nameStart:nameEnd])) {
			outerNumeric = true
			break
		}
		if nameStart == 0 || src[nameStart-1] != ':' {
			break
		}
		outerEnd = nameStart - 1
	}
	if !outerNumeric {
		return false
	}
	index := corpusStripOuterParens(src[indexStart:indexEnd])
	return len(index) != 0 && corpusProvenValueType(index, dialect) != corpusValueString && corpusProvenValueType(index, dialect) != corpusValueInvalid && graphicsNumericExpressionValid(index, dialect == format.EMEE)
}

func corpusExpressionHasInvalidStringIndex(src []byte, dialect format.Dialect) bool {
	for i := 0; i < len(src); {
		if src[i] == '"' || src[i] == '\'' {
			end := printVQuotedEnd(src, i)
			if end <= i || end > len(src) {
				return false
			}
			i = end
			continue
		}
		if !isFingerprintWord(src[i]) {
			i++
			continue
		}
		start := i
		i++
		for i < len(src) && isFingerprintWord(src[i]) {
			i++
		}
		if !corpusStringBuiltin(string(src[start:i]), dialect) {
			continue
		}
		j := i
		for j < len(src) && corpusHalfSpace(src[j]) {
			j++
		}
		if j >= len(src) || src[j] != ':' {
			continue
		}
		j++
		for j < len(src) && corpusHalfSpace(src[j]) {
			j++
		}
		end := corpusStringIndexEnd(src, j)
		if end <= j {
			return true
		}
		value := corpusProvenValueType(corpusStripOuterParens(src[j:end]), dialect)
		if value == corpusValueString || value == corpusValueInvalid {
			return true
		}
		i = end
	}
	return false
}

func corpusStandaloneStringIndex(src []byte, dialect format.Dialect) bool {
	src = corpusStripOuterParens(trimCorpusSpace(src))
	start := 0
	for start < len(src) && isFingerprintWord(src[start]) {
		start++
	}
	if start == 0 || !corpusStringBuiltin(string(src[:start]), dialect) {
		return false
	}
	for start < len(src) && corpusHalfSpace(src[start]) {
		start++
	}
	if start >= len(src) || src[start] != ':' {
		return false
	}
	start++
	for start < len(src) && corpusHalfSpace(src[start]) {
		start++
	}
	end := corpusStringIndexEnd(src, start)
	if end != len(src) {
		return false
	}
	value := corpusProvenValueType(corpusStripOuterParens(src[start:end]), dialect)
	return value != corpusValueString && value != corpusValueInvalid
}

func corpusStringIndexEnd(src []byte, start int) int {
	if start >= len(src) {
		return start
	}
	if src[start] == '"' || src[start] == '\'' {
		return printVQuotedEnd(src, start)
	}
	if src[start] == '@' && start+1 < len(src) && src[start+1] == '"' {
		return quotedFormEnd(src, start)
	}
	if src[start] == '\\' && start+1 < len(src) && src[start+1] == '@' {
		if _, _, end, ok := formConditional(src, start); ok {
			return end + 2
		}
		return len(src)
	}
	if src[start] == '(' || src[start] == '[' {
		stack := []byte{src[start]}
		for i := start + 1; i < len(src); {
			switch src[i] {
			case '"', '\'':
				end := printVQuotedEnd(src, i)
				if end <= i {
					return len(src)
				}
				i = end
				continue
			case '@':
				if i+1 < len(src) && src[i+1] == '"' {
					end := quotedFormEnd(src, i)
					if end <= i+2 {
						return len(src)
					}
					i = end
					continue
				}
			case '\\':
				if i+1 < len(src) && src[i+1] == '@' {
					_, _, end, ok := formConditional(src, i)
					if !ok {
						return len(src)
					}
					i = end + 2
					continue
				}
			case '(', '[':
				stack = append(stack, src[i])
			case ')', ']':
				if len(stack) == 0 || (src[i] == ')' && stack[len(stack)-1] != '(') || (src[i] == ']' && stack[len(stack)-1] != '[') {
					return len(src)
				}
				stack = stack[:len(stack)-1]
				if len(stack) == 0 {
					return i + 1
				}
			}
			i++
		}
		return len(src)
	}
	for i := start; i < len(src); i++ {
		if corpusHalfSpace(src[i]) || strings.ContainsRune("+-*/%&|^=!?<>,)]}", rune(src[i])) {
			return i
		}
		if src[i] == '"' || src[i] == '\'' {
			end := printVQuotedEnd(src, i)
			if end <= i {
				return len(src)
			}
			i = end - 1
		}
	}
	return len(src)
}

func corpusExpressionHasAssignmentOperator(src []byte) bool {
	quoted := byte(0)
	for i := 0; i < len(src); i++ {
		if quoted != 0 {
			if src[i] == '\\' {
				i++
			} else if src[i] == quoted {
				quoted = 0
			}
			continue
		}
		if src[i] == '"' || src[i] == '\'' {
			quoted = src[i]
			continue
		}
		if src[i] == '@' && i+1 < len(src) && src[i+1] == '"' {
			if end := quotedFormEnd(src, i); end > i+2 && end <= len(src) {
				i = end - 1
			}
			continue
		}
		if src[i] == '\\' && i+1 < len(src) && src[i+1] == '@' {
			if _, _, end, ok := formConditional(src, i); ok {
				i = end + 1
			}
			continue
		}
		if src[i] != '=' {
			continue
		}
		if i > 0 && strings.ContainsRune("<>=!", rune(src[i-1])) || i+1 < len(src) && src[i+1] == '=' {
			continue
		}
		return true
	}
	return false
}

func corpusConditionHasComparison(src []byte) bool {
	quoted := byte(0)
	depth := 0
	for i := 0; i < len(src); i++ {
		if quoted != 0 {
			if src[i] == '\\' {
				i++
			} else if src[i] == quoted {
				quoted = 0
			}
			continue
		}
		if src[i] == '"' || src[i] == '\'' {
			quoted = src[i]
			continue
		}
		switch src[i] {
		case '(', '[':
			depth++
		case ')', ']':
			if depth > 0 {
				depth--
			}
		case '=', '!', '<', '>':
			if depth == 0 {
				return true
			}
		}
	}
	return false
}

func normalizeFullwidthFormPayload(src []byte) []byte {
	return normalizeFullwidthFormPayloadDialect(src, format.CommonDialect)
}

func normalizeFullwidthFormPayloadDialect(src []byte, dialect format.Dialect) []byte {
	if nul := corpusFormNulBoundary(src); nul >= 0 {
		return append(normalizeFullwidthFormPayloadDialect(src[:nul], dialect), src[nul:]...)
	}
	out := make([]byte, 0, len(src))
	literal := 0
	flush := func(end int) {
		if end > literal {
			out = append(out, src[literal:end]...)
		}
	}
	for i := 0; i < len(src); {
		if src[i] == '\\' && i+1 < len(src) && src[i+1] == '@' {
			_, _, end, ok := formConditional(src, i)
			if ok {
				flush(i)
				out = append(out, normalizeFullwidthConditionalDialect(src[i:end+2], dialect)...)
				i = end + 2
				literal = i
				continue
			}
		}
		if src[i] == '@' && i+1 < len(src) && src[i+1] == '"' {
			end := quotedFormEnd(src, i)
			if end > i+2 && end <= len(src) {
				flush(i)
				out = append(out, normalizeFullwidthFormAtomDialect(src[i:end], dialect)...)
				i = end
				literal = i
				continue
			}
		}
		if src[i] == '\\' && i+1 < len(src) {
			i += 2
			continue
		}
		if src[i] != '{' && src[i] != '%' {
			i++
			continue
		}
		end := formIslandEnd(src, i, src[i])
		if end < 0 {
			i++
			continue
		}
		flush(i)
		out = append(out, src[i])
		island := src[i+1 : end]
		if formIslandNeedsExactDialect(island, src[i], true, dialect) {
			out = append(out, island...)
		} else {
			out = append(out, normalizeFullwidthExpressionBodyDialect(island, dialect)...)
		}
		out = append(out, src[end])
		i = end + 1
		literal = i
	}
	flush(len(src))
	return out
}

func stripMarkers(head []byte) (markers, rest []byte) {
	for {
		head = bytes.TrimLeft(head, " \t")
		if len(head) < 3 || (string(head[:3]) != ";!;" && string(head[:3]) != ";#;" && string(head[:3]) != ";^;") {
			return markers, head
		}
		markers = append(markers, head[:3]...)
		head = head[3:]
	}
}

func firstWord(src []byte) ([]byte, int) {
	if len(src) == 0 || !isCommandWordStart(src[0]) {
		return nil, 0
	}
	i := 1
	for i < len(src) && isCommandWordByte(src[i]) {
		i++
	}
	return src[:i], i
}

func isCommandWordStart(b byte) bool {
	return b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z' || b == '_'
}

func isCommandWordByte(b byte) bool {
	return isCommandWordStart(b) || b >= '0' && b <= '9'
}

func isDefineDirective(src []byte) bool {
	if len(src) < len("#DEFINE") || !strings.EqualFold(string(src[:len("#DEFINE")]), "#DEFINE") {
		return false
	}
	return len(src) == len("#DEFINE") || !isFingerprintWord(src[len("#DEFINE")])
}

func corpusKnownSharpDeclaration(src []byte) bool {
	name, _ := corpusSharpDeclarationName(src)
	switch name {
	case "DIM", "DIMS", "FUNCTION", "FUNCTIONS", "LATER", "LOCALSIZE", "LOCALSSIZE", "ONLY", "PRI", "SINGLE":
		return true
	default:
		return false
	}
}

func corpusSharpDeclarationName(src []byte) (string, int) {
	if len(src) == 0 || src[0] != '#' {
		return "", -1
	}
	if len(src) == 1 {
		return "", 1
	}
	end := 1
	for end < len(src) {
		if src[end] == ' ' || src[end] == '\t' || bytes.HasPrefix(src[end:], []byte("\u3000")) || strings.ContainsRune(".+-*/%=!<>|&^~#?)},:({[$'\"@;", rune(src[end])) {
			break
		}
		_, size := utf8.DecodeRune(src[end:])
		if size == 0 {
			return "", -1
		}
		end += size
	}
	return strings.ToUpper(string(src[1:end])), end
}

func corpusUnknownSharpNeedsExact(src []byte) bool {
	name, end := corpusSharpDeclarationName(src)
	if end < 0 {
		return true
	}
	for _, known := range []string{"DIM", "DIMS", "FUNCTION", "FUNCTIONS", "LATER", "LOCALSIZE", "LOCALSSIZE", "ONLY", "PRI", "SINGLE"} {
		if strings.HasPrefix(name, known) && name != known {
			return true
		}
	}
	tail := trimGrammarSpace(src[end:], true)
	if len(tail) == 0 {
		return false
	}
	if tail[0] == ';' {
		if !corpusProtectedComment(tail) {
			return false
		}
		return true
	}
	return len(tail) != 0 && strings.ContainsRune(".+-*/%=!<>|&^~#?)},:({[$'\"@;", rune(tail[0]))
}

func defineFingerprint(src []byte) []byte {
	wordEnd := len("#DEFINE")
	nameStart := wordEnd
	for nameStart < len(src) && (src[nameStart] == ' ' || src[nameStart] == '\t') {
		nameStart++
	}
	nameEnd := nameStart
	for nameEnd < len(src) && isFingerprintWord(src[nameEnd]) {
		nameEnd++
	}
	if nameStart == wordEnd || nameEnd == nameStart || nameEnd < len(src) && src[nameEnd] != ' ' && src[nameEnd] != '\t' {
		return append([]byte(nil), src...)
	}
	bodyStart := nameEnd
	for bodyStart < len(src) && (src[bodyStart] == ' ' || src[bodyStart] == '\t') {
		bodyStart++
	}
	if bodyStart == len(src) {
		bodyStart = nameEnd
	}
	out := append([]byte(nil), src[:wordEnd]...)
	out = append(out, 0)
	out = append(out, src[nameStart:nameEnd]...)
	out = append(out, 0)
	return append(out, src[bodyStart:]...)
}

func statementFingerprintDialect(command string, word, tail []byte, dialect format.Dialect) (string, []byte) {
	if corpusDialectRestrictedCommand(command) && dialect != format.EMEE {
		return "unknown", append(append([]byte(nil), word...), tail...)
	}
	if isNoArgCommandDialect(command, dialect) {
		return "void", joinFingerprint(word, noArgFingerprint(tail))
	}
	if command == "SAVEVAR" {
		return "expression", joinFingerprint(word, saveVarFingerprint(tail, dialect, false))
	}
	if isFormCallCommand(command) {
		return "form-call", joinFingerprint(word, formCallFingerprintDialect(tail, dialect))
	}
	if isPlainCallCommand(command) {
		return "expression", joinFingerprint(word, plainCallFingerprintDialect(tail, dialect))
	}
	if command == "STRFORM" {
		if fingerprint, ok := directSTRFORMLiteralFingerprint(tail, false); ok {
			return "strform-literal", joinFingerprint(word, fingerprint)
		}
	}
	if dialect == format.EMEE {
		switch command {
		case "PRINTVN":
			if corpusStatementTailNeedsExact(command, tail, dialect, false) && !commandTailPreservationProof(command, tail, dialect, false) {
				return "print", joinFingerprint(word, invalidPrintVCommandTailFingerprint(tail))
			}
			return "print", joinFingerprint(word, printVFingerprintDialect(tail, false, dialect))
		case "PRINTSN":
			if corpusStatementTailNeedsExact(command, tail, dialect, false) {
				return "expression", joinFingerprint(word, invalidCommandTailFingerprint(tail))
			}
			return "expression", joinFingerprint(word, expressionFingerprintDialect(tail, dialect))
		case "PRINTFORMSN":
			if fingerprint, ok := printFormsLiteralFingerprintDialect(tail, dialect); ok {
				return "printforms-literal", joinFingerprint(word, fingerprint)
			}
			return "expression", joinFingerprint(word, expressionFingerprintDialect(tail, dialect))
		case "PRINTFORMN":
			return "form", joinFingerprint(word, formFingerprintDialect(tail, dialect))
		}
	}
	if isPrintFormsCommand(command) {
		if fingerprint, ok := printFormsLiteralFingerprintDialect(tail, dialect); ok {
			return "printforms-literal", joinFingerprint(word, fingerprint)
		}
	}
	if isRawCommand(command, dialect) {
		return "raw", joinFingerprint(word, tail)
	}
	if kind, min, max, ok := corpusStructuredCommand(command, dialect); ok {
		return kind, joinFingerprint(word, structuredCommandFingerprintDialect(command, tail, kind, min, max, dialect))
	}
	if command == "VARSIZE" {
		return "variable", joinFingerprint(word, variableSizeFingerprint(tail))
	}
	if command == "DT_COLUMN_OPTIONS" && dialect == format.EMEE {
		return "keyword-value", joinFingerprint(word, dataTableColumnOptionsFingerprint(tail))
	}
	if isInputCommand(command, dialect) {
		return "input", joinFingerprint(word, inputFingerprint(command, tail, dialect))
	}
	if isInputExpressionCommand(command, dialect) {
		return "input", joinFingerprint(word, inputExpressionFingerprint(command, tail, dialect))
	}
	if command == "TIMES" {
		if corpusStatementTailNeedsExact(command, tail, dialect, false) {
			return "times", joinFingerprint(word, invalidCommandTailFingerprint(tail))
		}
		return "times", joinFingerprint(word, timesFingerprint(tail))
	}
	if isExtraVariableCommand(command) {
		return "variable", joinFingerprint(word, variableCommandFingerprint(tail))
	}
	if command == "SPLIT" || command == "SWAP" {
		return "extra", joinFingerprint(word, extraCommandFingerprint(command, tail))
	}
	if isGraphicsCommand(command) {
		return "graphics", joinFingerprint(word, graphicsFingerprint(command, tail, dialect))
	}
	if isFormCommandExtended(command) {
		return "form", joinFingerprint(word, formFingerprintDialect(tail, dialect))
	}
	if isPrintVCommand(command) {
		if corpusStatementTailNeedsExact(command, tail, dialect, false) && !commandTailPreservationProof(command, tail, dialect, false) {
			return "print", joinFingerprint(word, invalidPrintVCommandTailFingerprint(tail))
		}
		return "print", joinFingerprint(word, printVFingerprintDialect(tail, false, dialect))
	}
	if isMixedExpressionCommand(command) || isArrayCommand(command) || isKeywordCommand(command) {
		fingerprint := expressionFingerprintDialect(tail, dialect)
		if corpusStatementTailNeedsExact(command, tail, dialect, false) {
			fingerprint = invalidCommandTailFingerprint(tail)
		}
		if _, _, _, ok := corpusCommandArity(command, dialect); ok && !corpusCommandShapeValid(command, tail, dialect, false) {
			fingerprint = invalidCommandTailFingerprint(tail)
		}
		return "expression", joinFingerprint(word, fingerprint)
	}
	if isExpressionCommand(command) {
		if corpusStatementTailNeedsExact(command, tail, dialect, false) {
			return "expression", joinFingerprint(word, invalidCommandTailFingerprint(tail))
		}
		return "expression", joinFingerprint(word, expressionFingerprintDialect(tail, dialect))
	}
	if isExpressionLikeCommand(command) {
		if corpusStatementTailNeedsExact(command, tail, dialect, false) {
			return "expression", joinFingerprint(word, invalidCommandTailFingerprint(tail))
		}
		return "expression", joinFingerprint(word, expressionFingerprintDialect(tail, dialect))
	}
	if isDataCommand(command) {
		return "data", joinFingerprint(word, tail)
	}
	if isFormCommand(command) {
		return "form", joinFingerprint(word, formFingerprintDialect(tail, dialect))
	}
	if isPrintCommand(command) {
		return "print", joinFingerprint(word, tail)
	}
	if corpusBuiltinMethod(command, dialect) {
		if corpusStatementTailNeedsExact(command, tail, dialect, false) {
			return "method", joinFingerprint(word, invalidCommandTailFingerprint(tail))
		}
		return "method", joinFingerprint(word, expressionFingerprintDialect(tail, dialect))
	}
	return "command", joinFingerprint(word, tail)
}

func corpusCommandArity(command string, dialect format.Dialect) (min, max int, allowTrailing, ok bool) {
	allowTrailing = true
	unbounded := int(^uint(0) >> 1)
	switch command {
	case "SETFONT", "FONTSTYLE":
		return 0, 1, false, true
	case "LOADDATA", "LOADCHARA", "LOADVAR", "ASSERT", "TOOLTIP_SETDELAY", "TOOLTIP_SETDURATION":
		return 1, 1, false, true
	case "HTML_PRINT":
		max := 2
		if dialect == format.Emuera1824 {
			max = 1
		}
		return 1, max, true, true
	case "PRINTCPERLINE", "SAVENOS":
		return 0, 1, true, true
	case "SETCOLOR", "SETBGCOLOR":
		return 1, 3, true, true
	case "PRINTBUTTON", "PRINTBUTTONC", "PRINTBUTTONLC":
		return 2, 2, true, true
	case "BAR", "BARL":
		return 3, 3, true, true
	case "TWAIT", "SWAPCHARA", "COPYCHARA":
		return 2, 2, true, true
	case "ADDCHARA", "ADDSPCHARA", "ADDCOPYCHARA", "PICKUPCHARA", "DELCHARA":
		return 1, unbounded, true, true
	case "SAVEDATA":
		return 2, 2, true, true
	case "SAVECHARA":
		return 3, unbounded, true, true
	case "SAVEVAR":
		return 3, 255, true, true
	case "ARRAYREMOVE":
		return 3, 3, true, true
	case "ARRAYSHIFT":
		return 3, 5, true, true
	case "ARRAYCOPY":
		return 2, 2, true, true
	case "ARRAYSORT":
		return 1, 4, true, true
	case "SORTCHARA":
		return 0, 2, true, true
	case "HTML_TAGSPLIT":
		return 1, 3, true, true
	case "TIMES":
		return 2, 2, true, true
	case "PRINTDATA", "PRINTDATAL", "PRINTDATAW", "PRINTDATAK", "PRINTDATAKL", "PRINTDATAKW", "PRINTDATAD", "PRINTDATADL", "PRINTDATADW", "STRDATA":
		return 0, 1, true, true
	case "PRINTSN", "PRINTFORMSN":
		return 1, 1, false, true
	default:
		return 0, 0, false, false
	}
}

func corpusCommandShapeValid(command string, tail []byte, dialect format.Dialect, allowFullwidth bool) bool {
	min, max, allowTrailing, ok := corpusCommandArity(command, dialect)
	if !ok {
		return true
	}
	comment := incrementCommentStart(tail)
	if comment < 0 || comment > len(tail) {
		comment = len(tail)
	}
	code := tail[:comment]
	if len(trimGrammarSpaceBoth(code, allowFullwidth)) == 0 {
		return min == 0
	}
	commas := topLevelCommas(code)
	fields := splitFingerprintFields(code, commas)
	trailing := len(fields) > 1 && len(trimGrammarSpaceBoth(fields[len(fields)-1], allowFullwidth)) == 0
	if trailing {
		if !allowTrailing {
			return false
		}
		fields = fields[:len(fields)-1]
	}
	if len(fields) < min || len(fields) > max {
		return false
	}
	for _, field := range fields {
		if len(trimGrammarSpaceBoth(field, allowFullwidth)) == 0 {
			return false
		}
	}
	if command == "SETCOLOR" || command == "SETBGCOLOR" {
		if len(fields) != 1 && len(fields) != 3 {
			return false
		}
	}
	return corpusCommandFieldTypesValid(command, fields, dialect, allowFullwidth)
}

type corpusValueType uint8

const (
	corpusValueUnknown corpusValueType = iota
	corpusValueNumeric
	corpusValueString
	corpusValueInvalid
)

func corpusCommandFieldTypesValid(command string, fields [][]byte, dialect format.Dialect, allowFullwidth bool) bool {
	if command == "SAVEVAR" && !corpusSAVEVARDestinationsValid(fields, dialect, allowFullwidth) {
		return false
	}
	for i, field := range fields {
		if allowFullwidth {
			field = normalizeFullwidthExpressionSpaces(field)
		}
		field = trimCorpusSpace(field)
		expected := corpusCommandFieldType(command, i, dialect)
		actual := corpusProvenValueType(field, dialect)
		if actual == corpusValueInvalid || expected != corpusValueUnknown && actual != corpusValueUnknown && expected != actual {
			return false
		}
	}
	return true
}

func corpusCommandFieldType(command string, index int, dialect format.Dialect) corpusValueType {
	switch command {
	case "BAR", "BARL", "SETCOLOR", "SETBGCOLOR", "TWAIT", "SWAPCHARA", "COPYCHARA",
		"ADDCHARA", "ADDSPCHARA", "ADDCOPYCHARA", "DELCHARA", "PICKUPCHARA", "TOOLTIP_SETCOLOR",
		"FONTSTYLE", "LOADDATA", "ASSERT", "TOOLTIP_SETDELAY", "TOOLTIP_SETDURATION",
		"AWAIT", "INPUTMOUSEKEY", "RANDOMIZE", "BREAKBUTTON", "SETBGMVOLUME", "SETSOUNDVOLUME",
		"TOOLTIP_SETFONTSIZE", "TOOLTIP_CUSTOM", "TOOLTIP_FORMAT", "TOOLTIP_IMG", "PRINTCPERLINE", "SAVENOS":
		return corpusValueNumeric
	case "SETFONT", "LOADCHARA", "LOADVAR", "PLAYBGM", "TOOLTIP_SETFONT", "PRINTSN", "PRINTFORMSN", "ARRAYCOPY":
		return corpusValueString
	case "SAVEDATA":
		if index == 1 {
			return corpusValueString
		}
		return corpusValueNumeric
	case "SAVECHARA":
		if index < 2 {
			return corpusValueString
		}
		return corpusValueNumeric
	case "SAVEVAR":
		if index < 2 {
			return corpusValueString
		}
	case "PRINTBUTTON", "PRINTBUTTONC", "PRINTBUTTONLC":
		if index == 0 {
			return corpusValueString
		}
	case "ARRAYREMOVE":
		if index > 0 {
			return corpusValueNumeric
		}
	case "ARRAYSHIFT":
		if index == 1 || index >= 3 {
			return corpusValueNumeric
		}
	case "ARRAYSORT":
		if index >= 2 {
			return corpusValueNumeric
		}
	case "HTML_TAGSPLIT":
		if index < 2 {
			return corpusValueString
		}
		return corpusValueNumeric
	case "HTML_PRINT":
		if index == 0 {
			return corpusValueString
		}
		if dialect == format.EMEE {
			return corpusValueNumeric
		}
	case "HTML_PRINT_ISLAND", "PLAYSOUND":
		if index == 0 {
			return corpusValueString
		}
		return corpusValueNumeric
	}
	return corpusValueUnknown
}

func corpusProvenValueType(src []byte, dialect format.Dialect) corpusValueType {
	src = corpusStripOuterParens(src)
	if corpusBuiltinStringRepetition(src, dialect) {
		return corpusValueString
	}
	if corpusQuotedStringExpression(src) {
		return corpusValueString
	}
	if bytes.HasPrefix(src, []byte(`@"`)) {
		if end, complete := quotedFormEndComplete(src, 0); complete && end == len(src) {
			return corpusValueString
		}
	}
	if bytes.HasPrefix(src, []byte(`\@`)) {
		_, _, end, ok := formConditional(src, 0)
		if ok && end+2 == len(src) {
			return corpusValueString
		}
	}
	if bytes.IndexByte(src, 0) >= 0 {
		return corpusValueUnknown
	}
	if value := corpusDirectBuiltinValueType(src, dialect); value != corpusValueUnknown {
		return value
	}
	if graphicsNumericField(src, false) && !graphicsNumericUnknown(src) {
		return corpusValueNumeric
	}
	return corpusValueUnknown
}

func corpusBuiltinStringRepetition(src []byte, dialect format.Dialect) bool {
	star := bytes.IndexByte(src, '*')
	if star < 0 || !corpusStringBuiltin(string(trimCorpusSpace(src[:star])), dialect) {
		return false
	}
	factor := star + 1
	for i := factor; i <= len(src); i++ {
		if i < len(src) && src[i] == '(' {
			end := matchingCallDelimiter(src, i, '(', ')')
			if end < 0 {
				return false
			}
			i = end
			continue
		}
		if i < len(src) && src[i] != '*' {
			if strings.ContainsRune("+-/%=&|!<>^~?#,:@\\\"'", rune(src[i])) {
				return false
			}
			continue
		}
		operand := corpusStripOuterParens(src[factor:i])
		if corpusDirectBuiltinValueType(operand, dialect) != corpusValueNumeric {
			if _, ok := corpusFormConstant(operand); !ok {
				return false
			}
		}
		factor = i + 1
	}
	return true
}

func corpusStripOuterParens(src []byte) []byte {
	src = trimCorpusSpace(src)
	if len(src) < 2 || src[0] != '(' {
		return src
	}
	if len(src) >= 128 {
		if pairs, ok := corpusOuterParenPairs(src); ok && pairs[0] == len(src)-1 {
			left, right := 1, pairs[0]
			for {
				for left < right && corpusHalfSpace(src[left]) {
					left++
				}
				for right > left && corpusHalfSpace(src[right-1]) {
					right--
				}
				if left >= right || src[left] != '(' {
					return src[left:right]
				}
				close := pairs[left]
				if close == 0 || close >= right {
					return src[left:right]
				}
				trailing := true
				for i := close + 1; i < right; i++ {
					if !corpusHalfSpace(src[i]) {
						trailing = false
						break
					}
				}
				if !trailing {
					return src[left:right]
				}
				left++
				right = close
			}
		}
	}
	for len(src) >= 2 && src[0] == '(' {
		end := matchingCallDelimiter(src, 0, '(', ')')
		if end != len(src)-1 {
			break
		}
		src = trimCorpusSpace(src[1:end])
	}
	return src
}

func corpusOuterParenPairs(src []byte) ([]int, bool) {
	pairs := make([]int, len(src))
	stack := make([]int, 0, 8)
	for i := 0; i < len(src); i++ {
		switch src[i] {
		case '"', '\'':
			i = printVQuotedEnd(src, i) - 1
		case '@':
			if i+1 < len(src) && src[i+1] == '"' {
				if end := quotedFormEnd(src, i); end > i+2 && end <= len(src) {
					i = end - 1
				}
			}
		case '\\':
			if i+1 < len(src) {
				i++
			}
		case ';':
			return nil, false
		case '(':
			stack = append(stack, i)
		case ')':
			if len(stack) == 0 {
				return nil, false
			}
			open := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			pairs[open] = i
		}
	}
	if len(stack) != 0 {
		return nil, false
	}
	return pairs, true
}

func corpusDirectBuiltinValueType(src []byte, dialect format.Dialect) corpusValueType {
	if bytes.IndexByte(src, 0) >= 0 {
		return corpusValueUnknown
	}
	if colon := bytes.IndexByte(src, ':'); colon >= 0 {
		name := string(trimCorpusSpace(src[:colon]))
		index := trimCorpusSpace(src[colon+1:])
		if !corpusStringBuiltin(name, dialect) || len(index) == 0 {
			return corpusValueUnknown
		}
		for _, b := range index {
			if b < '0' || b > '9' {
				return corpusValueUnknown
			}
		}
		return corpusValueString
	}
	hasNumeric, hasString := false, false
	for i := 0; i < len(src); {
		b := src[i]
		if b == '"' || b == '\'' || b == '(' || b == '[' || b == '@' || b == '\\' || b >= 0x80 {
			return corpusValueUnknown
		}
		if b >= '0' && b <= '9' {
			hasNumeric = true
			i++
			continue
		}
		if (b < 'A' || b > 'Z') && (b < 'a' || b > 'z') && b != '_' {
			i++
			continue
		}
		if (b == 'x' || b == 'X') && i > 0 && src[i-1] == '0' || (b == 'e' || b == 'E' || b == 'p' || b == 'P') && i > 0 && src[i-1] >= '0' && src[i-1] <= '9' {
			i++
			continue
		}
		end := i + 1
		for end < len(src) && isGraphicsIdentifierByte(src[end]) {
			end++
		}
		next := end
		for next < len(src) && corpusHalfSpace(src[next]) {
			next++
		}
		if next < len(src) && (src[next] == '(' || src[next] == '@') {
			return corpusValueUnknown
		}
		name := string(src[i:end])
		switch {
		case corpusStringBuiltin(name, dialect):
			hasString = true
		case len(name) == 1 && name[0] >= 'A' && name[0] <= 'Z':
			hasNumeric = true
		case corpusNameKnown(corpusNumericBuiltinVariableSet, name):
			hasNumeric = true
		default:
			return corpusValueUnknown
		}
		i = end
	}
	if hasNumeric && hasString {
		return corpusValueInvalid
	}
	if hasString {
		return corpusValueString
	}
	if hasNumeric {
		return corpusValueNumeric
	}
	return corpusValueUnknown
}

func corpusKnownStringIndexNeedsExact(src []byte, dialect format.Dialect) bool {
	src = corpusStripOuterParens(src)
	colon := bytes.IndexByte(src, ':')
	if colon <= 0 {
		return false
	}
	name := string(trimCorpusSpace(src[:colon]))
	if !corpusStringBuiltin(name, dialect) {
		return false
	}
	index := corpusStripOuterParens(src[colon+1:])
	if len(index) == 0 {
		return true
	}
	if len(index) == 1 && index[0] >= 'A' && index[0] <= 'Z' || corpusNameKnown(corpusNumericBuiltinVariableSet, string(index)) {
		return true
	}
	return corpusProvenValueType(index, dialect) == corpusValueNumeric
}

func corpusStringIndexInvalidForPercent(src []byte, dialect format.Dialect) bool {
	src = corpusStripOuterParens(trimCorpusSpace(src))
	start := 0
	for start < len(src) && isFingerprintWord(src[start]) {
		start++
	}
	if start == 0 || !corpusStringBuiltin(string(src[:start]), dialect) {
		return false
	}
	for start < len(src) && corpusHalfSpace(src[start]) {
		start++
	}
	if start >= len(src) || src[start] != ':' {
		return false
	}
	start++
	for start < len(src) && corpusHalfSpace(src[start]) {
		start++
	}
	if start == len(src) {
		return true
	}
	end := corpusStringIndexEnd(src, start)
	if end != len(src) {
		return false
	}
	index := corpusStripOuterParens(src[start:end])
	if corpusMixedQuotedNumericExpressionNeedsExact(index, dialect) {
		return true
	}
	value := corpusProvenValueType(index, dialect)
	return value == corpusValueString || value == corpusValueInvalid
}

func corpusMixedQuotedNumericExpressionNeedsExact(src []byte, dialect format.Dialect) bool {
	src = corpusStripOuterParens(src)
	parts := make([][]byte, 0, 2)
	start := 0
	quoted := byte(0)
	depth := 0
	for i := 0; i < len(src); i++ {
		if quoted != 0 {
			if src[i] == '\\' {
				i++
			} else if src[i] == quoted {
				quoted = 0
			}
			continue
		}
		switch src[i] {
		case '"', '\'':
			quoted = src[i]
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
		case '+':
			if depth != 0 {
				continue
			}
			parts = append(parts, src[start:i])
			start = i + 1
		}
	}
	if len(parts) == 0 {
		return false
	}
	parts = append(parts, src[start:])
	hasString, hasNumeric := false, false
	for _, part := range parts {
		part = corpusStripOuterParens(part)
		if corpusQuotedStringExpression(part) {
			hasString = true
		} else if corpusProvenValueType(part, dialect) == corpusValueNumeric {
			hasNumeric = true
		} else {
			return false
		}
	}
	return hasString && hasNumeric
}

func corpusSAVEVARDestinationsValid(fields [][]byte, dialect format.Dialect, allowFullwidth bool) bool {
	seen := make(map[string]struct{})
	for _, field := range fields[2:] {
		if allowFullwidth {
			field = normalizeFullwidthExpressionSpaces(field)
		}
		field = corpusStripMandatoryMarkers(field)
		field = corpusStripOuterParens(field)
		unaryPlus := len(field) != 0 && field[0] == '+'
		if unaryPlus {
			field = corpusStripOuterParens(field[1:])
		}
		if !assignmentLHS(field) {
			return false
		}
		end := 0
		for end < len(field) && isLHSIdentifierByte(field[end]) {
			end++
		}
		name := string(field[:end])
		if unaryPlus && corpusStringBuiltin(name, dialect) {
			return false
		}
		if corpusNameKnown(corpusCharacterBuiltinVariableSet, name) || corpusNameKnown(corpusReadOnlyBuiltinVariableSet, name) || dialect == format.EMEE && corpusNameKnown(corpusModernReadOnlyBuiltinVariableSet, name) {
			return false
		}
		pos := end
		for pos < len(field) && corpusHalfSpace(field[pos]) {
			pos++
		}
		_, numeric := corpusNumericBuiltinVariableSet[name]
		numeric = numeric || len(name) == 1 && name[0] >= 'A' && name[0] <= 'Z'
		known := numeric || corpusStringBuiltin(name, dialect)
		if known && (pos == len(field) || field[pos] != '@') {
			if _, duplicate := seen[name]; duplicate {
				return false
			}
			seen[name] = struct{}{}
		}
	}
	return true
}

func corpusStripMandatoryMarkers(src []byte) []byte {
	marker := []byte(";!;")
	if !bytes.Contains(src, marker) {
		return src
	}
	out := make([]byte, 0, len(src))
	for len(src) != 0 {
		pos := bytes.Index(src, marker)
		if pos < 0 {
			return append(out, src...)
		}
		out = append(out, src[:pos]...)
		src = src[pos+len(marker):]
	}
	return out
}

func saveVarFingerprint(tail []byte, dialect format.Dialect, allowFullwidth bool) []byte {
	comment := corpusCommandCommentStart(tail)
	code := tail[:comment]
	fields := splitFingerprintFields(code, topLevelCommas(code))
	if allowFullwidth {
		for i, field := range fields {
			fields[i] = normalizeFullwidthExpressionSpaces(field)
		}
	}
	trailing := false
	if len(fields) > 1 && len(trimGrammarSpaceBoth(fields[len(fields)-1], allowFullwidth)) == 0 {
		trailing = true
		fields = fields[:len(fields)-1]
	}
	deferred := corpusSAVEVARDeferredShape(fields, dialect)
	if corpusSAVEVARMalformedDeferredFallback(fields, dialect) {
		return tokenFingerprintDialect(tail, dialect)
	}
	if len(fields) > 255 || len(fields) < 3 && !deferred || !deferred && !corpusCommandFieldTypesValid("SAVEVAR", fields, dialect, allowFullwidth) {
		return invalidCommandTailFingerprint(tail)
	}
	var out bytes.Buffer
	for i, field := range fields {
		if len(trimGrammarSpaceBoth(field, allowFullwidth)) == 0 {
			return invalidCommandTailFingerprint(tail)
		}
		if i != 0 {
			out.WriteByte(',')
		}
		out.Write(saveVarFieldFingerprint(field, allowFullwidth))
	}
	if trailing {
		out.WriteByte(',')
	}
	if comment < len(tail) {
		out.WriteByte('C')
		out.WriteByte(0)
		out.Write(commentFingerprint(tail[comment:]))
	}
	return out.Bytes()
}

func corpusSAVEVARMalformedDeferredFallback(fields [][]byte, dialect format.Dialect) bool {
	if len(fields) != 1 {
		return false
	}
	field := trimCorpusSpace(fields[0])
	if printVNumericLiteralNeedsExact(field) {
		return false
	}
	if corpusSAVEVARDeferredCallField(field, dialect) {
		return true
	}
	if len(field) < 2 || field[0] != '"' {
		return false
	}
	end := printVQuotedEnd(field, 0)
	for end < len(field) && corpusHalfSpace(field[end]) {
		end++
	}
	if end <= 1 || end >= len(field) || field[end] < '0' || field[end] > '9' {
		return false
	}
	for end < len(field) && field[end] >= '0' && field[end] <= '9' {
		end++
	}
	for end < len(field) && corpusHalfSpace(field[end]) {
		end++
	}
	return end < len(field) && field[end] == '(' && corpusSAVEVARFieldMayExpand(field, dialect)
}

func corpusSAVEVARTailMalformedDeferredFallback(tail []byte, dialect format.Dialect, allowFullwidth bool) bool {
	comment := corpusCommandCommentStart(tail)
	code := tail[:comment]
	fields := splitFingerprintFields(code, topLevelCommas(code))
	if allowFullwidth {
		for i, field := range fields {
			fields[i] = normalizeFullwidthExpressionSpaces(field)
		}
	}
	return corpusSAVEVARMalformedDeferredFallback(fields, dialect)
}

func corpusSAVEVARDeferredCallField(field []byte, dialect format.Dialect) bool {
	if len(field) == 0 || !isLHSIdentifierStart(field[0]) {
		return false
	}
	end := 1
	for end < len(field) && isLHSIdentifierByte(field[end]) {
		end++
	}
	name := string(field[:end])
	for end < len(field) && corpusHalfSpace(field[end]) {
		end++
	}
	if end >= len(field) || field[end] != '(' {
		return false
	}
	if len(name) == 1 && name[0] >= 'A' && name[0] <= 'Z' || corpusNameKnown(corpusNumericBuiltinVariableSet, name) || corpusStringBuiltin(name, dialect) || corpusBuiltinMethod(name, dialect) {
		return false
	}
	return true
}

func corpusSAVEVARDeferredShape(fields [][]byte, dialect format.Dialect) bool {
	if len(fields) == 0 {
		return false
	}
	for _, field := range fields {
		field = trimCorpusSpace(field)
		if len(field) == 0 || bytes.Contains(field, []byte("[[")) || bytes.Contains(field, []byte(";#;")) || bytes.Contains(field, []byte(";^;")) {
			return false
		}
	}
	for _, field := range fields {
		field = trimCorpusSpace(field)
		if corpusSAVEVARFieldMayExpand(field, dialect) {
			return true
		}
		if value := corpusProvenValueType(field, dialect); value == corpusValueNumeric || value == corpusValueInvalid {
			return false
		}
	}
	return false
}

func corpusSAVEVARFieldMayExpand(src []byte, dialect format.Dialect) bool {
	if len(src) == 0 {
		return false
	}
	if bytes.HasPrefix(src, []byte(`@"`)) {
		end, complete := quotedFormEndComplete(src, 0)
		return complete && end == len(src)
	}
	if bytes.HasPrefix(src, []byte(`\@`)) {
		_, _, end, ok := formConditional(src, 0)
		return ok && end+2 == len(src)
	}
	if corpusQuotedStringExpression(src) {
		return false
	}
	if value := corpusDirectBuiltinValueType(src, dialect); value != corpusValueUnknown {
		return false
	}
	for i := 0; i < len(src); {
		if src[i] == '"' {
			i = printVQuotedEnd(src, i)
			continue
		}
		if src[i] == 0 || src[i] >= 0x7f {
			return true
		}
		if src[i] >= '0' && src[i] <= '9' {
			i = fingerprintNumberEnd(src, i)
			continue
		}
		if isFingerprintWord(src[i]) && (src[i] < 0x80) {
			start := i
			i++
			for i < len(src) && isFingerprintWord(src[i]) && src[i] < 0x80 {
				i++
			}
			name := string(src[start:i])
			for i < len(src) && corpusHalfSpace(src[i]) {
				i++
			}
			if i < len(src) && (src[i] == '(' || src[i] == '@') {
				return true
			}
			if !corpusNameKnown(corpusNumericBuiltinVariableSet, name) && !corpusStringBuiltin(name, dialect) && !(len(name) == 1 && name[0] >= 'A' && name[0] <= 'Z') {
				return true
			}
			continue
		}
		i++
	}
	return false
}

func saveVarFieldFingerprint(src []byte, allowFullwidth bool) []byte {
	if allowFullwidth {
		src = normalizeFullwidthExpressionSpaces(src)
	}
	return expressionFingerprint(src)
}

func corpusCommandCommentStart(src []byte) int {
	for i := 0; i < len(src); i++ {
		switch src[i] {
		case '"':
			i = printVQuotedEnd(src, i) - 1
		case '@':
			if i+1 < len(src) && src[i+1] == '"' {
				if end := quotedFormEnd(src, i); end > i+2 && end <= len(src) {
					i = end - 1
				}
			}
		case '\\':
			if i+1 < len(src) && src[i+1] == '@' {
				if _, _, end, ok := formConditional(src, i); ok {
					i = end + 1
				}
			}
		case ';':
			if bytes.HasPrefix(src[i:], []byte(";!;")) {
				i += 2
				continue
			}
			return i
		}
	}
	return len(src)
}

func corpusStringBuiltin(name string, dialect format.Dialect) bool {
	if corpusNameKnown(corpusStringBuiltinVariableSet, name) {
		return true
	}
	return dialect == format.EMEE && corpusNameKnown(corpusModernStringBuiltinVariableSet, name)
}

func corpusNameKnown(set map[string]struct{}, name string) bool {
	_, ok := set[name]
	return ok
}

func corpusQuotedStringExpression(src []byte) bool {
	src = trimCorpusSpace(src)
	wantString := true
	for len(src) != 0 {
		src = bytes.TrimLeft(src, " \t")
		if len(src) == 0 {
			return !wantString
		}
		if wantString {
			if len(src) == 0 || src[0] != '"' {
				return false
			}
			end := printVQuotedEnd(src, 0)
			if end == 0 || end > len(src) || src[end-1] != '"' {
				return false
			}
			src, wantString = src[end:], false
			continue
		}
		if src[0] != '+' {
			return false
		}
		src, wantString = src[1:], true
	}
	return !wantString
}

func isPrintFormsCommand(command string) bool {
	switch command {
	case "PRINTFORMS", "PRINTFORMSL", "PRINTFORMSW", "PRINTFORMSK", "PRINTFORMSKL", "PRINTFORMSKW", "PRINTFORMSD", "PRINTFORMSDL", "PRINTFORMSDW", "PRINTSINGLEFORMS", "PRINTSINGLEFORMSK", "PRINTSINGLEFORMSD":
		return true
	default:
		return false
	}
}

func printFormsLiteralFingerprintDialect(tail []byte, dialect format.Dialect) ([]byte, bool) {
	start := 0
	for start < len(tail) && (tail[start] == ' ' || tail[start] == '\t') {
		start++
	}
	if start == 0 || start >= len(tail) || tail[start] != '"' {
		return nil, false
	}
	end := printVQuotedEnd(tail, start)
	if end <= start+1 || end > len(tail) || tail[end-1] != '"' {
		return nil, false
	}
	inner := tail[start+1 : end-1]
	if bytes.Contains(inner, []byte("[[")) || bytes.IndexByte(inner, '\\') >= 0 {
		return nil, false
	}
	for i := end; i < len(tail); i++ {
		if tail[i] != ' ' && tail[i] != '\t' && tail[i] != ';' {
			return nil, false
		}
		if tail[i] == ';' {
			break
		}
	}
	var out bytes.Buffer
	out.WriteByte(' ')
	out.WriteByte('"')
	if mandatoryMarkerIndex(tail[end:]) >= 0 {
		out.WriteByte('R')
		fmt.Fprintf(&out, "%d:", len(inner))
		out.Write(inner)
	} else {
		out.Write(formFingerprintDialect(inner, dialect))
	}
	out.WriteByte('"')
	rest := tail[end:]
	comment := bytes.IndexByte(rest, ';')
	if comment >= 0 {
		out.WriteByte(' ')
		out.Write(commentFingerprint(rest[comment:]))
	} else {
		out.Write(bytes.TrimRight(rest, " \t"))
	}
	return out.Bytes(), true
}

func directSTRFORMLiteralFingerprint(tail []byte, allowFullwidth bool) ([]byte, bool) {
	return directSTRFORMLiteralFingerprintDialect(tail, allowFullwidth, format.CommonDialect)
}

func directSTRFORMLiteralFingerprintDialect(tail []byte, allowFullwidth bool, dialect format.Dialect) ([]byte, bool) {
	if allowFullwidth {
		tail = normalizeFullwidthPrintFormsTailDialect(tail, dialect)
	}
	return printFormsLiteralFingerprintDialect(tail, dialect)
}

func corpusDialectRestrictedCommand(command string) bool {
	switch command {
	case "CALLSHARP", "FORCE_QUIT", "FORCE_QUIT_AND_RESTART", "QUIT_AND_RESTART", "SKIPLOG", "TRYCALLF", "TRYCALLFORMF":
		return true
	default:
		return false
	}
}

func corpusPrintDialectRestrictedCommand(command string) bool {
	switch command {
	case "PRINTN", "PRINTVN", "PRINTSN", "PRINTFORMN", "PRINTFORMSN":
		return true
	default:
		return false
	}
}

func corpusUnavailableCommandForFingerprint(command string, dialect format.Dialect) bool {
	return dialect != format.EMEE && (corpusDialectRestrictedCommand(command) || corpusPrintDialectRestrictedCommand(command))
}

func isRawCommand(command string, dialect format.Dialect) bool {
	if command == "CUSTOMDRAWLINE" || command == "PRINTN" {
		return true
	}
	if command == "BEGIN" || command == "CALLEVENT" || command == "ALIGNMENT" || command == "SETCOLORBYNAME" || command == "SETBGCOLORBYNAME" || command == "STRLEN" || command == "STRLENU" {
		return true
	}
	if command == "FORCE_BEGIN" {
		return dialect == format.EMEE
	}
	if dialect == format.EMEE {
		return false
	}
	switch command {
	case "PRINTVN", "PRINTSN", "PRINTFORMN", "PRINTFORMSN":
		return true
	default:
		return false
	}
}

func corpusStructuredCommand(command string, dialect format.Dialect) (kind string, min, max int, ok bool) {
	if command == "TOOLTIP_SETCOLOR" {
		return "expression", 2, 2, true
	}
	if dialect != format.EMEE {
		switch command {
		case "AWAIT", "INPUTMOUSEKEY", "RANDOMIZE":
			return "expression", 0, 1, true
		}
		return "", 0, 0, false
	}
	switch command {
	case "AWAIT", "INPUTMOUSEKEY", "RANDOMIZE", "BREAKBUTTON":
		return "expression", 0, 1, true
	case "SETBGMVOLUME", "SETSOUNDVOLUME", "TOOLTIP_SETFONTSIZE", "TOOLTIP_CUSTOM", "TOOLTIP_FORMAT", "TOOLTIP_IMG", "PLAYBGM", "TOOLTIP_SETFONT":
		return "expression", 1, 1, true
	case "PLAYSOUND", "HTML_PRINT_ISLAND":
		return "expression", 1, 2, true
	case "SETBGIMAGE":
		return "form-any", 1, 255, true
	case "REMOVEBGIMAGE":
		return "form-any", 1, 255, true
	default:
		return "", 0, 0, false
	}
}

func structuredCommandFingerprintDialect(command string, tail []byte, kind string, min, max int, dialect format.Dialect) []byte {
	if kind == "form-any" {
		return formAnyFingerprint(tail)
	}
	comment := incrementCommentStart(tail)
	if comment < 0 || comment > len(tail) {
		comment = len(tail)
	}
	code := tail[:comment]
	if len(trimCorpusSpace(code)) == 0 {
		if min == 0 {
			return noArgFingerprint(tail)
		}
		return invalidCommandTailFingerprint(tail)
	}
	parts := splitFingerprintFields(code, topLevelCommas(code))
	trailing := len(parts) > 1 && len(trimCorpusSpace(parts[len(parts)-1])) == 0
	if trailing {
		if !corpusStructuredAllowsTrailing(command) {
			return invalidCommandTailFingerprint(tail)
		}
		parts = parts[:len(parts)-1]
	}
	if len(parts) < min || len(parts) > max {
		return invalidCommandTailFingerprint(tail)
	}
	for _, part := range parts {
		if len(trimCorpusSpace(part)) == 0 {
			return invalidCommandTailFingerprint(tail)
		}
	}
	if !corpusCommandFieldTypesValid(command, parts, format.EMEE, false) {
		return invalidCommandTailFingerprint(tail)
	}
	var out bytes.Buffer
	out.Write(expressionFingerprintDialect(code, dialect))
	if comment < len(tail) {
		out.Write(commentFingerprint(tail[comment:]))
	}
	return out.Bytes()
}

func invalidCommandTailFingerprint(tail []byte) []byte {
	if mandatoryMarkerIndex(tail) >= 0 {
		return append([]byte{'R'}, expressionFingerprint(tail)...)
	}
	body := bytes.TrimLeft(tail, " \t")
	if len(body) != 0 && body[0] != ';' && body[0] != ',' {
		tail = commandTail(tail)
	}
	return append([]byte{'R'}, tail...)
}

func invalidPrintVCommandTailFingerprint(tail []byte) []byte {
	body := bytes.TrimLeft(tail, " \t")
	if len(body) != 0 && body[0] == ',' {
		tail = append([]byte{' '}, body...)
	}
	return invalidCommandTailFingerprint(tail)
}

func corpusStructuredAllowsTrailing(command string) bool {
	switch command {
	case "TOOLTIP_SETCOLOR", "PLAYSOUND", "HTML_PRINT_ISLAND":
		return true
	default:
		return false
	}
}

func formAnyFingerprint(tail []byte) []byte {
	if len(tail) == 0 || (tail[0] != ' ' && tail[0] != '\t' && tail[0] != ';') {
		return append([]byte{'R'}, tail...)
	}
	semicolon := tail[0] == ';'
	tail = tail[1:]
	tail = bytes.TrimLeft(tail, " ")
	var fingerprint []byte
	if len(tail) == 0 {
		fingerprint = []byte{'R'}
	} else {
		fingerprint = formFingerprint(tail)
	}
	if semicolon {
		return append([]byte{';'}, fingerprint...)
	}
	return fingerprint
}

func variableSizeFingerprint(tail []byte) []byte {
	comment := incrementCommentStart(tail)
	if comment < 0 || comment > len(tail) {
		comment = len(tail)
	}
	name := trimCorpusSpace(tail[:comment])
	if !bareIdentifier(name) {
		return append([]byte{'R'}, tail...)
	}
	out := append([]byte{'V'}, name...)
	if comment < len(tail) {
		out = append(out, commentFingerprint(tail[comment:])...)
	}
	return out
}

func bareIdentifier(src []byte) bool {
	if len(src) == 0 || !isLHSIdentifierStart(src[0]) {
		return false
	}
	for _, b := range src[1:] {
		if !isLHSIdentifierByte(b) {
			return false
		}
	}
	return true
}

func dataTableColumnOptionsFingerprint(tail []byte) []byte {
	comment := incrementCommentStart(tail)
	if comment < 0 || comment > len(tail) {
		comment = len(tail)
	}
	code := tail[:comment]
	parts := splitFingerprintFields(code, topLevelCommas(code))
	trailing := len(parts) > 0 && len(trimCorpusSpace(parts[len(parts)-1])) == 0
	if trailing {
		parts = parts[:len(parts)-1]
	}
	if len(parts) < 4 || (len(parts)-2)%2 != 0 {
		return append([]byte{'R'}, tail...)
	}
	var out bytes.Buffer
	for i, part := range parts {
		if i != 0 {
			out.WriteByte(',')
		}
		field := trimCorpusSpace(part)
		if len(field) == 0 {
			return append([]byte{'R'}, tail...)
		}
		if i >= 2 && i%2 == 0 {
			if !bytes.EqualFold(field, []byte("DEFAULT")) {
				return append([]byte{'R'}, tail...)
			}
			out.Write(field)
			continue
		}
		out.Write(expressionFingerprint(field))
	}
	if trailing {
		out.WriteByte(',')
	}
	if comment < len(tail) {
		out.Write(commentFingerprint(tail[comment:]))
	}
	return out.Bytes()
}

func isInputCommand(command string, dialect format.Dialect) bool {
	switch command {
	case "INPUTS", "ONEINPUTS":
		return true
	case "BINPUTS", "ONEBINPUTS":
		return dialect == format.EMEE
	default:
		return false
	}
}

func isInputExpressionCommand(command string, dialect format.Dialect) bool {
	switch command {
	case "INPUT", "ONEINPUT", "TINPUT", "TINPUTS", "TONEINPUT", "TONEINPUTS", "BINPUT", "ONEBINPUT":
		return command != "BINPUT" && command != "ONEBINPUT" || dialect == format.EMEE
	case "BINPUTS", "ONEBINPUTS":
		return false
	default:
		return false
	}
}

func corpusInputAllowsTrailing(command string, dialect format.Dialect) bool {
	if dialect != format.CommonDialect && dialect != format.Emuera1824 && dialect != format.EMEE {
		return false
	}
	switch command {
	case "TINPUT", "TINPUTS", "TONEINPUT", "TONEINPUTS":
		return true
	default:
		return false
	}
}

func isExtraVariableCommand(command string) bool {
	switch command {
	case "PRINTDATA", "PRINTDATAL", "PRINTDATAW", "PRINTDATAK", "PRINTDATAKL", "PRINTDATAKW", "PRINTDATAD", "PRINTDATADL", "PRINTDATADW", "STRDATA":
		return true
	default:
		return false
	}
}

func isGraphicsCommand(command string) bool {
	switch command {
	case "PRINT_IMG", "PRINT_RECT", "PRINT_SPACE":
		return true
	default:
		return false
	}
}

func graphicsFingerprint(command string, tail []byte, dialect format.Dialect) []byte {
	return graphicsFingerprintMode(command, tail, dialect, false)
}

func graphicsFingerprintMode(command string, tail []byte, dialect format.Dialect, allowFullwidth bool) []byte {
	comment := incrementCommentStart(tail)
	if comment < 0 || comment > len(tail) {
		comment = len(tail)
	}
	code := tail[:comment]
	parts := splitFingerprintFields(code, topLevelCommas(code))
	if allowFullwidth {
		normalized := make([]byte, 0, len(code))
		for i, part := range parts {
			if i != 0 {
				normalized = append(normalized, ',')
			}
			normalized = append(normalized, normalizeFullwidthExpressionSpaces(part)...)
		}
		code = normalized
		parts = splitFingerprintFields(code, topLevelCommas(code))
	}
	code = normalizeMandatoryMarkerTailTrivia(code, allowFullwidth)
	validationCode := code
	if marker := mandatoryMarkerIndex(code); marker >= 0 {
		suffix := code[marker+3:]
		if commas := topLevelCommas(suffix); len(commas) != 0 && len(trimCorpusSpace(suffix[:commas[0]])) == 0 {
			validationCode = append(append([]byte(nil), code[:marker]...), suffix[commas[0]:]...)
		} else {
			validationCode = code[:marker]
		}
	}
	parts = splitFingerprintFields(validationCode, topLevelCommas(validationCode))
	if len(parts) > 1 && len(trimCorpusSpace(parts[len(parts)-1])) == 0 {
		parts = parts[:len(parts)-1]
	}
	if comment < len(tail) && !corpusProtectedComment(tail[comment:]) && !graphicsCommandGapTrusted(tail, allowFullwidth) {
		return invalidCommandTailFingerprint(tail)
	}
	valid := true
	switch command {
	case "PRINT_IMG":
		if dialect != format.EMEE {
			valid = len(parts) == 1 && graphicsKnownStringFieldMode(parts[0], dialect)
			break
		}
		if len(parts) < 1 || len(parts) > 6 {
			valid = false
			break
		}
		if !graphicsKnownStringFieldMode(parts[0], dialect) {
			valid = false
			break
		}
		names, numeric := 1, false
		for _, part := range parts[1:] {
			switch {
			case graphicsStringFieldMode(part, dialect):
				if numeric || names == 3 {
					valid = false
				}
				names++
			case graphicsNumericFieldMode(part, true, dialect):
				if graphicsNumericUnknownMode(part, dialect) && !numeric && !graphicsDeferredStringExpression(part, dialect) {
					valid = false
				}
				numeric = true
			case graphicsDeferredStringExpression(part, dialect):
				if numeric {
					valid = false
				}
				numeric = true
			default:
				valid = false
			}
		}
		valid = valid && names <= 3
	case "PRINT_RECT":
		valid = (len(parts) == 1 || len(parts) == 4) && graphicsNumericFieldsMode(parts, dialect == format.EMEE, dialect)
	case "PRINT_SPACE":
		valid = len(parts) == 1 && graphicsNumericFieldMode(parts[0], dialect == format.EMEE, dialect)
	default:
		valid = false
	}
	if !valid {
		if comment < len(tail) && !corpusProtectedComment(tail[comment:]) && graphicsCommandGapTrusted(tail, allowFullwidth) && graphicsDeferredShape(command, parts, dialect) {
			return graphicsInvalidTailFingerprint(tail, comment, allowFullwidth)
		}
		return invalidCommandTailFingerprint(tail)
	}
	var out bytes.Buffer
	out.Write(expressionFingerprint(code))
	if comment < len(tail) {
		out.Write(commentFingerprint(tail[comment:]))
	}
	return out.Bytes()
}

func normalizeMandatoryMarkerTailTrivia(src []byte, allowFullwidth bool) []byte {
	marker := mandatoryMarkerIndex(src)
	if marker < 0 {
		return src
	}
	suffix := src[marker+3:]
	if len(trimGrammarSpaceBoth(suffix, allowFullwidth)) == 0 {
		return src
	}
	trimmed := trimGrammarSpaceRight(suffix, allowFullwidth)
	if len(trimmed) == len(suffix) {
		return src
	}
	out := make([]byte, 0, marker+3+len(trimmed))
	out = append(out, src[:marker+3]...)
	return append(out, trimmed...)
}

func mandatoryMarkerIndex(src []byte) int {
	for i := 0; i < len(src); i++ {
		switch src[i] {
		case '"':
			i = printVQuotedEnd(src, i) - 1
		case '@':
			if i+1 < len(src) && src[i+1] == '"' {
				if end := quotedFormEnd(src, i); end > i+2 && end <= len(src) {
					i = end - 1
				}
			}
		case '\\':
			if i+1 < len(src) && src[i+1] == '@' {
				if _, _, end, ok := formConditional(src, i); ok {
					i = end + 1
				}
			}
		case '{', '%':
			if end := formIslandEnd(src, i, src[i]); end >= 0 {
				i = end
			}
		case ';':
			if bytes.HasPrefix(src[i:], []byte(";!;")) {
				return i
			}
		}
	}
	return -1
}

func graphicsInvalidTailFingerprint(tail []byte, comment int, allowFullwidth bool) []byte {
	code := bytes.TrimRight(tail[:comment], " \t")
	if allowFullwidth {
		code = trimGrammarSpaceRight(code, true)
	}
	code = commandTail(code)
	out := []byte{'R'}
	out = append(out, code...)
	out = append(out, ' ')
	return append(out, commentFingerprint(tail[comment:])...)
}

func graphicsDeferredShape(command string, parts [][]byte, dialect format.Dialect) bool {
	if len(parts) == 0 || len(trimCorpusSpace(parts[0])) == 0 {
		return false
	}
	switch command {
	case "PRINT_SPACE":
		return len(parts) == 1 && graphicsDeferredNumericField(parts[0], dialect)
	case "PRINT_RECT":
		if len(parts) != 1 && len(parts) != 4 {
			return false
		}
		for _, part := range parts {
			if !graphicsDeferredNumericField(part, dialect) {
				return false
			}
		}
		return true
	case "PRINT_IMG":
		if dialect != format.EMEE || len(parts) < 1 || len(parts) > 6 || !graphicsKnownStringFieldMode(parts[0], dialect) {
			return false
		}
		names, numeric := 1, false
		for _, part := range parts[1:] {
			switch {
			case graphicsStringFieldMode(part, dialect):
				if numeric || names == 3 {
					return false
				}
				names++
			case graphicsNumericFieldMode(part, true, dialect):
				numeric = true
			case graphicsDeferredNumericField(part, dialect):
			default:
				return false
			}
		}
		return true
	default:
		return false
	}
}

func graphicsCommandGapTrusted(tail []byte, allowFullwidth bool) bool {
	if allowFullwidth || len(tail) == 0 || mandatoryMarkerSpan(tail, 0) > 0 {
		return true
	}
	return tail[0] == ' ' || tail[0] == '\t'
}

func graphicsDeferredNumericField(src []byte, dialect format.Dialect) bool {
	if graphicsNumericFieldMode(src, dialect == format.EMEE, dialect) {
		return true
	}
	trimmed := trimCorpusSpace(src)
	if len(trimmed) == 0 || corpusDirectBuiltinValueType(trimmed, dialect) == corpusValueString || corpusDirectBuiltinValueType(trimmed, dialect) == corpusValueInvalid {
		return false
	}
	if corpusQuotedStringExpression(trimmed) {
		return false
	}
	return graphicsNumericUnknownMode(trimmed, dialect)
}

func graphicsNumericFieldsMode(parts [][]byte, allowUnit bool, dialect format.Dialect) bool {
	for _, part := range parts {
		if !graphicsNumericFieldMode(part, allowUnit, dialect) {
			return false
		}
	}
	return true
}

func graphicsStringFieldMode(src []byte, dialect format.Dialect) bool {
	src = trimCorpusSpace(src)
	if len(src) == 0 {
		return false
	}
	if corpusQuotedStringExpression(src) {
		return true
	}
	if src[0] == '"' {
		return printVQuotedEnd(src, 0) == len(src)
	}
	if bytes.HasPrefix(src, []byte(`@"`)) {
		end := quotedFormEnd(src, 0)
		return end == len(src)
	}
	if bytes.HasPrefix(src, []byte(`\@`)) {
		_, _, end, ok := formConditional(src, 0)
		return ok && end+2 == len(src)
	}
	return corpusDirectBuiltinValueType(src, dialect) == corpusValueString
}

func graphicsDeferredStringExpression(src []byte, dialect format.Dialect) bool {
	src = trimCorpusSpace(src)
	if len(src) == 0 || corpusExpressionHasAssignmentOperator(src) || corpusStringIndexInvalidForPercent(src, dialect) {
		return false
	}
	if !graphicsNumericExpressionValid(src, true) || graphicsNumericExpressionValid(src, false) && corpusProvenValueType(src, dialect) == corpusValueNumeric {
		return false
	}
	if corpusExpressionHasQuotedFormOperand(src) {
		return true
	}
	if bytes.Contains(src, []byte{'"'}) {
		return graphicsStringRepetitionExpression(src, dialect) || graphicsIndexedStringExpression(src, dialect)
	}
	for i := 0; i < len(src); {
		if !isFingerprintWord(src[i]) {
			i++
			continue
		}
		start := i
		i++
		for i < len(src) && isFingerprintWord(src[i]) {
			i++
		}
		if corpusStringBuiltin(string(src[start:i]), dialect) && bytes.Contains(src, []byte{'*'}) {
			return true
		}
	}
	return false
}

func graphicsStringRepetitionExpression(src []byte, dialect format.Dialect) bool {
	src = corpusStripOuterParens(src)
	parts := corpusTopLevelBinaryParts(src, '*')
	if len(parts) != 2 {
		return false
	}
	left, right := trimCorpusSpace(parts[0]), trimCorpusSpace(parts[1])
	if corpusQuotedStringExpression(left) || corpusStringBuiltin(string(left), dialect) {
		return corpusProvenValueType(right, dialect) == corpusValueNumeric
	}
	return false
}

func graphicsIndexedStringExpression(src []byte, dialect format.Dialect) bool {
	colon := bytes.IndexByte(src, ':')
	if colon <= 0 || !corpusNameKnown(corpusNumericBuiltinVariableSet, string(trimCorpusSpace(src[:colon]))) {
		return false
	}
	index := corpusStripOuterParens(src[colon+1:])
	return corpusQuotedStringExpression(index) && graphicsNumericExpressionValid(src, true)
}

func corpusTopLevelBinaryParts(src []byte, operator byte) [][]byte {
	parts := make([][]byte, 0, 2)
	start := 0
	depth := 0
	for i := 0; i < len(src); i++ {
		switch src[i] {
		case '"', '\'':
			i = printVQuotedEnd(src, i) - 1
		case '@':
			if i+1 < len(src) && src[i+1] == '"' {
				if end := quotedFormEnd(src, i); end > i+2 && end <= len(src) {
					i = end - 1
				}
			}
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 && src[i] == operator {
				parts = append(parts, src[start:i])
				start = i + 1
			}
		}
	}
	if len(parts) == 0 {
		return nil
	}
	return append(parts, src[start:])
}

func corpusExpressionHasQuotedFormOperand(src []byte) bool {
	for i := 0; i+1 < len(src); i++ {
		if (src[i] == '@' && src[i+1] == '"') || (src[i] == '\\' && src[i+1] == '@') {
			return true
		}
	}
	return false
}

func graphicsKnownStringFieldMode(src []byte, dialect format.Dialect) bool {
	src = trimCorpusSpace(src)
	if len(src) == 0 || bytes.Contains(src, []byte(`[[`)) {
		return false
	}
	if value := corpusDirectBuiltinValueType(src, dialect); value == corpusValueNumeric && dialect != format.EMEE || value == corpusValueInvalid {
		return false
	}
	return len(tokenFingerprintCore(src)) != 0
}

func graphicsNumericExpressionValid(src []byte, allowString bool) bool {
	src = trimCorpusSpace(src)
	type group struct {
		open      byte
		call      bool
		index     bool
		questions int
	}
	groups := make([]group, 0, 2)
	wantOperand := true
	indexOperand := false
	numericReference := false
	questions := 0
	insideCall := func() bool {
		for _, current := range groups {
			if current.call || current.index {
				return true
			}
		}
		return false
	}
	for i := 0; i < len(src); {
		if end := mandatoryMarkerSpan(src, i); end > i {
			i = end
			continue
		}
		if corpusHalfSpace(src[i]) {
			for i < len(src) && corpusHalfSpace(src[i]) {
				i++
			}
			continue
		}
		if src[i] == '"' || src[i] == '\'' {
			if !insideCall() && !allowString && !indexOperand || !wantOperand {
				return false
			}
			end := printVQuotedEnd(src, i)
			if end <= i || end > len(src) || src[end-1] != src[i] {
				return false
			}
			numericReference = indexOperand
			i, wantOperand, indexOperand = end, false, false
			continue
		}
		if src[i] == '@' && i+1 < len(src) && src[i+1] == '"' {
			if !insideCall() && !allowString && !indexOperand || !wantOperand {
				return false
			}
			end := quotedFormEnd(src, i)
			if end <= i+2 || end > len(src) || src[end-1] != '"' {
				return false
			}
			numericReference = indexOperand
			i, wantOperand, indexOperand = end, false, false
			continue
		}
		if src[i] == '\\' && i+1 < len(src) && src[i+1] == '@' {
			if !insideCall() && !allowString && !indexOperand || !wantOperand {
				return false
			}
			_, _, end, ok := formConditional(src, i)
			if !ok || end+2 > len(src) {
				return false
			}
			numericReference = indexOperand
			i, wantOperand, indexOperand = end+2, false, false
			continue
		}
		if src[i] == '(' || src[i] == '[' {
			groups = append(groups, group{open: src[i], call: src[i] == '(' && !wantOperand, index: indexOperand, questions: questions})
			questions = 0
			numericReference = false
			indexOperand = false
			wantOperand = true
			i++
			continue
		}
		if src[i] == ')' || src[i] == ']' {
			if len(groups) == 0 || groups[len(groups)-1].open != map[byte]byte{')': '(', ']': '['}[src[i]] {
				return false
			}
			if questions != 0 || wantOperand && !groups[len(groups)-1].call {
				return false
			}
			questions = groups[len(groups)-1].questions
			numericReference = groups[len(groups)-1].index
			groups = groups[:len(groups)-1]
			indexOperand = false
			wantOperand = false
			i++
			continue
		}
		if src[i] == ',' {
			if questions != 0 || len(groups) == 0 || !groups[len(groups)-1].call {
				return false
			}
			wantOperand = true
			i++
			continue
		}
		if src[i] == '@' {
			if wantOperand || !graphicsLocalQualifier(src, i) {
				return false
			}
			wantOperand = true
			i++
			continue
		}
		if strings.ContainsRune("+-*/%=&|!<>^~", rune(src[i])) {
			if src[i] == '=' && (i+1 >= len(src) || src[i+1] != '=') {
				return false
			}
			if i+1 < len(src) && src[i+1] == '=' && src[i] != '!' && src[i] != '<' && src[i] != '>' && src[i] != '=' {
				return false
			}
			if wantOperand && i+1 < len(src) && (src[i] == '+' || src[i] == '-') && src[i+1] == src[i] {
				if i+2 >= len(src) || !isLHSIdentifierStart(src[i+2]) {
					return false
				}
				i += 2
				continue
			}
			if src[i] == '!' && i+1 < len(src) && src[i+1] == '=' {
				if wantOperand {
					return false
				}
				wantOperand = true
				i += 2
				continue
			}
			if !wantOperand && i+1 < len(src) && (src[i] == '!' && (src[i+1] == '&' || src[i+1] == '|') || src[i] == '^' && src[i+1] == '^') {
				wantOperand = true
				i += 2
				continue
			}
			if (src[i] == '+' || src[i] == '-') && i+1 < len(src) && src[i+1] == src[i] {
				if wantOperand {
					return false
				}
				wantOperand = false
				i += 2
				continue
			}
			if wantOperand {
				if !strings.ContainsRune("+-!~", rune(src[i])) {
					return false
				}
			} else if src[i] == '!' || src[i] == '~' {
				return false
			} else {
				wantOperand = true
			}
			i++
			if i < len(src) && (strings.ContainsRune("=|&+-", rune(src[i])) || src[i] == src[i-1] && (src[i] == '<' || src[i] == '>')) {
				i++
			}
			continue
		}
		if src[i] == ':' || src[i] == '?' || src[i] == '#' {
			if wantOperand {
				return false
			}
			indexOperand = src[i] == ':' && numericReference
			if src[i] == '?' {
				questions++
			} else if src[i] == '#' {
				if questions == 0 {
					return false
				}
				questions--
			}
			wantOperand = true
			i++
			continue
		}
		r, size := utf8.DecodeRune(src[i:])
		if size == 1 && r == utf8.RuneError && src[i] >= 0x80 {
			return false
		}
		if r >= utf8.RuneSelf {
			numericReference = indexOperand
			indexOperand = false
			if size == 1 {
				return false
			}
			if wantOperand {
				wantOperand = false
			}
			i += size
			for i < len(src) {
				next, n := utf8.DecodeRune(src[i:])
				if n == 1 && next == utf8.RuneError && src[i] >= 0x80 || next == ' ' || next == '\t' || strings.ContainsRune("()[],+-*/%=&|!<>^~:?#\"'@\\;", next) {
					break
				}
				i += n
			}
			continue
		}
		if r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r == '_' {
			wasIndex := indexOperand
			start := i
			indexOperand = false
			if !wantOperand {
				return false
			}
			i += size
			for i < len(src) {
				next, n := utf8.DecodeRune(src[i:])
				if n == 1 && next == utf8.RuneError && src[i] >= 0x80 || next == ' ' || next == '\t' || strings.ContainsRune("()[],+-*/%=&|!<>^~:?#\"'@\\;", next) {
					break
				}
				i += n
			}
			name := string(src[start:i])
			numericReference = wasIndex || corpusNameKnown(corpusNumericBuiltinVariableSet, name) || len(name) == 1 && name[0] >= 'A' && name[0] <= 'Z'
			wantOperand = false
			continue
		}
		return false
	}
	return len(groups) == 0 && questions == 0 && !wantOperand
}

func graphicsLocalQualifier(src []byte, at int) bool {
	start := at
	for start > 0 {
		b := src[start-1]
		if b != '_' && (b < 'A' || b > 'Z') && (b < 'a' || b > 'z') {
			break
		}
		start--
	}
	name := src[start:at]
	if !bytes.EqualFold(name, []byte("ARG")) && !bytes.EqualFold(name, []byte("ARGS")) && !bytes.EqualFold(name, []byte("LOCAL")) && !bytes.EqualFold(name, []byte("LOCALS")) {
		return false
	}
	return at+1 < len(src) && !corpusHalfSpace(src[at+1]) && !strings.ContainsRune(".+-*/%=!<>|&^~?#)}],:({[$\\'\"@;", rune(src[at+1]))
}

func graphicsNumericField(src []byte, allowUnit bool) bool {
	return graphicsNumericFieldMode(src, allowUnit, format.CommonDialect)
}

func graphicsNumericFieldMode(src []byte, allowUnit bool, dialect format.Dialect) bool {
	src = trimCorpusSpace(src)
	if len(src) == 0 || bytes.Contains(src, []byte(`[[`)) {
		return false
	}
	if bytes.IndexByte(src, 0) >= 0 {
		if !corpusNulBoundaryExpressionSafe(src, dialect) {
			return false
		}
		prefix := trimCorpusSpace(src[:bytes.IndexByte(src, 0)])
		return graphicsNumericFieldMode(prefix, allowUnit, dialect) && len(tokenFingerprintCore(src)) != 0
	}
	if len(src) >= 2 && strings.EqualFold(string(src[len(src)-2:]), "px") {
		unitStart := len(src) - 2
		glued := unitStart == 0 || !corpusHalfSpace(src[unitStart-1])
		if glued {
			if !allowUnit {
				return false
			}
			if !graphicsGluedUnitBoundary(src[:unitStart]) {
				return false
			}
			src = bytes.TrimRight(src[:unitStart], " \t")
		} else {
			if !allowUnit {
				return false
			}
			src = bytes.TrimRight(src[:unitStart], " \t")
		}
	}
	if value := corpusDirectBuiltinValueType(src, dialect); value == corpusValueString && dialect != format.EMEE || value == corpusValueInvalid {
		return false
	}
	return len(src) != 0 && graphicsNumericExpressionValid(src, dialect == format.EMEE) && len(tokenFingerprintCore(src)) != 0
}

func graphicsNumericUnknown(src []byte) bool {
	return graphicsNumericUnknownMode(src, format.CommonDialect)
}

func graphicsNumericUnknownMode(src []byte, dialect format.Dialect) bool {
	src = trimCorpusSpace(src)
	if bytes.IndexByte(src, 0) >= 0 {
		return true
	}
	if len(src) >= 2 && strings.EqualFold(string(src[len(src)-2:]), "px") {
		unitStart := len(src) - 2
		if unitStart == 0 || corpusHalfSpace(src[unitStart-1]) {
			src = bytes.TrimRight(src[:unitStart], " \t")
		} else if graphicsGluedUnitBoundary(src[:unitStart]) {
			src = src[:unitStart]
		}
	}
	if value := corpusDirectBuiltinValueType(src, dialect); value == corpusValueNumeric {
		return false
	}
	for i := 0; i < len(src); i++ {
		b := src[i]
		if b >= 0x80 {
			return true
		}
		if (b < 'A' || b > 'Z') && (b < 'a' || b > 'z') {
			continue
		}
		if (b == 'x' || b == 'X') && i > 0 && src[i-1] == '0' {
			j := i + 1
			for j < len(src) && ((src[j] >= '0' && src[j] <= '9') || (src[j] >= 'a' && src[j] <= 'f') || (src[j] >= 'A' && src[j] <= 'F')) {
				j++
			}
			i = j - 1
			continue
		}
		if (b == 'e' || b == 'E' || b == 'p' || b == 'P') && i > 0 && src[i-1] >= '0' && src[i-1] <= '9' && i+1 < len(src) && src[i+1] >= '0' && src[i+1] <= '9' {
			continue
		}
		if isGraphicsNumericIdentifier(src, i) {
			for i+1 < len(src) && isGraphicsIdentifierByte(src[i+1]) {
				i++
			}
			continue
		}
		return true
	}
	return false
}

func isGraphicsNumericIdentifier(src []byte, start int) bool {
	end := start + 1
	for end < len(src) && isGraphicsIdentifierByte(src[end]) {
		end++
	}
	next := end
	for next < len(src) && corpusHalfSpace(src[next]) {
		next++
	}
	if next < len(src) && (src[next] == '(' || src[next] == '@') {
		return false
	}
	name := string(src[start:end])
	if len(name) == 1 && name[0] >= 'A' && name[0] <= 'Z' {
		return true
	}
	_, known := corpusNumericBuiltinVariableSet[name]
	return known
}

func isGraphicsIdentifierByte(b byte) bool {
	return b == '_' || b >= '0' && b <= '9' || b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z' || b >= 0x80
}

const corpusNumericBuiltinVariables = `
ARG LOCAL ASSI ASSIPLAY BOUGHT COUNT DAY DOWN EJAC EXPLV FLAG GLOBAL ITEM ITEMSALES LOSEBASE MASTER MONEY NEXTCOM NOITEM PALAMLV PBAND PLAYER PREVCOM RANDDATA RESULT SELECTCOM TFLAG TARGET TIME UP RAND
ISASSI NO ABL BASE CDOWN CFLAG CUP DOWNBASE EQUIP EX EXP GOTJUEL JUEL MARK MAXBASE NOWEX PALAM RELATION SOURCE STAIN TALENT TCVAR TEQUIP
CDFLAG DA DB DC DD DE DITEMTYPE TA TB ITEMPRICE
GAMEBASE_GAMECODE GAMEBASE_VERSION GAMEBASE_ALLOWVERSION GAMEBASE_DEFAULTCHARA GAMEBASE_NOITEM CHARANUM LASTLOAD_VERSION LASTLOAD_NO LINECOUNT ISTIMEOUT __INT_MAX__ __INT_MIN__ __LINE__
`

const corpusStringBuiltinVariables = `
ARGS LOCALS GLOBALS RESULTS SAVESTR STR TSTR SAVEDATA_TEXT
CALLNAME MASTERNAME NAME NICKNAME CSTR
ABLNAME TALENTNAME EXPNAME MARKNAME PALAMNAME ITEMNAME TRAINNAME BASENAME SOURCENAME EXNAME EQUIPNAME TEQUIPNAME FLAGNAME TFLAGNAME CFLAGNAME TCVARNAME CSTRNAME STAINNAME CDFLAGNAME1 CDFLAGNAME2 STRNAME TSTRNAME SAVESTRNAME GLOBALNAME GLOBALSNAME
GAMEBASE_AUTHER GAMEBASE_AUTHOR GAMEBASE_INFO GAMEBASE_YEAR GAMEBASE_TITLE LASTLOAD_TEXT MONEYLABEL DRAWLINESTR EMUERA_VERSION __FILE__ __FUNCTION__ WINDOW_TITLE
`

const corpusReadOnlyBuiltinVariables = `
RAND ITEMPRICE
ABLNAME TALENTNAME EXPNAME MARKNAME PALAMNAME ITEMNAME TRAINNAME BASENAME SOURCENAME EXNAME EQUIPNAME TEQUIPNAME FLAGNAME TFLAGNAME CFLAGNAME TCVARNAME CSTRNAME STAINNAME CDFLAGNAME1 CDFLAGNAME2 STRNAME TSTRNAME SAVESTRNAME GLOBALNAME GLOBALSNAME
GAMEBASE_AUTHER GAMEBASE_AUTHOR GAMEBASE_INFO GAMEBASE_YEAR GAMEBASE_TITLE GAMEBASE_GAMECODE GAMEBASE_VERSION GAMEBASE_ALLOWVERSION GAMEBASE_DEFAULTCHARA GAMEBASE_NOITEM CHARANUM LASTLOAD_VERSION LASTLOAD_NO LINECOUNT ISTIMEOUT __INT_MAX__ __INT_MIN__ __LINE__
LASTLOAD_TEXT MONEYLABEL DRAWLINESTR EMUERA_VERSION __FILE__ __FUNCTION__
`

const corpusCharacterBuiltinVariables = `
ISASSI NO ABL BASE CDOWN CFLAG CUP DOWNBASE EQUIP EX EXP GOTJUEL JUEL MARK MAXBASE NOWEX PALAM RELATION SOURCE STAIN TALENT TCVAR TEQUIP CDFLAG CALLNAME MASTERNAME NAME NICKNAME CSTR
`

func corpusNameSet(names string) map[string]struct{} {
	set := make(map[string]struct{})
	for _, name := range strings.Fields(names) {
		set[name] = struct{}{}
	}
	return set
}

var (
	corpusNumericBuiltinVariableSet          = corpusNameSet(corpusNumericBuiltinVariables)
	corpusStringBuiltinVariableSet           = corpusNameSet(corpusStringBuiltinVariables)
	corpusReadOnlyBuiltinVariableSet         = corpusNameSet(corpusReadOnlyBuiltinVariables)
	corpusCharacterBuiltinVariableSet        = corpusNameSet(corpusCharacterBuiltinVariables)
	corpusModernStringBuiltinVariableSet     = corpusNameSet("DAYNAME TIMENAME MONEYNAME GAMEBASE_URL GAMEBASE_VERSIONNAME")
	corpusModernReadOnlyBuiltinVariableSet   = corpusNameSet("DAYNAME TIMENAME MONEYNAME GAMEBASE_URL GAMEBASE_VERSIONNAME")
	corpusPercentQuotedStringIndexBuiltinSet = corpusNameSet("CSTR TRAINNAME STR TSTR SAVESTR GLOBALS CDFLAGNAME1 CDFLAGNAME2")
)

func graphicsGluedUnitBoundary(src []byte) bool {
	src = bytes.TrimRight(src, " \t")
	if len(src) == 0 {
		return false
	}
	if src[len(src)-1] == ')' || src[len(src)-1] == ']' {
		return true
	}
	for i := 1; i+1 < len(src); i++ {
		if src[i] != 'e' && src[i] != 'E' && src[i] != 'p' && src[i] != 'P' {
			continue
		}
		if src[i-1] < '0' || src[i-1] > '9' || src[i+1] < '0' || src[i+1] > '9' {
			continue
		}
		j := i + 1
		for j < len(src) && src[j] >= '0' && src[j] <= '9' {
			j++
		}
		if j == len(src) {
			return true
		}
	}
	return false
}

func corpusHalfSpace(b byte) bool { return b == ' ' || b == '\t' }

func timesFingerprint(tail []byte) []byte {
	if mandatoryMarkerIndex(tail) >= 0 {
		return timesMarkerFingerprint(tail)
	}
	comment := incrementCommentStart(tail)
	if comment < 0 || comment > len(tail) {
		comment = len(tail)
	}
	code := tail[:comment]
	parts := splitFingerprintFields(code, topLevelCommas(code))
	trailing := len(parts) > 2 && len(trimCorpusSpace(parts[len(parts)-1])) == 0
	if trailing {
		parts = parts[:len(parts)-1]
	}
	if len(parts) != 2 {
		return append([]byte{'R'}, tail...)
	}
	coefficient := trimCorpusSpace(parts[1])
	if !validTimesCoefficientFingerprint(coefficient) {
		return append([]byte{'R'}, tail...)
	}
	var out bytes.Buffer
	out.Write(expressionFingerprint(parts[0]))
	out.WriteByte(',')
	out.Write(coefficient)
	if trailing {
		out.WriteByte(',')
	}
	if comment < len(tail) {
		out.Write(commentFingerprint(tail[comment:]))
	}
	return out.Bytes()
}

func timesMarkerFingerprint(tail []byte) []byte {
	comment := incrementCommentStart(tail)
	if comment < 0 || comment > len(tail) {
		comment = len(tail)
	}
	code := tail[:comment]
	commas := topLevelCommas(code)
	if len(commas) != 1 {
		return append([]byte{'R'}, tail...)
	}
	comma := commas[0]
	second := code[comma+1:]
	trimmed := trimLeadingGrammarSpace(second)
	if !bytes.HasPrefix(trimmed, []byte(";!;")) {
		second = trimmed
	}
	canonical := make([]byte, 0, len(code))
	canonical = append(canonical, code[:comma+1]...)
	canonical = append(canonical, second...)
	out := append([]byte{'R'}, canonical...)
	if comment < len(tail) {
		out = append(out, commentFingerprint(tail[comment:])...)
	}
	return out
}

func timesOverflowCoefficientSafe(tail []byte, allowFullwidth bool) bool {
	comment := incrementCommentStart(tail)
	if comment < 0 || comment > len(tail) {
		comment = len(tail)
	}
	fields := splitFingerprintFields(tail[:comment], topLevelCommas(tail[:comment]))
	if len(fields) != 2 {
		return false
	}
	coefficient := trimGrammarSpaceBoth(fields[1], allowFullwidth)
	return mandatoryMarkerIndex(coefficient) < 0 && validTimesCoefficientFingerprint(coefficient) && printVNumericLiteralNeedsExact(coefficient)
}

func validTimesCoefficientFingerprint(src []byte) bool {
	if len(src) == 0 {
		return false
	}
	i := 0
	if src[i] == '+' || src[i] == '-' {
		i++
		if i == len(src) {
			return false
		}
	}
	digits := 0
	for i < len(src) && src[i] >= '0' && src[i] <= '9' {
		i++
		digits++
	}
	if i < len(src) && src[i] == '.' {
		i++
		for i < len(src) && src[i] >= '0' && src[i] <= '9' {
			i++
			digits++
		}
	}
	if digits == 0 {
		return false
	}
	if i < len(src) && (src[i] == 'e' || src[i] == 'E') {
		i++
		if i < len(src) && src[i] == '-' {
			i++
		}
		exponent := i
		for i < len(src) && src[i] >= '0' && src[i] <= '9' {
			i++
		}
		if exponent == i {
			return false
		}
	}
	return i == len(src)
}

func variableCommandFingerprint(tail []byte) []byte {
	if len(trimCorpusSpace(tail)) == 0 {
		return []byte{'V'}
	}
	comment := incrementCommentStart(tail)
	if comment < 0 || comment > len(tail) {
		comment = len(tail)
	}
	code := tail[:comment]
	commas := topLevelCommas(code)
	trailing := len(commas) == 1 && len(trimCorpusSpace(code[commas[0]+1:])) == 0
	if len(commas) != 0 && !trailing {
		return append([]byte{'R'}, tail...)
	}
	if trailing {
		if len(trimCorpusSpace(code[:commas[0]])) == 0 {
			return append([]byte{'R'}, tail...)
		}
		code = code[:commas[0]]
	}
	left := trimCorpusSpace(code)
	if len(left) == 0 {
		if comment < len(tail) {
			return noArgFingerprint(tail)
		}
		return []byte{'V'}
	}
	if !assignmentLHS(left) {
		return append([]byte{'R'}, tail...)
	}
	var out bytes.Buffer
	out.Write(expressionFingerprint(code))
	if trailing {
		out.WriteByte(',')
	}
	if comment < len(tail) {
		out.Write(commentFingerprint(tail[comment:]))
	}
	return out.Bytes()
}

func extraCommandFingerprint(command string, tail []byte) []byte {
	comment := incrementCommentStart(tail)
	if comment < 0 || comment > len(tail) {
		comment = len(tail)
	}
	code := tail[:comment]
	commas := topLevelCommas(code)
	parts := splitFingerprintFields(code, commas)
	trailing := len(parts) > 1 && len(trimCorpusSpace(parts[len(parts)-1])) == 0
	if trailing {
		parts = parts[:len(parts)-1]
		commas = commas[:len(commas)-1]
	}
	wantMin, wantMax := 3, 4
	if command == "SWAP" {
		wantMin, wantMax = 2, 2
	}
	if len(parts) < wantMin || len(parts) > wantMax {
		return append([]byte{'R'}, tail...)
	}
	for i, part := range parts {
		field := trimCorpusSpace(part)
		if command == "SWAP" || i >= 2 {
			if !assignmentLHS(field) {
				return append([]byte{'R'}, tail...)
			}
		}
	}
	var out bytes.Buffer
	out.Write(expressionFingerprint(code))
	if comment < len(tail) {
		out.Write(commentFingerprint(tail[comment:]))
	}
	return out.Bytes()
}

func splitFingerprintFields(src []byte, commas []int) [][]byte {
	fields := make([][]byte, 0, len(commas)+1)
	start := 0
	for _, comma := range commas {
		fields = append(fields, src[start:comma])
		start = comma + 1
	}
	return append(fields, src[start:])
}

func inputExpressionFingerprint(command string, tail []byte, dialect format.Dialect) []byte {
	max := inputExpressionFieldLimit(command, dialect)
	commas := topLevelCommas(tail)
	if len(commas)+1 <= max {
		return expressionFingerprint(tail)
	}
	cut := commas[max-1]
	var out bytes.Buffer
	out.Write(expressionFingerprint(tail[:cut]))
	out.Write(tail[cut:])
	return out.Bytes()
}

func inputExpressionFieldLimit(command string, dialect format.Dialect) int {
	max := 3
	if strings.HasPrefix(command, "T") {
		max = 6
	}
	if dialect == format.Emuera1824 {
		if strings.HasPrefix(command, "T") {
			max = 4
		} else {
			max = 1
		}
	}
	return max
}

func inputFormSuffixOverflow(code []byte, commas []int, allowFullwidth bool) bool {
	fields := len(commas) + 1
	if len(commas) > 0 && len(trimGrammarSpaceBoth(code[commas[len(commas)-1]+1:], allowFullwidth)) == 0 {
		fields--
	}
	return fields > 2
}

func normalizeFullwidthInputTail(command string, tail []byte, dialect format.Dialect) []byte {
	first := inputFormComma(tail)
	if dialect == format.CommonDialect {
		if first >= 0 {
			return tail
		}
		return normalizeFullwidthFormPayloadDialect(tail, dialect)
	}
	if dialect != format.EMEE {
		return normalizeFullwidthFormPayloadDialect(tail, dialect)
	}
	if first < 0 {
		return normalizeFullwidthFormPayloadDialect(tail, dialect)
	}
	suffix := tail[first+1:]
	comment := incrementCommentStart(suffix)
	if comment < 0 || comment > len(suffix) {
		comment = len(suffix)
	}
	code := suffix[:comment]
	commas := topLevelCommas(code)
	if inputFormSuffixOverflow(code, commas, true) {
		cut := first + 1 + commas[1]
		prefix := normalizeFullwidthInputTail(command, tail[:cut], dialect)
		return append(prefix, tail[cut:]...)
	}
	out := make([]byte, 0, len(tail))
	out = append(out, normalizeFullwidthFormPayloadDialect(tail[:first], dialect)...)
	out = append(out, ',')
	field := 0
	for _, comma := range append(commas, len(code)) {
		out = append(out, normalizeFullwidthExpressionSpaces(code[field:comma])...)
		field = comma + 1
		if comma < len(code) {
			out = append(out, ',')
		}
	}
	if comment < len(suffix) {
		out = append(out, suffix[comment:]...)
	}
	return out
}

func normalizeFullwidthInputExpressionTail(command string, tail []byte, dialect format.Dialect) []byte {
	max := inputExpressionFieldLimit(command, dialect)
	commas := topLevelCommas(tail)
	if len(commas)+1 > max {
		cut := commas[max-1]
		out := normalizeFullwidthExpressionSpaces(tail[:cut])
		return append(out, tail[cut:]...)
	}
	return normalizeFullwidthExpressionSpaces(tail)
}

func normalizeFullwidthStructuredExpressionTail(command string, tail []byte, min, max int) []byte {
	comment := incrementCommentStart(tail)
	if comment < 0 || comment > len(tail) {
		comment = len(tail)
	}
	code := tail[:comment]
	parts := splitFingerprintFields(code, topLevelCommas(code))
	trailing := len(parts) > 1 && len(trimGrammarSpaceBoth(parts[len(parts)-1], true)) == 0
	if trailing {
		if !corpusStructuredAllowsTrailing(command) {
			return tail
		}
		parts = parts[:len(parts)-1]
	}
	if len(parts) < min || len(parts) > max {
		return tail
	}
	for _, part := range parts {
		if len(trimCorpusSpace(part)) == 0 {
			return tail
		}
	}
	if !corpusCommandFieldTypesValid(command, parts, format.EMEE, true) {
		return tail
	}
	return normalizeFullwidthExpressionSpaces(tail)
}

func normalizeFullwidthStructuredFormTail(tail []byte, min int) []byte {
	return normalizeFullwidthStructuredFormTailDialect(tail, min, format.CommonDialect)
}

func normalizeFullwidthStructuredFormTailDialect(tail []byte, min int, dialect format.Dialect) []byte {
	if len(trimCorpusSpace(tail)) == 0 && min != 0 {
		return tail
	}
	return normalizeFullwidthFormPayloadDialect(tail, dialect)
}

func inputFingerprint(command string, tail []byte, dialect format.Dialect) []byte {
	first := inputFormComma(tail)
	if dialect == format.CommonDialect {
		if first >= 0 {
			return append([]byte{'R'}, tail...)
		}
		return append([]byte{'F'}, formFingerprintDialect(tail, dialect)...)
	}
	if dialect != format.EMEE {
		return append([]byte{'F'}, formFingerprintDialect(tail, dialect)...)
	}
	if first < 0 {
		return append([]byte{'F'}, formFingerprintDialect(tail, dialect)...)
	}
	suffix := tail[first+1:]
	comment := incrementCommentStart(suffix)
	if comment < 0 || comment > len(suffix) {
		comment = len(suffix)
	}
	code := suffix[:comment]
	commas := topLevelCommas(code)
	if inputFormSuffixOverflow(code, commas, false) {
		return append([]byte{'R'}, tail...)
	}
	var out bytes.Buffer
	out.WriteByte('F')
	out.Write(formFingerprintDialect(tail[:first], dialect))
	field := 0
	for _, comma := range append(commas, len(code)) {
		out.WriteByte(',')
		out.Write(expressionFingerprintDialect(code[field:comma], dialect))
		field = comma + 1
	}
	if comment < len(suffix) {
		out.Write(commentFingerprint(suffix[comment:]))
	}
	return out.Bytes()
}

func hasTopLevelComma(src []byte) bool {
	return len(formTopLevelCommas(src)) != 0
}

func topLevelCommas(src []byte) []int {
	return topLevelCommasMode(src, true)
}

func formTopLevelCommas(src []byte) []int {
	return topLevelCommasMode(src, false)
}

func inputFormComma(src []byte) int {
	for i := 0; i < len(src); i++ {
		switch src[i] {
		case 0:
			return -1
		case '\\':
			if i+1 >= len(src) || src[i+1] == 0 {
				return -1
			}
			if src[i+1] == '@' {
				_, _, end, ok := formConditional(src, i)
				if !ok {
					return -1
				}
				i = end + 1
				continue
			}
			i++
		case '*', '+', '=', '/', '$':
			if i+2 < len(src) && src[i+1] == src[i] && src[i+2] == src[i] {
				i += 2
			}
		case '%', '{':
			end := formIslandEnd(src, i, src[i])
			if end < 0 {
				return -1
			}
			i = end
		case ',':
			return i
		}
	}
	return -1
}

func topLevelCommasMode(src []byte, skipQuotes bool) []int {
	commas := make([]int, 0, 2)
	depth := 0
	for i := 0; i < len(src); i++ {
		switch src[i] {
		case '"':
			if skipQuotes {
				i = printVQuotedEnd(src, i) - 1
			}
		case '@':
			if skipQuotes && i+1 < len(src) && src[i+1] == '"' {
				end := quotedFormEnd(src, i)
				if end > i+2 && end <= len(src) {
					i = end - 1
				}
			}
		case '\\':
			if i+1 < len(src) {
				if src[i+1] == '@' {
					if _, _, end, ok := formConditional(src, i); ok {
						i = end + 1
						continue
					}
				}
				i++
			}
		case '%':
			if !skipQuotes {
				if end := formIslandEnd(src, i, src[i]); end >= 0 {
					i = end
				}
			}
		case '{':
			if end := formIslandEnd(src, i, src[i]); end >= 0 {
				i = end
			} else {
				depth++
			}
		case ';':
			if bytes.HasPrefix(src[i:], []byte(";!;")) {
				i += 2
				continue
			}
			return commas
		case '(', '[':
			depth++
		case ')', ']':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				commas = append(commas, i)
			}
		}
	}
	return commas
}

func isFormCommandExtended(command string) bool {
	switch command {
	case "THROW", "PUTFORM", "DRAWLINEFORM", "PRINTPLAINFORM", "DEBUGPRINTFORM", "DEBUGPRINTFORML", "STRLENFORM", "STRLENFORMU", "ENCODETOUNI", "REUSELASTLINE":
		return true
	default:
		return isFormCommand(command)
	}
}

func isMixedExpressionCommand(command string) bool {
	switch command {
	case "SETCOLOR", "SETBGCOLOR", "SETFONT", "FONTSTYLE", "HTML_PRINT", "LOADDATA", "LOADCHARA", "LOADVAR", "ASSERT", "PRINTCPERLINE", "SAVENOS", "SAVEVAR", "TOOLTIP_SETDELAY", "TOOLTIP_SETDURATION", "PRINTBUTTON", "PRINTBUTTONC", "PRINTBUTTONLC", "BAR", "BARL", "TWAIT", "SWAPCHARA", "COPYCHARA", "ADDCHARA", "ADDSPCHARA", "DELCHARA", "ADDCOPYCHARA", "PICKUPCHARA", "SAVEDATA", "SAVECHARA", "ARRAYREMOVE", "ARRAYSHIFT", "ARRAYCOPY", "ARRAYSORT", "SORTCHARA", "HTML_TAGSPLIT":
		return true
	case "PRINTSN", "PRINTFORMSN":
		return true
	default:
		return false
	}
}

func isArrayCommand(command string) bool {
	switch command {
	case "ARRAYCOPY", "ARRAYREMOVE", "ARRAYSHIFT", "ARRAYSORT":
		return true
	default:
		return false
	}
}

func isKeywordCommand(command string) bool {
	return command == "SORTCHARA"
}

func isKnownStatementCommandDialect(command string, dialect format.Dialect) bool {
	if corpusDialectRestrictedCommand(command) && dialect != format.EMEE {
		return false
	}
	_, _, _, structured := corpusStructuredCommand(command, dialect)
	if isExpressionCommand(command) || isExpressionLikeCommand(command) || isNoArgCommandDialect(command, dialect) || isFormCallCommand(command) || isFormCommandExtended(command) || isPrintCommand(command) || isDataCommand(command) || isInputCommand(command, dialect) || isInputExpressionCommand(command, dialect) || isExtraVariableCommand(command) || command == "TIMES" || command == "SPLIT" || command == "SWAP" || isGraphicsCommand(command) || isMixedExpressionCommand(command) || isArrayCommand(command) || isKeywordCommand(command) || command == "VARSIZE" || command == "DT_COLUMN_OPTIONS" && dialect == format.EMEE || structured {
		return true
	}
	switch command {
	case "CUSTOMDRAWLINE", "DEBUGPRINTFORM", "DEBUGPRINTFORML", "DEBUGPRINTL", "DRAWLINEFORM", "PRINTC", "PRINTCD", "PRINTCK", "PRINTCPERLINE", "PRINTFORMN", "PRINTFORMSN", "PRINTLC", "PRINTLCD", "PRINTLCK", "PRINTN", "PRINTPLAINFORM", "PRINTVN", "REUSELASTLINE", "TIMES", "VARI", "VARS":
		return true
	default:
		return corpusBuiltinMethod(command, dialect)
	}
}

func isNoArgCommandDialect(command string, dialect format.Dialect) bool {
	switch command {
	case "RESETCOLOR", "RESETBGCOLOR", "FONTBOLD", "FONTITALIC", "FONTREGULAR", "SAVEGLOBAL", "LOADGLOBAL", "RESETDATA", "RESETGLOBAL", "DEBUGCLEAR", "DUMPRAND", "INITRAND", "RESTART", "SAVEGAME", "LOADGAME", "ADDVOIDCHARA":
		return true
	case "OUTPUTLOG":
		return dialect != format.EMEE
	case "INPUTANY", "STOPBGM", "STOPSOUND", "UPDATECHECK", "HTML_PRINT_ISLAND_CLEAR", "CLEARBGIMAGE":
		return dialect == format.EMEE
	case "DO", "REND", "ELSE", "CASEELSE", "CATCH", "ENDIF", "ENDSELECT", "NEXT", "WEND", "ENDNOSKIP", "ENDCATCH", "ENDDATA", "ENDLIST", "ENDFUNC", "BREAK", "CONTINUE", "NOSKIP", "DATALIST", "TRYCALLLIST", "TRYJUMPLIST", "TRYGOTOLIST", "PRINT_ITEM", "PRINT_SHOPITEM", "DRAWLINE", "UPCHECK", "ADDDEFCHARA", "QUIT", "CLEARTEXTBOX", "STOPCALLTRAIN", "DELALLCHARA", "GETTIME", "QUIT_AND_RESTART", "FORCE_QUIT", "FORCE_QUIT_AND_RESTART", "WAIT", "FORCEWAIT", "WAITANYKEY":
		return true
	default:
		return false
	}
}

func noArgFingerprint(tail []byte) []byte {
	if len(tail) == 0 || len(bytes.Trim(tail, " \t")) == 0 {
		return []byte{'V'}
	}
	if tail[0] == ';' {
		return append([]byte{'S'}, tail...)
	}
	trimmed := bytes.TrimLeft(tail, " \t")
	if trimmed[0] == ';' {
		out := []byte{'V', 'C', 0}
		return append(out, commentFingerprint(trimmed)...)
	}
	return append([]byte{'V', 'R', 0}, tail...)
}

func isFormCallCommand(command string) bool {
	switch command {
	case "FUNC", "CALLFORM", "CALLFORMF", "JUMPFORM", "GOTOFORM", "TRYCALLFORM", "TRYCALLFORMF", "TRYJUMPFORM", "TRYGOTOFORM", "TRYCCALLFORM", "TRYCJUMPFORM", "TRYCGOTOFORM":
		return true
	default:
		return false
	}
}

func isPlainCallCommand(command string) bool {
	switch command {
	case "CALL", "CALLF", "JUMP", "GOTO", "TRYCALL", "TRYJUMP", "TRYGOTO", "TRYCALLF", "TRYCCALL", "TRYCJUMP", "TRYCGOTO", "CALLSHARP":
		return true
	default:
		return false
	}
}

func formCallFingerprintDialect(tail []byte, dialect format.Dialect) []byte {
	return callFingerprintModeDialect(tail, false, dialect, true)
}

func formCallFingerprintFullwidthDialect(tail []byte, dialect format.Dialect) []byte {
	return callFingerprintModeDialect(tail, true, dialect, true)
}

func plainCallFingerprintDialect(tail []byte, dialect format.Dialect) []byte {
	return callFingerprintModeDialect(tail, false, dialect, false)
}

func corpusFormCallTailNeedsExact(tail []byte, dialect format.Dialect, allowFullwidth bool) bool {
	tail = commandTail(tail)
	start := 0
	for start < len(tail) {
		if tail[start] == ' ' || tail[start] == '\t' {
			start++
			continue
		}
		if allowFullwidth && bytes.HasPrefix(tail[start:], []byte("\u3000")) {
			start += 3
			continue
		}
		break
	}
	if start >= len(tail) {
		return false
	}
	delimiter := formCallDelimiter(tail, start)
	if delimiter < 0 {
		return false
	}
	if tail[delimiter] == '(' || tail[delimiter] == '[' {
		close := byte(')')
		if tail[delimiter] == '[' {
			close = ']'
		}
		end := matchingCallDelimiter(tail, delimiter, tail[delimiter], close)
		return end < 0 || corpusFormCallArgumentsNeedExact(tail[delimiter+1:end], dialect, allowFullwidth)
	}
	if tail[delimiter] == ',' {
		return corpusFormCallArgumentsNeedExact(tail[delimiter+1:], dialect, allowFullwidth)
	}
	return false
}

func callFingerprintModeDialect(tail []byte, allowFullwidth bool, dialect format.Dialect, formTarget bool) []byte {
	tail = commandTail(tail)
	start := 0
	for start < len(tail) && (tail[start] == ' ' || tail[start] == '\t') {
		start++
	}
	if start == len(tail) {
		return tail
	}
	delimiter := formCallDelimiter(tail, start)
	if !formTarget {
		delimiter = plainCallDelimiter(tail, start)
	}
	nameEnd := len(tail)
	if delimiter >= 0 {
		nameEnd = delimiter
	}
	for nameEnd > start && (tail[nameEnd-1] == ' ' || tail[nameEnd-1] == '\t') {
		if escapedCallTargetByte(tail, nameEnd-1) {
			break
		}
		nameEnd--
	}
	var out bytes.Buffer
	out.WriteString("T")
	target := tail[start:nameEnd]
	if formTarget && allowFullwidth {
		target = normalizeFullwidthFormPayloadDialect(target, dialect)
	}
	if formTarget {
		out.Write(formFingerprintDialect(target, dialect))
	} else {
		fmt.Fprintf(&out, "%d:", len(target))
		out.Write(target)
	}
	out.WriteByte(0)
	if delimiter < 0 {
		return append(out.Bytes(), commentFingerprint(nil)...)
	}
	pos := delimiter
	for pos < len(tail) && (tail[pos] == ' ' || tail[pos] == '\t' || allowFullwidth && bytes.HasPrefix(tail[pos:], []byte("\xE3\x80\x80"))) {
		if allowFullwidth && bytes.HasPrefix(tail[pos:], []byte("\xE3\x80\x80")) {
			pos += 3
			continue
		}
		pos++
	}
	for pos < len(tail) && (tail[pos] == '[' || tail[pos] == '(') {
		open := tail[pos]
		close := byte(')')
		if open == '[' {
			close = ']'
		}
		end := matchingCallDelimiter(tail, pos, open, close)
		if end < 0 {
			if mandatoryMarkerIndex(tail[pos:]) >= 0 {
				return append(out.Bytes(), expressionFingerprintDialect(tail[pos:], dialect)...)
			}
			return append(out.Bytes(), tail[pos:]...)
		}
		out.WriteByte(open)
		args := tail[pos+1 : end]
		if corpusFormCallArgumentsNeedExact(args, dialect, allowFullwidth) {
			out.WriteByte('R')
			fmt.Fprintf(&out, "%d:", len(args))
			out.Write(args)
		} else {
			if allowFullwidth {
				args = normalizeFullwidthExpressionSpacesDialect(args, dialect)
			}
			out.Write(expressionFingerprintDialect(args, dialect))
		}
		out.WriteByte(close)
		pos = end + 1
		for pos < len(tail) && (tail[pos] == ' ' || tail[pos] == '\t' || allowFullwidth && bytes.HasPrefix(tail[pos:], []byte("\xE3\x80\x80"))) {
			if allowFullwidth && bytes.HasPrefix(tail[pos:], []byte("\xE3\x80\x80")) {
				pos += 3
				continue
			}
			pos++
		}
		if pos >= len(tail) || (tail[pos] != '[' && tail[pos] != '(') {
			break
		}
	}
	if pos < len(tail) && tail[pos] == ',' {
		out.WriteByte(',')
		args := tail[pos+1:]
		commentArgs := trimGrammarSpace(args, allowFullwidth)
		if len(commentArgs) != 0 && commentArgs[0] == ';' && !bytes.HasPrefix(commentArgs, []byte(";!;")) {
			out.Write(commentFingerprint(commentArgs))
		} else if corpusFormCallArgumentsNeedExact(args, dialect, allowFullwidth) {
			out.WriteByte('R')
			fmt.Fprintf(&out, "%d:", len(args))
			out.Write(args)
		} else {
			if allowFullwidth {
				args = normalizeFullwidthExpressionSpacesDialect(args, dialect)
			}
			out.Write(expressionFingerprintDialect(args, dialect))
		}
	} else if pos < len(tail) && tail[pos] == ';' {
		out.Write(commentFingerprint(tail[pos:]))
	} else if pos < len(tail) {
		out.Write(tail[pos:])
	}
	return out.Bytes()
}

func escapedCallTargetByte(src []byte, index int) bool {
	backslashes := 0
	for i := index - 1; i >= 0 && src[i] == '\\'; i-- {
		backslashes++
	}
	return backslashes%2 != 0
}

func formCallDelimiter(src []byte, start int) int {
	for i := start; i < len(src); i++ {
		switch src[i] {
		case '\\':
			if i+1 < len(src) {
				i++
			}
		case '@':
			if i+1 < len(src) && src[i+1] == '"' {
				end := quotedFormEnd(src, i)
				if end > i+2 && end <= len(src) {
					i = end - 1
				}
			}
		case '%', '{':
			if end := formIslandEnd(src, i, src[i]); end >= 0 {
				i = end
			}
		case '(', '[', ',', ';':
			return i
		}
	}
	return -1
}

func plainCallDelimiter(src []byte, start int) int {
	for i := start; i < len(src); i++ {
		switch src[i] {
		case '\\':
			if i+1 < len(src) {
				i++
			}
		case '(', '[', ',', ';':
			return i
		}
	}
	return -1
}

func matchingCallDelimiter(src []byte, start int, open, close byte) int {
	depth := 0
	for i := start; i < len(src); i++ {
		switch src[i] {
		case '"':
			i = printVQuotedEnd(src, i) - 1
		case '@':
			if i+1 < len(src) && src[i+1] == '"' {
				end := quotedFormEnd(src, i)
				if end > i+2 && end <= len(src) {
					i = end - 1
				}
			}
		case '\\':
			if i+1 < len(src) {
				i++
			}
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return i
			}
		case ';':
			return -1
		}
	}
	return -1
}

func isDataCommand(command string) bool {
	switch command {
	case "PRINTDATA", "PRINTDATAL", "PRINTDATAW", "PRINTDATAK", "PRINTDATAKL", "PRINTDATAKW", "PRINTDATAD", "PRINTDATADL", "PRINTDATADW", "DATA", "ENDDATA", "DATALIST", "ENDLIST", "STRDATA":
		return true
	default:
		return false
	}
}

func isFormCommand(command string) bool {
	switch command {
	case "PRINTFORM", "PRINTFORML", "PRINTFORMW", "PRINTFORMK", "PRINTFORMKL", "PRINTFORMKW", "PRINTFORMD", "PRINTFORMDL", "PRINTFORMDW", "PRINTFORMC", "PRINTFORMLC", "PRINTFORMCK", "PRINTFORMLCK", "PRINTFORMCD", "PRINTFORMLCD", "PRINTSINGLEFORM", "PRINTSINGLEFORMK", "PRINTSINGLEFORMD", "DATAFORM", "RETURNFORM", "CALLFORM", "GOTOFORM", "JUMPFORM", "TRYJUMPFORM", "TRYCALLFORM", "TRYGOTOFORM", "TRYCJUMPFORM", "TRYCCALLFORM", "TRYCGOTOFORM", "CALLFORMF":
		return true
	default:
		return false
	}
}

func isPrintCommand(command string) bool {
	switch command {
	case "PRINT", "PRINTL", "PRINTW", "PRINTV", "PRINTVL", "PRINTVW", "PRINTVK", "PRINTVKL", "PRINTVKW", "PRINTVD", "PRINTVDL", "PRINTVDW", "PRINTK", "PRINTKL", "PRINTKW", "PRINTD", "PRINTDL", "PRINTDW", "DEBUGPRINT", "DEBUGPRINTL", "PRINTSINGLE", "PRINTSINGLEV", "PRINTSINGLEVK", "PRINTSINGLEVD", "PRINTSINGLEK", "PRINTSINGLED", "PRINTPLAIN":
		return true
	default:
		return false
	}
}

func isPrintVCommand(command string) bool {
	switch command {
	case "PRINTV", "PRINTVL", "PRINTVW", "PRINTVK", "PRINTVKL", "PRINTVKW", "PRINTVD", "PRINTVDL", "PRINTVDW", "PRINTSINGLEV", "PRINTSINGLEVK", "PRINTSINGLEVD":
		return true
	default:
		return false
	}
}

func isPrintVCommandDialect(command string, dialect format.Dialect) bool {
	return isPrintVCommand(command) || dialect == format.EMEE && command == "PRINTVN"
}

func isFormCommandDialect(command string, dialect format.Dialect) bool {
	return isFormCommandExtended(command) || dialect == format.EMEE && command == "PRINTFORMN"
}

func isPrintFormsCommandDialect(command string, dialect format.Dialect) bool {
	return isPrintFormsCommand(command) || dialect == format.EMEE && command == "PRINTFORMSN"
}

func printVFingerprintDialect(tail []byte, allowFullwidth bool, dialect format.Dialect) []byte {
	tail = commandTail(tail)
	if len(bytes.Trim(tail, " \t")) == 0 {
		return []byte{'V'}
	}
	if fingerprint, ok := printVAttachedSeparatorFingerprint(tail, allowFullwidth, dialect); ok {
		return fingerprint
	}
	if !printVHasExpression(tail) {
		if fingerprint, ok := printVPlainTailFingerprintDialect(tail, allowFullwidth, dialect); ok {
			return fingerprint
		}
		if comment := incrementCommentStart(tail); comment >= 0 && comment < len(tail) {
			code := bytes.TrimRight(tail[:comment], " \t")
			out := append([]byte(nil), code...)
			out = append(out, ' ')
			return append(out, commentFingerprint(tail[comment:])...)
		}
		if printVTrailingGrammar(tail, allowFullwidth) {
			return trimPrintVTrailingGrammar(tail, allowFullwidth)
		}
		return tail
	}
	var out bytes.Buffer
	for i := 0; i < len(tail); {
		if tail[i] == '\'' {
			end := printVAtomEnd(tail, i)
			fingerprintToken(&out, tail[i:end])
			i = end
			continue
		}
		start := i
		for i < len(tail) {
			switch tail[i] {
			case '\'':
				goto segmentDone
			case '"':
				i = printVQuotedEnd(tail, i)
			default:
				i++
			}
		}
	segmentDone:
		out.Write(tokenFingerprintDialect(tail[start:i], dialect))
	}
	return out.Bytes()
}

func printVAttachedSeparatorFingerprint(tail []byte, allowFullwidth bool, dialect format.Dialect) ([]byte, bool) {
	if len(tail) < 1 || tail[0] != ';' {
		return nil, false
	}
	body := make([]byte, 1, len(tail))
	body[0] = ' '
	body = append(body, tail[1:]...)
	if printVNumericLiteralNeedsExact(body) || corpusExpressionLexicallyIncomplete(body) || printVBoundaryPrefixNeedsExact(body, dialect) || corpusStatementTailNeedsExact("PRINTV", body, dialect, allowFullwidth) {
		return nil, false
	}
	fingerprint := printVFingerprintDialect(body, allowFullwidth, dialect)
	out := []byte{'C', 0, ';'}
	return append(out, fingerprint...), true
}

func printVTrailingGrammar(tail []byte, allowFullwidth bool) bool {
	if comment := incrementCommentStart(tail); comment >= 0 && comment < len(tail) {
		return !corpusProtectedComment(tail[comment:])
	}
	body := trimPrintVTrailingGrammar(tail, allowFullwidth)
	body = trimLeadingGrammarSpace(body)
	if len(body) == 0 {
		return true
	}
	if printVHasKnownAdjacentOperands(body, allowFullwidth) {
		return false
	}
	for i := 0; i < len(body); {
		if body[i] == ' ' || body[i] == '\t' {
			i++
			continue
		}
		if allowFullwidth && bytes.HasPrefix(body[i:], []byte("\u3000")) {
			i += 3
			continue
		}
		r, size := utf8.DecodeRune(body[i:])
		if size == 1 && r == utf8.RuneError && body[i] >= 0x80 {
			return false
		}
		if r == 0 || r >= utf8.RuneSelf || isFingerprintWord(body[i]) || r == '.' {
			i += size
			continue
		}
		return false
	}
	return true
}

func printVHasKnownAdjacentOperands(body []byte, allowFullwidth bool) bool {
	known := 0
	for i := 0; i < len(body); {
		for i < len(body) && (body[i] == ' ' || body[i] == '\t' || allowFullwidth && bytes.HasPrefix(body[i:], []byte("\u3000"))) {
			if allowFullwidth && bytes.HasPrefix(body[i:], []byte("\u3000")) {
				i += 3
			} else {
				i++
			}
		}
		start := i
		for i < len(body) && body[i] != ' ' && body[i] != '\t' && !(allowFullwidth && bytes.HasPrefix(body[i:], []byte("\u3000"))) {
			i++
		}
		if start == i {
			continue
		}
		token := body[start:i]
		if bytes.IndexByte(token, 0) >= 0 {
			continue
		}
		for _, dialect := range []format.Dialect{format.CommonDialect, format.EMEE} {
			value := corpusDirectBuiltinValueType(token, dialect)
			if value == corpusValueNumeric || value == corpusValueString {
				known++
				break
			}
		}
		if known >= 2 {
			return true
		}
	}
	return false
}

func trimPrintVTrailingGrammar(tail []byte, allowFullwidth bool) []byte {
	if allowFullwidth {
		return trimGrammarSpaceRight(tail, true)
	}
	return bytes.TrimRight(tail, " \t")
}

func normalizeFullwidthPrintVPlainTail(tail []byte) []byte {
	for offset := 0; offset < len(tail); {
		rel := bytes.IndexByte(tail[offset:], 0)
		if rel < 0 {
			break
		}
		nul := offset + rel
		if !corpusNulEmbeddedInIdentifier(tail, nul) {
			prefix := normalizeFullwidthPrintVPlainTail(tail[:nul])
			return append(prefix, tail[nul:]...)
		}
		offset = nul + 1
	}
	if comment := incrementCommentStart(tail); comment >= 0 && comment < len(tail) {
		code := normalizeFullwidthPrintVPlainTail(tail[:comment])
		return append(code, tail[comment:]...)
	}
	if printVHasExpression(tail) || !bytes.Contains(tail, []byte("\u3000")) || !printVTrailingGrammar(tail, true) {
		return tail
	}
	out := make([]byte, 0, len(tail))
	for i := 0; i < len(tail); {
		if bytes.HasPrefix(tail[i:], []byte("\u3000")) {
			out = append(out, ' ')
			i += 3
			continue
		}
		out = append(out, tail[i])
		i++
	}
	return out
}

func normalizeFullwidthPrintVTail(tail []byte) []byte {
	if !printVHasExpression(tail) {
		return tail
	}
	var out bytes.Buffer
	start := 0
	for i := 0; i < len(tail); {
		if tail[i] == '\'' {
			out.Write(normalizeFullwidthExpressionSpaces(tail[start:i]))
			end := printVAtomEnd(tail, i)
			out.Write(tail[i:end])
			i = end
			start = i
			continue
		}
		if tail[i] == '"' {
			i = printVQuotedEnd(tail, i)
			continue
		}
		i++
	}
	out.Write(normalizeFullwidthExpressionSpaces(tail[start:]))
	return out.Bytes()
}

func printVQuotedEnd(src []byte, start int) int {
	end, _ := printVQuotedEndComplete(src, start)
	return end
}

func printVQuotedEndComplete(src []byte, start int) (int, bool) {
	for i := start + 1; i < len(src); i++ {
		if src[i] == '\\' {
			if i+1 < len(src) {
				i++
				continue
			}
			return len(src), false
		}
		if src[i] == '"' {
			return i + 1, true
		}
	}
	return len(src), false
}

func printVAtomEnd(src []byte, start int) int {
	for i := start + 1; i < len(src); i++ {
		if src[i] == '\\' {
			i++
			continue
		}
		if src[i] == 0 {
			return len(src)
		}
		if src[i] == ',' {
			return i
		}
	}
	return len(src)
}

func fingerprintToken(out *bytes.Buffer, token []byte) {
	fmt.Fprintf(out, "%d:", len(token))
	out.Write(token)
}

func printVHasExpression(src []byte) bool {
	quoted := byte(0)
	for i := 0; i < len(src); i++ {
		if quoted != 0 {
			if src[i] == '\\' {
				i++
			} else if src[i] == quoted {
				quoted = 0
			}
			continue
		}
		if src[i] == '"' || src[i] == '\'' {
			quoted = src[i]
			return true
		}
		if src[i] == '[' && i+1 < len(src) && src[i+1] == '[' {
			return true
		}
		switch src[i] {
		case ',', '(', ')', '[', ']', '{', '}', '+', '-', '*', '/', '%', '=', '!', '<', '>', '|', '&', '^', '~', '?', '#', ':':
			return true
		}
	}
	return false
}

func printVPlainTailFingerprint(src []byte, allowFullwidth bool) ([]byte, bool) {
	return printVPlainTailFingerprintDialect(src, allowFullwidth, format.CommonDialect)
}

func printVPlainTailFingerprintDialect(src []byte, allowFullwidth bool, dialect format.Dialect) ([]byte, bool) {
	if mandatoryMarkerIndex(src) >= 0 {
		return nil, false
	}
	if allowFullwidth {
		src = normalizeFullwidthPrintVPlainTail(src)
	}
	comment := incrementCommentStart(src)
	codeEnd := len(src)
	if comment >= 0 && comment < len(src) {
		codeEnd = comment
	}
	if bytes.Contains(src[:codeEnd], []byte("\u3000")) {
		return nil, false
	}
	code := src[:codeEnd]
	leadingDotIdentifier := printVLeadingDotNumericIdentifier(code)
	embeddedNULIdentifier := printVNulEmbeddedIdentifierBody(code)
	if (bytes.IndexByte(code, 0) < 0 && !corpusIdentifierTokenHasControl(code) || leadingDotIdentifier || embeddedNULIdentifier) && !printVNumericLiteralNeedsExact(code) && !corpusExpressionNeedsExact(code, dialect, false, true, false) {
		return tokenFingerprintDialect(src, dialect), true
	}
	var out bytes.Buffer
	lastTokenStart, lastTokenEnd, lastTokenKind, pendingGapStart := -1, -1, 0, -1
	for i := 0; i < codeEnd; {
		if src[i] == 0 {
			out.Write(src[i:])
			return out.Bytes(), true
		}
		if src[i] == ' ' || src[i] == '\t' {
			pendingGapStart = i
			i++
			for i < codeEnd && (src[i] == ' ' || src[i] == '\t') {
				i++
			}
			continue
		}
		if src[i] >= '0' && src[i] <= '9' {
			end, ok := printVRuntimeNumberEnd(src[:codeEnd], i)
			if !ok {
				return nil, false
			}
			fingerprintToken(&out, src[i:end])
			lastTokenStart, lastTokenEnd, lastTokenKind, pendingGapStart = i, end, 1, -1
			i = end
			continue
		}
		if (src[i] > 0 && src[i] < 0x20 || src[i] == 0x7f) && isLHSIdentifierStart(src[i]) {
			if i+1 < codeEnd && src[i+1] == '.' && !printVLeadingDotNumericIdentifier(src[i:codeEnd]) {
				suffixStart := i
				fingerprintToken(&out, trimPrintVTrailingGrammar(src[suffixStart:codeEnd], allowFullwidth))
				i = codeEnd
				continue
			}
		}
		if !isLHSIdentifierStart(src[i]) {
			suffixStart := i
			if pendingGapStart >= 0 && pendingGapStart == lastTokenEnd {
				if lastTokenKind != 2 || !printVIdentifierTokenDeferred(src[lastTokenStart:lastTokenEnd], dialect) {
					suffixStart = pendingGapStart
				}
			}
			fingerprintToken(&out, trimPrintVTrailingGrammar(src[suffixStart:codeEnd], allowFullwidth))
			i = codeEnd
			continue
		}
		start := i
		for i < codeEnd && isLHSIdentifierByte(src[i]) {
			i++
		}
		fingerprintToken(&out, src[start:i])
		lastTokenStart, lastTokenEnd, lastTokenKind, pendingGapStart = start, i, 2, -1
	}
	if comment >= 0 && comment < len(src) {
		out.WriteByte('C')
		out.WriteByte(0)
		out.Write(commentFingerprint(src[comment:]))
	}
	return out.Bytes(), true
}

func printVNulEmbeddedIdentifierBody(src []byte) bool {
	found := false
	for i, b := range src {
		if b != 0 {
			continue
		}
		if !corpusNulEmbeddedInIdentifier(src, i) {
			return false
		}
		found = true
	}
	return found
}

func printVLeadingDotNumericIdentifier(src []byte) bool {
	start := 0
	prefixControl := false
	for start < len(src) && corpusHalfSpace(src[start]) {
		start++
	}
	if start >= len(src) {
		return false
	}
	if src[start] != '.' {
		if !((src[start] > 0 && src[start] < 0x20 || src[start] == 0x7f) && isLHSIdentifierStart(src[start])) {
			return false
		}
		prefixControl = true
		for start < len(src) && isLHSIdentifierByte(src[start]) {
			start++
		}
	}
	if start >= len(src) || src[start] != '.' || start+1 >= len(src) || src[start+1] < '0' || src[start+1] > '9' {
		return false
	}
	end, ok := printVRuntimeNumberEnd(src, start+1)
	if !ok {
		return false
	}
	for end < len(src) && corpusHalfSpace(src[end]) {
		end++
	}
	if end >= len(src) || !isLHSIdentifierStart(src[end]) {
		return false
	}
	control := prefixControl
	for end < len(src) && isLHSIdentifierByte(src[end]) {
		control = control || src[end] == 0 || src[end] > 0 && src[end] < 0x20 || src[end] == 0x7f
		end++
	}
	return control
}

func printVIdentifierTokenDeferred(src []byte, dialect format.Dialect) bool {
	for _, b := range src {
		if (b > 0 && b < 0x20 || b == 0x7f) && isLHSIdentifierStart(b) {
			return true
		}
	}
	return !corpusExpressionIdentifierKnown(src, dialect)
}

func printVHasNumericIdentifierBoundary(src []byte) bool {
	return printVNumericIdentifierBoundary(src, -1)
}

func corpusNumericIdentifierBoundarySafe(src []byte) bool {
	src = trimCorpusSpace(src)
	if len(src) == 0 || mandatoryMarkerIndex(src) >= 0 || bytes.Contains(src, []byte("[[")) || bytes.IndexByte(src, 0) >= 0 || corpusExpressionHasAssignmentOperator(src) || corpusExpressionEndsWithOperator(src) || corpusExpressionLexicallyIncomplete(src) || corpusExpressionHasNestedNamedCall(src) {
		return false
	}
	return printVHasNumericIdentifierBoundary(src) && !printVNumericLiteralNeedsExact(src)
}

func printVNumericIdentifierBoundary(src []byte, joinStart int) bool {
	code := src
	if comment := incrementCommentStart(code); comment >= 0 && comment < len(code) {
		code = code[:comment]
	}
	for i := 0; i < len(code); {
		switch code[i] {
		case '\'', '"':
			i = printVQuotedEnd(code, i)
			continue
		case '@':
			if i+1 < len(code) && code[i+1] == '"' {
				if end := quotedFormEnd(code, i); end > i+2 && end <= len(code) {
					i = end
					continue
				}
			}
		case '\\':
			if i+1 < len(code) && code[i+1] == '@' {
				if _, _, end, ok := formConditional(code, i); ok {
					i = end + 2
					continue
				}
			}
		}
		if code[i] < '0' || code[i] > '9' || i > 0 && isLHSIdentifierByte(code[i-1]) {
			i++
			continue
		}
		end, ok := printVRuntimeNumberEnd(code, i)
		if !ok {
			if end <= i {
				i++
			} else {
				i = end
			}
			continue
		}
		word := end
		for word < len(code) && (code[word] == ' ' || code[word] == '\t') {
			word++
		}
		if word < len(code) && isLHSIdentifierStart(code[word]) && (joinStart < 0 || end <= joinStart && word >= joinStart) {
			return true
		}
		i = end
	}
	return false
}

func printVExpressionProofSource(src []byte) ([]byte, bool) {
	if bytes.IndexByte(src, '\'') < 0 {
		return src, true
	}
	out := make([]byte, 0, len(src))
	depth := 0
	for i := 0; i < len(src); {
		start := i
		switch src[i] {
		case '\'':
			i++
			for i < len(src) && src[i] != ',' && src[i] != 0 {
				if src[i] == '\\' {
					if i+1 == len(src) || src[i+1] == 0 {
						return nil, false
					}
					i++
				}
				i++
			}
			if depth == 0 {
				out = append(out, "RESULT"...)
			} else {
				out = append(out, "UNKNOWN"...)
			}
			if i < len(src) && src[i] == 0 {
				return out, true
			}
			continue
		case '"':
			i = printVQuotedEnd(src, i)
		case '@':
			if i+1 < len(src) && src[i+1] == '"' {
				i = quotedFormEnd(src, i)
			} else {
				i++
			}
		case '\\':
			if i+1 < len(src) && src[i+1] == '@' {
				if _, _, end, ok := formConditional(src, i); ok {
					i = end + 2
				} else {
					i++
				}
			} else {
				i++
			}
		case '(', '[':
			depth++
			i++
		case ')', ']':
			if depth > 0 {
				depth--
			}
			i++
		case ';':
			if !bytes.HasPrefix(src[i:], []byte(";!;")) {
				return append(out, src[i:]...), true
			}
			i += 3
		default:
			i++
		}
		out = append(out, src[start:i]...)
	}
	return out, true
}

func printVBoundaryPrefixNeedsExact(src []byte, dialect format.Dialect) bool {
	for i := 0; i < len(src); {
		switch src[i] {
		case '\'':
			i = printVAtomEnd(src, i)
			continue
		case '"':
			i = printVQuotedEnd(src, i)
			continue
		case '@':
			if i+1 < len(src) && src[i+1] == '"' {
				if end := quotedFormEnd(src, i); end > i+2 && end <= len(src) {
					i = end
					continue
				}
			}
		case '\\':
			if i+1 < len(src) && src[i+1] == '@' {
				if _, _, end, ok := formConditional(src, i); ok {
					i = end + 2
					continue
				}
			}
		case ';':
			return false
		case 0:
			if !corpusNulEmbeddedInIdentifier(src, i) {
				prefix := trimCorpusSpace(src[:i])
				return len(prefix) != 0 && printVExpressionPrefixNeedsExact(prefix, dialect)
			}
		}
		i++
	}
	return false
}

func printVExpressionPrefixNeedsExact(src []byte, dialect format.Dialect) bool {
	deferredField := false
	fields := splitFingerprintFields(src, topLevelCommas(src))
	if len(fields) > 1 && len(trimCorpusSpace(fields[len(fields)-1])) == 0 {
		fields = fields[:len(fields)-1]
	}
	for _, field := range fields {
		field = trimCorpusSpace(field)
		if len(field) == 0 {
			return true
		}
		if deferredField {
			continue
		}
		if corpusExpressionHasDeferredOperand(field, dialect) {
			deferredField = true
			continue
		}
		if corpusExpressionNeedsExact(field, dialect, false, true, false) {
			return true
		}
	}
	return false
}

func printVNumericLiteralNeedsExact(src []byte) bool {
	for i := 0; i < len(src); {
		if src[i] == 0 {
			if !corpusNulEmbeddedInIdentifier(src, i) {
				return false
			}
			i++
			continue
		}
		switch src[i] {
		case '\'':
			i = printVAtomEnd(src, i)
			continue
		case '"':
			i = printVQuotedEnd(src, i)
			continue
		case '@':
			if i+1 < len(src) && src[i+1] == '"' {
				if end := quotedFormEnd(src, i); end > i+2 && end <= len(src) {
					i = end
					continue
				}
			}
		case '\\':
			if i+1 < len(src) && src[i+1] == '@' {
				if _, _, end, ok := formConditional(src, i); ok {
					i = end + 2
					continue
				}
			}
		}
		if src[i] < '0' || src[i] > '9' || i > 0 && isLHSIdentifierByte(src[i-1]) {
			i++
			continue
		}
		end, ok := printVRuntimeNumberEnd(src, i)
		if !ok {
			return true
		}
		i = end
	}
	return false
}

func printVRuntimeNumberEnd(src []byte, start int) (int, bool) {
	i, base := start, 10
	if i+1 < len(src) && src[i] == '0' {
		switch src[i+1] {
		case 'x', 'X':
			base, i = 16, i+2
		case 'b', 'B':
			base, i = 2, i+2
		}
	}
	if base != 10 && i < len(src) && src[i] == '+' {
		i++
	}
	digitStart := i
	for i < len(src) && fingerprintDigit(src[i], base) {
		i++
	}
	if i == digitStart {
		return i, false
	}
	if base == 2 && i < len(src) && src[i] >= '2' && src[i] <= '9' {
		for i < len(src) && src[i] >= '0' && src[i] <= '9' {
			i++
		}
		return i, false
	}
	if r, _ := utf8.DecodeRune(src[i:]); r > 127 && r <= 0xffff && unicode.IsDigit(r) {
		return i, false
	}
	mantissaEnd := i
	exponent := i < len(src) && ((base == 10 || base == 2) && (src[i] == 'e' || src[i] == 'E' || src[i] == 'p' || src[i] == 'P') ||
		base == 16 && (src[i] == 'p' || src[i] == 'P'))
	exponentBase := 0
	var exponentDigits []byte
	if !exponent {
		if !printVNumberLiteralFits(src[digitStart:mantissaEnd], nil, base, 0) {
			return i, false
		}
		return i, true
	}
	if src[i] == 'e' || src[i] == 'E' {
		exponentBase = 10
	} else {
		exponentBase = 2
	}
	i++
	invalid := false
	exponentStart := i
	if i < len(src) && src[i] == '-' && base != 10 {
		invalid = true
	}
	if i < len(src) && (src[i] == '+' || src[i] == '-') {
		i++
	}
	exponentDigitsStart := i
	for i < len(src) && fingerprintDigit(src[i], base) {
		i++
	}
	if base == 2 && i < len(src) && src[i] >= '2' && src[i] <= '9' {
		invalid = true
		for i < len(src) && src[i] >= '0' && src[i] <= '9' {
			i++
		}
	}
	if i == exponentDigitsStart {
		return i, false
	}
	if r, _ := utf8.DecodeRune(src[i:]); r > 127 && r <= 0xffff && unicode.IsDigit(r) {
		return i, false
	}
	exponentDigits = src[exponentStart:i]
	if invalid || !printVNumberLiteralFits(src[digitStart:mantissaEnd], exponentDigits, base, exponentBase) {
		return i, false
	}
	return i, true
}

func printVNumberLiteralFits(mantissa, exponent []byte, base, exponentBase int) bool {
	value, ok := printVBitInteger(mantissa, base)
	if !ok {
		return false
	}
	if exponentBase == 0 {
		return true
	}
	power, ok := printVBitInteger(exponent, base)
	if !ok {
		return false
	}
	if int32(power) == 0 {
		return true
	}
	number := float64(value) * math.Pow(float64(exponentBase), float64(int32(power)))
	return !math.IsNaN(number) && !math.IsInf(number, 0) && number >= float64(math.MinInt64) && number <= float64(math.MaxInt64)
}

func printVBitInteger(src []byte, base int) (int64, bool) {
	if base == 10 {
		value, err := strconv.ParseInt(string(src), base, 64)
		return value, err == nil
	}
	if len(src) > 0 && src[0] == '+' {
		src = src[1:]
	}
	value, err := strconv.ParseUint(string(src), base, 64)
	return int64(value), err == nil
}

func commandTail(tail []byte) []byte {
	if len(tail) == 0 || (tail[0] != ' ' && tail[0] != '\t') || mandatoryMarkerSpan(tail, 0) > 0 {
		return tail
	}
	tail = bytes.TrimLeft(tail, " \t")
	if len(tail) == 0 {
		return []byte{' '}
	}
	return append([]byte{' '}, tail...)
}

func isExpressionLikeCommand(command string) bool {
	switch command {
	case "RETURN", "REDRAW", "CLEARLINE", "CUPCHECK", "CALLTRAIN", "DOTRAIN", "DELDATA", "RESET_STAIN", "FORCEKANA", "SKIPDISP", "SKIPLOG", "PRINT_ABL", "PRINT_TALENT", "PRINT_MARK", "PRINT_EXP", "PRINT_PALAM", "RETURNF", "PRINTS", "PRINTSL", "PRINTSW", "PRINTSK", "PRINTSKL", "PRINTSKW", "PRINTSD", "PRINTSDL", "PRINTSDW", "PRINTSINGLES", "PRINTSINGLEVK", "PRINTSINGLEVD", "PRINTSINGLESK", "PRINTSINGLESD", "PRINTFORMS", "PRINTFORMSL", "PRINTFORMSW", "PRINTFORMSK", "PRINTFORMSKL", "PRINTFORMSKW", "PRINTFORMSD", "PRINTFORMSDL", "PRINTFORMSDW", "PRINTSINGLEFORMS", "PRINTSINGLEFORMSK", "PRINTSINGLEFORMSD", "FOR", "SETBIT", "CLEARBIT", "INVERTBIT", "VARSET", "CVARSET", "POWER", "CALL", "CALLF", "CALLSHARP", "JUMP", "GOTO", "TRYCALL", "TRYJUMP", "TRYGOTO", "TRYCALLF", "TRYCCALL", "TRYCJUMP", "TRYCGOTO", "TRYCALLLIST", "TRYJUMPLIST", "TRYGOTOLIST", "FUNC", "ELSE", "ELSEIF", "ENDIF", "CASEELSE", "ENDSELECT", "REND", "NEXT", "WEND", "ENDLOOP", "CONTINUE", "BREAK", "CATCH", "ENDCATCH", "ENDDATA", "ENDLIST", "ENDFUNC", "ENDNOSKIP":
		return true
	default:
		return false
	}
}

func isExpressionCommand(command string) bool {
	switch command {
	case "IF", "ELSEIF", "SIF", "WHILE", "REPEAT", "LOOP", "SELECTCASE", "CASE":
		return true
	default:
		return false
	}
}

func assignmentLHS(src []byte) bool {
	src = bytes.Trim(src, " \t")
	if len(src) == 0 || !isLHSIdentifierStart(src[0]) {
		return false
	}
	i := 1
	for i < len(src) && isLHSIdentifierByte(src[i]) {
		i++
	}
	for i < len(src) && (src[i] == ' ' || src[i] == '\t') {
		i++
	}
	if i < len(src) && src[i] == '@' {
		i++
		for i < len(src) && (src[i] == ' ' || src[i] == '\t') {
			i++
		}
		if i == len(src) || !isLHSIdentifierStart(src[i]) {
			return false
		}
		i++
		for i < len(src) && isLHSIdentifierByte(src[i]) {
			i++
		}
		for i < len(src) && (src[i] == ' ' || src[i] == '\t') {
			i++
		}
	}
	if i == len(src) {
		return true
	}
	if src[i] != ':' && src[i] != '[' {
		return false
	}
	var delimiters []byte
	for i < len(src) {
		switch src[i] {
		case ' ', '\t':
			i++
		case '"':
			end := printVQuotedEnd(src, i)
			if end <= i || end > len(src) || src[end-1] != '"' {
				return false
			}
			i = end
		case '@':
			refStart := i + 1
			for refStart < len(src) && (src[refStart] == ' ' || src[refStart] == '\t') {
				refStart++
			}
			if refStart >= len(src) {
				return false
			}
			if isLHSIdentifierStart(src[refStart]) {
				i = refStart + 1
				for i < len(src) && isLHSIdentifierByte(src[i]) {
					i++
				}
				continue
			}
			if src[i+1] != '"' {
				return false
			}
			end := quotedFormEnd(src, i)
			if end <= i+2 || end > len(src) || src[end-1] != '"' {
				return false
			}
			i = end
		case '(':
			delimiters = append(delimiters, ')')
			i++
		case '[':
			delimiters = append(delimiters, ']')
			i++
		case ')', ']':
			if len(delimiters) == 0 || delimiters[len(delimiters)-1] != src[i] {
				return false
			}
			delimiters = delimiters[:len(delimiters)-1]
			i++
		case '\\', '\'', ';', '$', '{', '}':
			return false
		default:
			if src[i] < 0x20 {
				return false
			}
			i++
		}
	}
	return len(delimiters) == 0
}

func isLHSIdentifierStart(b byte) bool {
	if b == 0 || b == '\r' || b == '\n' {
		return false
	}
	if b >= '0' && b <= '9' {
		return false
	}
	switch b {
	case ' ', '\t', '+', '-', '*', '/', '%', '=', '!', '<', '>', '|', '&', '^', '~', '?', '#', ')', '}', ']', ',', ':', '(', '{', '[', '$', '\\', '\'', '"', '@', '.', ';':
		return false
	default:
		return true
	}
}

func isLHSIdentifierByte(b byte) bool {
	return b == 0 || isLHSIdentifierStart(b) || b >= '0' && b <= '9'
}

func corpusStatementNulBoundary(command string, tail []byte, dialect format.Dialect, allowFullwidth bool) int {
	if bytes.IndexByte(tail, 0) < 0 || corpusUnavailableCommandForFingerprint(command, dialect) || isRawCommand(command, dialect) {
		return -1
	}
	if isPlainCallCommand(command) || isFormCallCommand(command) {
		delimiter := plainCallDelimiter(tail, 0)
		if isFormCallCommand(command) {
			delimiter = formCallDelimiter(tail, 0)
		}
		targetEnd := len(tail)
		if delimiter >= 0 {
			targetEnd = delimiter
		}
		nul := bytes.IndexByte(tail[:targetEnd], 0)
		if isFormCallCommand(command) {
			nul = corpusFormNulBoundaryMode(tail[:targetEnd], allowFullwidth)
		}
		if nul >= 0 {
			return nul
		}
		if delimiter < 0 || tail[delimiter] == ';' {
			return -1
		}
		if nul = corpusExpressionNulBoundaryMode(tail[delimiter:], false, allowFullwidth); nul >= 0 {
			return delimiter + nul
		}
		return -1
	}
	if isInputCommand(command, dialect) && dialect == format.EMEE {
		if commas := formTopLevelCommas(tail); len(commas) != 0 {
			first := commas[0]
			if nul := corpusFormNulBoundaryMode(tail[:first], allowFullwidth); nul >= 0 {
				return nul
			}
			if nul := corpusExpressionNulBoundaryMode(tail[first+1:], false, allowFullwidth); nul >= 0 {
				return first + 1 + nul
			}
			return -1
		}
	}
	if isFormCommandDialect(command, dialect) || isInputCommand(command, dialect) {
		return corpusFormNulBoundaryMode(tail, allowFullwidth)
	}
	if kind, _, _, ok := corpusStructuredCommand(command, dialect); ok && kind == "form-any" {
		return corpusFormNulBoundaryMode(tail, allowFullwidth)
	}
	if isPrintVCommandDialect(command, dialect) {
		return corpusExpressionNulBoundaryMode(tail, true, allowFullwidth)
	}
	return corpusExpressionNulBoundaryMode(tail, false, allowFullwidth)
}

func corpusExpressionNulBoundary(src []byte) int {
	return corpusExpressionNulBoundaryMode(src, false, false)
}

func corpusExpressionNulBoundaryMode(src []byte, printV, allowFullwidth bool) int {
	if bytes.IndexByte(src, 0) < 0 {
		return -1
	}
	for i := 0; i < len(src); {
		switch src[i] {
		case 0:
			if !corpusNulEmbeddedInIdentifierMode(src, i, allowFullwidth) {
				return i
			}
		case '"', '\'':
			if printV && src[i] == '\'' {
				i = printVAtomEnd(src, i)
			} else {
				i = printVQuotedEnd(src, i)
			}
			continue
		case '@':
			if i+1 < len(src) && src[i+1] == '"' {
				i = quotedFormEnd(src, i)
				continue
			}
		case '\\':
			if i+1 < len(src) && src[i+1] == '@' {
				if _, _, end, ok := formConditional(src, i); ok {
					i = end + 2
					continue
				}
			}
		case ';':
			if bytes.HasPrefix(src[i:], []byte(";!;")) {
				i += 3
				continue
			}
			return -1
		}
		i++
	}
	return -1
}

func corpusFormNulBoundary(src []byte) int {
	return corpusFormNulBoundaryMode(src, false)
}

func corpusFormNulBoundaryMode(src []byte, allowFullwidth bool) int {
	if bytes.IndexByte(src, 0) < 0 {
		return -1
	}
	for i := 0; i < len(src); {
		switch src[i] {
		case 0:
			return i
		case '\\':
			if i+1 < len(src) && src[i+1] == '@' {
				if question, hash, end, ok := formConditional(src, i); ok {
					if nul := corpusExpressionNulBoundaryMode(src[i+2:question], false, allowFullwidth); nul >= 0 {
						return i + 2 + nul
					}
					branchEnd := end
					if hash >= 0 {
						branchEnd = hash
					}
					if nul := corpusFormNulBoundaryMode(src[question+1:branchEnd], allowFullwidth); nul >= 0 {
						return question + 1 + nul
					}
					if hash >= 0 {
						if nul := corpusFormNulBoundaryMode(src[hash+1:end], allowFullwidth); nul >= 0 {
							return hash + 1 + nul
						}
					}
					i = end + 2
					continue
				}
			}
			if i+1 < len(src) && src[i+1] != 0 {
				i += 2
				continue
			}
		case '{', '%':
			if end := formIslandEnd(src, i, src[i]); end >= 0 {
				if nul := corpusExpressionNulBoundaryMode(src[i+1:end], false, allowFullwidth); nul >= 0 {
					return i + 1 + nul
				}
				i = end + 1
				continue
			}
		}
		i++
	}
	return -1
}

func corpusNulEmbeddedInIdentifier(src []byte, index int) bool {
	return corpusNulEmbeddedInIdentifierMode(src, index, false)
}

func corpusNulEmbeddedInIdentifierMode(src []byte, index int, allowFullwidth bool) bool {
	if index <= 0 || !isLHSIdentifierByte(src[index-1]) {
		return false
	}
	start := index - 1
	for start >= 0 {
		if allowFullwidth && start >= 2 && bytes.Equal(src[start-2:start+1], []byte("\u3000")) {
			break
		}
		if src[start] == 0 {
			start--
			continue
		}
		if !isLHSIdentifierByte(src[start]) {
			break
		}
		start--
	}
	tokenStart := start + 1
	if tokenStart >= index {
		return false
	}
	if isLHSIdentifierStart(src[tokenStart]) {
		return true
	}
	if src[tokenStart] < '0' || src[tokenStart] > '9' {
		return false
	}
	end, _ := printVRuntimeNumberEnd(src, tokenStart)
	return end < index
}

func incrementFingerprint(src []byte) ([]byte, bool) {
	comment := incrementCommentStart(src)
	code := src[:comment]
	var suffix []byte
	suffixBoundary := false
	for offset := 0; offset < len(code); {
		rel := bytes.IndexByte(code[offset:], 0)
		if rel < 0 {
			break
		}
		nul := offset + rel
		if incrementBoundaryBeforeNUL(code[:nul]) {
			suffix = append([]byte(nil), src[nul:]...)
			suffixBoundary = true
			code = code[:nul]
			comment = len(src)
			break
		}
		offset = nul + 1
	}
	code = bytes.TrimRight(code, " \t")
	if len(code) < 2 {
		return nil, false
	}
	op := code[len(code)-2:]
	lhs := code[:len(code)-2]
	prefix := false
	if bytes.Equal(code[:2], []byte("++")) || bytes.Equal(code[:2], []byte("--")) {
		op, lhs, prefix = code[:2], code[2:], true
	}
	if !bytes.Equal(op, []byte("++")) && !bytes.Equal(op, []byte("--")) || !assignmentLHS(lhs) {
		return nil, false
	}
	out := []byte{'I', 0}
	if prefix {
		out = append(out, 'P')
	} else {
		out = append(out, 'S')
	}
	out = append(out, 0)
	out = append(out, tokenFingerprint(lhs)...)
	out = append(out, 0)
	out = append(out, op...)
	if suffixBoundary {
		out = append(out, suffix...)
	}
	if comment < len(src) {
		out = append(out, 0)
		out = append(out, commentFingerprint(src[comment:])...)
	}
	return out, true
}

func incrementBoundaryBeforeNUL(code []byte) bool {
	trimmed := bytes.TrimRight(code, " \t")
	if len(trimmed) < 2 {
		return false
	}
	if bytes.HasPrefix(trimmed, []byte("++")) || bytes.HasPrefix(trimmed, []byte("--")) {
		return incrementPrefixOperandBoundary(trimmed[2:], len(trimmed) != len(code))
	}
	if bytes.HasSuffix(trimmed, []byte("++")) || bytes.HasSuffix(trimmed, []byte("--")) {
		return assignmentLHS(trimmed[:len(trimmed)-2])
	}
	return false
}

func incrementPrefixOperandBoundary(operand []byte, separated bool) bool {
	operand = bytes.TrimRight(operand, " \t")
	if !assignmentLHS(operand) || len(operand) == 0 {
		return false
	}
	if separated {
		return true
	}
	if bytes.ContainsAny(operand[len(operand)-1:], ")]") || operand[len(operand)-1] == '"' {
		return true
	}
	if operand[len(operand)-1] < '0' || operand[len(operand)-1] > '9' {
		return false
	}
	return bytes.ContainsAny(operand, ":[")
}

func incrementCommentStart(src []byte) int {
	for i := 0; i < len(src); i++ {
		switch src[i] {
		case '"':
			i = printVQuotedEnd(src, i) - 1
		case '@':
			if i+1 < len(src) && src[i+1] == '"' {
				end := quotedFormEnd(src, i)
				if end > i+2 && end <= len(src) {
					i = end - 1
				}
			}
		case '\\':
			if i+1 < len(src) && src[i+1] == '@' {
				if _, _, end, ok := formConditional(src, i); ok {
					i = end + 1
					continue
				}
			}
		case ';':
			if bytes.HasPrefix(src[i:], []byte(";!;")) {
				i += 2
				continue
			}
			return i
		}
	}
	return len(src)
}

func joinFingerprint(head, tail []byte) []byte {
	out := append([]byte(nil), head...)
	out = append(out, 0)
	return append(out, tail...)
}

func commentFingerprint(src []byte) []byte {
	if len(src) == 0 {
		return nil
	}
	if corpusProtectedComment(src) && !ordinaryInlineDoubleSemicolon(src) {
		return append([]byte(nil), src...)
	}
	rest := bytes.TrimLeft(bytes.TrimRight(src[1:], " \t"), " \t")
	out := []byte{';'}
	if len(rest) != 0 {
		out = append(out, ' ')
	}
	return append(out, rest...)
}

func ordinaryInlineDoubleSemicolon(src []byte) bool {
	if len(src) < 2 || src[0] != ';' {
		return false
	}
	if src[1] == ';' {
		return len(src) == 2 || src[2] != ';'
	}
	i := 1
	for i < len(src) && (src[i] == ' ' || src[i] == '\t') {
		i++
	}
	return i > 1 && i < len(src) && src[i] == ';' && (i+1 == len(src) || src[i+1] != ';')
}

func standaloneCommentFingerprint(src []byte) []byte {
	if len(src) == 0 {
		return nil
	}
	rest := bytes.TrimLeft(bytes.TrimRight(src[1:], " \t"), " \t")
	out := []byte{';'}
	if len(rest) != 0 {
		out = append(out, ' ')
	}
	return append(out, rest...)
}

func corpusProtectedComment(src []byte) bool {
	if len(src) == 0 || src[0] != ';' {
		return false
	}
	text := bytes.TrimLeft(src[1:], " \t")
	if len(text) == 0 || bytes.HasPrefix(text, []byte("[[")) {
		return len(text) != 0
	}
	if text[0] == '[' || text[0] == '@' || text[0] == '#' || text[0] == '!' || text[0] == '^' || text[0] == ';' {
		return true
	}
	if bytes.ContainsRune(text, '|') {
		return true
	}
	for _, r := range string(text) {
		if r >= 0x2500 && r <= 0x257f {
			return true
		}
	}
	isDecorative := func(b byte) bool {
		switch b {
		case '-', '=', '~', '*', '+', '#', '|', '/', '\\':
			return true
		default:
			return false
		}
	}
	if !isDecorative(text[0]) {
		return false
	}
	n := 0
	for n < len(text) && isDecorative(text[n]) {
		n++
	}
	return n >= 3
}

func assignmentFingerprint(src []byte, index, operator int) []byte {
	left := tokenFingerprint(src[:index])
	right := bytes.TrimLeft(src[index+operator:], " \t")
	assignment := src[index : index+operator]
	if !bytes.Equal(assignment, []byte("=")) && !bytes.Equal(assignment, []byte("==")) {
		right = expressionFingerprint(right)
	}
	out := append(left, 0)
	out = append(out, assignment...)
	out = append(out, 0)
	return append(out, right...)
}

func assignmentOperator(src []byte) (int, int) {
	operators := []string{"<<=", ">>=", "+=", "-=", "*=", "/=", "%=", "|=", "&=", "^=", "'=", "==", "="}
	quoted := byte(0)
	depth := 0
	for i := 0; i < len(src); i++ {
		if quoted != 0 {
			if src[i] == '\\' {
				i++
			} else if src[i] == quoted {
				quoted = 0
			}
			continue
		}
		if src[i] == '"' || src[i] == '\'' {
			if src[i] == '\'' && i+1 < len(src) && src[i+1] == '=' {
				return i, 2
			}
			quoted = src[i]
			continue
		}
		if src[i] == '@' && i+1 < len(src) && src[i+1] == '"' {
			if end := quotedFormEnd(src, i); end > i+2 && end <= len(src) {
				i = end - 1
			}
			continue
		}
		if src[i] == '\\' && i+1 < len(src) && src[i+1] == '@' {
			if _, _, end, ok := formConditional(src, i); ok {
				i = end + 1
				continue
			}
		}
		if src[i] == ';' {
			return -1, 0
		}
		if src[i] == '[' && i+1 < len(src) && src[i+1] == '[' {
			if end := bytes.Index(src[i+2:], []byte("]]")); end >= 0 {
				i += end + 3
			}
			continue
		}
		switch src[i] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
		}
		if depth != 0 {
			continue
		}
		for _, operator := range operators {
			if operator == "=" && i > 0 && strings.ContainsRune("<>!", rune(src[i-1])) {
				continue
			}
			if bytes.HasPrefix(src[i:], []byte(operator)) {
				return i, len(operator)
			}
		}
	}
	return -1, 0
}

func expressionFingerprint(src []byte) []byte {
	return expressionFingerprintDialect(src, format.CommonDialect)
}

func expressionFingerprintDialect(src []byte, dialect format.Dialect) []byte {
	if nul := corpusExpressionNulBoundary(src); nul >= 0 {
		return appendPreservedField(appendPreservedField([]byte{'N'}, expressionFingerprintDialect(src[:nul], dialect)), src[nul:])
	}
	if len(src) > 0 && src[0] == ';' && !bytes.HasPrefix(src, []byte(";!;")) {
		return append([]byte{'S'}, src...)
	}
	if comment := incrementCommentStart(src); comment >= 0 && comment < len(src) {
		out := tokenFingerprintDialect(src[:comment], dialect)
		out = append(out, 'C', 0)
		return append(out, commentFingerprint(src[comment:])...)
	}
	return tokenFingerprintDialect(src, dialect)
}

func formFingerprint(src []byte) []byte {
	return formFingerprintDialect(src, format.CommonDialect)
}

func formFingerprintDialect(src []byte, dialect format.Dialect) []byte {
	var out bytes.Buffer
	formFingerprintIntoDialect(&out, src, dialect)
	return out.Bytes()
}

func formFingerprintIntoDialect(out *bytes.Buffer, src []byte, dialect format.Dialect) {
	if nul := corpusFormNulBoundary(src); nul >= 0 {
		formFingerprintIntoDialect(out, src[:nul], dialect)
		out.WriteByte('N')
		fmt.Fprintf(out, "%d:", len(src)-nul)
		out.Write(src[nul:])
		return
	}
	literal := 0
	flush := func(end int) {
		if end <= literal {
			return
		}
		out.WriteByte('L')
		fmt.Fprintf(out, "%d:", end-literal)
		out.Write(src[literal:end])
	}
	for i := 0; i < len(src); {
		if src[i] == '\\' && i+1 < len(src) && src[i+1] == '@' {
			question, hash, end, ok := formConditional(src, i)
			if ok {
				flush(i)
				out.WriteByte('C')
				if formConditionalNeedsExactDialect(src[i:end+2], dialect) {
					condition := src[i+2 : question]
					out.WriteByte('R')
					fmt.Fprintf(out, "%d:", len(condition))
					out.Write(condition)
				} else {
					out.Write(tokenFingerprintDialect(src[i+2:question], dialect))
				}
				out.WriteByte('?')
				if hash >= 0 {
					formFingerprintIntoDialect(out, src[question+1:hash], dialect)
					out.WriteByte('#')
					formFingerprintIntoDialect(out, src[hash+1:end], dialect)
				} else {
					formFingerprintIntoDialect(out, src[question+1:end], dialect)
				}
				out.WriteString("\\@")
				i = end + 2
				literal = i
				continue
			}
		}
		if src[i] == '\\' && i+1 < len(src) {
			i += 2
			continue
		}
		if src[i] != '{' && src[i] != '%' {
			i++
			continue
		}
		close := src[i]
		end := formIslandEnd(src, i, close)
		if end < 0 {
			i++
			continue
		}
		flush(i)
		out.WriteByte('P')
		out.WriteByte(src[i])
		island := src[i+1 : end]
		if formIslandNeedsExactDialect(island, close, false, dialect) {
			out.WriteByte('R')
			fmt.Fprintf(out, "%d:", len(island))
			out.Write(island)
		} else {
			out.Write(tokenFingerprintDialect(island, dialect))
		}
		out.WriteByte(close)
		i = end + 1
		literal = i
	}
	flush(len(src))
}

func formIslandNeedsExactDialect(island []byte, close byte, allowFullwidth bool, dialect format.Dialect) bool {
	if bytes.Contains(island, []byte(";!;")) || bytes.Contains(island, []byte("[[")) {
		return true
	}
	candidate := island
	if allowFullwidth {
		candidate = normalizeFullwidthExpressionBodyDialect(candidate, dialect)
	}
	candidate = bytes.ReplaceAll(candidate, []byte("\r"), []byte(" "))
	candidate = bytes.ReplaceAll(candidate, []byte("\n"), []byte(" "))
	candidate = trimCorpusSpace(candidate)
	if len(candidate) == 0 {
		return true
	}
	if bytes.IndexByte(candidate, 0) < 0 && printVHasNumericIdentifierBoundary(candidate) && !corpusNumericIdentifierBoundarySafe(candidate) {
		return true
	}
	if corpusFormIncrementReadOnly(candidate) {
		return true
	}
	if index, operator := assignmentOperator(candidate); index >= 0 && !(operator == 2 && bytes.Equal(candidate[index:index+operator], []byte("=="))) {
		return true
	}
	if close == '%' {
		fields := splitFingerprintFields(candidate, topLevelCommas(candidate))
		if len(fields) > 3 {
			return true
		}
		if len(fields) > 1 && len(trimCorpusSpace(fields[len(fields)-1])) == 0 {
			fields = fields[:len(fields)-1]
		}
		if len(fields) == 0 {
			return true
		}
		if corpusExpressionHasKnownStringIndex(fields[0], dialect) && !corpusStandaloneStringIndex(fields[0], dialect) && !corpusPercentQuotedStringIndexAllowed(fields[0], dialect) && !corpusPercentStringMultiplicationExpression(fields[0], dialect) && !corpusIndexedStringCallExpressionSafe(fields[0], dialect) && !corpusIndexedStringFormExpressionSafe(fields[0], dialect) && !corpusStringConcatenationExpression(fields[0], dialect) && !corpusExpressionHasDeferredOperand(fields[0], dialect) {
			return true
		}
		if corpusMixedQuotedNumericExpressionNeedsExact(fields[0], dialect) {
			return true
		}
		if corpusPercentMixedExpressionNeedsExact(fields[0], dialect) {
			return true
		}
		if corpusStringIndexInvalidForPercent(fields[0], dialect) && !corpusPercentQuotedStringIndexAllowed(fields[0], dialect) {
			return true
		}
		if corpusComparisonExpressionValid(fields[0], dialect) && corpusComparisonExpressionHasKnownOperands(fields[0], dialect) {
			return true
		}
		deferredStringMultiplication := corpusPercentStringMultiplicationExpression(fields[0], dialect)
		if len(fields) == 1 {
			if deferredStringMultiplication {
				return false
			}
			value := corpusProvenValueType(fields[0], dialect)
			if value == corpusValueNumeric || value == corpusValueInvalid {
				return true
			}
			if conditional := bytes.Index(fields[0], []byte(`\@`)); conditional >= 0 {
				if _, _, end, ok := formConditional(fields[0], conditional); ok && end+2 == len(fields[0]) {
					return false
				}
			}
			return !corpusNumericIdentifierBoundarySafe(fields[0]) && !graphicsNumericExpressionValid(fields[0], true) && !corpusOpaqueExpressionShape(fields[0]) && !corpusOpaqueUnknownMacroShapeDialect(fields[0], dialect)
		}
		firstValue := corpusProvenValueType(fields[0], dialect)
		if !graphicsStringFieldMode(fields[0], dialect) && firstValue != corpusValueUnknown && !deferredStringMultiplication {
			return true
		}
		firstString := firstValue == corpusValueString
		if len(fields) > 1 {
			width := trimCorpusSpace(fields[1])
			if corpusStringIndexInvalidForPercent(width, dialect) || (corpusExpressionHasKnownStringIndex(width, dialect) && !corpusStandaloneStringIndex(width, dialect)) || corpusPercentMixedExpressionNeedsExact(width, dialect) {
				return true
			}
			if len(width) == 0 || !graphicsNumericExpressionValid(width, false) {
				if corpusNumericIdentifierBoundarySafe(width) {
					return false
				}
				if corpusOpaqueUnknownMacroShapeDialect(width, dialect) || bytes.Contains(width, []byte{'"'}) && firstValue == corpusValueUnknown {
					return false
				}
				return true
			}
			if len(fields) == 3 {
				if corpusStringIndexInvalidForPercent(fields[2], dialect) {
					return true
				}
				return formAlignmentNeedsExact(fields[2], dialect, firstString || firstValue == corpusValueNumeric && corpusProvenValueType(width, dialect) == corpusValueNumeric)
			}
		}
		return false
	}
	fields := splitFingerprintFields(candidate, topLevelCommas(candidate))
	if len(fields) > 3 {
		return true
	}
	if len(fields) > 1 && len(trimCorpusSpace(fields[len(fields)-1])) == 0 {
		fields = fields[:len(fields)-1]
	}
	if len(fields) == 0 {
		return true
	}
	first := trimCorpusSpace(fields[0])
	if corpusKnownStringIndexNeedsExact(first, dialect) {
		return true
	}
	if corpusExpressionHasKnownStringIndex(first, dialect) && !corpusStandaloneStringIndex(first, dialect) && !corpusComparisonExpressionValid(first, dialect) && !corpusIndexedStringCallExpressionSafe(first, dialect) && !corpusIndexedStringFormExpressionSafe(first, dialect) && !corpusExpressionHasDeferredOperand(first, dialect) {
		return true
	}
	if len(first) == 0 || !corpusNumericIdentifierBoundarySafe(first) && !graphicsNumericExpressionValid(first, false) && !corpusComparisonExpressionValid(first, dialect) && !corpusOpaqueUnknownMacroShapeDialect(first, dialect) && !corpusExpressionHasDeferredOperand(first, dialect) {
		return true
	}
	firstValue := corpusProvenValueType(first, dialect)
	if firstValue == corpusValueString || firstValue == corpusValueInvalid {
		return true
	}
	firstNumeric := firstValue == corpusValueNumeric
	if len(fields) == 1 {
		return false
	}
	width := trimCorpusSpace(fields[1])
	if firstNumeric && corpusKnownStringIndexNeedsExact(width, dialect) {
		return true
	}
	if len(width) == 0 || !graphicsNumericExpressionValid(width, false) {
		if corpusNumericIdentifierBoundarySafe(width) {
			return false
		}
		if corpusOpaqueUnknownMacroShapeDialect(width, dialect) {
			return false
		}
		if !corpusKnownMixedExpressionNeedsExact(width, dialect) && corpusExpressionHasDeferredOperand(width, dialect) {
			return false
		}
		if bytes.Contains(width, []byte{'"'}) || bytes.Contains(width, []byte{'\''}) {
			return firstNumeric
		}
		return true
	}
	if formWidthNeedsExactDialect(first, width, dialect) {
		return true
	}
	if len(fields) == 3 {
		return formAlignmentNeedsExact(fields[2], dialect, firstNumeric && corpusProvenValueType(width, dialect) == corpusValueNumeric)
	}
	return false
}

func corpusPercentQuotedStringIndexAllowed(src []byte, dialect format.Dialect) bool {
	src = corpusStripOuterParens(trimCorpusSpace(src))
	start := 0
	for start < len(src) && isFingerprintWord(src[start]) {
		start++
	}
	if start == 0 {
		return false
	}
	name := string(src[:start])
	if !corpusNameKnown(corpusPercentQuotedStringIndexBuiltinSet, name) || !corpusStringBuiltin(name, dialect) {
		return false
	}
	for start < len(src) && corpusHalfSpace(src[start]) {
		start++
	}
	if start >= len(src) || src[start] != ':' {
		return false
	}
	start++
	for start < len(src) && corpusHalfSpace(src[start]) {
		start++
	}
	end := corpusStringIndexEnd(src, start)
	return start < end && end == len(src) && src[start] == '"' && end > start+1 && src[end-1] == '"'
}

func corpusOpaqueExpressionShape(src []byte) bool {
	depth := 0
	quoted := byte(0)
	call := false
	for i := 0; i < len(src); i++ {
		if quoted != 0 {
			if src[i] == '\\' {
				i++
			} else if src[i] == quoted {
				quoted = 0
			}
			continue
		}
		switch src[i] {
		case '"', '\'':
			quoted = src[i]
		case '(':
			depth++
			call = true
		case ')':
			if depth == 0 {
				return false
			}
			depth--
		case '{', '%':
			if end := formIslandEnd(src, i, src[i]); end >= 0 {
				i = end
			}
		}
	}
	return call && quoted == 0 && depth == 0
}

func corpusOpaqueUnknownMacroShapeDialect(src []byte, dialect format.Dialect) bool {
	src = trimCorpusSpace(src)
	if len(src) == 0 {
		return false
	}
	fields := bytes.Fields(src)
	if len(fields) == 1 && bytes.HasSuffix(fields[0], []byte{'+'}) {
		word := bytes.TrimSuffix(fields[0], []byte{'+'})
		return bareIdentifier(word) && corpusProvenValueType(word, dialect) == corpusValueUnknown
	}
	if len(fields) == 2 && bytes.Equal(fields[1], []byte("+")) {
		return bareIdentifier(fields[0]) && corpusProvenValueType(fields[0], dialect) == corpusValueUnknown
	}
	if len(fields) < 2 {
		return false
	}
	for _, field := range fields {
		if !bareIdentifier(field) || corpusProvenValueType(field, dialect) != corpusValueUnknown || !corpusPreprocessorCondition(field) {
			return false
		}
	}
	return true
}

func formWidthNeedsExactDialect(first, width []byte, dialect format.Dialect) bool {
	if corpusProvenValueType(first, dialect) != corpusValueNumeric {
		return false
	}
	width = trimCorpusSpace(width)
	value, ok := corpusFormConstant(width)
	if !ok && corpusFormConstantShape(width) {
		return true
	}
	return ok && int32(uint32(value)) < 0
}

func corpusFormIncrementReadOnly(src []byte) bool {
	src = trimCorpusSpace(src)
	if len(src) < 3 {
		return false
	}
	if bytes.HasPrefix(src, []byte("++")) || bytes.HasPrefix(src, []byte("--")) {
		src = trimCorpusSpace(src[2:])
	} else if bytes.HasSuffix(src, []byte("++")) || bytes.HasSuffix(src, []byte("--")) {
		src = trimCorpusSpace(src[:len(src)-2])
	} else {
		return false
	}
	if len(src) == 0 || !isGraphicsIdentifierByte(src[0]) {
		return false
	}
	end := 1
	for end < len(src) && isGraphicsIdentifierByte(src[end]) {
		end++
	}
	return end == len(src) && corpusNameKnown(corpusReadOnlyBuiltinVariableSet, string(src))
}

func corpusFormConstantShape(src []byte) bool {
	if _, ok := corpusConstantLex(src); ok {
		return corpusFormConstantHasEmptyGroup(src)
	}
	hex := false
	for i := 0; i < len(src); i++ {
		b := src[i]
		switch {
		case b == ' ' || b == '\t' || b == '(' || b == ')' || strings.ContainsRune("+-*/%&|^~!?<>=#", rune(b)):
		case b >= '0' && b <= '9':
		case (b == 'x' || b == 'X') && i > 0 && src[i-1] == '0':
			hex = true
		case hex && b >= 'A' && b <= 'F' || hex && b >= 'a' && b <= 'f':
		case (b == 'e' || b == 'E' || b == 'p' || b == 'P') && i > 0 && src[i-1] >= '0' && src[i-1] <= '9':
		default:
			return false
		}
	}
	return true
}

func corpusFormConstantHasEmptyGroup(src []byte) bool {
	for i := 0; i < len(src); i++ {
		if src[i] != '(' {
			continue
		}
		i++
		for i < len(src) && corpusHalfSpace(src[i]) {
			i++
		}
		if i < len(src) && src[i] == ')' {
			return true
		}
	}
	return false
}

func formAlignmentNeedsExact(src []byte, dialect format.Dialect, firstNumeric bool) bool {
	src = trimCorpusSpace(src)
	if len(src) == 0 {
		return true
	}
	if !graphicsNumericExpressionValid(src, true) && !graphicsStringFieldMode(src, dialect) {
		return true
	}
	end := 0
	for end < len(src) && isGraphicsIdentifierByte(src[end]) {
		end++
	}
	if end == 0 {
		return true
	}
	name := string(src[:end])
	if (strings.EqualFold(name, "LEFT") || strings.EqualFold(name, "RIGHT")) && len(trimCorpusSpace(src[end:])) != 0 {
		return true
	}
	if firstNumeric && (corpusNameKnown(corpusCharacterBuiltinVariableSet, name) || corpusNameKnown(corpusReadOnlyBuiltinVariableSet, name) || corpusNameKnown(corpusNumericBuiltinVariableSet, name)) {
		return true
	}
	if firstNumeric && corpusStringBuiltin(name, dialect) {
		return corpusStringIndexInvalidForPercent(src, dialect)
	}
	return false
}

func formConditional(src []byte, start int) (question, hash, end int, ok bool) {
	if start+1 >= len(src) || src[start] != '\\' || src[start+1] != '@' {
		return 0, 0, 0, false
	}
	question = formConditionalDelimiter(src, start+2, '?')
	if question < 0 {
		return 0, 0, 0, false
	}
	hash = formConditionalBranchDelimiter(src, question+1, '#')
	if hash < 0 {
		end = formConditionalBranchEnd(src, question+1)
	} else {
		end = formConditionalBranchEnd(src, hash+1)
	}
	if end < 0 {
		return 0, 0, 0, false
	}
	return question, hash, end, true
}

func formConditionalDelimiter(src []byte, start int, delimiter byte) int {
	quoted := byte(0)
	depth := 0
	for i := start; i < len(src); i++ {
		if quoted != 0 {
			if src[i] == '\\' {
				i++
			} else if src[i] == quoted {
				quoted = 0
			}
			continue
		}
		if src[i] == '"' {
			quoted = src[i]
			continue
		}
		if src[i] == '@' && i+1 < len(src) && src[i+1] == '"' {
			end := quotedFormEnd(src, i)
			if end > i+2 && end <= len(src) {
				i = end - 1
			}
			continue
		}
		if src[i] == '\\' {
			i++
			continue
		}
		if src[i] == '[' && i+1 < len(src) && src[i+1] == '[' {
			if end := bytes.Index(src[i+2:], []byte("]]")); end >= 0 {
				i += end + 3
			}
			continue
		}
		switch src[i] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 && src[i] == delimiter {
				return i
			}
		}
	}
	return -1
}

func formConditionalBranchDelimiter(src []byte, start int, delimiter byte) int {
	for i := start; i < len(src); {
		if src[i] == delimiter {
			return i
		}
		if src[i] == '\\' {
			if i+1 >= len(src) {
				return -1
			}
			if src[i+1] == '@' {
				return -1
			}
			i += 2
			continue
		}
		if src[i] == '%' || src[i] == '{' {
			if end := formIslandEnd(src, i, src[i]); end >= 0 {
				i = end + 1
				continue
			}
			return -1
		}
		i++
	}
	return -1
}

func formConditionalBranchEnd(src []byte, start int) int {
	for i := start; i < len(src); {
		if src[i] == '\\' {
			if i+1 >= len(src) {
				return -1
			}
			if src[i+1] == '@' {
				return i
			}
			i += 2
			continue
		}
		if src[i] == '%' || src[i] == '{' {
			if end := formIslandEnd(src, i, src[i]); end >= 0 {
				i = end + 1
				continue
			}
			return -1
		}
		i++
	}
	return -1
}

func tokenFingerprint(src []byte) []byte {
	return tokenFingerprintDialect(src, format.CommonDialect)
}

func tokenFingerprintDialect(src []byte, dialect format.Dialect) []byte {
	if len(src) > 0 && src[0] == ';' && !bytes.HasPrefix(src, []byte(";!;")) {
		return append([]byte{'S'}, src...)
	}
	if comment := incrementCommentStart(src); comment >= 0 && comment < len(src) {
		out := tokenFingerprintCoreDialect(src[:comment], dialect)
		out = append(out, 'C', 0)
		return append(out, commentFingerprint(src[comment:])...)
	}
	return tokenFingerprintCoreDialect(src, dialect)
}

func tokenFingerprintCore(src []byte) []byte {
	return tokenFingerprintCoreDialect(src, format.CommonDialect)
}

func tokenFingerprintCoreDialect(src []byte, dialect format.Dialect) []byte {
	var out bytes.Buffer
	checkedTriviaEnd := 0
	markerSpan := func(start int) int {
		if start < checkedTriviaEnd {
			return start
		}
		end := mandatoryMarkerSpan(src, start)
		if end == start {
			checkedTriviaEnd = len(src) - len(trimLeadingGrammarSpace(src[start:]))
		}
		return end
	}
	for i := 0; i < len(src); {
		if end := markerSpan(i); end > i {
			out.WriteByte('M')
			fingerprintToken(&out, src[i:end])
			i = end
			continue
		}
		if src[i] == 0 {
			out.Write(src[i:])
			return out.Bytes()
		}
		if src[i] == ' ' || src[i] == '\t' {
			for i < len(src) && (src[i] == ' ' || src[i] == '\t') {
				i++
			}
			continue
		}
		start := i
		if src[i] == '\\' && i+1 < len(src) && src[i+1] == '@' {
			_, _, end, ok := formConditional(src, i)
			if ok {
				fingerprint := conditionalFingerprintDialect(src, i, end, dialect)
				fmt.Fprintf(&out, "F%d:", len(fingerprint))
				out.Write(fingerprint)
				i = end + 2
				continue
			}
			i += 2
		} else if src[i] == '"' || src[i] == '\'' {
			quote := src[i]
			i++
			for i < len(src) {
				if src[i] == '\\' && i+1 < len(src) {
					i += 2
					continue
				}
				i++
				if src[i-1] == quote {
					break
				}
			}
		} else if src[i] == '@' && i+1 < len(src) && src[i+1] == '"' {
			end := quotedFormEnd(src, i)
			if end <= i+2 {
				i++
			} else {
				fingerprint := formAtomFingerprintDialect(src[i:end], dialect)
				fmt.Fprintf(&out, "F%d:", len(fingerprint))
				out.Write(fingerprint)
				i = end
				continue
			}
		} else if src[i] == '[' && i+1 < len(src) && src[i+1] == '[' {
			if end := bytes.Index(src[i+2:], []byte("]]")); end >= 0 {
				i += end + 4
			} else {
				i = len(src)
			}
		} else if src[i] >= '0' && src[i] <= '9' {
			i = fingerprintNumberEnd(src, i)
		} else if isLHSIdentifierStart(src[i]) {
			start := i
			i++
			for i < len(src) && isLHSIdentifierByte(src[i]) {
				if bytes.HasPrefix(src[i:], []byte("\u3000")) {
					if markerSpan(i) > i {
						break
					}
					for i < len(src) && bytes.HasPrefix(src[i:], []byte("\u3000")) {
						i += 3
					}
					continue
				}
				i++
			}
			for nul := bytes.IndexByte(src[start:i], 0); nul >= 0; {
				absolute := start + nul
				if !corpusNulEmbeddedInIdentifier(src, absolute) {
					fingerprintToken(&out, src[start:absolute])
					out.Write(src[absolute:])
					return out.Bytes()
				}
				next := bytes.IndexByte(src[absolute+1:i], 0)
				if next < 0 {
					break
				}
				nul = absolute + 1 + next - start
			}
		} else if src[i] == '\\' && i+1 < len(src) {
			i += 2
		} else {
			i++
			if i < len(src) && isRuntimeDoubleFingerprintOperator(src[start], src[i]) {
				i++
			}
		}
		fmt.Fprintf(&out, "%d:", i-start)
		out.Write(src[start:i])
	}
	return out.Bytes()
}

func isRuntimeDoubleFingerprintOperator(first, second byte) bool {
	switch first {
	case '+', '-':
		return second == first
	case '=':
		return second == '='
	case '!':
		return second == '=' || second == '&' || second == '|'
	case '<', '>':
		return second == '=' || second == first
	case '|', '&', '^':
		return second == first
	default:
		return false
	}
}

func fingerprintNumberEnd(src []byte, start int) int {
	i, base := start, 10
	if i+1 < len(src) && src[i] == '0' {
		switch src[i+1] {
		case 'x', 'X':
			base, i = 16, i+2
		case 'b', 'B':
			base, i = 2, i+2
		}
	}
	if base != 10 && i < len(src) && src[i] == '+' {
		i++
	}
	digitStart := i
	for i < len(src) && fingerprintDigit(src[i], base) {
		i++
	}
	if i == digitStart {
		return start + 1
	}
	if i < len(src) && ((base == 10 && (src[i] == 'e' || src[i] == 'E')) || src[i] == 'p' || src[i] == 'P' || (base == 2 && (src[i] == 'e' || src[i] == 'E'))) {
		i++
		if i < len(src) && (src[i] == '+' || src[i] == '-') {
			i++
		}
		for i < len(src) && fingerprintDigit(src[i], base) {
			i++
		}
	}
	return i
}

func fingerprintDigit(b byte, base int) bool {
	if b >= '0' && b <= '9' {
		return int(b-'0') < base
	}
	return base == 16 && (b >= 'a' && b <= 'f' || b >= 'A' && b <= 'F')
}

func quotedFormEnd(src []byte, start int) int {
	end, _ := quotedFormEndComplete(src, start)
	return end
}

func quotedFormEndComplete(src []byte, start int) (int, bool) {
	if start+1 >= len(src) || src[start] != '@' || src[start+1] != '"' {
		return -1, false
	}
	for i := start + 2; i < len(src); {
		switch src[i] {
		case '\\':
			if i+1 < len(src) && src[i+1] == '@' {
				_, _, end, ok := formConditional(src, i)
				if ok {
					i = end + 2
					continue
				}
			}
			if i+1 < len(src) {
				i += 2
			} else {
				return len(src), false
			}
		case '%', '{':
			if end := formIslandEnd(src, i, src[i]); end >= 0 {
				i = end + 1
			} else {
				return len(src), false
			}
		case '"':
			return i + 1, true
		default:
			i++
		}
	}
	return len(src), false
}

func formIslandEnd(src []byte, start int, close byte) int {
	if close == '{' {
		close = '}'
	}
	depth := 0
	for i := start + 1; i < len(src); {
		switch src[i] {
		case '"':
			i = printVQuotedEnd(src, i)
			continue
		case '@':
			if i+1 < len(src) && src[i+1] == '"' {
				end := quotedFormEnd(src, i)
				if end <= i+2 {
					return -1
				}
				i = end
				continue
			}
		case '\\':
			if i+1 < len(src) && src[i+1] == '@' {
				_, _, end, ok := formConditional(src, i)
				if !ok {
					return -1
				}
				i = end + 2
				continue
			}
		case '(', '[':
			depth++
		case ')', ']':
			if depth > 0 {
				depth--
			}
		case '%', '}':
			if depth == 0 && src[i] == close {
				return i
			}
		}
		i++
	}
	return -1
}

func formAtomFingerprint(src []byte) []byte {
	return formAtomFingerprintDialect(src, format.CommonDialect)
}

func formAtomFingerprintDialect(src []byte, dialect format.Dialect) []byte {
	end, complete := quotedFormEndComplete(src, 0)
	if !complete || end != len(src) || end < 3 {
		return append([]byte(nil), src...)
	}
	var out bytes.Buffer
	out.WriteString("Q2:")
	out.Write(src[:2])
	out.Write(formFingerprintDialect(src[2:end-1], dialect))
	out.WriteByte('"')
	return out.Bytes()
}

func conditionalFingerprintDialect(src []byte, start, end int, dialect format.Dialect) []byte {
	question, hash, _, ok := formConditional(src, start)
	if !ok {
		return append([]byte(nil), src[start:end+2]...)
	}
	var out bytes.Buffer
	out.WriteString("C")
	if formConditionalNeedsExactDialect(src[start:end+2], dialect) {
		condition := src[start+2 : question]
		out.WriteByte('R')
		fmt.Fprintf(&out, "%d:", len(condition))
		out.Write(condition)
	} else {
		out.Write(tokenFingerprintDialect(src[start+2:question], dialect))
	}
	out.WriteByte('?')
	if hash >= 0 {
		out.Write(formFingerprintDialect(src[question+1:hash], dialect))
		out.WriteByte('#')
		out.Write(formFingerprintDialect(src[hash+1:end], dialect))
	} else {
		out.Write(formFingerprintDialect(src[question+1:end], dialect))
	}
	out.WriteString("\\@")
	return out.Bytes()
}
