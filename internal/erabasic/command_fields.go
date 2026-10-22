package erabasic

func CommandFieldHasMandatoryMarker(src []byte) bool {
	return commandFieldMarker(src, false)
}

func CommandFieldHasDeferredMarker(src []byte) bool {
	return commandFieldMarker(src, true)
}

func commandFieldMarker(src []byte, deferred bool) bool {
	for i := 0; i < len(src); {
		if next, atom := SyntaxAtomEnd(src, i, len(src)); atom {
			if next < 0 {
				return false
			}
			i = next
			continue
		}
		if deferred {
			if ExecutableMarker(src, i) && !MandatoryInlineMarker(src, i) || tripleMarker(src, i, ';', '^') {
				return true
			}
		} else if MandatoryInlineMarker(src, i) {
			return true
		}
		i++
	}
	return false
}

func tripleMarker(src []byte, pos int, first, second byte) bool {
	return pos+3 <= len(src) && src[pos] == first && src[pos+1] == second && src[pos+2] == ';'
}

func CommandPrefixParts(src []byte, parts []SyntaxSpan, max, min int, allowFullwidthSpace, allowTrailing bool) ([]SyntaxSpan, bool) {
	if len(parts) == 0 {
		return parts, false
	}
	last := parts[len(parts)-1]
	trailing := TrimInputLeft(src, last.Start, last.End, allowFullwidthSpace) >= TrimInputRight(src, last.Start, last.End, allowFullwidthSpace)
	if allowTrailing && trailing && len(parts)-1 >= min {
		parts = parts[:len(parts)-1]
	}
	if len(parts) <= max {
		return parts, false
	}
	return parts[:max], true
}

func BuiltinArrayRole(tokens []ExprToken) (known, valid bool) {
	tokens = StripCommandMarkers(tokens)
	info, ok := builtinVariableInfo(tokens)
	if !ok {
		return false, true
	}
	return true, info.Rank == 1
}

func StripCommandMarkers(tokens []ExprToken) []ExprToken {
	if len(tokens) == 0 {
		return make([]ExprToken, 0)
	}
	for _, token := range tokens {
		if token.Kind == ExprMarker {
			filtered := make([]ExprToken, 0, len(tokens))
			for _, token := range tokens {
				if token.Kind != ExprMarker {
					filtered = append(filtered, token)
				}
			}
			return filtered
		}
	}
	return tokens
}

func (p *SemanticContext) CommandFieldTokens(left, right int) ([]ExprToken, error) {
	tokens, err := p.LexSyntaxRange(left, right)
	if err != nil {
		return nil, err
	}
	return StripCommandMarkers(tokens), nil
}

func commandFieldHasUnpairedTernaryHash(tokens []ExprToken) bool {
	ternary := 0
	for _, token := range tokens {
		if token.Kind != ExprOperator {
			continue
		}
		switch string(token.Text) {
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

func (p *SemanticContext) CommandFieldHasUnpairedTernaryHashAt(left, right int) bool {
	tokens, err := p.CommandFieldTokens(left, right)
	return err == nil && commandFieldHasUnpairedTernaryHash(tokens)
}

func (p *SemanticContext) CommandFieldVariableArgument(left, right int) bool {
	tokens, err := p.CommandFieldTokens(left, right)
	return err == nil && VariableArgumentTokens(tokens)
}

func (p *SemanticContext) CommandFieldKeyword(left, right int, keyword string) bool {
	tokens, err := p.CommandFieldTokens(left, right)
	return err == nil && len(tokens) == 1 && tokens[0].Kind == ExprIdentifier && string(tokens[0].Text) == keyword
}
