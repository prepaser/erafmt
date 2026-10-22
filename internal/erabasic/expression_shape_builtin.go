package erabasic

func provenOrdinaryExpressionShape(tokens []ExprToken, dialect Dialect) error {
	if !expressionShapeIdentifiersKnown(tokens, dialect) {
		return nil
	}
	if err := adjacentExpressionOperands(tokens, dialect); err != nil {
		return err
	}
	if err := nestedExpressionComma(tokens); err != nil {
		return err
	}
	if err := stringBuiltinPostfix(tokens, dialect); err != nil {
		return err
	}
	for _, token := range tokens {
		if token.Kind != ExprOperator || string(token.Text) != "=" {
			continue
		}
		return expressionShapeError(token, "assignment operator in expression")
	}
	for i := 0; i < len(tokens); {
		end, qualified, indexed, known := knownBuiltinAtomEnd(tokens, i, dialect)
		if !known {
			i++
			continue
		}
		if qualified {
			return expressionShapeError(tokens[i+1], "non-local builtin variable reference")
		}
		if end < len(tokens) {
			if indexed {
				switch string(tokens[end].Text) {
				case "(":
					return expressionShapeError(tokens[end], "builtin variable cannot be called")
				case "[":
					return expressionShapeError(tokens[end], "builtin variable cannot use bracket indexing")
				case ".":
					return expressionShapeError(tokens[end], "builtin variable namespace is not supported")
				}
			}
			if ExpressionWordLike(tokens[end]) {
				return expressionShapeError(tokens[end], "adjacent operands in expression")
			}
		}
		i = end
	}
	return nil
}

func adjacentExpressionOperands(tokens []ExprToken, dialect Dialect) error {
	if !expressionShapeIdentifiersKnown(tokens, dialect) {
		return nil
	}
	for i := 1; i < len(tokens); i++ {
		if ExpressionWordLike(tokens[i-1]) && ExpressionWordLike(tokens[i]) {
			return expressionShapeError(tokens[i], "adjacent operands in expression")
		}
	}
	return nil
}

func stringBuiltinPostfix(tokens []ExprToken, dialect Dialect) error {
	for start := range tokens {
		end, qualified, indexed, known := knownBuiltinAtomEnd(tokens, start, dialect)
		if !known || qualified || end <= start || end > len(tokens) {
			continue
		}
		if start+1 < end && string(tokens[start+1].Text) == "@" {
			continue
		}
		if !indexed && end < len(tokens) && string(tokens[end].Text) == ":" {
			if info, ok := BuiltinVariableInfoByName(string(tokens[start].Text), dialect); ok && info.Character {
				candidate := end
				for candidate < len(tokens) && string(tokens[candidate].Text) == ":" {
					if candidate+1 >= len(tokens) || tokens[candidate+1].Kind != exprNumber && tokens[candidate+1].Kind != exprString {
						break
					}
					candidate += 2
				}
				if candidate > end {
					end, indexed = candidate, true
				}
			}
		}
		value, valid := knownBuiltinAtomValue(tokens[start:end], dialect)
		if !valid || value != KnownValueString {
			continue
		}
		if end < len(tokens) && tokens[end].Kind == ExprOperator && isIncrementOperator(tokens[end]) {
			return expressionShapeError(tokens[end], "string variable cannot use a postfix increment")
		}
		if start > 0 && tokens[start-1].Kind == ExprOperator && isIncrementOperator(tokens[start-1]) {
			return expressionShapeError(tokens[start-1], "string variable cannot use a prefix increment")
		}
	}
	return nil
}

func knownBuiltinAtomValue(tokens []ExprToken, dialect Dialect) (KnownValueType, bool) {
	if len(tokens) == 0 || tokens[0].Kind != ExprIdentifier {
		return KnownValueUnknown, false
	}
	if len(tokens) > 1 && string(tokens[1].Text) == ":" {
		value, valid, proven := indexedBuiltinType(tokens, dialect)
		return value, proven && valid
	}
	info, known := BuiltinVariableInfoByName(string(tokens[0].Text), dialect)
	return info.Value, known
}

func expressionShapeIdentifiersKnown(tokens []ExprToken, dialect Dialect) bool {
	for i, token := range tokens {
		if token.Kind != ExprIdentifier || IsCaseKeyword(token.Text) {
			continue
		}
		if _, known := BuiltinVariableInfoByName(string(token.Text), dialect); !known {
			if i >= 2 && string(tokens[i-1].Text) == "@" && tokens[i-2].Kind == ExprIdentifier {
				name := string(tokens[i-2].Text)
				if _, builtin := BuiltinVariableInfoByName(name, dialect); builtin && name != "ARG" && name != "ARGS" && name != "LOCAL" && name != "LOCALS" {
					continue
				}
			}
			return false
		}
	}
	return true
}

// knownBuiltinAtomEnd returns the end of a direct builtin variable reference.
// The bool result is true for a qualified non-local reference, which is a
// known syntax error even though the qualifier itself is not a builtin name.
func knownBuiltinAtomEnd(tokens []ExprToken, start int, dialect Dialect) (end int, qualified, indexed, known bool) {
	if start < 0 || start >= len(tokens) || tokens[start].Kind != ExprIdentifier {
		return start + 1, false, false, false
	}
	name := string(tokens[start].Text)
	info, known := BuiltinVariableInfoByName(name, dialect)
	if !known {
		return start + 1, false, false, false
	}
	end = start + 1
	if end < len(tokens) && string(tokens[end].Text) == "@" {
		if end+1 < len(tokens) && tokens[end+1].Kind == ExprIdentifier {
			end += 2
			if end < len(tokens) && string(tokens[end].Text) == ":" && end+1 < len(tokens) && (tokens[end+1].Kind == exprNumber || tokens[end+1].Kind == exprString) {
				if name != "ARG" && name != "ARGS" && name != "LOCAL" && name != "LOCALS" {
					return end + 2, true, true, true
				}
				return end + 2, false, true, true
			}
			if name != "ARG" && name != "ARGS" && name != "LOCAL" && name != "LOCALS" {
				return end, true, false, true
			}
			return end, false, false, true
		}
		return end, false, false, true
	}
	if info.Rank == 0 && !info.Character || end >= len(tokens) || string(tokens[end].Text) != ":" {
		return end, false, false, true
	}
	for end < len(tokens) && string(tokens[end].Text) == ":" {
		if end+1 < len(tokens) && string(tokens[end+1].Text) == "(" {
			close, matched := boundedExpressionMatchingParen(tokens, end+1)
			if !matched || CommandMacroPossibleForDialect(tokens[end+1:close+1], dialect) || containsExpressionForm(tokens[end+1:close+1]) {
				return end, false, false, true
			}
			end = close + 1
			continue
		}
		if end+1 >= len(tokens) || tokens[end+1].Kind != exprNumber && tokens[end+1].Kind != exprString {
			return end, false, false, true
		}
		end += 2
	}
	return end, false, true, true
}
