package erabasic

import (
	"bytes"
	"strings"
)

type registeredMode uint8

const (
	RegisteredScalar registeredMode = iota
	RegisteredFields
	RegisteredForm
	RegisteredPrintV
	RegisteredRaw
	RegisteredNoArg
	RegisteredInput
	RegisteredTimedInput
	RegisteredInputS
	RegisteredArraySort
	RegisteredSortChara
	RegisteredHTMLTagSplit
	RegisteredVariable
	RegisteredDTColumnOptions
	RegisteredFormAny
	RegisteredBit
	RegisteredSetter
)

const (
	dialectCommonMask uint8 = 1 << iota
	dialectEmueraMask
	dialectEMEEMask
)

type RegisteredSpec struct {
	Mode      registeredMode
	Min       uint8
	Max       uint8
	Dialects  uint8
	Unbounded bool
}

var registeredCommandSpecs = buildRegisteredCommandSpecs()

func buildRegisteredCommandSpecs() map[string]RegisteredSpec {
	m := make(map[string]RegisteredSpec)
	add := func(mode registeredMode, min, max uint8, dialects uint8, names ...string) {
		for _, name := range names {
			m[name] = RegisteredSpec{Mode: mode, Min: min, Max: max, Dialects: dialects}
		}
	}
	addUnbounded := func(mode registeredMode, min uint8, dialects uint8, names ...string) {
		for _, name := range names {
			m[name] = RegisteredSpec{Mode: mode, Min: min, Dialects: dialects, Unbounded: true}
		}
	}
	all := dialectCommonMask | dialectEmueraMask | dialectEMEEMask
	add(RegisteredScalar, 0, 1, all, "SETFONT", "FONTSTYLE")
	add(RegisteredScalar, 1, 1, all, "LOADDATA", "ASSERT", "LOADCHARA", "LOADVAR", "TOOLTIP_SETDELAY", "TOOLTIP_SETDURATION")
	add(RegisteredFields, 0, 1, all, "PRINTCPERLINE", "SAVENOS")
	add(RegisteredFields, 1, 2, all, "HTML_PRINT")
	add(RegisteredFields, 1, 3, all, "SETCOLOR", "SETBGCOLOR")
	add(RegisteredFields, 2, 2, all, "PRINTBUTTON", "PRINTBUTTONC", "PRINTBUTTONLC")
	add(RegisteredFields, 3, 3, all, "BAR", "BARL")
	add(RegisteredFields, 2, 2, all, "TWAIT", "SWAPCHARA", "COPYCHARA")
	addUnbounded(RegisteredFields, 1, all, "ADDCHARA", "ADDSPCHARA", "ADDCOPYCHARA", "PICKUPCHARA", "DELCHARA")
	add(RegisteredFields, 2, 2, all, "SAVEDATA")
	addUnbounded(RegisteredFields, 3, all, "SAVECHARA")
	add(RegisteredFields, 3, 255, all, "SAVEVAR")
	add(RegisteredFields, 3, 3, all, "ARRAYREMOVE")
	add(RegisteredFields, 3, 5, all, "ARRAYSHIFT")
	add(RegisteredFields, 2, 2, all, "ARRAYCOPY")
	add(RegisteredArraySort, 1, 4, all, "ARRAYSORT")
	add(RegisteredSortChara, 0, 2, all, "SORTCHARA")
	add(RegisteredHTMLTagSplit, 1, 3, all, "HTML_TAGSPLIT")
	add(RegisteredInput, 0, 3, all, "INPUT", "ONEINPUT")
	add(RegisteredTimedInput, 2, 6, all, "TINPUT", "TINPUTS", "TONEINPUT", "TONEINPUTS")
	add(RegisteredInputS, 0, 2, all, "INPUTS", "ONEINPUTS")
	add(RegisteredInput, 0, 3, dialectEMEEMask, "BINPUT", "ONEBINPUT")
	add(RegisteredInputS, 0, 2, dialectEMEEMask, "BINPUTS", "ONEBINPUTS")
	add(RegisteredForm, 0, 0, all, "THROW", "PUTFORM", "PRINTPLAINFORM", "DEBUGPRINTFORM", "DEBUGPRINTFORML", "STRLENFORM", "STRLENFORMU", "ENCODETOUNI", "REUSELASTLINE")
	add(RegisteredBit, 2, 255, all, "SETBIT", "CLEARBIT", "INVERTBIT")
	add(RegisteredSetter, 0, 255, all, "VARSET", "CVARSET", "POWER")
	add(RegisteredForm, 1, 1, all, "DRAWLINEFORM")
	add(RegisteredNoArg, 0, 0, all, "WAIT", "FORCEWAIT", "WAITANYKEY")
	add(RegisteredRaw, 0, 0, dialectEMEEMask, "PRINTN")
	add(RegisteredPrintV, 0, 0, dialectEMEEMask, "PRINTVN")
	add(RegisteredScalar, 1, 1, dialectEMEEMask, "PRINTSN")
	add(RegisteredForm, 0, 0, dialectEMEEMask, "PRINTFORMN")
	add(RegisteredScalar, 1, 1, dialectEMEEMask, "PRINTFORMSN")
	add(
		RegisteredNoArg,
		0,
		0,
		all,
		"RESETCOLOR",
		"RESETBGCOLOR",
		"FONTBOLD",
		"FONTITALIC",
		"FONTREGULAR",
		"SAVEGLOBAL",
		"LOADGLOBAL",
		"RESETDATA",
		"RESETGLOBAL",
		"DEBUGCLEAR",
		"DUMPRAND",
		"INITRAND",
		"RESTART",
		"SAVEGAME",
		"LOADGAME",
		"ADDVOIDCHARA",
	)
	add(RegisteredRaw, 0, 0, all, "BEGIN", "CALLEVENT", "ALIGNMENT", "SETCOLORBYNAME", "SETBGCOLORBYNAME", "STRLEN", "STRLENU")
	add(RegisteredRaw, 0, 0, dialectEMEEMask, "FORCE_BEGIN")
	add(RegisteredScalar, 0, 1, all, "AWAIT", "INPUTMOUSEKEY", "RANDOMIZE")
	add(RegisteredNoArg, 0, 0, dialectEMEEMask, "INPUTANY", "STOPBGM", "STOPSOUND", "UPDATECHECK", "HTML_PRINT_ISLAND_CLEAR", "CLEARBGIMAGE")
	add(RegisteredScalar, 0, 1, dialectEMEEMask, "BREAKBUTTON")
	add(RegisteredScalar, 1, 1, dialectEMEEMask, "SETBGMVOLUME", "SETSOUNDVOLUME", "TOOLTIP_SETFONTSIZE", "TOOLTIP_CUSTOM", "TOOLTIP_FORMAT", "TOOLTIP_IMG", "PLAYBGM", "TOOLTIP_SETFONT")
	add(RegisteredFields, 1, 2, dialectEMEEMask, "PLAYSOUND", "HTML_PRINT_ISLAND")
	add(RegisteredFields, 2, 2, all, "TOOLTIP_SETCOLOR")
	add(RegisteredVariable, 1, 1, all, "VARSIZE")
	add(RegisteredDTColumnOptions, 4, 255, dialectEMEEMask, "DT_COLUMN_OPTIONS")
	add(RegisteredFormAny, 1, 255, dialectEMEEMask, "SETBGIMAGE", "REMOVEBGIMAGE")
	return m
}

func RegisteredCommandSpec(command string, dialect Dialect) (RegisteredSpec, bool) {
	spec, ok := registeredCommandSpecs[command]
	return spec, ok && registeredDialect(spec.Dialects, dialect)
}

func registeredDialect(mask uint8, dialect Dialect) bool {
	switch dialect {
	case Emuera1824:
		return mask&dialectEmueraMask != 0
	case EMEE:
		return mask&dialectEMEEMask != 0
	default:
		return mask&dialectCommonMask != 0
	}
}

const baseBuiltinMethodNames = `
GETCHARA GETSPCHARA CSVNAME CSVCALLNAME CSVNICKNAME CSVMASTERNAME CSVCSTR CSVBASE CSVABL CSVMARK CSVEXP CSVRELATION CSVTALENT CSVCFLAG CSVEQUIP CSVJUEL FINDCHARA FINDLASTCHARA EXISTCSV
VARSIZE CHKFONT CHKDATA ISSKIP MOUSESKIP MESSKIP GETCOLOR GETDEFCOLOR GETFOCUSCOLOR GETBGCOLOR GETDEFBGCOLOR GETSTYLE GETFONT BARSTR CURRENTALIGN CURRENTREDRAW COLOR_FROMNAME COLOR_FROMRGB CHKCHARADATA FIND_CHARADATA
MONEYSTR PRINTCPERLINE PRINTCLENGTH SAVENOS GETTIME GETTIMES GETMILLISECOND GETSECOND RAND MIN MAX ABS POWER SQRT CBRT LOG LOG10 EXPONENT SIGN LIMIT
SUMARRAY SUMCARRAY MATCH CMATCH GROUPMATCH NOSAMES ALLSAMES MAXARRAY MAXCARRAY MINARRAY MINCARRAY GETBIT GETNUM GETPALAMLV GETEXPLV FINDELEMENT FINDLASTELEMENT INRANGE INRANGEARRAY INRANGECARRAY GETNUMB ARRAYMSORT
STRLENS STRLENSU SUBSTRING SUBSTRINGU STRFIND STRFINDU STRCOUNT TOSTR TOINT TOUPPER TOLOWER TOHALF TOFULL LINEISEMPTY REPLACE UNICODE UNICODEBYTE CONVERT ISNUMERIC ESCAPE ENCODETOUNI CHARATU GETLINESTR STRFORM STRJOIN GETCONFIG GETCONFIGS
HTML_GETPRINTEDSTR HTML_POPPRINTINGSTR HTML_TOPLAINTEXT HTML_ESCAPE
SPRITECREATED SPRITEWIDTH SPRITEHEIGHT SPRITEMOVE SPRITESETPOS SPRITEPOSX SPRITEPOSY CLIENTWIDTH CLIENTHEIGHT GETKEY GETKEYTRIGGERED MOUSEX MOUSEY ISACTIVE SAVETEXT LOADTEXT
GCREATED GWIDTH GHEIGHT GGETCOLOR SPRITEGETCOLOR GCREATE GCREATEFROMFILE GDISPOSE GCLEAR GFILLRECTANGLE GDRAWSPRITE GSETCOLOR GDRAWG GDRAWGWITHMASK GSETBRUSH GSETFONT GSETPEN SPRITECREATE SPRITEDISPOSE CBGSETG CBGSETSPRITE CBGCLEAR CBGCLEARBUTTON CBGREMOVERANGE CBGREMOVEBMAP CBGSETBMAPG CBGSETBUTTONSPRITE GSAVE GLOAD SPRITEANIMECREATE SPRITEANIMEADDFRAME SETANIMETIMER`
const modernOnlyBuiltinMethodNames = `
MOUSEB OUTPUTLOG HTML_STRINGLEN HTML_SUBSTRING HTML_STRINGLINES EXISTFILE EXISTVAR ISDEFINED ENUMFUNCBEGINSWITH ENUMFUNCENDSWITH ENUMFUNCWITH ENUMVARBEGINSWITH
ENUMVARENDSWITH ENUMVARWITH ENUMMACROBEGINSWITH ENUMMACROENDSWITH ENUMMACROWITH ENUMFILES GETVAR GETVARS SETVAR VARSETEX ARRAYMSORTEX REGEXPMATCH
XML_DOCUMENT XML_RELEASE XML_GET XML_GET_BYNAME XML_SET XML_SET_BYNAME XML_EXIST XML_TOSTR XML_ADDNODE XML_ADDNODE_BYNAME XML_REMOVENODE XML_REMOVENODE_BYNAME
XML_REPLACE XML_REPLACE_BYNAME XML_ADDATTRIBUTE XML_ADDATTRIBUTE_BYNAME XML_REMOVEATTRIBUTE XML_REMOVEATTRIBUTE_BYNAME MAP_CREATE MAP_EXIST MAP_RELEASE MAP_GET MAP_CLEAR MAP_SIZE
MAP_HAS MAP_SET MAP_REMOVE MAP_GETKEYS MAP_TOXML MAP_FROMXML DT_CREATE DT_EXIST DT_RELEASE DT_NOCASE DT_CLEAR DT_COLUMN_ADD
DT_COLUMN_NAMES DT_COLUMN_EXIST DT_COLUMN_REMOVE DT_COLUMN_LENGTH DT_ROW_ADD DT_ROW_SET DT_ROW_REMOVE DT_ROW_LENGTH DT_CELL_GET DT_CELL_ISNULL DT_CELL_GETS DT_CELL_SET
DT_SELECT DT_TOXML DT_FROMXML MOVETEXTBOX RESUMETEXTBOX EXISTSOUND EXISTFUNCTION GDRAWGWITHROTATE GDRAWTEXT GGETFONT GGETFONTSIZE GGETFONTSTYLE
GGETTEXTSIZE GGETBRUSH GGETPEN GGETPENWIDTH GETMEMORYUSAGE CLEARMEMORY GETTEXTBOX SETTEXTBOX ERDNAME SPRITEDISPOSEALL GDRAWLINE GETDISPLAYLINE
GDASHSTYLE GETDOINGFUNCTION FLOWINPUT FLOWINPUTS GETMETH GETMETHS EXISTMETH BITMAP_CACHE_ENABLE HOTKEY_STATE HOTKEY_STATE_INIT`
const modernBuiltinMethodNames = baseBuiltinMethodNames + modernOnlyBuiltinMethodNames
const (
	builtinMethodBaseMask = 1 << iota
	builtinMethodModernMask
)

var builtinMethodMasks = buildBuiltinMethodMasks()

func buildBuiltinMethodMasks() map[string]uint8 {
	m := make(map[string]uint8)
	for _, name := range strings.Fields(baseBuiltinMethodNames) {
		m[strings.ToUpper(name)] |= builtinMethodBaseMask
	}
	for _, name := range strings.Fields(modernBuiltinMethodNames) {
		m[strings.ToUpper(name)] |= builtinMethodModernMask
	}
	return m
}

func ReservedBuiltinMethod(name string) bool {
	_, ok := builtinMethodMasks[name]
	return ok
}

func methodDialect(mask uint8, dialect Dialect) bool {
	switch dialect {
	case Emuera1824:
		return mask&builtinMethodBaseMask != 0
	case EMEE:
		return mask&builtinMethodModernMask != 0
	default:
		return mask&(builtinMethodBaseMask|builtinMethodModernMask) == builtinMethodBaseMask|builtinMethodModernMask
	}
}

func BuiltinMethodAvailable(command string, dialect Dialect) bool {
	if ReservedInstruction(command) && (command != "OUTPUTLOG" || dialect != EMEE) {
		return false
	}
	mask, ok := builtinMethodMasks[command]
	return ok && methodDialect(mask, dialect)
}

type extraCommandKind uint8

const (
	extraNone extraCommandKind = iota
	ExtraTimes
	ExtraVariable
	ExtraSplit
	ExtraSwap
)

func ExtraCommand(command string) extraCommandKind {
	switch command {
	case "TIMES":
		return ExtraTimes
	case "PRINTDATA", "PRINTDATAL", "PRINTDATAW", "PRINTDATAK", "PRINTDATAKL", "PRINTDATAD", "PRINTDATADL", "PRINTDATADW", "STRDATA":
		return ExtraVariable
	case "SPLIT":
		return ExtraSplit
	case "SWAP":
		return ExtraSwap
	default:
		return extraNone
	}
}

func (p *SemanticContext) RegisteredBody(head []byte, wordEnd int) (int, int, int, bool) {
	return RegisteredBodyWithSpace(head, wordEnd, p.AllowFullwidthSpace, p.ScanSession(head, 0, len(head)))
}

func (p *SemanticContext) InstructionBody(head []byte, wordEnd int) ([]byte, int, int, int, bool) {
	return InstructionBodyWithSpace(head, wordEnd, p.AllowFullwidthSpace, p.ScanSession(head, 0, len(head)))
}

func (p *SemanticContext) RegisteredFieldValid(command string, index int, src []byte) bool {
	switch command {
	case "ARRAYREMOVE", "ARRAYSHIFT":
		return index != 0 || p.VariableArgument(src)
	case "PRINTCPERLINE", "SAVENOS":
		return p.VariableArgument(src)
	default:
		return true
	}
}

func (p *SemanticContext) VariableArgument(src []byte) bool {
	src = TrimInput(src, p.AllowFullwidthSpace)
	if bytes.Contains(src, []byte("[[")) {
		return true
	}
	session := p.ScanSession(src, 0, len(src))
	tokens, err := session.LexRange(ExpressionSourceOriginal, 0, len(src), 0)
	return err == nil && variableArgumentShape(tokens)
}

func (p *SemanticContext) CommandFieldMayExpand(left, right int) bool {
	tokens, err := p.CommandFieldTokens(left, right)
	return err == nil && CommandMacroPossibleForDialect(tokens, p.Dialect)
}

func (p *SemanticContext) CommandRawTypeValid(expected KnownValueType, left, right int) bool {
	tokens, err := p.CommandFieldTokens(left, right)
	if err != nil || CommandMacroPossibleForDialect(tokens, p.Dialect) || SetterMacroPossibleForDialect(tokens, p.Dialect) {
		return true
	}
	if BuiltinScalarIndexesMissing(tokens, p.Dialect) {
		return false
	}
	return CommandTypeMatches(expected, KnownExpressionTypeForDialect(tokens, p.Dialect))
}
