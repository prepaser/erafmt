package format

import "strings"

type builtinVariable struct {
	value         knownValueType
	rank          uint8
	character     bool
	readOnly      bool
	dialects      uint8
	headerDefault headerBuiltinDefaultKind
}

var builtinVariables = func() map[string]builtinVariable {
	variables := make(map[string]builtinVariable)
	for name := byte('A'); name <= 'Z'; name++ {
		variables[string(name)] = builtinVariable{value: knownValueNumeric, rank: 1, headerDefault: headerBuiltinDefaultRuntime}
	}
	for _, group := range []struct {
		names string
		info  builtinVariable
	}{
		{"ARG LOCAL", builtinVariable{value: knownValueNumeric, rank: 1, headerDefault: headerBuiltinDefaultRuntime}},
		{"ARGS LOCALS", builtinVariable{value: knownValueString, rank: 1, headerDefault: headerBuiltinDefaultRuntime}},
		{"ASSI ASSIPLAY BOUGHT COUNT DAY DOWN EJAC EXPLV FLAG GLOBAL ITEM ITEMSALES LOSEBASE MASTER MONEY NEXTCOM NOITEM PALAMLV PBAND PLAYER PREVCOM RANDDATA RESULT SELECTCOM TFLAG TARGET TIME UP", builtinVariable{value: knownValueNumeric, rank: 1, headerDefault: headerBuiltinDefaultRuntime}},
		{"RAND", builtinVariable{value: knownValueNumeric, rank: 1, readOnly: true}},
		{"GLOBALS RESULTS SAVESTR STR TSTR", builtinVariable{value: knownValueString, rank: 1, headerDefault: headerBuiltinDefaultRuntime}},
		{"SAVEDATA_TEXT", builtinVariable{value: knownValueString, headerDefault: headerBuiltinDefaultRuntime}},
		{"ISASSI NO", builtinVariable{value: knownValueNumeric, character: true, headerDefault: headerBuiltinDefaultRuntime}},
		{"ABL BASE CDOWN CFLAG CUP DOWNBASE EQUIP EX EXP GOTJUEL JUEL MARK MAXBASE NOWEX PALAM RELATION SOURCE STAIN TALENT TCVAR TEQUIP", builtinVariable{value: knownValueNumeric, rank: 1, character: true, headerDefault: headerBuiltinDefaultRuntime}},
		{"CDFLAG", builtinVariable{value: knownValueNumeric, rank: 2, character: true, headerDefault: headerBuiltinDefaultRuntime}},
		{"CALLNAME MASTERNAME NAME NICKNAME", builtinVariable{value: knownValueString, character: true, headerDefault: headerBuiltinDefaultRuntime}},
		{"CSTR", builtinVariable{value: knownValueString, rank: 1, character: true, headerDefault: headerBuiltinDefaultRuntime}},
		{"DA DB DC DD DE DITEMTYPE", builtinVariable{value: knownValueNumeric, rank: 2, headerDefault: headerBuiltinDefaultRuntime}},
		{"TA TB", builtinVariable{value: knownValueNumeric, rank: 3, headerDefault: headerBuiltinDefaultRuntime}},
		{"ITEMPRICE", builtinVariable{value: knownValueNumeric, rank: 1, readOnly: true, headerDefault: headerBuiltinDefaultConstant}},
		{"ABLNAME TALENTNAME EXPNAME MARKNAME PALAMNAME ITEMNAME TRAINNAME BASENAME SOURCENAME EXNAME EQUIPNAME TEQUIPNAME FLAGNAME TFLAGNAME CFLAGNAME TCVARNAME CSTRNAME STAINNAME CDFLAGNAME1 CDFLAGNAME2 STRNAME TSTRNAME SAVESTRNAME GLOBALNAME GLOBALSNAME", builtinVariable{value: knownValueString, rank: 1, readOnly: true, headerDefault: headerBuiltinDefaultConstant}},
		{"GAMEBASE_AUTHER GAMEBASE_AUTHOR GAMEBASE_INFO GAMEBASE_YEAR GAMEBASE_TITLE", builtinVariable{value: knownValueString, readOnly: true, headerDefault: headerBuiltinDefaultConstant}},
		{"GAMEBASE_GAMECODE GAMEBASE_VERSION GAMEBASE_ALLOWVERSION GAMEBASE_DEFAULTCHARA GAMEBASE_NOITEM __INT_MAX__ __INT_MIN__ __LINE__", builtinVariable{value: knownValueNumeric, readOnly: true, headerDefault: headerBuiltinDefaultConstant}},
		{"CHARANUM LASTLOAD_VERSION LASTLOAD_NO LINECOUNT ISTIMEOUT", builtinVariable{value: knownValueNumeric, readOnly: true, headerDefault: headerBuiltinDefaultRuntime}},
		{"LASTLOAD_TEXT", builtinVariable{value: knownValueString, readOnly: true, headerDefault: headerBuiltinDefaultRuntime}},
		{"MONEYLABEL DRAWLINESTR EMUERA_VERSION __FILE__ __FUNCTION__", builtinVariable{value: knownValueString, readOnly: true, headerDefault: headerBuiltinDefaultConstant}},
		{"WINDOW_TITLE", builtinVariable{value: knownValueString, headerDefault: headerBuiltinDefaultRuntime}},
	} {
		for _, name := range strings.Fields(group.names) {
			variables[name] = group.info
		}
	}
	return variables
}()

var modernBuiltinVariables = map[string]builtinVariable{
	"DAYNAME":              {value: knownValueString, rank: 1, readOnly: true, dialects: dialectEMEEMask, headerDefault: headerBuiltinDefaultConstant},
	"TIMENAME":             {value: knownValueString, rank: 1, readOnly: true, dialects: dialectEMEEMask, headerDefault: headerBuiltinDefaultConstant},
	"MONEYNAME":            {value: knownValueString, rank: 1, readOnly: true, dialects: dialectEMEEMask, headerDefault: headerBuiltinDefaultConstant},
	"GAMEBASE_URL":         {value: knownValueString, readOnly: true, dialects: dialectEMEEMask, headerDefault: headerBuiltinDefaultConstant},
	"GAMEBASE_VERSIONNAME": {value: knownValueString, readOnly: true, dialects: dialectEMEEMask, headerDefault: headerBuiltinDefaultConstant},
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

func builtinStringIndexAllowed(name string, info builtinVariable, sourceIndex, sourceCount int, dialect Dialect) bool {
	if dialect != EMEE && (name == "DAY" || name == "TIME" || name == "MONEY") {
		return false
	}
	mask, known := builtinStringIndexPositions[name]
	if !known {
		return false
	}
	index := sourceIndex
	if info.character && info.rank == 1 && sourceCount == 1 {
		index++
	}
	return mask&(1<<index) != 0
}

func builtinVariableInfo(tokens []exprToken) (builtinVariable, bool) {
	return builtinVariableInfoForDialect(tokens, CommonDialect)
}

func builtinVariableInfoForDialect(tokens []exprToken, dialect Dialect) (builtinVariable, bool) {
	if len(tokens) != 1 || tokens[0].kind != exprIdentifier {
		return builtinVariable{}, false
	}
	return builtinVariableInfoByName(string(tokens[0].text), dialect)
}

func builtinVariableInfoByName(name string, dialect Dialect) (builtinVariable, bool) {
	if info, known := builtinVariables[name]; known {
		return info, true
	}
	info, known := modernBuiltinVariables[name]
	if !known || !registeredDialect(info.dialects, dialect) {
		return builtinVariable{}, false
	}
	return info, true
}

func builtinVariableRoleInfo(tokens []exprToken, dialect Dialect) (builtinVariable, bool) {
	tokens = stripCommandMarkers(tokens)
	if len(tokens) == 0 || len(tokens) > 256 {
		return builtinVariable{}, false
	}
	tokens = stripExpressionParens(tokens)
	if len(tokens) > 0 && tokens[0].kind == exprOperator && string(tokens[0].text) == "+" {
		tokens = stripExpressionParens(tokens[1:])
	}
	if !variableArgumentTokens(tokens) {
		return builtinVariable{}, false
	}
	name, exact := variableSystemName(tokens)
	if !exact {
		return builtinVariable{}, false
	}
	info, known := builtinVariableInfoByName(name, dialect)
	if !known {
		return builtinVariable{}, false
	}
	if len(tokens) == 1 {
		return info, true
	}
	if value, valid, proven := indexedBuiltinType(tokens, dialect); proven {
		if valid && value == info.value {
			return info, true
		}
		return builtinVariable{}, false
	}
	if len(tokens) < 3 || string(tokens[1].text) != ":" {
		return builtinVariable{}, false
	}
	end, value, proven, invalid := directBuiltinIndexAtom(tokens, 0, dialect)
	if end == len(tokens) && proven && !invalid && value == info.value {
		return info, true
	}
	return builtinVariable{}, false
}

func builtinVariableIdentity(tokens []exprToken, dialect Dialect) (string, bool) {
	tokens = stripCommandMarkers(tokens)
	if !variableArgumentTokens(tokens) {
		return "", false
	}
	tokens = stripExpressionParens(tokens)
	if len(tokens) > 0 && tokens[0].kind == exprOperator && string(tokens[0].text) == "+" {
		tokens = stripExpressionParens(tokens[1:])
	}
	name, exact := variableSystemName(tokens)
	if !exact {
		return "", false
	}
	info, known := builtinVariableInfoByName(name, dialect)
	if !known {
		return "", false
	}
	if len(tokens) == 1 {
		return name, true
	}
	if info.rank == 0 || string(tokens[1].text) != ":" {
		return "", false
	}
	start, depth, indexes := 2, 0, 0
	for i := 2; i <= len(tokens); i++ {
		if i < len(tokens) {
			switch string(tokens[i].text) {
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
		if i != len(tokens) && (string(tokens[i].text) != ":" || depth != 0) {
			continue
		}
		if start == i {
			return "", false
		}
		index := tokens[start:i]
		if _, kind := directIntegerLiteral(index); kind != knownValueNumeric && !builtinIndexExpressionProven(index, dialect) {
			return "", false
		}
		indexes++
		start = i + 1
	}
	if indexes != int(info.rank) {
		return "", false
	}
	return name, true
}

func builtinIndexExpressionProven(tokens []exprToken, dialect Dialect) bool {
	tokens = stripExpressionParens(tokens)
	if len(tokens) == 0 || commandMacroPossibleForDialect(tokens, dialect) {
		return false
	}
	return knownExpressionTypeForDialect(tokens, dialect) == knownValueNumeric
}

func builtinVariableReadOnly(tokens []exprToken, dialect Dialect) bool {
	name, proven := builtinVariableIdentity(tokens, dialect)
	if !proven {
		return false
	}
	info, known := builtinVariableInfoByName(name, dialect)
	return known && info.readOnly
}

func knownVariableType(tokens []exprToken) knownValueType {
	tokens = stripExpressionParens(tokens)
	if info, known := builtinVariableInfo(tokens); known {
		return info.value
	}
	if name, exact := variableSystemName(tokens); exact {
		switch name {
		case "RESULT", "RESULTS", "CFLAG", "CSTR":
			return builtinVariables[name].value
		}
	}
	return knownValueUnknown
}
