package format

import "bytes"

func isCaseKeyword(text []byte) bool {
	return len(text) == 2 && (bytes.EqualFold(text, []byte("IS")) || bytes.EqualFold(text, []byte("TO")))
}

func provenCaseShape(tokens []exprToken, dialect Dialect) error {
	if !expressionShapeIdentifiersKnown(tokens, dialect) {
		return nil
	}
	if err := nestedExpressionComma(tokens); err != nil {
		return err
	}
	if err := nestedCaseKeyword(tokens); err != nil {
		return err
	}
	for _, clause := range splitExpressionClauses(tokens) {
		if len(clause) == 0 {
			continue
		}
		classifyOperators(clause, true)
		if err := misplacedCaseKeywords(clause); err != nil {
			return err
		}
		keywords := caseClauseKeywords(clause)
		if expressionShapeIdentifiersKnown(clause, dialect) {
			if len(keywords.to) > 0 {
				if err := provenIncrementOperatorState(clause[:keywords.to[0]], dialect); err != nil {
					return err
				}
				if err := provenIncrementOperatorState(clause[keywords.to[0]+1:], dialect); err != nil {
					return err
				}
			} else if err := provenIncrementOperatorState(clause, dialect); err != nil {
				return err
			}
		}
		if len(keywords.is) == 0 && len(keywords.to) == 0 {
			if err := provenCaseOperandShape(clause, dialect); err != nil {
				return err
			}
			continue
		}
		if len(keywords.is) > 1 || len(keywords.to) > 1 {
			duplicate := keywords.is
			if len(keywords.to) > 1 && (len(duplicate) < 2 || keywords.to[1] < duplicate[1]) {
				duplicate = keywords.to
			}
			return expressionShapeError(clause[duplicate[1]], "duplicate CASE keyword")
		}
		if len(keywords.is) > 0 {
			is := keywords.is[0]
			if is != 0 {
				return expressionShapeError(clause[is], "CASE IS must start a clause")
			}
			if len(keywords.to) > 0 {
				return expressionShapeError(clause[keywords.to[0]], "CASE IS cannot contain TO")
			}
			if is+1 >= len(clause) {
				return expressionShapeError(clause[is], "CASE IS requires an operator")
			}
			op := clause[is+1]
			if op.kind != exprOperator || !caseBinaryOperator(string(op.text)) {
				return expressionShapeError(op, "CASE IS requires a binary operator")
			}
			if is+2 >= len(clause) {
				return expressionShapeError(op, "CASE IS requires an expression")
			}
			if err := provenCaseOperandShape(clause[is+2:], dialect); err != nil {
				return err
			}
			continue
		}
		to := keywords.to[0]
		if to == 0 {
			return expressionShapeError(clause[to], "CASE TO requires a left expression")
		}
		if to == len(clause)-1 {
			return expressionShapeError(clause[to], "CASE TO requires a right expression")
		}
		if err := provenCaseOperandShape(clause[:to], dialect); err != nil {
			return err
		}
		if err := provenCaseOperandShape(clause[to+1:], dialect); err != nil {
			return err
		}
	}
	return nil
}

func caseBinaryOperator(op string) bool {
	return op != "=" && op != "!" && op != "~" && op != "++" && op != "--" && op != "?" && op != "#" && constantPriority(op) != 0
}

func provenCaseOperandShape(tokens []exprToken, dialect Dialect) error {
	if len(tokens) == 0 || !expressionShapeIdentifiersKnown(tokens, dialect) {
		return nil
	}
	if tokens[len(tokens)-1].kind == exprOperator {
		if !isIncrementOperator(tokens[len(tokens)-1]) {
			return expressionShapeError(tokens[len(tokens)-1], "CASE expression is incomplete")
		}
		if !provenCaseIncrement(tokens, dialect) {
			return expressionShapeError(tokens[len(tokens)-1], "CASE expression has an invalid increment")
		}
	}
	if tokens[0].kind == exprOperator {
		switch string(tokens[0].text) {
		case "+", "-", "!", "~":
		case "++", "--":
			if !provenCaseIncrement(tokens, dialect) {
				return expressionShapeError(tokens[0], "CASE expression has an invalid increment")
			}
		default:
			return expressionShapeError(tokens[0], "CASE expression has an invalid leading operator")
		}
	}
	if err := provenKnownExpressionBoundaries(tokens, dialect); err != nil {
		return err
	}
	return provenOrdinaryExpressionShape(tokens, dialect)
}

func provenCaseIncrement(tokens []exprToken, dialect Dialect) bool {
	prefix := isIncrementOperator(tokens[0])
	postfix := isIncrementOperator(tokens[len(tokens)-1])
	if prefix && postfix {
		return false
	}
	value, valid, proven := builtinIncrementType(tokens, dialect)
	if proven {
		return valid && value == knownValueNumeric
	}
	start, end := 0, len(tokens)
	if prefix {
		start++
	}
	if end > start && postfix {
		end--
	}
	if start >= end {
		return false
	}
	operand := tokens[start:end]
	if _, _, known := constantExpressionType(operand); known {
		return false
	}
	if close, matched := boundedExpressionMatchingParen(operand, 0); matched && close == len(operand)-1 {
		_, _, known := builtinExpressionType(operand, dialect)
		return !known
	}
	return true
}

const (
	maxBuiltinProofTokens = 4096
	maxBuiltinProofDepth  = 256
)

func boundedExpressionMatchingParen(tokens []exprToken, start int) (int, bool) {
	if start < 0 || start >= len(tokens) || string(tokens[start].text) != "(" {
		return -1, false
	}
	end := start + maxBuiltinProofTokens
	if end > len(tokens) {
		end = len(tokens)
	}
	depth := 0
	for i := start; i < end; i++ {
		switch string(tokens[i].text) {
		case "(":
			depth++
			if depth > maxBuiltinProofDepth {
				return -1, false
			}
		case ")":
			depth--
			if depth == 0 {
				return i, true
			}
		}
	}
	return -1, false
}

type caseClauseKeywordPositions struct {
	is []int
	to []int
}

func caseClauseKeywords(tokens []exprToken) caseClauseKeywordPositions {
	var positions caseClauseKeywordPositions
	depth := 0
	for i, token := range tokens {
		if token.kind == exprSymbol {
			switch string(token.text) {
			case "(", "[":
				depth++
			case ")", "]":
				if depth > 0 {
					depth--
				}
			}
		}
		if depth != 0 || token.kind != exprIdentifier || !token.caseKeyword {
			continue
		}
		switch {
		case bytes.EqualFold(token.text, []byte("IS")):
			positions.is = append(positions.is, i)
		case bytes.EqualFold(token.text, []byte("TO")):
			positions.to = append(positions.to, i)
		}
	}
	return positions
}

func misplacedCaseKeywords(tokens []exprToken) error {
	depth := 0
	for i, token := range tokens {
		if token.kind == exprSymbol {
			switch string(token.text) {
			case "(", "[":
				depth++
			case ")", "]":
				if depth > 0 {
					depth--
				}
			}
		}
		if depth != 0 || token.kind != exprIdentifier || !isCaseKeyword(token.text) || token.caseKeyword {
			continue
		}
		if i > 0 && string(tokens[i-1].text) == "@" {
			continue
		}
		return expressionShapeError(token, "reserved CASE keyword in expression")
	}
	return nil
}

func splitExpressionClauses(tokens []exprToken) [][]exprToken {
	clauses := make([][]exprToken, 0, 1)
	start, depth := 0, 0
	for i, token := range tokens {
		if token.kind == exprSymbol {
			switch string(token.text) {
			case "(", "[":
				depth++
			case ")", "]":
				if depth > 0 {
					depth--
				}
			case ",":
				if depth == 0 {
					clauses = append(clauses, tokens[start:i])
					start = i + 1
				}
			}
		}
	}
	return append(clauses, tokens[start:])
}

func nestedExpressionComma(tokens []exprToken) error {
	callStack := make([]bool, 0, 4)
	callDepth := 0
	for i, token := range tokens {
		if token.kind != exprSymbol {
			continue
		}
		switch string(token.text) {
		case "(", "[":
			call := string(token.text) == "(" && i > 0 && tokens[i-1].kind == exprIdentifier
			callStack = append(callStack, call)
			if call {
				callDepth++
			}
		case ")", "]":
			if len(callStack) > 0 {
				if callStack[len(callStack)-1] {
					callDepth--
				}
				callStack = callStack[:len(callStack)-1]
			}
		case ",":
			if len(callStack) > 0 && callDepth == 0 {
				return expressionShapeError(token, "nested expression contains a comma")
			}
		}
	}
	return nil
}

func nestedCaseKeyword(tokens []exprToken) error {
	depth := 0
	for i, token := range tokens {
		if token.kind == exprSymbol {
			switch string(token.text) {
			case "(", "[":
				depth++
			case ")", "]":
				if depth > 0 {
					depth--
				}
			}
		}
		if depth > 0 && token.kind == exprIdentifier && isCaseKeyword(token.text) && (i == 0 || string(tokens[i-1].text) != "@") {
			return expressionShapeError(token, "reserved CASE keyword in expression")
		}
	}
	return nil
}
