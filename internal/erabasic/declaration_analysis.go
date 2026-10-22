package erabasic

import (
	"bytes"
	"unicode"
	"unicode/utf8"
)

type declarationLanguageKind uint8

const (
	declarationLocal declarationLanguageKind = iota
	declarationGlobal
)

func DeclarationKind(kind Kind) declarationLanguageKind {
	if kind == ERH {
		return declarationGlobal
	}
	return declarationLocal
}

type DeclarationWordAnalysis struct {
	Name      string
	WordEnd   int
	BodyStart int
}

type declarationIdentifierAnalysis struct {
	Valid  bool
	Name   string
	tokens []ExprToken
}

type declarationInitializerAnalysis struct{ Issue *declarationDiagnostic }
type declarationObjectDefineAnalysis struct {
	WordEnd   int
	NameStart int
	NameEnd   int
	BodyStart int
	Valid     bool
}

func declarationWarning(offset int, message string) *declarationDiagnostic {
	return &declarationDiagnostic{Offset: offset, Severity: DeclarationDiagnosticWarning, Code: "suspicious_initializer", Message: message}
}

type DeclarationModifierAnalysis struct {
	Set   DeclarationModifierSet
	Known bool
	Issue string
}

type DeclarationModifierSet struct {
	IsConst bool
	Dynamic bool
	Static  bool
	Save    bool
	Chara   bool
	Ref     bool
	Global  bool
	Issue   string
}

type localSizeConstantStatus uint8

const (
	localSizeConstantUnknown localSizeConstantStatus = iota
	LocalSizeConstantValid
	LocalSizeConstantInvalid
)

type LabelAnalysis struct {
	start int
	End   int
	Valid bool
	Tail  bool
}

func AsciiWordEnd(src []byte, start int) int {
	for start < len(src) && ((src[start] >= 'A' && src[start] <= 'Z') || (src[start] >= 'a' && src[start] <= 'z')) {
		start++
	}
	return start
}

func AnalyzeDeclarationWord(head []byte, allowFullwidthSpace bool) DeclarationWordAnalysis {
	wordEnd := DeclarationWordEnd(head)
	result := DeclarationWordAnalysis{WordEnd: wordEnd}
	if wordEnd < 0 {
		return result
	}
	result.Name = string(bytes.ToUpper(head[1:wordEnd]))
	result.BodyStart = wordEnd
	for result.BodyStart < len(head) {
		size := InputSpaceSize(head, result.BodyStart, allowFullwidthSpace)
		if size == 0 {
			break
		}
		result.BodyStart += size
	}
	return result
}

func AnalyzeDeclarationIdentifier(tokens []ExprToken) declarationIdentifierAnalysis {
	result := declarationIdentifierAnalysis{tokens: tokens}
	if len(tokens) != 1 || tokens[0].Kind != ExprIdentifier {
		return result
	}
	result.Valid = true
	result.Name = string(tokens[0].Text)
	return result
}

func declarationInitializerType(tokens []ExprToken) KnownValueType {
	value, valid, known := constantExpressionType(tokens)
	if known && valid {
		return value
	}
	return KnownValueUnknown
}

func (c *SemanticContext) DeclarationInitializerFields(start, end int) []SyntaxSpan {
	fields := c.SplitTopLevel(c.Src, start, end, ',')
	if len(fields) > 1 {
		left, right := ExtraFieldBoundsWithSpace(c.Src, fields[len(fields)-1], c.AllowFullwidthSpace)
		if left == right {
			return fields[:len(fields)-1]
		}
	}
	return fields
}

func (c *SemanticContext) DeclarationTopEqual(start, end int) (int, bool) {
	session := c.ScanSession(c.Src, start, end)
	position, ok := TopEqual(c.Src[start:end], 0, end-start, session)
	if !ok {
		return 0, false
	}
	return start + position, true
}

func (c *SemanticContext) DeclarationInitializerDimensionsKnown(spans []SyntaxSpan) bool {
	for _, span := range spans {
		left := TrimInputLeft(c.Src, span.Start, span.End, c.AllowFullwidthSpace)
		right := TrimInputRight(c.Src, left, span.End, c.AllowFullwidthSpace)
		if left == right {
			return false
		}
		tokens, err := c.LexSyntaxRange(left, right)
		if err != nil || CommandMacroPossible(tokens) {
			return false
		}
	}
	return true
}

func (c *SemanticContext) DeclarationInitializerSize(spans []SyntaxSpan) (bool, int64) {
	if len(spans) == 0 {
		return true, 0
	}
	if len(spans) > 1 {
		for _, span := range spans {
			left := TrimInputLeft(c.Src, span.Start, span.End, c.AllowFullwidthSpace)
			right := TrimInputRight(c.Src, left, span.End, c.AllowFullwidthSpace)
			tokens, err := c.LexSyntaxRange(left, right)
			if err != nil || CommandMacroPossible(tokens) {
				return false, 0
			}
			if _, valid := constantIntegerValue(tokens); !valid {
				return false, 0
			}
		}
		return true, 0
	}
	left := TrimInputLeft(c.Src, spans[0].Start, spans[0].End, c.AllowFullwidthSpace)
	right := TrimInputRight(c.Src, left, spans[0].End, c.AllowFullwidthSpace)
	tokens, err := c.LexSyntaxRange(left, right)
	if err != nil || CommandMacroPossible(tokens) {
		return false, 0
	}
	value, valid := constantIntegerValue(tokens)
	return valid, value
}

func (c *SemanticContext) DeclarationInitializerAnalysis(stringType bool, modifiers DeclarationModifierSet, fieldsStart, fieldsEnd, initStart, initEnd int) declarationInitializerAnalysis {
	if modifiers.Ref {
		return declarationInitializerAnalysis{Issue: declarationWarning(initStart, "reference declaration cannot have an initializer")}
	}
	if modifiers.Chara {
		return declarationInitializerAnalysis{Issue: declarationWarning(initStart, "CHARADATA declaration cannot have an initializer")}
	}
	dimensions := c.SplitTopLevel(c.Src, fieldsStart, fieldsEnd, ',')
	if len(dimensions) > 1 && !c.DeclarationInitializerDimensionsKnown(dimensions[1:]) {
		return declarationInitializerAnalysis{}
	}
	initializer := c.DeclarationInitializerFields(initStart, initEnd)
	if len(initializer) == 0 {
		return declarationInitializerAnalysis{Issue: declarationWarning(initStart, "declaration initializer is empty")}
	}
	expected := KnownValueNumeric
	if stringType {
		expected = KnownValueString
	}
	for _, field := range initializer {
		left := TrimInputLeft(c.Src, field.Start, field.End, c.AllowFullwidthSpace)
		right := TrimInputRight(c.Src, left, field.End, c.AllowFullwidthSpace)
		if left == right {
			return declarationInitializerAnalysis{Issue: declarationWarning(field.Start, "initializer contains an empty value")}
		}
		tokens, err := c.LexSyntaxRange(left, right)
		if err != nil || CommandMacroPossible(tokens) {
			return declarationInitializerAnalysis{}
		}
		actual := declarationInitializerType(tokens)
		if actual == KnownValueUnknown {
			return declarationInitializerAnalysis{}
		}
		if actual != expected {
			return declarationInitializerAnalysis{Issue: declarationWarning(left, "initializer type does not match declaration type; macro expansion may change its role")}
		}
	}
	if len(dimensions) > 1 {
		knownSize, size := c.DeclarationInitializerSize(dimensions[1:])
		if !knownSize {
			return declarationInitializerAnalysis{}
		}
		if len(dimensions)-1 >= 2 {
			return declarationInitializerAnalysis{Issue: declarationWarning(initStart, "multi-dimensional declaration cannot have an initializer; macro expansion may change its role")}
		}
		if size > 0 && (int64(len(initializer)) > size || modifiers.IsConst && int64(len(initializer)) != size) {
			return declarationInitializerAnalysis{Issue: declarationWarning(initStart, "initializer count does not match the declaration size; macro expansion may change its role")}
		}
	}
	return declarationInitializerAnalysis{}
}

func (c *SemanticContext) DeclarationMacroNameSpan(span SyntaxSpan) bool {
	left := TrimInputLeft(c.Src, span.Start, span.End, c.AllowFullwidthSpace)
	right := TrimInputRight(c.Src, left, span.End, c.AllowFullwidthSpace)
	tokens, err := c.LexSyntaxRange(left, right)
	if err != nil || len(tokens) < 2 || tokens[0].Kind != ExprIdentifier {
		return false
	}
	for _, token := range tokens {
		if token.Kind != ExprIdentifier {
			return false
		}
	}
	name := string(bytes.ToUpper(tokens[0].Text))
	if setterKnownNameForDialect(tokens[0].Text, c.Dialect) || ReservedInstruction(name) && !CommandUnavailable(name, c.Dialect) {
		return false
	}
	mask, ok := builtinMethodMasks[name]
	return !ok || !methodDialect(mask, c.Dialect)
}

func (c *SemanticContext) DeclarationReservedName(start, end int) bool {
	spans := c.SplitTopLevel(c.Src, start, end, ',')
	if len(spans) == 0 {
		return false
	}
	left := TrimInputLeft(c.Src, spans[0].Start, spans[0].End, c.AllowFullwidthSpace)
	right := TrimInputRight(c.Src, left, spans[0].End, c.AllowFullwidthSpace)
	tokens, err := c.LexSyntaxRange(left, right)
	if err != nil || len(tokens) != 1 || tokens[0].Kind != ExprIdentifier || !bytes.Equal(tokens[0].Text, bytes.ToUpper(tokens[0].Text)) {
		return false
	}
	name := string(tokens[0].Text)
	if ReservedInstruction(name) && !CommandUnavailable(name, c.Dialect) || setterKnownNameForDialect(tokens[0].Text, c.Dialect) {
		return true
	}
	if reservedDeclarationNames[name] {
		return true
	}
	mask, ok := builtinMethodMasks[name]
	return ok && methodDialect(mask, c.Dialect)
}

func (c *SemanticContext) PrivateDeclarationNameMacroPossible(spans []SyntaxSpan) bool {
	if len(spans) == 0 {
		return false
	}
	left := TrimInputLeft(c.Src, spans[0].Start, spans[0].End, c.AllowFullwidthSpace)
	right := TrimInputRight(c.Src, left, spans[0].End, c.AllowFullwidthSpace)
	tokens, err := c.LexSyntaxRange(left, right)
	if err != nil || len(tokens) != 1 || tokens[0].Kind != ExprIdentifier {
		return false
	}
	r, _ := utf8.DecodeRune(tokens[0].Text)
	return r <= 0xffff && unicode.IsDigit(r) && (r < '0' || r > '9')
}

func (c *SemanticContext) DeclarationOmittedDimensionProvenOrdinary(spans []SyntaxSpan, modifiersStart, start int) bool {
	modifiers, known := DeclarationModifierInfo(c.Src, modifiersStart, start, c.AllowFullwidthSpace)
	if known && modifiers.Ref {
		return false
	}
	forcedOrdinary := known && (modifiers.IsConst || modifiers.Static || modifiers.Save || modifiers.Chara || modifiers.Global)
	positive := false
	for _, span := range spans[1:] {
		left := TrimInputLeft(c.Src, span.Start, span.End, c.AllowFullwidthSpace)
		right := TrimInputRight(c.Src, left, span.End, c.AllowFullwidthSpace)
		if left == right {
			continue
		}
		tokens, err := c.LexSyntaxRange(left, right)
		if err != nil || CommandMacroPossible(tokens) {
			return false
		}
		value, valid := constantIntegerValue(tokens)
		if !valid || value <= 0 {
			return false
		}
		positive = true
	}
	return positive || forcedOrdinary
}

func (c *SemanticContext) DeclarationCharadataDimensionsProven(spans []SyntaxSpan) (SyntaxSpan, bool) {
	count := 0
	var third SyntaxSpan
	for _, span := range spans[1:] {
		left := TrimInputLeft(c.Src, span.Start, span.End, c.AllowFullwidthSpace)
		right := TrimInputRight(c.Src, left, span.End, c.AllowFullwidthSpace)
		if left == right {
			return SyntaxSpan{}, false
		}
		tokens, err := c.LexSyntaxRange(left, right)
		if err != nil || CommandMacroPossibleForDialect(tokens, c.Dialect) {
			return SyntaxSpan{}, false
		}
		value, kind := DirectIntegerLiteral(tokens)
		if kind != KnownValueNumeric || value <= 0 {
			return SyntaxSpan{}, false
		}
		count++
		if count == 3 {
			third = span
		}
	}
	return third, count > 2
}

func (c *SemanticContext) DeclarationValuesProven(start, end, initializerStart, initializerEnd int, stringType bool, modifiers DeclarationModifierSet) bool {
	fields := c.SplitTopLevel(c.Src, start, end, ',')
	if len(fields) > 4 {
		return false
	}
	total := int64(1)
	for _, field := range fields[1:] {
		left, right := ExtraFieldBoundsWithSpace(c.Src, field, c.AllowFullwidthSpace)
		if left == right && modifiers.Ref {
			continue
		}
		tokens, err := c.LexSyntaxRange(left, right)
		if err != nil {
			return false
		}
		value, valid := constantIntegerValue(tokens)
		if !valid {
			return false
		}
		if modifiers.Ref {
			if value != 0 {
				return false
			}
			continue
		}
		if value <= 0 || value > 1000000 || total > 1000000/value {
			return false
		}
		total *= value
	}
	if initializerStart < 0 {
		return !modifiers.IsConst
	}
	if modifiers.Ref || modifiers.Chara || len(fields) > 2 {
		return false
	}
	initializer := c.DeclarationInitializerFields(initializerStart, initializerEnd)
	count := int64(len(initializer))
	if count > 1000000 || len(fields) == 2 && (count > total || modifiers.IsConst && count != total) {
		return false
	}
	for _, field := range initializer {
		left, right := ExtraFieldBoundsWithSpace(c.Src, field, c.AllowFullwidthSpace)
		tokens, err := c.LexSyntaxRange(left, right)
		if err != nil {
			return false
		}
		if stringType {
			for len(tokens) >= 3 && string(tokens[0].Text) == "(" && string(tokens[len(tokens)-1].Text) == ")" {
				tokens = tokens[1 : len(tokens)-1]
			}
			if len(tokens) != 1 || tokens[0].Kind != exprString {
				return false
			}
		} else if _, valid := constantIntegerValue(tokens); !valid {
			return false
		}
	}
	return true
}

func (c *SemanticContext) DeclarationLiteralZero(start, end int) bool {
	tokens, err := c.LexSyntaxRange(start, end)
	if err != nil {
		return false
	}
	value, kind := DirectIntegerLiteral(tokens)
	return kind == KnownValueNumeric && value == 0
}

type declarationDimensionAnalysis struct {
	Issues       []declarationDiagnostic
	ValuesProven bool
	Reject       bool
}

func (c *SemanticContext) DeclarationDimensionChecks(spans []SyntaxSpan, modifiersStart, start, end int, allowRefOmitted, expressionEnabled bool, kind declarationLanguageKind, modifiers DeclarationModifierSet, modifiersKnown bool, initializerStart, initializerEnd int, stringType bool) declarationDimensionAnalysis {
	result := declarationDimensionAnalysis{}
	if expressionEnabled && kind == declarationGlobal && modifiersKnown && modifiers.Chara {
		if third, ok := c.DeclarationCharadataDimensionsProven(spans); ok {
			result.Issues = append(result.Issues, declarationDiagnostic{Offset: third.Start, Severity: DeclarationDiagnosticWarning, Code: "suspicious_dimension", Message: "CHARADATA declarations support at most two dimensions; macro expansion may change its role"})
			result.Reject = true
			return result
		}
	}
	skipDirectValidation := declarationHasRefModifier(c.Src, modifiersStart, start, c.AllowFullwidthSpace) && !allowRefOmitted
	if expressionEnabled && !skipDirectValidation {
		total := int64(1)
		dimensionCount := 0
		capacityKnown := true
		for _, span := range spans[1:] {
			left := TrimInputLeft(c.Src, span.Start, span.End, c.AllowFullwidthSpace)
			right := TrimInputRight(c.Src, left, span.End, c.AllowFullwidthSpace)
			if left == right {
				continue
			}
			tokens, err := c.LexSyntaxRange(left, right)
			if err != nil {
				continue
			}
			if CommandMacroPossible(tokens) {
				break
			}
			dimensionCount++
			if !allowRefOmitted && dimensionCount > 3 {
				result.Issues = append(result.Issues, declarationDiagnostic{Offset: left, Severity: DeclarationDiagnosticWarning, Code: "suspicious_dimension", Message: "declaration has more than three dimensions; macro expansion may change its role"})
				result.Reject = true
				return result
			}
			value, valueKind := DirectIntegerLiteral(tokens)
			if valueKind == KnownValueUnknown {
				capacityKnown = false
				continue
			}
			if valueKind == KnownValueString {
				if allowRefOmitted {
					result.Issues = append(result.Issues, declarationDiagnostic{Offset: left, Severity: declarationDiagnosticUnsupported, Code: "unsupported_syntax", Message: "reference declaration dimensions must be omitted or zero"})
				} else {
					result.Issues = append(result.Issues, declarationDiagnostic{Offset: left, Severity: DeclarationDiagnosticWarning, Code: "suspicious_dimension", Message: "string literal in a dimension field; macro expansion may change its role"})
				}
				result.Reject = true
				return result
			}
			if allowRefOmitted {
				if value != 0 {
					result.Issues = append(result.Issues, declarationDiagnostic{Offset: left, Severity: declarationDiagnosticUnsupported, Code: "unsupported_syntax", Message: "reference declaration dimensions must be omitted or zero"})
					result.Reject = true
					return result
				}
				continue
			}
			if value <= 0 || value > 1000000 {
				result.Issues = append(result.Issues, declarationDiagnostic{Offset: left, Severity: DeclarationDiagnosticWarning, Code: "suspicious_dimension", Message: "literal is outside dimension range 1..1000000; macro expansion may change its role"})
				result.Reject = true
				return result
			}
			if capacityKnown && total > 1000000/value {
				result.Issues = append(result.Issues, declarationDiagnostic{Offset: left, Severity: DeclarationDiagnosticWarning, Code: "suspicious_dimension", Message: "declaration exceeds the total dimension capacity of 1000000; macro expansion may change its role"})
				result.Reject = true
				return result
			}
			total *= value
		}
	}
	if c.hasOmittedDimension(spans) {
		if !allowRefOmitted || len(spans)-1 > 3 {
			if expressionEnabled && c.DeclarationOmittedDimensionProvenOrdinary(spans, modifiersStart, start) {
				for _, span := range spans[1:] {
					left := TrimInputLeft(c.Src, span.Start, span.End, c.AllowFullwidthSpace)
					right := TrimInputRight(c.Src, left, span.End, c.AllowFullwidthSpace)
					if left == right {
						result.Issues = append(result.Issues, declarationDiagnostic{Offset: span.Start, Severity: declarationDiagnosticUnsupported, Code: "unsupported_syntax", Message: "ordinary declaration dimensions cannot be omitted"})
						break
					}
				}
			}
			result.Reject = true
			return result
		}
		for _, span := range spans[1:] {
			left := TrimInputLeft(c.Src, span.Start, span.End, c.AllowFullwidthSpace)
			right := TrimInputRight(c.Src, left, span.End, c.AllowFullwidthSpace)
			if left == right {
				continue
			}
			if !c.DeclarationLiteralZero(left, right) {
				result.Reject = true
				return result
			}
		}
	}
	result.ValuesProven = modifiersKnown && declarationModifierIssue(kind, modifiers) == "" && c.DeclarationValuesProven(start, end, initializerStart, initializerEnd, stringType, modifiers)
	return result
}

func (c *SemanticContext) hasOmittedDimension(spans []SyntaxSpan) bool {
	for _, span := range spans[1:] {
		left := TrimInputLeft(c.Src, span.Start, span.End, c.AllowFullwidthSpace)
		right := TrimInputRight(c.Src, left, span.End, c.AllowFullwidthSpace)
		if left >= right {
			return true
		}
	}
	return false
}

func (c *SemanticContext) DeclarationInitializerShapeIssue(start, end int) error {
	tokens, err := c.LexSyntaxRange(start, end)
	if err != nil || ExpressionNeedsDeferral(tokens, c.Dialect) {
		return nil
	}
	return ProvenExpressionShape(tokens, c.Dialect, false)
}

func (c *SemanticContext) ObjectDefineAnalysis(head, lineBody []byte, base int) declarationObjectDefineAnalysis {
	result := declarationObjectDefineAnalysis{}
	if len(head) < len("#DEFINE") || !bytes.Equal(head[:len("#DEFINE")], []byte("#DEFINE")) {
		return result
	}
	if bytes.IndexByte(lineBody, 0) >= 0 || bytes.IndexByte(head, 0) >= 0 || bytes.Contains(head, []byte("[[")) || bytes.Contains(head, []byte(";!;")) || bytes.Contains(head, []byte(";#;")) {
		return result
	}
	result.WordEnd = len("#DEFINE")
	result.NameStart = result.WordEnd
	for result.NameStart < len(head) {
		size := InputSpaceSize(head, result.NameStart, c.AllowFullwidthSpace)
		if size == 0 {
			break
		}
		result.NameStart += size
	}
	if result.NameStart == result.WordEnd || result.NameStart >= len(head) {
		return result
	}
	result.NameEnd = expressionIdentifierEnd(head, result.NameStart)
	if result.NameEnd == result.NameStart || result.NameEnd < 0 {
		return result
	}
	if result.NameEnd < len(head) && head[result.NameEnd] == '(' {
		return result
	}
	if result.NameEnd < len(head) && InputSpaceSize(head, result.NameEnd, c.AllowFullwidthSpace) == 0 {
		return result
	}
	result.BodyStart = result.NameEnd
	for result.BodyStart < len(head) {
		size := InputSpaceSize(head, result.BodyStart, c.AllowFullwidthSpace)
		if size == 0 {
			break
		}
		result.BodyStart += size
	}
	if result.BodyStart == result.NameEnd || result.BodyStart >= len(head) {
		return result
	}
	tokens, err := c.LexSyntaxRange(base+result.NameStart, base+result.NameEnd)
	if err != nil || len(tokens) != 1 || tokens[0].Kind != ExprIdentifier {
		return declarationObjectDefineAnalysis{}
	}
	result.Valid = true
	return result
}

func AnalyzeDeclarationModifiers(src []byte, start, end int, allowFullwidthSpace bool, kind declarationLanguageKind) DeclarationModifierAnalysis {
	set, known := DeclarationModifierInfo(src, start, end, allowFullwidthSpace)
	result := DeclarationModifierAnalysis{Set: set, Known: known}
	if known {
		result.Issue = declarationModifierIssue(kind, set)
	}
	return result
}

func LocalSizeConstantProof(tokens []ExprToken) (localSizeConstantStatus, int64) {
	if len(tokens) == 0 || len(tokens) > 4096 {
		return localSizeConstantUnknown, 0
	}
	depth := 0
	hasString := false
	for _, token := range tokens {
		switch token.Kind {
		case ExprIdentifier, ExprForm, ExprMarker:
			return localSizeConstantUnknown, 0
		case exprString:
			hasString = true
		case exprNumber:
			if _, ok := numericLiteralValue(token.Text); !ok {
				return localSizeConstantUnknown, 0
			}
		case ExprOperator:
			op := string(token.Text)
			if constantPriority(op) == 0 && op != "+" && op != "-" && op != "!" && op != "~" {
				return LocalSizeConstantInvalid, 0
			}
		case exprSymbol:
			switch string(token.Text) {
			case "(":
				depth++
				if depth > 256 {
					return localSizeConstantUnknown, 0
				}
			case ")":
				if depth == 0 {
					return LocalSizeConstantInvalid, 0
				}
				depth--
			default:
				return LocalSizeConstantInvalid, 0
			}
		default:
			return localSizeConstantUnknown, 0
		}
	}
	if depth != 0 {
		return LocalSizeConstantInvalid, 0
	}
	valueType, valid, known := constantExpressionType(tokens)
	if !known || !valid || valueType != KnownValueNumeric {
		return LocalSizeConstantInvalid, 0
	}
	if hasString {
		return localSizeConstantUnknown, 0
	}
	value, ok := constantIntegerValue(tokens)
	if !ok {
		return LocalSizeConstantInvalid, 0
	}
	return LocalSizeConstantValid, value
}

func DeclarationModifierInfo(src []byte, start, end int, allowFullwidthSpace bool) (DeclarationModifierSet, bool) {
	var modifiers DeclarationModifierSet
	for start < end {
		for start < end {
			size := InputSpaceSize(src, start, allowFullwidthSpace)
			if size == 0 {
				break
			}
			start += size
		}
		if start >= end {
			return modifiers, true
		}
		wordEnd := AsciiWordEnd(src, start)
		if wordEnd == start || wordEnd > end {
			return DeclarationModifierSet{}, false
		}
		switch string(bytes.ToUpper(src[start:wordEnd])) {
		case "CONST":
			if modifiers.IsConst {
				modifiers.Issue = "CONST modifier is duplicated"
			}
			modifiers.IsConst = true
		case "DYNAMIC":
			if modifiers.Dynamic {
				modifiers.Issue = "DYNAMIC modifier is duplicated"
			}
			modifiers.Dynamic = true
		case "STATIC":
			if modifiers.Static {
				modifiers.Issue = "STATIC modifier is duplicated"
			}
			modifiers.Static = true
		case "SAVEDATA":
			if modifiers.Save {
				modifiers.Issue = "SAVEDATA modifier is duplicated"
			}
			modifiers.Save = true
		case "CHARADATA":
			if modifiers.Chara {
				modifiers.Issue = "CHARADATA modifier is duplicated"
			}
			modifiers.Chara = true
		case "REF":
			if modifiers.Ref {
				modifiers.Issue = "REF modifier is duplicated"
			}
			modifiers.Ref = true
		case "GLOBAL":
			modifiers.Global = true
		default:
			return DeclarationModifierSet{}, false
		}
		start = wordEnd
	}
	return modifiers, true
}

func declarationModifierIssue(kind declarationLanguageKind, modifiers DeclarationModifierSet) string {
	if modifiers.Issue != "" {
		return modifiers.Issue
	}
	if kind == declarationLocal {
		if modifiers.Global {
			return "GLOBAL is not valid for a local declaration"
		}
		if modifiers.Save {
			return "SAVEDATA is not valid for a local declaration"
		}
		if modifiers.Chara {
			return "CHARADATA is not valid for a local declaration"
		}
	} else {
		if modifiers.Dynamic {
			return "DYNAMIC is not valid for a global declaration"
		}
		if modifiers.Static {
			return "STATIC is not valid for a global declaration"
		}
	}
	if modifiers.IsConst && (modifiers.Dynamic || modifiers.Save || modifiers.Chara || modifiers.Ref || modifiers.Global) {
		return "CONST modifier conflicts with another declaration modifier"
	}
	if modifiers.Ref && (modifiers.Static || modifiers.Save || modifiers.Chara || modifiers.Global || modifiers.IsConst) {
		return "REF modifier conflicts with another declaration modifier"
	}
	if modifiers.Dynamic && modifiers.Static {
		return "DYNAMIC and STATIC modifiers conflict"
	}
	if modifiers.Static && (modifiers.Save || modifiers.Chara || modifiers.Global) {
		return "STATIC modifier conflicts with another declaration modifier"
	}
	if modifiers.Dynamic && modifiers.Chara {
		return "DYNAMIC modifier conflicts with CHARADATA"
	}
	if modifiers.Chara && modifiers.Global {
		return "CHARADATA modifier conflicts with GLOBAL"
	}
	return ""
}

func declarationHasRefModifier(src []byte, start, end int, allowFullwidthSpace bool) bool {
	for start < end {
		for start < end {
			size := InputSpaceSize(src, start, allowFullwidthSpace)
			if size == 0 {
				break
			}
			start += size
		}
		if start >= end {
			return false
		}
		wordEnd := AsciiWordEnd(src, start)
		if wordEnd == start || wordEnd > end {
			return false
		}
		word := string(bytes.ToUpper(src[start:wordEnd]))
		if word == "REF" {
			if wordEnd == end || InputSpaceSize(src, wordEnd, allowFullwidthSpace) != 0 || IsDelimiter(rune(src[wordEnd])) {
				return true
			}
			return false
		}
		if !DeclarationModifier(word) || wordEnd >= end || InputSpaceSize(src, wordEnd, allowFullwidthSpace) == 0 {
			return false
		}
		start = wordEnd
	}
	return false
}

func DeclarationModifiersAllowRefOmissionWithSpace(src []byte, start, end int, allowFullwidthSpace bool) bool {
	hasRef := false
	hasDynamic := false
	for start < end {
		for start < end {
			size := InputSpaceSize(src, start, allowFullwidthSpace)
			if size == 0 {
				break
			}
			start += size
		}
		if start >= end {
			break
		}
		wordEnd := AsciiWordEnd(src, start)
		if wordEnd == start || wordEnd > end {
			return false
		}
		switch string(bytes.ToUpper(src[start:wordEnd])) {
		case "REF":
			if hasRef {
				return false
			}
			hasRef = true
		case "DYNAMIC":
			if hasDynamic {
				return false
			}
			hasDynamic = true
		default:
			return false
		}
		start = wordEnd
	}
	return hasRef
}

func FunctionStyleDefine(head []byte) bool {
	wordEnd := DeclarationWordEnd(head)
	if wordEnd != len("#DEFINE") || !bytes.Equal(head[:wordEnd], []byte("#DEFINE")) {
		return false
	}
	pos := wordEnd
	for pos < len(head) {
		size := InputSpaceSize(head, pos, false)
		if size == 0 {
			break
		}
		pos += size
	}
	nameEnd := expressionIdentifierEnd(head, pos)
	return nameEnd > pos && nameEnd < len(head) && head[nameEnd] == '(' && !bytes.Contains(head, []byte("[["))
}

func DeclarationModifier(word string) bool {
	switch word {
	case "CONST", "DYNAMIC", "STATIC", "SAVEDATA", "CHARADATA", "REF", "GLOBAL":
		return true
	default:
		return false
	}
}

func declarationName(head []byte) string {
	if len(head) < 2 || head[0] != '#' {
		return ""
	}
	end := DeclarationWordEnd(head)
	if end < 0 {
		return ""
	}
	return string(bytes.ToUpper(head[1:end]))
}

func DeclarationWordEnd(head []byte) int {
	if len(head) < 2 || head[0] != '#' {
		return -1
	}
	for pos := 1; pos < len(head); pos++ {
		if HalfSpace(head[pos]) || bytes.HasPrefix(head[pos:], fullwidthSpace) {
			return pos
		}
		switch head[pos] {
		case '.', '+', '-', '*', '/', '%', '=', '!', '<', '>', '|', '&', '^', '~', '#', '?', ')', '}', ']', ',', ':', '(', '{', '[', '$', '\\', '\'', '"', '@', ';':
			return pos
		}
	}
	return len(head)
}

func BareSharpFunction(head []byte, name string, allowFullwidthSpace bool) bool {
	wordEnd := DeclarationWordEnd(head)
	if wordEnd != len(name)+1 || !bytes.Equal(head[:wordEnd], []byte("#"+name)) {
		return false
	}
	return len(TrimInput(head[wordEnd:], allowFullwidthSpace)) == 0
}

func AnalyzeLabel(src []byte, marker byte, start int, session *ExpressionSession) LabelAnalysis {
	result := LabelAnalysis{start: start, End: start}
	if start >= len(src) {
		return result
	}
	first, _ := utf8.DecodeRune(src[start:])
	if isASCIIDigit(first) {
		return result
	}
	end := start
	for end < len(src) {
		r, size := utf8.DecodeRune(src[end:])
		if r == utf8.RuneError && size == 1 {
			return result
		}
		if isDelimiterWithSpace(r, session != nil && session.AllowFullwidthSpace) {
			break
		}
		end += size
	}
	if end == start {
		return result
	}
	if session == nil || !bytes.Equal(session.Sources[ExpressionSourceOriginal], src) {
		session = NewExpressionSessionWithSpace(src, session != nil && session.AllowFullwidthSpace)
	}
	tokens, err := session.LexRange(ExpressionSourceOriginal, start, end, start)
	if err != nil || len(tokens) != 1 || tokens[0].Kind != ExprIdentifier {
		return LabelAnalysis{start: start, End: end}
	}
	result.End = end
	result.Valid = true
	i := TrimInputLeft(src, end, len(src), session.AllowFullwidthSpace)
	if marker == '$' {
		result.Tail = i == len(src) || src[i] == ';'
	} else {
		result.Tail = i == len(src) || src[i] == ';' || src[i] == '[' || src[i] == '(' || src[i] == ','
	}
	return result
}
