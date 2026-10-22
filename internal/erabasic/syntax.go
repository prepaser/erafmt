package erabasic

import (
	"bytes"
	"strings"
	"unicode/utf8"
)

func SyntaxHasProtectedMetadataTail(head []byte) bool {
	if len(head) == 0 || (head[0] != '#' && head[0] != '$') {
		return false
	}
	end, found := SyntaxCommentEnd(head, 0, len(head))
	return found && end+1 < len(head) && head[end+1] == ';'
}

func syntaxExpressionEndsWithHash(tokens []ExprToken) bool {
	return len(tokens) != 0 && tokens[len(tokens)-1].Kind == ExprOperator && string(tokens[len(tokens)-1].Text) == "#"
}

type AssignmentOp struct {
	Start int
	End   int
	Plain bool
}

func AssignmentBoundary(src []byte, sessions ...*ExpressionSession) (int, AssignmentOp, bool) {
	if bytes.IndexByte(src, 0) >= 0 {
		end, _ := SyntaxCommentEnd(src, 0, len(src), sessions...)
		src = src[:end]
	}
	depth := 0
	for i := 0; i < len(src); i++ {
		if next, atom := SyntaxAtomEnd(src, i, len(src), sessions...); atom {
			if next < 0 {
				return 0, AssignmentOp{}, false
			}
			i = next - 1
			continue
		}
		switch src[i] {
		case '(', '[':
			depth++
		case ')', ']':
			if depth > 0 {
				depth--
			}
		case ';':
			return 0, AssignmentOp{}, false
		}
		if depth != 0 {
			continue
		}
		for _, op := range []string{"<<=", ">>=", "+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=", "'=", "==", "="} {
			if !bytes.HasPrefix(src[i:], []byte(op)) {
				continue
			}
			if op == "=" && i > 0 && strings.ContainsRune("<>!", rune(src[i-1])) {
				continue
			}
			return i, AssignmentOp{Start: i, End: i + len(op), Plain: op == "=" || op == "=="}, true
		}
	}
	return 0, AssignmentOp{}, false
}

func VariableLHSFromTokens(tokens []ExprToken, err error) (string, bool) {
	if err != nil || len(tokens) == 0 || tokens[0].Kind != ExprIdentifier {
		return "", false
	}
	next := 1
	if next < len(tokens) && string(tokens[next].Text) == "@" {
		if next+1 >= len(tokens) || tokens[next+1].Kind != ExprIdentifier {
			return "", false
		}
		next += 2
	}
	return strings.ToUpper(string(tokens[0].Text)), len(tokens) == next || string(tokens[next].Text) == ":" || string(tokens[next].Text) == "["
}

func scanSyntaxForm(src []byte, start, end int, mode FormMode, sessions []*ExpressionSession) (int, error) {
	if len(sessions) == 0 || sessions[0] == nil {
		return ScanForm(src[start:end], mode)
	}
	next, err := sessions[0].ScanFormAt(ExpressionSourceOriginal, start, end, mode)
	return next - start, err
}

func InstructionBodyWithSpace(head []byte, wordEnd int, allowFullwidthSpace bool, sessions ...*ExpressionSession) (body []byte, bodyStart, end, comment int, hasArg bool) {
	bodyStart = wordEnd
	if wordEnd >= len(head) {
		return nil, bodyStart, len(head), -1, false
	}
	if !InputSpaceAt(head, wordEnd, allowFullwidthSpace) && head[wordEnd] != ';' {
		return nil, bodyStart, len(head), -1, false
	}
	bodyStart += InputSpaceSize(head, wordEnd, allowFullwidthSpace)
	if bodyStart == wordEnd {
		bodyStart++
	}
	if bodyStart > len(head) {
		bodyStart = len(head)
	}
	end, found := SyntaxCommentEnd(head, bodyStart, len(head), sessions...)
	if found {
		comment = end
	} else {
		comment = -1
	}
	return head[bodyStart:end], bodyStart, end, comment, true
}

func SyntaxAtomEnd(src []byte, pos, end int, sessions ...*ExpressionSession) (int, bool) {
	if pos < 0 || pos >= end || end > len(src) {
		return -1, false
	}
	if src[pos] == '\\' && pos+1 < end && src[pos+1] == '@' {
		n, err := scanSyntaxForm(src, pos, end, FormConditional, sessions)
		if err != nil {
			return -1, true
		}
		return pos + n, true
	}
	if src[pos] == '@' && pos+1 < end && src[pos+1] == '"' {
		n, err := scanSyntaxForm(src, pos, end, FormQuoted, sessions)
		if err != nil {
			return -1, true
		}
		return pos + n, true
	}
	if src[pos] != '"' {
		return pos, false
	}
	n := QuotedEnd(src[pos:end], 0)
	if n < 0 {
		return -1, true
	}
	return pos + n, true
}

func SyntaxCommentEnd(src []byte, start, end int, sessions ...*ExpressionSession) (int, bool) {
	hasNUL := bytes.IndexByte(src[start:end], 0) >= 0
	for i := start; i < end; i++ {
		if next, atom := SyntaxAtomEnd(src, i, end, sessions...); atom {
			if next < 0 {
				return end, false
			}
			i = next - 1
			continue
		}
		if src[i] == 0 {
			return i, false
		}
		if src[i] == ';' {
			if MandatoryInlineMarker(src[:end], i) {
				i += 2
				continue
			}
			if ExecutableMarker(src[:end], i) {
				return end, false
			}
			return i, true
		}
		if hasNUL {
			r, size := utf8.DecodeRune(src[i:end])
			if isASCIIDigit(r) {
				next, err := scanNumber(src[:end], i)
				if err == nil {
					i = next - 1
				}
			} else if !IsDelimiter(r) && r != '\u3000' {
				i = expressionIdentifierEnd(src[:end], i) - 1
			} else {
				i += size - 1
			}
		}
	}
	return end, false
}

func CallPrefix(src []byte, start, end int) (int, int, bool) {
	for i := start; i < end; i++ {
		switch src[i] {
		case 0:
			return i, -1, true
		case '\\':
			if i+1 >= end || src[i+1] == 0 {
				return i, -1, false
			}
			i = escapedEnd(src[:end], i) - 1
		case '[', '(', ',', ';':
			return i, i, true
		}
	}
	return end, -1, true
}

func trimCallTargetRight(src []byte, start, end int) int {
	last := start
	for i := start; i < end; {
		if src[i] == '\\' {
			i = escapedEnd(src[:end], i)
			last = i
			continue
		}
		_, size := utf8.DecodeRune(src[i:end])
		if !HalfSpace(src[i]) {
			last = i + size
		}
		i += size
	}
	return last
}

func MatchingDelimiter(src []byte, start, end int, open, close byte, sessions ...*ExpressionSession) int {
	depth := 0
	for i := start; i < end; i++ {
		if next, atom := SyntaxAtomEnd(src, i, end, sessions...); atom {
			if next < 0 {
				return -1
			}
			i = next - 1
			continue
		}
		if src[i] == open {
			depth++
		}
		if src[i] == close {
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

type SyntaxSpan struct{ Start, End int }

func SplitTopLevel(src []byte, start, end int, separator byte, sessions ...*ExpressionSession) []SyntaxSpan {
	var out []SyntaxSpan
	part, depth := start, 0
	for i := start; i < end; i++ {
		if next, atom := SyntaxAtomEnd(src, i, end, sessions...); atom {
			if next < 0 {
				return []SyntaxSpan{{start, end}}
			}
			i = next - 1
			continue
		}
		switch src[i] {
		case '(', '[':
			depth++
		case ')', ']':
			if depth > 0 {
				depth--
			}
		case separator:
			if depth == 0 {
				if len(out) >= MaxExpressionTokens-1 {
					if len(sessions) != 0 && sessions[0] != nil {
						sessions[0].resource(i, "field count exceeds the formatter resource limit")
					}
					return []SyntaxSpan{{start, end}}
				}
				out = append(out, SyntaxSpan{part, i})
				part = i + 1
			}
		}
	}
	return append(out, SyntaxSpan{part, end})
}

func TopEqual(src []byte, start, end int, sessions ...*ExpressionSession) (int, bool) {
	depth := 0
	for i := start; i < end; i++ {
		if next, atom := SyntaxAtomEnd(src, i, end, sessions...); atom {
			if next < 0 {
				return 0, false
			}
			i = next - 1
			continue
		}
		switch src[i] {
		case '(', '[':
			depth++
		case ')', ']':
			if depth > 0 {
				depth--
			}
		case ';':
			if depth == 0 {
				return 0, false
			}
		}
		if src[i] == '=' && depth == 0 && (i+1 >= end || src[i+1] != '=') && defaultEqualBoundary(src, start, i) {
			return i, true
		}
	}
	return 0, false
}

func defaultEqualBoundary(src []byte, start, equal int) bool {
	left := TrimInputRight(src, start, equal, true)
	return left == start || !strings.ContainsRune("=<>!+-*/%&|^'", rune(src[left-1]))
}

func SyntaxCommand(command string) bool {
	if ExpressionCommand(command) || SimpleIntegerCommand(command) || SimpleStringCommand(command) || PrintVCommand(command) || PrintFormCommand(command) || CallCommand(command) || ForCommand(command) || StructuredListCommand(command) {
		return true
	}
	if noArgCommand(command) {
		return true
	}
	return false
}

func RawCommand(command string) bool {
	switch command {
	case "CUSTOMDRAWLINE", "DEBUGPRINT", "DEBUGPRINTL", "PRINTC", "PRINTCD", "PRINTCK", "PRINTLC", "PRINTLCD", "PRINTLCK", "PRINT", "PRINTL", "PRINTW", "PRINTK", "PRINTKL", "PRINTKW", "PRINTD", "PRINTDL", "PRINTDW", "PRINTSINGLE", "PRINTSINGLEK", "PRINTSINGLED", "PRINTPLAIN", "TIMES", "VARI", "VARS":
		return true
	default:
		return false
	}
}

func CommandDialectReason(command string, dialect Dialect) string {
	if CommandUnavailable(command, dialect) {
		return "unavailable in selected dialect"
	}
	return "not recognized by the formatter"
}

func noArgCommand(command string) bool {
	switch command {
	case "ELSE", "CASEELSE", "CATCH", "ENDIF", "ENDSELECT", "NEXT", "WEND", "DO", "REND", "LOOP", "ENDNOSKIP", "ENDCATCH", "ENDDATA", "ENDLIST", "ENDFUNC", "BREAK", "CONTINUE", "NOSKIP", "DATALIST", "TRYCALLLIST", "TRYJUMPLIST", "TRYGOTOLIST", "PRINT_ITEM", "PRINT_SHOPITEM", "DRAWLINE", "UPCHECK", "ADDDEFCHARA", "QUIT", "CLEARTEXTBOX", "STOPCALLTRAIN", "DELALLCHARA", "GETTIME", "QUIT_AND_RESTART", "FORCE_QUIT", "FORCE_QUIT_AND_RESTART", "OUTPUTLOG":
		return true
	default:
		return false
	}
}

func SimpleIntegerCommand(command string) bool {
	switch command {
	case "RETURN", "REDRAW", "CLEARLINE", "CUPCHECK", "CALLTRAIN", "DOTRAIN", "DELDATA", "RESET_STAIN", "FORCEKANA", "SKIPDISP", "SKIPLOG", "PRINT_ABL", "PRINT_TALENT", "PRINT_MARK", "PRINT_EXP", "PRINT_PALAM":
		return true
	default:
		return false
	}
}

func SimpleStringCommand(command string) bool {
	switch command {
	case "RETURNF", "PRINTS", "PRINTSL", "PRINTSW", "PRINTSK", "PRINTSKL", "PRINTSKW", "PRINTSD", "PRINTSDL", "PRINTSDW", "PRINTSINGLES", "PRINTSINGLESK", "PRINTSINGLESD", "PRINTFORMS", "PRINTFORMSL", "PRINTFORMSW", "PRINTFORMSK", "PRINTFORMSKL", "PRINTFORMSKW", "PRINTFORMSD", "PRINTFORMSDL", "PRINTFORMSDW", "PRINTSINGLEFORMS", "PRINTSINGLEFORMSK", "PRINTSINGLEFORMSD":
		return true
	default:
		return false
	}
}

func PrintVCommand(command string) bool {
	switch command {
	case "PRINTV", "PRINTVL", "PRINTVW", "PRINTVK", "PRINTVKL", "PRINTVKW", "PRINTVD", "PRINTVDL", "PRINTVDW", "PRINTSINGLEV", "PRINTSINGLEVK", "PRINTSINGLEVD":
		return true
	default:
		return false
	}
}

func PrintFormCommand(command string) bool {
	switch command {
	case "PRINTFORM", "PRINTFORML", "PRINTFORMW", "PRINTFORMK", "PRINTFORMKL", "PRINTFORMKW", "PRINTFORMD", "PRINTFORMDL", "PRINTFORMDW", "PRINTFORMC", "PRINTFORMLC", "PRINTFORMCK", "PRINTFORMLCK", "PRINTFORMCD", "PRINTFORMLCD", "PRINTSINGLEFORM", "PRINTSINGLEFORMK", "PRINTSINGLEFORMD", "DATAFORM", "RETURNFORM":
		return true
	default:
		return false
	}
}

func CallCommand(command string) bool {
	switch command {
	case "CALL", "CALLF", "CALLFORM", "CALLFORMF", "JUMP", "JUMPFORM", "GOTO", "GOTOFORM", "TRYCALL", "TRYCALLFORM", "TRYJUMP", "TRYJUMPFORM", "TRYGOTO", "TRYGOTOFORM", "TRYCALLF", "TRYCALLFORMF", "TRYCCALL", "TRYCCALLFORM", "TRYCJUMP", "TRYCJUMPFORM", "TRYCGOTO", "TRYCGOTOFORM", "CALLSHARP", "FUNC":
		return true
	default:
		return false
	}
}

func ForCommand(command string) bool {
	return command == "FOR"
}

func StructuredListCommand(command string) bool {
	switch command {
	case "SETBIT", "CLEARBIT", "INVERTBIT", "VARSET", "CVARSET", "POWER":
		return true
	default:
		return false
	}
}
