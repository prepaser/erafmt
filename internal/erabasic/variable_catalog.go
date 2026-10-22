package erabasic

import "strings"

type BuiltinVariable struct {
	Value         KnownValueType
	Rank          uint8
	Character     bool
	ReadOnly      bool
	Dialects      uint8
	HeaderDefault headerBuiltinDefaultKind
}

var builtinVariables = func() map[string]BuiltinVariable {
	variables := make(map[string]BuiltinVariable)
	for name := byte('A'); name <= 'Z'; name++ {
		variables[string(name)] = BuiltinVariable{Value: KnownValueNumeric, Rank: 1, HeaderDefault: headerBuiltinDefaultRuntime}
	}
	for _, group := range []struct {
		names string
		info  BuiltinVariable
	}{
		{"ARG LOCAL", BuiltinVariable{Value: KnownValueNumeric, Rank: 1, HeaderDefault: headerBuiltinDefaultRuntime}},
		{"ARGS LOCALS", BuiltinVariable{Value: KnownValueString, Rank: 1, HeaderDefault: headerBuiltinDefaultRuntime}},
		{
			"ASSI ASSIPLAY BOUGHT COUNT DAY DOWN EJAC EXPLV FLAG GLOBAL ITEM ITEMSALES LOSEBASE MASTER MONEY NEXTCOM NOITEM PALAMLV PBAND PLAYER PREVCOM RANDDATA RESULT SELECTCOM TFLAG TARGET TIME UP",
			BuiltinVariable{Value: KnownValueNumeric, Rank: 1, HeaderDefault: headerBuiltinDefaultRuntime},
		},
		{"RAND", BuiltinVariable{Value: KnownValueNumeric, Rank: 1, ReadOnly: true}},
		{"GLOBALS RESULTS SAVESTR STR TSTR", BuiltinVariable{Value: KnownValueString, Rank: 1, HeaderDefault: headerBuiltinDefaultRuntime}},
		{"SAVEDATA_TEXT", BuiltinVariable{Value: KnownValueString, HeaderDefault: headerBuiltinDefaultRuntime}},
		{"ISASSI NO", BuiltinVariable{Value: KnownValueNumeric, Character: true, HeaderDefault: headerBuiltinDefaultRuntime}},
		{
			"ABL BASE CDOWN CFLAG CUP DOWNBASE EQUIP EX EXP GOTJUEL JUEL MARK MAXBASE NOWEX PALAM RELATION SOURCE STAIN TALENT TCVAR TEQUIP",
			BuiltinVariable{Value: KnownValueNumeric, Rank: 1, Character: true, HeaderDefault: headerBuiltinDefaultRuntime},
		},
		{"CDFLAG", BuiltinVariable{Value: KnownValueNumeric, Rank: 2, Character: true, HeaderDefault: headerBuiltinDefaultRuntime}},
		{"CALLNAME MASTERNAME NAME NICKNAME", BuiltinVariable{Value: KnownValueString, Character: true, HeaderDefault: headerBuiltinDefaultRuntime}},
		{"CSTR", BuiltinVariable{Value: KnownValueString, Rank: 1, Character: true, HeaderDefault: headerBuiltinDefaultRuntime}},
		{"DA DB DC DD DE DITEMTYPE", BuiltinVariable{Value: KnownValueNumeric, Rank: 2, HeaderDefault: headerBuiltinDefaultRuntime}},
		{"TA TB", BuiltinVariable{Value: KnownValueNumeric, Rank: 3, HeaderDefault: headerBuiltinDefaultRuntime}},
		{"ITEMPRICE", BuiltinVariable{Value: KnownValueNumeric, Rank: 1, ReadOnly: true, HeaderDefault: headerBuiltinDefaultConstant}},
		{
			"ABLNAME TALENTNAME EXPNAME MARKNAME PALAMNAME ITEMNAME TRAINNAME BASENAME SOURCENAME EXNAME EQUIPNAME TEQUIPNAME FLAGNAME TFLAGNAME CFLAGNAME TCVARNAME CSTRNAME STAINNAME CDFLAGNAME1 CDFLAGNAME2 STRNAME TSTRNAME SAVESTRNAME GLOBALNAME GLOBALSNAME",
			BuiltinVariable{Value: KnownValueString, Rank: 1, ReadOnly: true, HeaderDefault: headerBuiltinDefaultConstant},
		},
		{"GAMEBASE_AUTHER GAMEBASE_AUTHOR GAMEBASE_INFO GAMEBASE_YEAR GAMEBASE_TITLE", BuiltinVariable{Value: KnownValueString, ReadOnly: true, HeaderDefault: headerBuiltinDefaultConstant}},
		{
			"GAMEBASE_GAMECODE GAMEBASE_VERSION GAMEBASE_ALLOWVERSION GAMEBASE_DEFAULTCHARA GAMEBASE_NOITEM __INT_MAX__ __INT_MIN__ __LINE__",
			BuiltinVariable{Value: KnownValueNumeric, ReadOnly: true, HeaderDefault: headerBuiltinDefaultConstant},
		},
		{"CHARANUM LASTLOAD_VERSION LASTLOAD_NO LINECOUNT ISTIMEOUT", BuiltinVariable{Value: KnownValueNumeric, ReadOnly: true, HeaderDefault: headerBuiltinDefaultRuntime}},
		{"LASTLOAD_TEXT", BuiltinVariable{Value: KnownValueString, ReadOnly: true, HeaderDefault: headerBuiltinDefaultRuntime}},
		{"MONEYLABEL DRAWLINESTR EMUERA_VERSION __FILE__ __FUNCTION__", BuiltinVariable{Value: KnownValueString, ReadOnly: true, HeaderDefault: headerBuiltinDefaultConstant}},
		{"WINDOW_TITLE", BuiltinVariable{Value: KnownValueString, HeaderDefault: headerBuiltinDefaultRuntime}},
	} {
		for _, name := range strings.Fields(group.names) {
			variables[name] = group.info
		}
	}
	return variables
}()
var modernBuiltinVariables = map[string]BuiltinVariable{
	"DAYNAME":              {Value: KnownValueString, Rank: 1, ReadOnly: true, Dialects: dialectEMEEMask, HeaderDefault: headerBuiltinDefaultConstant},
	"TIMENAME":             {Value: KnownValueString, Rank: 1, ReadOnly: true, Dialects: dialectEMEEMask, HeaderDefault: headerBuiltinDefaultConstant},
	"MONEYNAME":            {Value: KnownValueString, Rank: 1, ReadOnly: true, Dialects: dialectEMEEMask, HeaderDefault: headerBuiltinDefaultConstant},
	"GAMEBASE_URL":         {Value: KnownValueString, ReadOnly: true, Dialects: dialectEMEEMask, HeaderDefault: headerBuiltinDefaultConstant},
	"GAMEBASE_VERSIONNAME": {Value: KnownValueString, ReadOnly: true, Dialects: dialectEMEEMask, HeaderDefault: headerBuiltinDefaultConstant},
}

var builtinStringIndexPositions = func() map[string]uint8 {
	positions := make(map[string]uint8)
	add := func(mask uint8, names string) {
		for _, name := range strings.Fields(names) {
			positions[name] = mask
		}
	}
	add(1<<0, "ITEM ITEMSALES ITEMPRICE LOSEBASE UP DOWN TRAINNAME FLAG TFLAG STR TSTR SAVESTR GLOBAL GLOBALS CDFLAGNAME1 CDFLAGNAME2")
	add(1<<1, "ABL EXP TALENT MARK PALAM JUEL GOTJUEL CUP CDOWN BASE MAXBASE DOWNBASE SOURCE EX NOWEX EQUIP TEQUIP CFLAG TCVAR CSTR STAIN RELATION")
	add((1<<1)|(1<<2), "CDFLAG")
	add(1<<0, "DAY TIME MONEY")
	return positions
}()

func builtinStringIndexAllowed(name string, info BuiltinVariable, sourceIndex, sourceCount int, dialect Dialect) bool {
	if dialect != EMEE && (name == "DAY" || name == "TIME" || name == "MONEY") {
		return false
	}
	mask, known := builtinStringIndexPositions[name]
	if !known {
		return false
	}
	index := sourceIndex
	if info.Character && info.Rank == 1 && sourceCount == 1 {
		index++
	}
	return mask&(1<<index) != 0
}

func builtinVariableInfo(tokens []ExprToken) (BuiltinVariable, bool) {
	return BuiltinVariableInfoForDialect(tokens, CommonDialect)
}

func BuiltinVariableInfoForDialect(tokens []ExprToken, dialect Dialect) (BuiltinVariable, bool) {
	if len(tokens) != 1 || tokens[0].Kind != ExprIdentifier {
		return BuiltinVariable{}, false
	}
	return BuiltinVariableInfoByName(string(tokens[0].Text), dialect)
}

func BuiltinVariableInfoByName(name string, dialect Dialect) (BuiltinVariable, bool) {
	if info, known := builtinVariables[name]; known {
		return info, true
	}
	info, known := modernBuiltinVariables[name]
	if !known || !registeredDialect(info.Dialects, dialect) {
		return BuiltinVariable{}, false
	}
	return info, true
}

func BuiltinVariableRoleInfo(tokens []ExprToken, dialect Dialect) (BuiltinVariable, bool) {
	tokens = StripCommandMarkers(tokens)
	if len(tokens) == 0 || len(tokens) > 256 {
		return BuiltinVariable{}, false
	}
	tokens = stripExpressionParens(tokens)
	if len(tokens) > 0 && tokens[0].Kind == ExprOperator && string(tokens[0].Text) == "+" {
		tokens = stripExpressionParens(tokens[1:])
	}
	if !VariableArgumentTokens(tokens) {
		return BuiltinVariable{}, false
	}
	name, exact := VariableSystemName(tokens)
	if !exact {
		return BuiltinVariable{}, false
	}
	info, known := BuiltinVariableInfoByName(name, dialect)
	if !known {
		return BuiltinVariable{}, false
	}
	if len(tokens) == 1 {
		return info, true
	}
	if value, valid, proven := indexedBuiltinType(tokens, dialect); proven {
		if valid && value == info.Value {
			return info, true
		}
		return BuiltinVariable{}, false
	}
	if len(tokens) < 3 || string(tokens[1].Text) != ":" {
		return BuiltinVariable{}, false
	}
	end, value, proven, invalid := directBuiltinIndexAtom(tokens, 0, dialect)
	if end == len(tokens) && proven && !invalid && value == info.Value {
		return info, true
	}
	return BuiltinVariable{}, false
}

func BuiltinVariableIdentity(tokens []ExprToken, dialect Dialect) (string, bool) {
	tokens = StripCommandMarkers(tokens)
	if !VariableArgumentTokens(tokens) {
		return "", false
	}
	tokens = stripExpressionParens(tokens)
	if len(tokens) > 0 && tokens[0].Kind == ExprOperator && string(tokens[0].Text) == "+" {
		tokens = stripExpressionParens(tokens[1:])
	}
	name, exact := VariableSystemName(tokens)
	if !exact {
		return "", false
	}
	info, known := BuiltinVariableInfoByName(name, dialect)
	if !known {
		return "", false
	}
	if len(tokens) == 1 {
		return name, true
	}
	if info.Rank == 0 || string(tokens[1].Text) != ":" {
		return "", false
	}
	start, depth, indexes := 2, 0, 0
	for i := 2; i <= len(tokens); i++ {
		if i < len(tokens) {
			switch string(tokens[i].Text) {
			case "(":
				depth++
			case ")":
				if depth > 0 {
					depth--
				}
			case ":":
				if depth != 0 {
					continue
				}
			}
		}
		if i != len(tokens) && (string(tokens[i].Text) != ":" || depth != 0) {
			continue
		}
		if start == i {
			return "", false
		}
		index := tokens[start:i]
		if _, kind := DirectIntegerLiteral(index); kind != KnownValueNumeric && !builtinIndexExpressionProven(index, dialect) {
			return "", false
		}
		indexes++
		start = i + 1
	}
	if indexes != int(info.Rank) {
		return "", false
	}
	return name, true
}

func builtinIndexExpressionProven(tokens []ExprToken, dialect Dialect) bool {
	tokens = stripExpressionParens(tokens)
	if len(tokens) == 0 || CommandMacroPossibleForDialect(tokens, dialect) {
		return false
	}
	return KnownExpressionTypeForDialect(tokens, dialect) == KnownValueNumeric
}

func BuiltinVariableReadOnly(tokens []ExprToken, dialect Dialect) bool {
	name, proven := BuiltinVariableIdentity(tokens, dialect)
	if !proven {
		return false
	}
	info, known := BuiltinVariableInfoByName(name, dialect)
	return known && info.ReadOnly
}

func knownVariableType(tokens []ExprToken) KnownValueType {
	tokens = stripExpressionParens(tokens)
	if info, known := builtinVariableInfo(tokens); known {
		return info.Value
	}
	if name, exact := VariableSystemName(tokens); exact {
		switch name {
		case "RESULT", "RESULTS", "CFLAG", "CSTR":
			return builtinVariables[name].Value
		}
	}
	return KnownValueUnknown
}
