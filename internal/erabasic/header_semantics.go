package erabasic

import "strconv"

type headerArgumentIssue struct {
	Offset  int
	Message string
}

type declarationDiagnosticSeverity uint8

const (
	DeclarationDiagnosticWarning declarationDiagnosticSeverity = iota
	declarationDiagnosticUnsupported
)

type declarationDiagnostic struct {
	Offset   int
	Severity declarationDiagnosticSeverity
	Code     string
	Message  string
}

type headerArgumentDiagnostic struct {
	Offset   int
	Severity Severity
	Code     string
	Message  string
}

type headerArgumentRole struct {
	Value      KnownValueType
	CanDefault bool
	Known      bool
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
	Value   KnownValueType
	Valid   bool
	Proven  bool
	Builtin bool
	Runtime bool
}

func HeaderArgumentRangeWithSession(src []byte, session *ExpressionSession) (int, int, bool) {
	if len(src) < 2 || src[0] != '@' {
		return 0, 0, false
	}
	start := TrimInputLeftWithSession(src, 1, len(src), session)
	label := AnalyzeLabel(src, '@', start, session)
	nameEnd := label.End
	if !label.Valid {
		return 0, 0, false
	}
	pos := skipHeaderSpaceWithSession(src, nameEnd, session)
	if pos >= len(src) || src[pos] == ';' {
		return 0, 0, false
	}
	if src[pos] == '[' {
		close, result := scanHeaderDelimiterWithSession(src, pos, session)
		if result.Status != headerValid {
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
		if result.Status != headerValid {
			return 0, 0, false
		}
		return pos + 1, close, true
	case ',':
		end, result := scanHeaderCodeEndWithSession(src, pos+1, session)
		if result.Status != headerValid {
			return 0, 0, false
		}
		return pos + 1, end, true
	default:
		return 0, 0, false
	}
}

func AnalyzeHeaderArgumentsWithSession(src []byte, start, end int, session *ExpressionSession, dialect Dialect) []declarationDiagnostic {
	spans := splitHeaderTopLevelWithSession(src, start, end, ',', session)
	diagnostics := make([]declarationDiagnostic, 0, 1)
	identities := make(map[string]struct{})
	for index, span := range spans {
		left, right := trimHeaderSpanWithSession(src, span.Start, span.End, session)
		if left == right {
			if len(spans) == 1 || index == len(spans)-1 && index > 0 {
				continue
			}
			return append(diagnostics, declarationDiagnostic{Offset: left, Severity: declarationDiagnosticUnsupported, Code: "unsupported_syntax", Message: "function signature contains an empty argument"})
		}
		equal, hasDefault := topHeaderEqualWithSession(src, left, right, session)
		argStart, argEnd := left, right
		defaultStart, defaultEnd := 0, 0
		if hasDefault {
			argStart, argEnd = trimHeaderSpanWithSession(src, left, equal, session)
			defaultStart, defaultEnd = trimHeaderSpanWithSession(src, equal+1, right, session)
		}
		role, issue := headerArgumentRoleWithSession(argStart, argEnd, session, dialect)
		if issue.Message != "" {
			return append(diagnostics, declarationDiagnostic{Offset: issue.Offset, Severity: declarationDiagnosticUnsupported, Code: "unsupported_syntax", Message: issue.Message})
		}
		if !role.Known {
			return diagnostics
		}
		if identity, ok := headerArgumentIdentity(argStart, argEnd, session, dialect); ok {
			if _, duplicate := identities[identity]; duplicate {
				diagnostics = append(diagnostics, declarationDiagnostic{Offset: argStart, Severity: DeclarationDiagnosticWarning, Code: "duplicate_parameter", Message: "function parameter is duplicated: " + identity})
			} else {
				identities[identity] = struct{}{}
			}
		}
		if hasDefault {
			if !role.CanDefault {
				return append(diagnostics, declarationDiagnostic{Offset: argStart, Severity: declarationDiagnosticUnsupported, Code: "unsupported_syntax", Message: "function argument defaults are allowed only for ARG, ARGS, or private variables"})
			}
			if defaultStart == defaultEnd {
				return append(diagnostics, declarationDiagnostic{Offset: defaultStart, Severity: declarationDiagnosticUnsupported, Code: "unsupported_syntax", Message: "function signature contains an empty default"})
			}
			tokens, err := session.LexRange(ExpressionSourceOriginal, defaultStart, defaultEnd, defaultStart)
			if err != nil || containsHeaderUncertainty(tokens) {
				return diagnostics
			}
			if !ExpressionNeedsDeferral(tokens, dialect) {
				if shapeErr := ProvenExpressionShape(tokens, dialect, false); shapeErr != nil {
					offset := defaultStart
					message := shapeErr.Error()
					if expressionErr, ok := shapeErr.(*ExprError); ok {
						offset = expressionErr.Offset
						message = expressionErr.Msg
					}
					return append(diagnostics, declarationDiagnostic{Offset: offset, Severity: declarationDiagnosticUnsupported, Code: "unsupported_syntax", Message: message})
				}
			}
			value, valid, known := constantExpressionType(tokens)
			if !known {
				proof := headerBuiltinExpressionProof(tokens, dialect)
				if !proof.Proven {
					return diagnostics
				}
				if proof.Runtime {
					return append(diagnostics, declarationDiagnostic{Offset: defaultStart, Severity: declarationDiagnosticUnsupported, Code: "unsupported_syntax", Message: "function argument default must be a constant expression"})
				}
				value, valid, known = proof.Value, proof.Valid, true
			}
			if !valid {
				return append(diagnostics, declarationDiagnostic{Offset: defaultStart, Severity: declarationDiagnosticUnsupported, Code: "unsupported_syntax", Message: "function argument default must be a constant expression"})
			}
			if role.Value != KnownValueUnknown && value != role.Value {
				return append(diagnostics, declarationDiagnostic{Offset: defaultStart, Severity: declarationDiagnosticUnsupported, Code: "unsupported_syntax", Message: "function argument default type does not match its variable"})
			}
			if value == KnownValueNumeric {
				switch headerIntegerEvaluation(tokens) {
				case headerIntegerInvalid:
					if proof := headerBuiltinExpressionProof(tokens, dialect); proof.Proven && proof.Builtin && !proof.Runtime {
						return diagnostics
					}
					return append(diagnostics, declarationDiagnostic{Offset: defaultStart, Severity: declarationDiagnosticUnsupported, Code: "unsupported_syntax", Message: "function argument default must be a valid numeric constant"})
				case headerIntegerUnknown:
					if proof := headerBuiltinExpressionProof(tokens, dialect); !proof.Proven || !proof.Builtin || proof.Runtime {
						return diagnostics
					}
				}
			}
		}
	}
	return diagnostics
}

func HeaderArgumentDiagnosticsWithSession(src []byte, start, end int, session *ExpressionSession, dialect Dialect) []headerArgumentDiagnostic {
	analysis := AnalyzeHeaderArgumentsWithSession(src, start, end, session, dialect)
	diagnostics := make([]headerArgumentDiagnostic, len(analysis))
	for i, diagnostic := range analysis {
		severity := Unsupported
		if diagnostic.Severity == DeclarationDiagnosticWarning {
			severity = Warning
		}
		diagnostics[i] = headerArgumentDiagnostic{Offset: diagnostic.Offset, Severity: severity, Code: diagnostic.Code, Message: diagnostic.Message}
	}
	return diagnostics
}

func headerArgumentRoleWithSession(start, end int, session *ExpressionSession, dialect Dialect) (headerArgumentRole, headerArgumentIssue) {
	tokens, err := session.LexRange(ExpressionSourceOriginal, start, end, start)
	if err != nil || containsHeaderUncertainty(tokens) {
		return headerArgumentRole{}, headerArgumentIssue{}
	}
	trimmed := stripExpressionParens(tokens)
	if _, _, known := constantExpressionType(trimmed); known {
		return headerArgumentRole{}, headerArgumentIssue{Offset: start, Message: "function argument must be an assignable variable"}
	}
	if len(trimmed) > 0 && trimmed[0].Kind == ExprOperator && string(trimmed[0].Text) == "+" && VariableArgumentTokens(trimmed) {
		trimmed = stripExpressionParens(trimmed[1:])
	}
	name, indexes, hasIndexes, ok := headerVariableParts(trimmed)
	if !ok {
		if headerExpressionIdentifiersKnown(trimmed, dialect) {
			return headerArgumentRole{}, headerArgumentIssue{Offset: start, Message: "function argument must be an assignable variable"}
		}
		return headerArgumentRole{}, headerArgumentIssue{}
	}
	info, known := BuiltinVariableInfoByName(name, dialect)
	if !known {
		return headerArgumentRole{}, headerArgumentIssue{}
	}
	role := headerArgumentRole{Value: info.Value, CanDefault: name == "ARG" || name == "ARGS", Known: true}
	if info.ReadOnly {
		return role, headerArgumentIssue{Offset: start, Message: "function argument must be an assignable variable"}
	}
	if name != "ARG" && name != "ARGS" && !VariableArgumentTokens(trimmed) {
		if headerExpressionIdentifiersKnown(trimmed, dialect) {
			return role, headerArgumentIssue{Offset: start, Message: "function argument must be an assignable variable"}
		}
		return headerArgumentRole{}, headerArgumentIssue{}
	}
	if !hasIndexes {
		if info.Rank > 1 {
			return role, headerArgumentIssue{Offset: start, Message: "function argument has the wrong number of indexes"}
		}
		if info.Character {
			return role, headerArgumentIssue{Offset: start, Message: "function argument has the wrong number of indexes"}
		}
		return role, headerArgumentIssue{}
	}
	for _, index := range indexes {
		if CommandMacroPossibleForDialect(index, dialect) {
			return headerArgumentRole{}, headerArgumentIssue{}
		}
	}
	requiredIndexes := int(info.Rank)
	if info.Character {
		if len(indexes) == requiredIndexes {
			return role, headerArgumentIssue{Offset: start, Message: "function argument has the wrong number of indexes"}
		}
		requiredIndexes++
	}
	if len(indexes) != requiredIndexes {
		return role, headerArgumentIssue{Offset: start, Message: "function argument has the wrong number of indexes"}
	}
	for _, index := range indexes {
		if issue := headerArgumentIndexIssueWithDialect(index, dialect); issue.Message != "" {
			return role, issue
		}
		if headerArgumentIndexUncertain(index) {
			return headerArgumentRole{}, headerArgumentIssue{}
		}
	}
	return role, headerArgumentIssue{}
}

func headerExpressionIdentifiersKnown(tokens []ExprToken, dialect Dialect) bool {
	if len(tokens) == 0 || containsHeaderUncertainty(tokens) {
		return false
	}
	for index, token := range tokens {
		if token.Kind != ExprIdentifier {
			continue
		}
		if index >= 2 && string(tokens[index-1].Text) == "@" && tokens[index-2].Kind == ExprIdentifier {
			return false
		}
		name := string(token.Text)
		if IsCaseKeyword(token.Text) {
			continue
		}
		if _, known := BuiltinVariableInfoByName(name, dialect); known {
			continue
		}
		if mask, known := builtinMethodMasks[name]; known && methodDialect(mask, dialect) {
			continue
		}
		if ReservedInstruction(name) {
			continue
		}
		return false
	}
	return true
}

func headerArgumentIdentity(start, end int, session *ExpressionSession, dialect Dialect) (string, bool) {
	tokens, err := session.LexRange(ExpressionSourceOriginal, start, end, start)
	if err != nil || containsHeaderUncertainty(tokens) {
		return "", false
	}
	tokens = stripExpressionParens(tokens)
	name, indexes, hasIndexes, ok := headerVariableParts(tokens)
	if !ok {
		return "", false
	}
	info, known := BuiltinVariableInfoByName(name, dialect)
	if !known {
		return "", false
	}
	if !hasIndexes {
		if info.Rank == 0 {
			return name, true
		}
		identity := name
		for index := 0; index < int(info.Rank); index++ {
			identity += ":0"
		}
		return identity, true
	}
	requiredIndexes := int(info.Rank)
	if info.Character {
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

func headerBuiltinDefaultKindForName(name string, dialect Dialect) (headerBuiltinDefaultKind, BuiltinVariable, bool) {
	info, known := BuiltinVariableInfoByName(name, dialect)
	if !known || info.HeaderDefault == headerBuiltinDefaultUnknown {
		return headerBuiltinDefaultUnknown, BuiltinVariable{}, false
	}
	return info.HeaderDefault, info, true
}

func headerBuiltinAtom(tokens []ExprToken, start int, dialect Dialect) (headerBuiltinDefaultKind, KnownValueType, int, bool) {
	if start < 0 || start >= len(tokens) || tokens[start].Kind != ExprIdentifier {
		return headerBuiltinDefaultUnknown, KnownValueUnknown, start, false
	}
	kind, info, known := headerBuiltinDefaultKindForName(string(tokens[start].Text), dialect)
	if !known {
		return headerBuiltinDefaultUnknown, KnownValueUnknown, start, false
	}
	end := start + 1
	if end < len(tokens) && (string(tokens[end].Text) == "(" || string(tokens[end].Text) == "[" || string(tokens[end].Text) == "@") {
		return headerBuiltinDefaultUnknown, KnownValueUnknown, start, false
	}
	if end < len(tokens) && string(tokens[end].Text) == ":" {
		maxIndexes := int(info.Rank)
		if info.Character {
			maxIndexes++
		}
		for indexes := 0; end < len(tokens) && string(tokens[end].Text) == ":"; indexes++ {
			if indexes >= maxIndexes {
				return headerBuiltinDefaultUnknown, KnownValueUnknown, start, false
			}
			if end >= len(tokens) || string(tokens[end].Text) != ":" || end+1 >= len(tokens) {
				return headerBuiltinDefaultUnknown, KnownValueUnknown, start, false
			}
			_, valueKind := DirectIntegerLiteral(tokens[end+1 : end+2])
			if valueKind != KnownValueNumeric {
				return headerBuiltinDefaultUnknown, KnownValueUnknown, start, false
			}
			end += 2
		}
	}
	if end < len(tokens) && string(tokens[end].Text) == ":" {
		return headerBuiltinDefaultUnknown, KnownValueUnknown, start, false
	}
	return kind, info.Value, end, true
}

func headerBuiltinExpressionProof(tokens []ExprToken, dialect Dialect) headerBuiltinExpression {
	if len(tokens) == 0 || len(tokens) > 4096 || CommandMacroPossibleForDialect(tokens, dialect) {
		return headerBuiltinExpression{}
	}
	typed := make([]ExprToken, 0, len(tokens))
	builtin := false
	runtime := false
	for index := 0; index < len(tokens); {
		token := tokens[index]
		switch token.Kind {
		case ExprIdentifier:
			kind, value, end, known := headerBuiltinAtom(tokens, index, dialect)
			if !known {
				return headerBuiltinExpression{}
			}
			if kind == headerBuiltinDefaultRuntime {
				runtime = true
			}
			builtin = true
			token.Kind = exprNumber
			if value == KnownValueString {
				token.Kind = exprString
			}
			typed = append(typed, token)
			index = end
		case exprNumber, exprString:
			typed = append(typed, token)
			index++
		case ExprOperator:
			op := string(token.Text)
			if constantPriority(op) == 0 && op != "!" && op != "~" {
				return headerBuiltinExpression{}
			}
			typed = append(typed, token)
			index++
		case exprSymbol:
			if string(token.Text) != "(" && string(token.Text) != ")" {
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
	return headerBuiltinExpression{Value: value, Valid: valid, Proven: true, Builtin: builtin, Runtime: runtime}
}

func headerVariableParts(tokens []ExprToken) (name string, indexes [][]ExprToken, hasIndexes, ok bool) {
	if len(tokens) == 0 || tokens[0].Kind != ExprIdentifier {
		return "", nil, false, false
	}
	name = string(tokens[0].Text)
	if len(tokens) == 1 {
		return name, nil, false, true
	}
	if string(tokens[1].Text) != ":" {
		return "", nil, false, false
	}
	start, depth := 2, 0
	for i := 2; i <= len(tokens); i++ {
		if i < len(tokens) {
			switch string(tokens[i].Text) {
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
		if i == len(tokens) || string(tokens[i].Text) == ":" && depth == 0 {
			if start == i {
				return "", nil, false, false
			}
			indexes = append(indexes, tokens[start:i])
			start = i + 1
		}
	}
	return name, indexes, true, true
}

func headerArgumentIndexIssueWithDialect(tokens []ExprToken, dialect Dialect) headerArgumentIssue {
	if containsHeaderUncertainty(tokens) {
		return headerArgumentIssue{}
	}
	value, valid, known := constantExpressionType(tokens)
	if !known {
		proof := headerBuiltinExpressionProof(tokens, dialect)
		if !proof.Proven {
			return headerArgumentIssue{}
		}
		if proof.Runtime {
			return headerArgumentIssue{Offset: tokens[0].Off, Message: "function argument indexes must be numeric constants"}
		}
		value, valid, known = proof.Value, proof.Valid, true
	}
	if valid && value == KnownValueString {
		return headerArgumentIssue{}
	}
	if !valid || value != KnownValueNumeric {
		return headerArgumentIssue{Offset: tokens[0].Off, Message: "function argument indexes must be numeric constants"}
	}
	_, ok := constantIntegerValue(tokens)
	if !ok {
		if proof := headerBuiltinExpressionProof(tokens, dialect); proof.Proven && proof.Builtin && !proof.Runtime {
			return headerArgumentIssue{}
		}
		switch headerIntegerEvaluation(tokens) {
		case headerIntegerInvalid:
			return headerArgumentIssue{Offset: tokens[0].Off, Message: "function argument index must be a valid numeric constant"}
		case headerIntegerUnknown:
			return headerArgumentIssue{}
		}
	}
	return headerArgumentIssue{}
}

func headerIntegerEvaluation(tokens []ExprToken) headerIntegerStatus {
	if len(tokens) > 4096 {
		return headerIntegerUnknown
	}
	if _, ok := constantIntegerValue(tokens); ok {
		return headerIntegerValid
	}
	for _, token := range tokens {
		if token.Kind == exprString {
			return headerIntegerUnknown
		}
	}
	return headerIntegerInvalid
}

func headerArgumentIndexUncertain(tokens []ExprToken) bool {
	if containsHeaderUncertainty(tokens) {
		return true
	}
	value, valid, known := constantExpressionType(tokens)
	if !known {
		return true
	}
	return valid && (value == KnownValueString || value == KnownValueNumeric && headerIntegerEvaluation(tokens) == headerIntegerUnknown)
}

func containsHeaderUncertainty(tokens []ExprToken) bool {
	for _, token := range tokens {
		if token.Kind == ExprForm || token.Kind == ExprMarker {
			return true
		}
	}
	return false
}
