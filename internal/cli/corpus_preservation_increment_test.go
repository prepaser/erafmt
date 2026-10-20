package cli

import (
	"bytes"
	"strings"

	"github.com/prepaser/erafmt/internal/format"
)

func corpusIncrementExpressionNeedsExact(src []byte, dialect format.Dialect) bool {
	positions := corpusIncrementOperatorPositions(src)
	if len(positions) == 0 {
		return false
	}
	deferred := corpusExpressionHasDeferredOperand(src, dialect)
	for i := 1; i < len(positions); i++ {
		middle := trimCorpusSpace(src[positions[i-1]+2 : positions[i]])
		if len(middle) == 0 {
			if deferred {
				continue
			}
			return true
		}
		if !corpusIncrementBinaryOperator(middle) {
			return true
		}
	}
	for i, position := range positions {
		before := trimCorpusSpace(src[:position])
		after := trimCorpusSpace(src[position+2:])
		incrementChain := deferred && i > 0 && len(trimCorpusSpace(src[positions[i-1]+2:position])) == 0
		prefix := len(before) == 0 || corpusIncrementPrefixBoundary(before[len(before)-1])
		knownInvalid := corpusIncrementKnownInvalidOperand(src, position, dialect, deferred)
		if !prefix && corpusIncrementNumericIdentifierGap(after) {
			knownInvalid = false
		}
		if prefix && deferred && corpusExpressionHasDeferredOperand(before, dialect) {
			knownInvalid = false
		}
		if !incrementChain && knownInvalid {
			logicalPrefix := len(before) != 0 && before[len(before)-1] == '!' && corpusExpressionHasDeferredOperand(before[:len(before)-1], dialect)
			if !deferred || !logicalPrefix {
				return true
			}
		}
		if prefix {
			if len(before) != 0 && before[len(before)-1] == '!' {
				if !deferred {
					return true
				}
				continue
			}
			if corpusIncrementReadonlyName(after) && !deferred {
				return true
			}
			continue
		}
		if len(after) != 0 && !corpusIncrementNextIsBinary(after) && !corpusIncrementNumericIdentifierGap(after) {
			if !deferred {
				return true
			}
			continue
		}
		if corpusIncrementLogicalPrefixBefore(before) {
			if !deferred {
				return true
			}
			continue
		}
		if corpusIncrementReadonlySuffix(before) && !deferred {
			return true
		}
	}
	return false
}

func corpusIncrementKnownInvalidOperand(src []byte, position int, dialect format.Dialect, deferred bool) bool {
	before := trimCorpusSpace(src[:position])
	after := trimCorpusSpace(src[position+2:])
	prefix := len(before) == 0 || corpusIncrementPrefixBoundary(before[len(before)-1])
	if prefix {
		operand := corpusIncrementLeadingOperand(after)
		return corpusIncrementOperandKnownInvalid(operand, dialect, deferred)
	}
	operand := corpusIncrementTrailingOperand(before)
	return corpusIncrementOperandKnownInvalid(operand, dialect, deferred)
}

func corpusIncrementLeadingOperand(src []byte) []byte {
	src = trimCorpusSpace(src)
	if len(src) == 0 {
		return nil
	}
	if src[0] == '"' {
		end := printVQuotedEnd(src, 0)
		if end > 0 && end <= len(src) {
			return src[:end]
		}
	}
	if src[0] >= '0' && src[0] <= '9' {
		end, _ := printVRuntimeNumberEnd(src, 0)
		if end > 0 {
			return src[:end]
		}
	}
	if src[0] == '(' {
		if end := matchingCallDelimiter(src, 0, '(', ')'); end >= 0 {
			return src[:end+1]
		}
	}
	end := 0
	for end < len(src) && isLHSIdentifierByte(src[end]) {
		end++
	}
	if end == 0 {
		return nil
	}
	for end < len(src) && corpusHalfSpace(src[end]) {
		end++
	}
	if end < len(src) && src[end] == '(' {
		if close := matchingCallDelimiter(src, end, '(', ')'); close >= 0 {
			return src[:close+1]
		}
	}
	return src[:end]
}

func corpusIncrementTrailingOperand(src []byte) []byte {
	src = trimCorpusSpace(src)
	if len(src) == 0 {
		return nil
	}
	if src[len(src)-1] == ')' || src[len(src)-1] == ']' {
		open, close := byte('('), byte(')')
		if src[len(src)-1] == ']' {
			open, close = '[', ']'
		}
		for i := 0; i < len(src); i++ {
			if src[i] != open {
				continue
			}
			if end := matchingCallDelimiter(src, i, open, close); end == len(src)-1 {
				start := i
				for start > 0 {
					if isLHSIdentifierByte(src[start-1]) {
						start--
						continue
					}
					if src[start-1] != ':' {
						break
					}
					start--
					for start > 0 && isLHSIdentifierByte(src[start-1]) {
						start--
					}
				}
				return src[start:]
			}
		}
	}
	if src[len(src)-1] == '"' {
		start := bytes.LastIndexByte(src[:len(src)-1], '"')
		if start >= 0 {
			return src[start:]
		}
	}
	end := len(src)
	for end > 0 && isLHSIdentifierByte(src[end-1]) {
		end--
	}
	for end > 0 && src[end-1] == ':' {
		end--
		for end > 0 && isLHSIdentifierByte(src[end-1]) {
			end--
		}
	}
	return src[end:]
}

func corpusIncrementOperandKnownInvalid(src []byte, dialect format.Dialect, deferred bool) bool {
	src = trimCorpusSpace(src)
	if len(src) == 0 {
		return false
	}
	if src[0] == '(' && matchingCallDelimiter(src, 0, '(', ')') == len(src)-1 {
		inner := corpusStripOuterParens(src)
		if corpusExpressionHasDeferredOperand(inner, dialect) {
			return false
		}
		if corpusProvenValueType(inner, dialect) == corpusValueNumeric || corpusProvenValueType(inner, dialect) == corpusValueString {
			return true
		}
		return corpusIncrementOperandKnownInvalid(inner, dialect, false)
	}
	if deferred {
		if src[len(src)-1] != ')' {
			return false
		}
	}
	if src[0] == '"' {
		end, complete := printVQuotedEndComplete(src, 0)
		return complete && end == len(src)
	}
	if src[0] >= '0' && src[0] <= '9' {
		end, ok := printVRuntimeNumberEnd(src, 0)
		return ok && end == len(src)
	}
	if corpusDirectBuiltinValueType(src, dialect) == corpusValueString {
		return true
	}
	if src[len(src)-1] != ')' {
		return false
	}
	for i := 0; i < len(src); i++ {
		if src[i] != '(' {
			continue
		}
		end := matchingCallDelimiter(src, i, '(', ')')
		if end != len(src)-1 {
			continue
		}
		start := i
		for start > 0 && isLHSIdentifierByte(src[start-1]) {
			start--
		}
		name := string(src[start:i])
		return corpusBuiltinMethod(name, dialect) && !corpusExpressionHasDeferredOperand(src[i+1:end], dialect)
	}
	return false
}

func corpusIncrementNumericIdentifierGap(src []byte) bool {
	src = trimCorpusSpace(src)
	if len(src) == 0 || src[0] < '0' || src[0] > '9' {
		return false
	}
	end, ok := printVRuntimeNumberEnd(src, 0)
	for end < len(src) && corpusHalfSpace(src[end]) {
		end++
	}
	return ok && end < len(src) && isLHSIdentifierStart(src[end])
}

func corpusIncrementLogicalPrefixBefore(src []byte) bool {
	index := bytes.LastIndexByte(src, '!')
	if index < 0 {
		return false
	}
	left := trimCorpusSpace(src[:index])
	if len(left) != 0 && !corpusIncrementPrefixBoundary(left[len(left)-1]) {
		return false
	}
	right := trimCorpusSpace(src[index+1:])
	return len(right) != 0 && !corpusIncrementBinaryOperator(right)
}

func corpusIncrementOperatorPositions(src []byte) []int {
	positions := make([]int, 0, 2)
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
		case '+', '-':
			if i+1 < len(src) && src[i+1] == src[i] {
				positions = append(positions, i)
				i++
			}
		}
	}
	return positions
}

func corpusIncrementBinaryOperator(src []byte) bool {
	for i := 0; i < len(src); i++ {
		if strings.ContainsRune("+-*/%&|^=!?<>,:", rune(src[i])) {
			return true
		}
	}
	return false
}

func corpusIncrementNextIsBinary(src []byte) bool {
	if len(src) == 0 {
		return true
	}
	if strings.ContainsRune("+-*/%&|^=!?<>,:)]}", rune(src[0])) {
		return true
	}
	for _, keyword := range []string{"IS", "TO"} {
		if len(src) >= len(keyword) && strings.EqualFold(string(src[:len(keyword)]), keyword) && (len(src) == len(keyword) || !isFingerprintWord(src[len(keyword)])) {
			return true
		}
	}
	return false
}

func corpusIncrementPrefixBoundary(b byte) bool {
	return strings.ContainsRune("(,[{+-*/%&|^=!?<>,:#", rune(b))
}

func corpusIncrementReadonlyName(src []byte) bool {
	src = trimCorpusSpace(src)
	end := 0
	for end < len(src) && isGraphicsIdentifierByte(src[end]) {
		end++
	}
	if end == 0 || !corpusNameKnown(corpusReadOnlyBuiltinVariableSet, string(src[:end])) {
		return false
	}
	rest := bytes.TrimLeft(src[end:], " \t")
	return len(rest) == 0 || corpusIncrementNextIsBinary(rest)
}

func corpusIncrementReadonlySuffix(src []byte) bool {
	src = trimCorpusSpace(src)
	end := len(src)
	start := end
	for start > 0 && isGraphicsIdentifierByte(src[start-1]) {
		start--
	}
	if start == end || !corpusNameKnown(corpusReadOnlyBuiltinVariableSet, string(src[start:end])) {
		return false
	}
	for start > 0 && corpusHalfSpace(src[start-1]) {
		start--
	}
	return start == 0 || corpusIncrementPrefixBoundary(src[start-1])
}
