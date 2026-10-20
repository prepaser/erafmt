package format

import "strings"

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

func reservedBuiltinMethod(name string) bool {
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

func (p *formatter) syntaxMethod(i int, head []byte, base int, command string, wordEnd int) (int, bool) {
	if reservedInstructions[command] && (command != "OUTPUTLOG" || p.dialect != EMEE) {
		return -1, false
	}
	mask, ok := builtinMethodMasks[command]
	if !ok || !methodDialect(mask, p.dialect) {
		return -1, false
	}
	if wordEnd < len(head) && head[wordEnd] == ';' {
		p.syntaxOpaque(i)
		return -1, true
	}

	mark := len(p.edits)
	body, bodyStart, end, comment, hasArg := p.instructionBody(head, wordEnd)
	if !hasArg {
		if offset, message, valid := p.simpleMethodArgumentsValid(command, base+wordEnd, base+wordEnd); !valid {
			p.syntaxProblem(i, offset, message)
			p.syntaxOpaque(i)
			return -1, true
		}
		p.setCodeEnd(i, base+wordEnd)
		return base + wordEnd, true
	}
	if comment >= 0 {
		end = comment
	}
	if bodyStart >= len(head) || len(trimInput(body, p.allowFullwidthSpace)) == 0 {
		if offset, message, valid := p.simpleMethodArgumentsValid(command, base+end, base+end); !valid {
			p.syntaxProblem(i, offset, message)
			p.syntaxOpaque(i)
			return -1, true
		}
		p.setCodeEnd(i, base+wordEnd)
		return base + wordEnd, true
	}

	if p.rules[CommandSpacing] && wordEnd < len(head) && p.inputSpaceIn(head, wordEnd) {
		arg := trimInputLeft(head, bodyStart, end, p.allowFullwidthSpace)
		if arg < end && head[arg] != ';' {
			p.addEdit(i, base+wordEnd, base+arg, []byte{' '}, CommandSpacing)
		}
	}

	if offset, message, valid := p.simpleMethodArgumentsValid(command, base+bodyStart, base+end); !valid {
		p.truncateEdits(mark)
		p.syntaxProblem(i, offset, message)
		p.syntaxOpaque(i)
		return -1, true
	}
	var formatted bool
	if command == "STRFORM" {
		formatted = p.syntaxPrintFormsLiteral(i, base+bodyStart, base+end)
	} else {
		formatted = p.syntaxOrdinaryExpression(i, base+bodyStart, base+end, false)
	}
	if !formatted {
		p.truncateEdits(mark)
		p.syntaxOpaque(i)
		return -1, true
	}
	codeEnd := trimInputRight(p.src, base+bodyStart, base+end, p.allowFullwidthSpace)
	p.setCodeEnd(i, codeEnd)
	return codeEnd, true
}
