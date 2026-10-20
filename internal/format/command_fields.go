package format

func commandFieldHasMandatoryMarker(src []byte) bool {
	return commandFieldMarker(src, false)
}

func commandFieldHasDeferredMarker(src []byte) bool {
	return commandFieldMarker(src, true)
}

func commandFieldMarker(src []byte, deferred bool) bool {
	for i := 0; i < len(src); {
		if next, atom := syntaxAtomEnd(src, i, len(src)); atom {
			if next < 0 {
				return false
			}
			i = next
			continue
		}
		if deferred {
			if executableMarker(src, i) && !mandatoryInlineMarker(src, i) || tripleMarker(src, i, ';', '^') {
				return true
			}
		} else if mandatoryInlineMarker(src, i) {
			return true
		}
		i++
	}
	return false
}

func tripleMarker(src []byte, pos int, first, second byte) bool {
	return pos+3 <= len(src) && src[pos] == first && src[pos+1] == second && src[pos+2] == ';'
}

func commandPrefixParts(src []byte, parts []syntaxSpan, max, min int, allowFullwidthSpace, allowTrailing bool) ([]syntaxSpan, bool) {
	if len(parts) == 0 {
		return parts, false
	}
	last := parts[len(parts)-1]
	trailing := trimInputLeft(src, last.start, last.end, allowFullwidthSpace) >= trimInputRight(src, last.start, last.end, allowFullwidthSpace)
	if allowTrailing && trailing && len(parts)-1 >= min {
		parts = parts[:len(parts)-1]
	}
	if len(parts) <= max {
		return parts, false
	}
	return parts[:max], true
}

func (p *formatter) ignoredCommandArguments(i, offset int, command string) {
	p.warning(i, offset, "ignored_arguments", command+" has runtime-ignored arguments")
}

func builtinArrayRole(tokens []exprToken) (known, valid bool) {
	tokens = stripCommandMarkers(tokens)
	info, ok := builtinVariableInfo(tokens)
	if !ok {
		return false, true
	}
	return true, info.rank == 1
}

func stripCommandMarkers(tokens []exprToken) []exprToken {
	if len(tokens) == 0 {
		return make([]exprToken, 0)
	}
	for _, token := range tokens {
		if token.kind == exprMarker {
			filtered := make([]exprToken, 0, len(tokens))
			for _, token := range tokens {
				if token.kind != exprMarker {
					filtered = append(filtered, token)
				}
			}
			return filtered
		}
	}
	return tokens
}

func (p *formatter) commandFieldTokens(left, right int) ([]exprToken, error) {
	tokens, err := p.lexSyntaxRange(left, right)
	if err != nil {
		return nil, err
	}
	return stripCommandMarkers(tokens), nil
}

func commandFieldHasUnpairedTernaryHash(tokens []exprToken) bool {
	ternary := 0
	for _, token := range tokens {
		if token.kind != exprOperator {
			continue
		}
		switch string(token.text) {
		case "?":
			ternary++
		case "#":
			if ternary == 0 {
				return true
			}
			ternary--
		}
	}
	return false
}

func (p *formatter) commandFieldHasUnpairedTernaryHashAt(left, right int) bool {
	tokens, err := p.commandFieldTokens(left, right)
	return err == nil && commandFieldHasUnpairedTernaryHash(tokens)
}

func commandFieldVariableArgument(p *formatter, left, right int) bool {
	tokens, err := p.commandFieldTokens(left, right)
	return err == nil && variableArgumentTokens(tokens)
}

func commandFieldKeyword(p *formatter, left, right int, keyword string) bool {
	tokens, err := p.commandFieldTokens(left, right)
	return err == nil && len(tokens) == 1 && tokens[0].kind == exprIdentifier && string(tokens[0].text) == keyword
}
