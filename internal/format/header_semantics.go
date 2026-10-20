package format

import (
	"bytes"
	"strconv"
)

type headerArgumentIssue struct {
	offset  int
	message string
}

type headerArgumentDiagnostic struct {
	offset   int
	severity Severity
	code     string
	message  string
}

type headerArgumentRole struct {
	value      knownValueType
	canDefault bool
	known      bool
}

type headerIntegerStatus uint8

const (
	headerIntegerValid headerIntegerStatus = iota
	headerIntegerInvalid
	headerIntegerUnknown
)

type headerBuiltinDefaultKind uint8

const (
	headerBuiltinDefaultUnknown headerBuiltinDefaultKind = iota
	headerBuiltinDefaultConstant
	headerBuiltinDefaultRuntime
)

type headerBuiltinExpression struct {
	value   knownValueType
	valid   bool
	proven  bool
	builtin bool
	runtime bool
}

func (p *formatter) headerArgumentsValid(i int, head []byte, base int) bool {
	if !p.expressionParseEnabled() {
		return true
	}
	session := p.syntaxSession
	if session == nil || !bytes.Equal(session.sources[expressionSourceOriginal], head) {
		session = p.newExpressionSession(head)
	}
	start, end, ok := headerArgumentRangeWithSession(head, session)
	if !ok {
		return true
	}
	for _, diagnostic := range headerArgumentDiagnosticsWithSession(head, start, end, session, p.dialect) {
		if diagnostic.severity == Warning {
			p.warning(i, base+diagnostic.offset, diagnostic.code, diagnostic.message)
			continue
		}
		p.syntaxProblem(i, base+diagnostic.offset, diagnostic.message)
		return false
	}
	return true
}

func headerArgumentRangeWithSession(src []byte, session *expressionSession) (int, int, bool) {
	if len(src) < 2 || src[0] != '@' {
		return 0, 0, false
	}
	start := trimInputLeftWithSession(src, 1, len(src), session)
	nameEnd, valid := labelIdentifierEndWithSession(src, start, session)
	if !valid {
		return 0, 0, false
	}
	pos := skipHeaderSpaceWithSession(src, nameEnd, session)
	if pos >= len(src) || src[pos] == ';' {
		return 0, 0, false
	}
	if src[pos] == '[' {
		close, result := scanHeaderDelimiterWithSession(src, pos, session)
		if result.status != headerValid {
			return 0, 0, false
		}
		pos = skipHeaderSpaceWithSession(src, close+1, session)
		if pos >= len(src) || src[pos] == ';' {
			return 0, 0, false
		}
	}
	switch src[pos] {
	case '(':
		close, result := scanHeaderDelimiterWithSession(src, pos, session)
		if result.status != headerValid {
			return 0, 0, false
		}
		return pos + 1, close, true
	case ',':
		end, result := scanHeaderCodeEndWithSession(src, pos+1, session)
		if result.status != headerValid {
			return 0, 0, false
		}
		return pos + 1, end, true
	default:
		return 0, 0, false
	}
}

func headerArgumentDiagnosticsWithSession(src []byte, start, end int, session *expressionSession, dialect Dialect) []headerArgumentDiagnostic {
	spans := splitHeaderTopLevelWithSession(src, start, end, ',', session)
	diagnostics := make([]headerArgumentDiagnostic, 0, 1)
	identities := make(map[string]struct{})
	for index, span := range spans {
		left, right := trimHeaderSpanWithSession(src, span.start, span.end, session)
		if left == right {
			if len(spans) == 1 || index == len(spans)-1 && index > 0 {
				continue
			}
			return append(diagnostics, headerArgumentDiagnostic{offset: left, severity: Unsupported, code: "unsupported_syntax", message: "function signature contains an empty argument"})
		}

		equal, hasDefault := topHeaderEqualWithSession(src, left, right, session)
		argStart, argEnd := left, right
		defaultStart, defaultEnd := 0, 0
		if hasDefault {
			argStart, argEnd = trimHeaderSpanWithSession(src, left, equal, session)
			defaultStart, defaultEnd = trimHeaderSpanWithSession(src, equal+1, right, session)
		}
		role, issue := headerArgumentRoleWithSession(argStart, argEnd, session, dialect)
		if issue.message != "" {
			return append(diagnostics, headerArgumentDiagnostic{offset: issue.offset, severity: Unsupported, code: "unsupported_syntax", message: issue.message})
		}
		if !role.known {
			return diagnostics
		}
		if identity, ok := headerArgumentIdentity(argStart, argEnd, session, dialect); ok {
			if _, duplicate := identities[identity]; duplicate {
				diagnostics = append(diagnostics, headerArgumentDiagnostic{offset: argStart, severity: Warning, code: "duplicate_parameter", message: "function parameter is duplicated: " + identity})
			} else {
				identities[identity] = struct{}{}
			}
		}
		if hasDefault {
			if !role.canDefault {
				return append(diagnostics, headerArgumentDiagnostic{offset: argStart, severity: Unsupported, code: "unsupported_syntax", message: "function argument defaults are allowed only for ARG, ARGS, or private variables"})
			}
			if defaultStart == defaultEnd {
				return append(diagnostics, headerArgumentDiagnostic{offset: defaultStart, severity: Unsupported, code: "unsupported_syntax", message: "function signature contains an empty default"})
			}
			tokens, err := session.lexRange(expressionSourceOriginal, defaultStart, defaultEnd, defaultStart)
			if err != nil || containsHeaderUncertainty(tokens) {
				return diagnostics
			}
			if !expressionNeedsDeferral(tokens, dialect) {
				if shapeErr := provenExpressionShape(tokens, dialect, false); shapeErr != nil {
					offset := defaultStart
					message := shapeErr.Error()
					if expressionErr, ok := shapeErr.(*exprError); ok {
						offset = expressionErr.offset
						message = expressionErr.msg
					}
					return append(diagnostics, headerArgumentDiagnostic{offset: offset, severity: Unsupported, code: "unsupported_syntax", message: message})
				}
			}
			value, valid, known := constantExpressionType(tokens)
			if !known {
				proof := headerBuiltinExpressionProof(tokens, dialect)
				if !proof.proven {
					return diagnostics
				}
				if proof.runtime {
					return append(diagnostics, headerArgumentDiagnostic{offset: defaultStart, severity: Unsupported, code: "unsupported_syntax", message: "function argument default must be a constant expression"})
				}
				value, valid, known = proof.value, proof.valid, true
			}
			if !valid {
				return append(diagnostics, headerArgumentDiagnostic{offset: defaultStart, severity: Unsupported, code: "unsupported_syntax", message: "function argument default must be a constant expression"})
			}
			if role.value != knownValueUnknown && value != role.value {
				return append(diagnostics, headerArgumentDiagnostic{offset: defaultStart, severity: Unsupported, code: "unsupported_syntax", message: "function argument default type does not match its variable"})
			}
			if value == knownValueNumeric {
				switch headerIntegerEvaluation(tokens) {
				case headerIntegerInvalid:
					if proof := headerBuiltinExpressionProof(tokens, dialect); proof.proven && proof.builtin && !proof.runtime {
						return diagnostics
					}
					return append(diagnostics, headerArgumentDiagnostic{offset: defaultStart, severity: Unsupported, code: "unsupported_syntax", message: "function argument default must be a valid numeric constant"})
				case headerIntegerUnknown:
					if proof := headerBuiltinExpressionProof(tokens, dialect); !proof.proven || !proof.builtin || proof.runtime {
						return diagnostics
					}
				}
			}
		}
	}
	return diagnostics
}

func headerArgumentRoleWithSession(start, end int, session *expressionSession, dialect Dialect) (headerArgumentRole, headerArgumentIssue) {
	tokens, err := session.lexRange(expressionSourceOriginal, start, end, start)
	if err != nil || containsHeaderUncertainty(tokens) {
		return headerArgumentRole{}, headerArgumentIssue{}
	}
	trimmed := stripExpressionParens(tokens)
	if _, _, known := constantExpressionType(trimmed); known {
		return headerArgumentRole{}, headerArgumentIssue{offset: start, message: "function argument must be an assignable variable"}
	}
	if len(trimmed) > 0 && trimmed[0].kind == exprOperator && string(trimmed[0].text) == "+" && variableArgumentTokens(trimmed) {
		trimmed = stripExpressionParens(trimmed[1:])
	}

	name, indexes, hasIndexes, ok := headerVariableParts(trimmed)
	if !ok {
		if headerExpressionIdentifiersKnown(trimmed, dialect) {
			return headerArgumentRole{}, headerArgumentIssue{offset: start, message: "function argument must be an assignable variable"}
		}
		return headerArgumentRole{}, headerArgumentIssue{}
	}
	info, known := builtinVariableInfoByName(name, dialect)
	if !known {
		return headerArgumentRole{}, headerArgumentIssue{}
	}

	role := headerArgumentRole{value: info.value, canDefault: name == "ARG" || name == "ARGS", known: true}
	if info.readOnly {
		return role, headerArgumentIssue{offset: start, message: "function argument must be an assignable variable"}
	}
	if name != "ARG" && name != "ARGS" && !variableArgumentTokens(trimmed) {
		if headerExpressionIdentifiersKnown(trimmed, dialect) {
			return role, headerArgumentIssue{offset: start, message: "function argument must be an assignable variable"}
		}
		return headerArgumentRole{}, headerArgumentIssue{}
	}
	if !hasIndexes {
		if info.rank > 1 {
			return role, headerArgumentIssue{offset: start, message: "function argument has the wrong number of indexes"}
		}
		if info.character {
			return role, headerArgumentIssue{offset: start, message: "function argument has the wrong number of indexes"}
		}
		return role, headerArgumentIssue{}
	}
	for _, index := range indexes {
		if commandMacroPossibleForDialect(index, dialect) {
			return headerArgumentRole{}, headerArgumentIssue{}
		}
	}
	requiredIndexes := int(info.rank)
	if info.character {
		if len(indexes) == requiredIndexes {
			return role, headerArgumentIssue{offset: start, message: "function argument has the wrong number of indexes"}
		}
		requiredIndexes++
	}
	if len(indexes) != requiredIndexes {
		return role, headerArgumentIssue{offset: start, message: "function argument has the wrong number of indexes"}
	}
	for _, index := range indexes {
		if issue := headerArgumentIndexIssueWithDialect(index, dialect); issue.message != "" {
			return role, issue
		}
		if headerArgumentIndexUncertain(index) {
			return headerArgumentRole{}, headerArgumentIssue{}
		}
	}
	return role, headerArgumentIssue{}
}

func headerExpressionIdentifiersKnown(tokens []exprToken, dialect Dialect) bool {
	if len(tokens) == 0 || containsHeaderUncertainty(tokens) {
		return false
	}
	for index, token := range tokens {
		if token.kind != exprIdentifier {
			continue
		}
		if index >= 2 && string(tokens[index-1].text) == "@" && tokens[index-2].kind == exprIdentifier {
			return false
		}
		name := string(token.text)
		if isCaseKeyword(token.text) {
			continue
		}
		if _, known := builtinVariableInfoByName(name, dialect); known {
			continue
		}
		if mask, known := builtinMethodMasks[name]; known && methodDialect(mask, dialect) {
			continue
		}
		if reservedInstructions[name] {
			continue
		}
		return false
	}
	return true
}

func headerArgumentIdentity(start, end int, session *expressionSession, dialect Dialect) (string, bool) {
	tokens, err := session.lexRange(expressionSourceOriginal, start, end, start)
	if err != nil || containsHeaderUncertainty(tokens) {
		return "", false
	}
	tokens = stripExpressionParens(tokens)
	name, indexes, hasIndexes, ok := headerVariableParts(tokens)
	if !ok {
		return "", false
	}
	info, known := builtinVariableInfoByName(name, dialect)
	if !known {
		return "", false
	}
	if !hasIndexes {
		if info.rank == 0 {
			return name, true
		}
		identity := name
		for index := 0; index < int(info.rank); index++ {
			identity += ":0"
		}
		return identity, true
	}
	requiredIndexes := int(info.rank)
	if info.character {
		requiredIndexes++
	}
	if len(indexes) != requiredIndexes {
		return "", false
	}
	identity := name
	for _, index := range indexes {
		value, known := constantIntegerValue(index)
		if !known {
			return "", false
		}
		identity += ":" + strconv.FormatInt(value, 10)
	}
	return identity, true
}

func headerBuiltinDefaultKindForName(name string, dialect Dialect) (headerBuiltinDefaultKind, builtinVariable, bool) {
	info, known := builtinVariableInfoByName(name, dialect)
	if !known || info.headerDefault == headerBuiltinDefaultUnknown {
		return headerBuiltinDefaultUnknown, builtinVariable{}, false
	}
	return info.headerDefault, info, true
}

func headerBuiltinAtom(tokens []exprToken, start int, dialect Dialect) (headerBuiltinDefaultKind, knownValueType, int, bool) {
	if start < 0 || start >= len(tokens) || tokens[start].kind != exprIdentifier {
		return headerBuiltinDefaultUnknown, knownValueUnknown, start, false
	}
	kind, info, known := headerBuiltinDefaultKindForName(string(tokens[start].text), dialect)
	if !known {
		return headerBuiltinDefaultUnknown, knownValueUnknown, start, false
	}
	end := start + 1
	if end < len(tokens) && (string(tokens[end].text) == "(" || string(tokens[end].text) == "[" || string(tokens[end].text) == "@") {
		return headerBuiltinDefaultUnknown, knownValueUnknown, start, false
	}
	if end < len(tokens) && string(tokens[end].text) == ":" {
		maxIndexes := int(info.rank)
		if info.character {
			maxIndexes++
		}
		for indexes := 0; end < len(tokens) && string(tokens[end].text) == ":"; indexes++ {
			if indexes >= maxIndexes {
				return headerBuiltinDefaultUnknown, knownValueUnknown, start, false
			}
			if end >= len(tokens) || string(tokens[end].text) != ":" || end+1 >= len(tokens) {
				return headerBuiltinDefaultUnknown, knownValueUnknown, start, false
			}
			_, valueKind := directIntegerLiteral(tokens[end+1 : end+2])
			if valueKind != knownValueNumeric {
				return headerBuiltinDefaultUnknown, knownValueUnknown, start, false
			}
			end += 2
		}
	}
	if end < len(tokens) && string(tokens[end].text) == ":" {
		return headerBuiltinDefaultUnknown, knownValueUnknown, start, false
	}
	return kind, info.value, end, true
}

func headerBuiltinExpressionProof(tokens []exprToken, dialect Dialect) headerBuiltinExpression {
	if len(tokens) == 0 || len(tokens) > 4096 || commandMacroPossibleForDialect(tokens, dialect) {
		return headerBuiltinExpression{}
	}
	typed := make([]exprToken, 0, len(tokens))
	builtin := false
	runtime := false
	for index := 0; index < len(tokens); {
		token := tokens[index]
		switch token.kind {
		case exprIdentifier:
			kind, value, end, known := headerBuiltinAtom(tokens, index, dialect)
			if !known {
				return headerBuiltinExpression{}
			}
			if kind == headerBuiltinDefaultRuntime {
				runtime = true
			}
			builtin = true
			token.kind = exprNumber
			if value == knownValueString {
				token.kind = exprString
			}
			typed = append(typed, token)
			index = end
		case exprNumber, exprString:
			typed = append(typed, token)
			index++
		case exprOperator:
			op := string(token.text)
			if constantPriority(op) == 0 && op != "!" && op != "~" {
				return headerBuiltinExpression{}
			}
			typed = append(typed, token)
			index++
		case exprSymbol:
			if string(token.text) != "(" && string(token.text) != ")" {
				return headerBuiltinExpression{}
			}
			typed = append(typed, token)
			index++
		default:
			return headerBuiltinExpression{}
		}
	}
	value, valid, known := constantExpressionType(typed)
	if !known {
		return headerBuiltinExpression{}
	}
	return headerBuiltinExpression{value: value, valid: valid, proven: true, builtin: builtin, runtime: runtime}
}

func headerVariableParts(tokens []exprToken) (name string, indexes [][]exprToken, hasIndexes, ok bool) {
	if len(tokens) == 0 || tokens[0].kind != exprIdentifier {
		return "", nil, false, false
	}
	name = string(tokens[0].text)
	if len(tokens) == 1 {
		return name, nil, false, true
	}
	if string(tokens[1].text) != ":" {
		return "", nil, false, false
	}
	start, depth := 2, 0
	for i := 2; i <= len(tokens); i++ {
		if i < len(tokens) {
			switch string(tokens[i].text) {
			case "(", "[":
				depth++
			case ")", "]":
				if depth > 0 {
					depth--
				}
			case ":":
				if depth != 0 {
					continue
				}
			}
		}
		if i == len(tokens) || string(tokens[i].text) == ":" && depth == 0 {
			if start == i {
				return "", nil, false, false
			}
			indexes = append(indexes, tokens[start:i])
			start = i + 1
		}
	}
	return name, indexes, true, true
}

func headerArgumentIndexIssueWithDialect(tokens []exprToken, dialect Dialect) headerArgumentIssue {
	if containsHeaderUncertainty(tokens) {
		return headerArgumentIssue{}
	}
	value, valid, known := constantExpressionType(tokens)
	if !known {
		proof := headerBuiltinExpressionProof(tokens, dialect)
		if !proof.proven {
			return headerArgumentIssue{}
		}
		if proof.runtime {
			return headerArgumentIssue{offset: tokens[0].off, message: "function argument indexes must be numeric constants"}
		}
		value, valid, known = proof.value, proof.valid, true
	}
	if valid && value == knownValueString {
		return headerArgumentIssue{}
	}
	if !valid || value != knownValueNumeric {
		return headerArgumentIssue{offset: tokens[0].off, message: "function argument indexes must be numeric constants"}
	}
	_, ok := constantIntegerValue(tokens)
	if !ok {
		if proof := headerBuiltinExpressionProof(tokens, dialect); proof.proven && proof.builtin && !proof.runtime {
			return headerArgumentIssue{}
		}
		switch headerIntegerEvaluation(tokens) {
		case headerIntegerInvalid:
			return headerArgumentIssue{offset: tokens[0].off, message: "function argument index must be a valid numeric constant"}
		case headerIntegerUnknown:
			return headerArgumentIssue{}
		}
	}
	return headerArgumentIssue{}
}

func headerIntegerEvaluation(tokens []exprToken) headerIntegerStatus {
	if len(tokens) > 4096 {
		return headerIntegerUnknown
	}
	if _, ok := constantIntegerValue(tokens); ok {
		return headerIntegerValid
	}
	for _, token := range tokens {
		if token.kind == exprString {
			return headerIntegerUnknown
		}
	}
	return headerIntegerInvalid
}

func headerArgumentIndexUncertain(tokens []exprToken) bool {
	if containsHeaderUncertainty(tokens) {
		return true
	}
	value, valid, known := constantExpressionType(tokens)
	if !known {
		return true
	}
	return valid && (value == knownValueString || value == knownValueNumeric && headerIntegerEvaluation(tokens) == headerIntegerUnknown)
}

func containsHeaderUncertainty(tokens []exprToken) bool {
	for _, token := range tokens {
		if token.kind == exprForm || token.kind == exprMarker {
			return true
		}
	}
	return false
}
