package format

func provenOrdinaryExpressionShape(tokens []exprToken, dialect Dialect) error {
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
		if token.kind != exprOperator || string(token.text) != "=" {
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
				switch string(tokens[end].text) {
				case "(":
					return expressionShapeError(tokens[end], "builtin variable cannot be called")
				case "[":
					return expressionShapeError(tokens[end], "builtin variable cannot use bracket indexing")
				case ".":
					return expressionShapeError(tokens[end], "builtin variable namespace is not supported")
				}
			}
			if expressionWordLike(tokens[end]) {
				return expressionShapeError(tokens[end], "adjacent operands in expression")
			}
		}
		i = end
	}
	return nil
}

func adjacentExpressionOperands(tokens []exprToken, dialect Dialect) error {
	if !expressionShapeIdentifiersKnown(tokens, dialect) {
		return nil
	}
	for i := 1; i < len(tokens); i++ {
		if expressionWordLike(tokens[i-1]) && expressionWordLike(tokens[i]) {
			return expressionShapeError(tokens[i], "adjacent operands in expression")
		}
	}
	return nil
}

func stringBuiltinPostfix(tokens []exprToken, dialect Dialect) error {
	for start := range tokens {
		end, qualified, indexed, known := knownBuiltinAtomEnd(tokens, start, dialect)
		if !known || qualified || end <= start || end > len(tokens) {
			continue
		}
		if start+1 < end && string(tokens[start+1].text) == "@" {
			continue
		}
		if !indexed && end < len(tokens) && string(tokens[end].text) == ":" {
			if info, ok := builtinVariableInfoByName(string(tokens[start].text), dialect); ok && info.character {
				candidate := end
				for candidate < len(tokens) && string(tokens[candidate].text) == ":" {
					if candidate+1 >= len(tokens) || tokens[candidate+1].kind != exprNumber && tokens[candidate+1].kind != exprString {
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
		if !valid || value != knownValueString {
			continue
		}
		if end < len(tokens) && tokens[end].kind == exprOperator && isIncrementOperator(tokens[end]) {
			return expressionShapeError(tokens[end], "string variable cannot use a postfix increment")
		}
		if start > 0 && tokens[start-1].kind == exprOperator && isIncrementOperator(tokens[start-1]) {
			return expressionShapeError(tokens[start-1], "string variable cannot use a prefix increment")
		}
	}
	return nil
}

func knownBuiltinAtomValue(tokens []exprToken, dialect Dialect) (knownValueType, bool) {
	if len(tokens) == 0 || tokens[0].kind != exprIdentifier {
		return knownValueUnknown, false
	}
	if len(tokens) > 1 && string(tokens[1].text) == ":" {
		value, valid, proven := indexedBuiltinType(tokens, dialect)
		return value, proven && valid
	}
	info, known := builtinVariableInfoByName(string(tokens[0].text), dialect)
	return info.value, known
}

func expressionShapeIdentifiersKnown(tokens []exprToken, dialect Dialect) bool {
	for i, token := range tokens {
		if token.kind != exprIdentifier || isCaseKeyword(token.text) {
			continue
		}
		if _, known := builtinVariableInfoByName(string(token.text), dialect); !known {
			if i >= 2 && string(tokens[i-1].text) == "@" && tokens[i-2].kind == exprIdentifier {
				name := string(tokens[i-2].text)
				if _, builtin := builtinVariableInfoByName(name, dialect); builtin && name != "ARG" && name != "ARGS" && name != "LOCAL" && name != "LOCALS" {
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
func knownBuiltinAtomEnd(tokens []exprToken, start int, dialect Dialect) (end int, qualified, indexed, known bool) {
	if start < 0 || start >= len(tokens) || tokens[start].kind != exprIdentifier {
		return start + 1, false, false, false
	}
	name := string(tokens[start].text)
	info, known := builtinVariableInfoByName(name, dialect)
	if !known {
		return start + 1, false, false, false
	}
	end = start + 1
	if end < len(tokens) && string(tokens[end].text) == "@" {
		if end+1 < len(tokens) && tokens[end+1].kind == exprIdentifier {
			end += 2
			if end < len(tokens) && string(tokens[end].text) == ":" && end+1 < len(tokens) && (tokens[end+1].kind == exprNumber || tokens[end+1].kind == exprString) {
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
	if info.rank == 0 && !info.character || end >= len(tokens) || string(tokens[end].text) != ":" {
		return end, false, false, true
	}
	for end < len(tokens) && string(tokens[end].text) == ":" {
		if end+1 < len(tokens) && string(tokens[end+1].text) == "(" {
			close, matched := boundedExpressionMatchingParen(tokens, end+1)
			if !matched || commandMacroPossibleForDialect(tokens[end+1:close+1], dialect) || containsExpressionForm(tokens[end+1:close+1]) {
				return end, false, false, true
			}
			end = close + 1
			continue
		}
		if end+1 >= len(tokens) || tokens[end+1].kind != exprNumber && tokens[end+1].kind != exprString {
			return end, false, false, true
		}
		end += 2
	}
	return end, false, true, true
}
