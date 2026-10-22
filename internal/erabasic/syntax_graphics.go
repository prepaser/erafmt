package erabasic

type GraphicsFieldKind uint8

const (
	graphicsInvalid GraphicsFieldKind = iota
	GraphicsString
	graphicsNumeric
	GraphicsNumericCertain
	GraphicsNumericUnknown
)

type graphicsExpressionGroup struct {
	Open byte
	Call bool
}

func GraphicsEffectiveFields(src []byte, parts []SyntaxSpan, allowFullwidthSpace bool) []SyntaxSpan {
	if len(parts) > 1 {
		last := parts[len(parts)-1]
		left := TrimInputLeft(src, last.Start, last.End, allowFullwidthSpace)
		right := TrimInputRight(src, left, last.End, allowFullwidthSpace)
		if left == right {
			return parts[:len(parts)-1]
		}
	}
	return parts
}

func graphicsExpressionMayBeUnknown(tokens []ExprToken) bool {
	for _, token := range tokens {
		if token.Kind == ExprIdentifier || token.Kind == ExprForm {
			return true
		}
	}
	return false
}

func graphicsKnownType(tokens []ExprToken) KnownValueType {
	for _, token := range tokens {
		if token.Kind == ExprMarker {
			return KnownValueUnknown
		}
	}
	if CommandMacroPossible(tokens) {
		return KnownValueUnknown
	}
	return KnownExpressionType(tokens)
}

func graphicsNumericOperandsValid(tokens []ExprToken) bool {
	groups := make([]graphicsExpressionGroup, 0, 2)
	wantOperand := true
	for _, token := range tokens {
		switch token.Kind {
		case ExprIdentifier, exprNumber:
			wantOperand = false
		case exprString, ExprForm:
			if !graphicsInsideCall(groups) {
				return false
			}
			wantOperand = false
		case ExprOperator:
			op := string(token.Text)
			if (op == "++" || op == "--") && !wantOperand {
				continue
			}
			wantOperand = true
		case exprSymbol:
			switch token.Text[0] {
			case '(', '[':
				groups = append(groups, graphicsExpressionGroup{Open: token.Text[0], Call: token.Text[0] == '(' && !wantOperand})
				wantOperand = true
			case ')', ']':
				if len(groups) == 0 {
					return false
				}
				current := groups[len(groups)-1]
				if (current.Open != '(' || token.Text[0] != ')') && (current.Open != '[' || token.Text[0] != ']') {
					return false
				}
				groups = groups[:len(groups)-1]
				wantOperand = false
			case ',':
				wantOperand = true
			case ':', '@', '.':
				wantOperand = true
			default:
				wantOperand = true
			}
		}
	}
	return len(groups) == 0
}

func graphicsInsideCall(groups []graphicsExpressionGroup) bool {
	for _, current := range groups {
		if current.Call {
			return true
		}
	}
	return false
}

func graphicsConstantSyntax(tokens []ExprToken) bool {
	for _, t := range tokens {
		if t.Kind != exprNumber && t.Kind != ExprOperator && t.Kind != exprSymbol {
			return false
		}
	}
	return true
}

func graphicsExpressionCompleteWithSession(src []byte, left, right int, allowFullwidthSpace bool, session *ExpressionSession, offset, base int) bool {
	tokens, err := session.LexRange(ExpressionSourceOriginal, offset, offset+right-left, base)
	if err != nil || len(tokens) == 0 {
		return false
	}
	strict := graphicsConstantSyntax(tokens)
	type group struct {
		open    byte
		call    bool
		content bool
		ternary int
	}
	var groups []group
	wantOperand := true
	ternary := 0
	for _, token := range tokens {
		switch token.Kind {
		case ExprIdentifier, exprNumber, exprString, ExprForm:
			if !wantOperand {
				return false
			}
			wantOperand = false
			if len(groups) > 0 {
				groups[len(groups)-1].content = true
			}
		case ExprOperator:
			op := string(token.Text)
			if len(groups) > 0 {
				groups[len(groups)-1].content = true
			}
			if wantOperand {
				switch op {
				case "+", "-", "!", "~", "++", "--":
				default:
					return false
				}
				continue
			}
			if op == "++" || op == "--" {
				continue
			}
			if op == "!" || op == "~" {
				return false
			}
			switch op {
			case "+", "-", "*", "/", "%", "==", ">", "<", ">=", "<=", "!=", "&&", "||", "^^", "!&", "!|", "&", "|", "^", ">>", "<<":
				wantOperand = true
			case "?":
				ternary++
				wantOperand = true
			case "#":
				boundary := 0
				if len(groups) > 0 {
					boundary = groups[len(groups)-1].ternary
				}
				if ternary == 0 || strict && ternary <= boundary {
					return false
				}
				ternary--
				wantOperand = true
			default:
				return false
			}
		case exprSymbol:
			switch token.Text[0] {
			case '(', '[':
				if len(groups) > 0 {
					groups[len(groups)-1].content = true
				}
				groups = append(groups, group{open: token.Text[0], call: token.Text[0] == '(' && !wantOperand, ternary: ternary})
				wantOperand = true
			case ')', ']':
				if len(groups) == 0 || (token.Text[0] == ')' && groups[len(groups)-1].open != '(') || (token.Text[0] == ']' && groups[len(groups)-1].open != '[') {
					return false
				}
				current := groups[len(groups)-1]
				if strict && ternary != current.ternary {
					return false
				}
				if wantOperand && !(token.Text[0] == ')' && current.call && !current.content) {
					return false
				}
				groups = groups[:len(groups)-1]
				wantOperand = false
				if len(groups) > 0 {
					groups[len(groups)-1].content = true
				}
			case ',':
				if len(groups) == 0 || groups[len(groups)-1].open != '(' || !groups[len(groups)-1].call || strict && ternary != groups[len(groups)-1].ternary || (wantOperand && groups[len(groups)-1].content) {
					return false
				}
				groups[len(groups)-1].content = false
				wantOperand = true
			case ':', '@':
				if wantOperand {
					return false
				}
				if len(groups) > 0 {
					groups[len(groups)-1].content = true
				}
				wantOperand = true
			default:
				return false
			}
		}
	}
	return !wantOperand && len(groups) == 0 && ternary == 0
}
