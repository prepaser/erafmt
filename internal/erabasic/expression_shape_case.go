package erabasic

import "bytes"

func IsCaseKeyword(text []byte) bool {
	return len(text) == 2 && (bytes.EqualFold(text, []byte("IS")) || bytes.EqualFold(text, []byte("TO")))
}

func provenCaseShape(tokens []ExprToken, dialect Dialect) error {
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
			if len(keywords.To) > 0 {
				if err := provenIncrementOperatorState(clause[:keywords.To[0]], dialect); err != nil {
					return err
				}
				if err := provenIncrementOperatorState(clause[keywords.To[0]+1:], dialect); err != nil {
					return err
				}
			} else if err := provenIncrementOperatorState(clause, dialect); err != nil {
				return err
			}
		}
		if len(keywords.Is) == 0 && len(keywords.To) == 0 {
			if err := provenCaseOperandShape(clause, dialect); err != nil {
				return err
			}
			continue
		}
		if len(keywords.Is) > 1 || len(keywords.To) > 1 {
			duplicate := keywords.Is
			if len(keywords.To) > 1 && (len(duplicate) < 2 || keywords.To[1] < duplicate[1]) {
				duplicate = keywords.To
			}
			return expressionShapeError(clause[duplicate[1]], "duplicate CASE keyword")
		}
		if len(keywords.Is) > 0 {
			is := keywords.Is[0]
			if is != 0 {
				return expressionShapeError(clause[is], "CASE IS must start a clause")
			}
			if len(keywords.To) > 0 {
				return expressionShapeError(clause[keywords.To[0]], "CASE IS cannot contain TO")
			}
			if is+1 >= len(clause) {
				return expressionShapeError(clause[is], "CASE IS requires an operator")
			}
			op := clause[is+1]
			if op.Kind != ExprOperator || !caseBinaryOperator(string(op.Text)) {
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
		to := keywords.To[0]
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

func provenCaseOperandShape(tokens []ExprToken, dialect Dialect) error {
	if len(tokens) == 0 || !expressionShapeIdentifiersKnown(tokens, dialect) {
		return nil
	}
	if tokens[len(tokens)-1].Kind == ExprOperator {
		if !isIncrementOperator(tokens[len(tokens)-1]) {
			return expressionShapeError(tokens[len(tokens)-1], "CASE expression is incomplete")
		}
		if !provenCaseIncrement(tokens, dialect) {
			return expressionShapeError(tokens[len(tokens)-1], "CASE expression has an invalid increment")
		}
	}
	if tokens[0].Kind == ExprOperator {
		switch string(tokens[0].Text) {
		case "+", "-", "!", "~":
		case "++", "--":
			if !provenCaseIncrement(tokens, dialect) {
				return expressionShapeError(tokens[0], "CASE expression has an invalid increment")
			}
		default:
			return expressionShapeError(tokens[0], "CASE expression has an invalid leading operator")
		}
	}
	if err := ProvenKnownExpressionBoundaries(tokens, dialect); err != nil {
		return err
	}
	return provenOrdinaryExpressionShape(tokens, dialect)
}

func provenCaseIncrement(tokens []ExprToken, dialect Dialect) bool {
	prefix := isIncrementOperator(tokens[0])
	postfix := isIncrementOperator(tokens[len(tokens)-1])
	if prefix && postfix {
		return false
	}
	value, valid, proven := builtinIncrementType(tokens, dialect)
	if proven {
		return valid && value == KnownValueNumeric
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
		_, _, known := BuiltinExpressionType(operand, dialect)
		return !known
	}
	return true
}

const (
	maxBuiltinProofTokens = 4096
	maxBuiltinProofDepth  = 256
)

func boundedExpressionMatchingParen(tokens []ExprToken, start int) (int, bool) {
	if start < 0 || start >= len(tokens) || string(tokens[start].Text) != "(" {
		return -1, false
	}
	end := start + maxBuiltinProofTokens
	if end > len(tokens) {
		end = len(tokens)
	}
	depth := 0
	for i := start; i < end; i++ {
		switch string(tokens[i].Text) {
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
	Is []int
	To []int
}

func caseClauseKeywords(tokens []ExprToken) caseClauseKeywordPositions {
	var positions caseClauseKeywordPositions
	depth := 0
	for i, token := range tokens {
		if token.Kind == exprSymbol {
			switch string(token.Text) {
			case "(", "[":
				depth++
			case ")", "]":
				if depth > 0 {
					depth--
				}
			}
		}
		if depth != 0 || token.Kind != ExprIdentifier || !token.CaseKeyword {
			continue
		}
		switch {
		case bytes.EqualFold(token.Text, []byte("IS")):
			positions.Is = append(positions.Is, i)
		case bytes.EqualFold(token.Text, []byte("TO")):
			positions.To = append(positions.To, i)
		}
	}
	return positions
}

func misplacedCaseKeywords(tokens []ExprToken) error {
	depth := 0
	for i, token := range tokens {
		if token.Kind == exprSymbol {
			switch string(token.Text) {
			case "(", "[":
				depth++
			case ")", "]":
				if depth > 0 {
					depth--
				}
			}
		}
		if depth != 0 || token.Kind != ExprIdentifier || !IsCaseKeyword(token.Text) || token.CaseKeyword {
			continue
		}
		if i > 0 && string(tokens[i-1].Text) == "@" {
			continue
		}
		return expressionShapeError(token, "reserved CASE keyword in expression")
	}
	return nil
}

func splitExpressionClauses(tokens []ExprToken) [][]ExprToken {
	clauses := make([][]ExprToken, 0, 1)
	start, depth := 0, 0
	for i, token := range tokens {
		if token.Kind == exprSymbol {
			switch string(token.Text) {
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

func nestedExpressionComma(tokens []ExprToken) error {
	callStack := make([]bool, 0, 4)
	callDepth := 0
	for i, token := range tokens {
		if token.Kind != exprSymbol {
			continue
		}
		switch string(token.Text) {
		case "(", "[":
			call := string(token.Text) == "(" && i > 0 && tokens[i-1].Kind == ExprIdentifier
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

func nestedCaseKeyword(tokens []ExprToken) error {
	depth := 0
	for i, token := range tokens {
		if token.Kind == exprSymbol {
			switch string(token.Text) {
			case "(", "[":
				depth++
			case ")", "]":
				if depth > 0 {
					depth--
				}
			}
		}
		if depth > 0 && token.Kind == ExprIdentifier && IsCaseKeyword(token.Text) && (i == 0 || string(tokens[i-1].Text) != "@") {
			return expressionShapeError(token, "reserved CASE keyword in expression")
		}
	}
	return nil
}
