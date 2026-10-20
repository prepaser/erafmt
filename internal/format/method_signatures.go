package format

import "strings"

type methodSignature struct {
	arguments      []knownValueType
	variants       [][]knownValueType
	variadic       knownValueType
	minimum        int
	minimumSet     bool
	sameAsFirst    bool
	matrix         methodMatrixKind
	matrixNullable bool
	validation     methodValidation
	rejectNulls    bool
	nullable       []bool
}

type methodValidation uint8

const (
	methodValidationNone methodValidation = iota
	methodValidationDataTableRowAdd
	methodValidationDataTableRowRemove
	methodValidationDataTableToXML
	methodValidationEnumFiles
	methodValidationDynamicMethod
	methodValidationRand
	methodValidationGetNum
	methodValidationFindElement
	methodValidationArrayMultiSort
	methodValidationSumArray
	methodValidationMatch
	methodValidationInRangeArray
	methodValidationStrJoin
	methodValidationRegexpMatch
	methodValidationGetBit
	methodValidationArrayExtrema
	methodValidationDataTableSelect
	methodValidationXMLGet
	methodValidationXMLAddNode
	methodValidationXMLRemoveNode
	methodValidationFindChara
	methodValidationGraphicsDrawText
	methodValidationGraphicsDrawGWithRotate
)

type methodMatrixKind uint8

const (
	methodMatrixNone methodMatrixKind = iota
	methodMatrixSprite
	methodMatrixGraphics
)

var simpleMethodSignatures = map[string]methodSignature{
	"RAND":                {minimum: 1, validation: methodValidationRand},
	"MIN":                 {variadic: knownValueNumeric, minimum: 1},
	"MAX":                 {variadic: knownValueNumeric, minimum: 1},
	"GROUPMATCH":          {sameAsFirst: true, minimum: 2},
	"NOSAMES":             {sameAsFirst: true, minimum: 2},
	"ALLSAMES":            {sameAsFirst: true, minimum: 2},
	"GETCHARA":            {arguments: []knownValueType{knownValueNumeric, knownValueNumeric}, minimum: 1},
	"GETSPCHARA":          {arguments: []knownValueType{knownValueNumeric}},
	"HTML_GETPRINTEDSTR":  {arguments: []knownValueType{knownValueNumeric}, minimumSet: true, rejectNulls: true},
	"ISSKIP":              {},
	"MOUSESKIP":           {},
	"MESSKIP":             {},
	"GETCOLOR":            {},
	"GETDEFCOLOR":         {},
	"GETFOCUSCOLOR":       {},
	"GETBGCOLOR":          {},
	"GETDEFBGCOLOR":       {},
	"GETSTYLE":            {},
	"GETFONT":             {},
	"CURRENTALIGN":        {},
	"CURRENTREDRAW":       {},
	"PRINTCLENGTH":        {},
	"GETTIMES":            {},
	"GETMILLISECOND":      {},
	"GETSECOND":           {},
	"LINEISEMPTY":         {},
	"CLIENTWIDTH":         {},
	"CLIENTHEIGHT":        {},
	"MOUSEX":              {},
	"MOUSEY":              {},
	"ISACTIVE":            {},
	"GCREATED":            {arguments: []knownValueType{knownValueNumeric}},
	"GWIDTH":              {arguments: []knownValueType{knownValueNumeric}},
	"GHEIGHT":             {arguments: []knownValueType{knownValueNumeric}},
	"GCREATE":             {arguments: []knownValueType{knownValueNumeric, knownValueNumeric, knownValueNumeric}},
	"GDISPOSE":            {arguments: []knownValueType{knownValueNumeric}},
	"GGETCOLOR":           {arguments: []knownValueType{knownValueNumeric, knownValueNumeric, knownValueNumeric}},
	"GSETCOLOR":           {arguments: []knownValueType{knownValueNumeric, knownValueNumeric, knownValueNumeric, knownValueNumeric}},
	"GFILLRECTANGLE":      {arguments: []knownValueType{knownValueNumeric, knownValueNumeric, knownValueNumeric, knownValueNumeric, knownValueNumeric}},
	"GDRAWGWITHMASK":      {arguments: []knownValueType{knownValueNumeric, knownValueNumeric, knownValueNumeric, knownValueNumeric, knownValueNumeric}},
	"GSETBRUSH":           {arguments: []knownValueType{knownValueNumeric, knownValueNumeric}},
	"GSETFONT":            {arguments: []knownValueType{knownValueNumeric, knownValueString, knownValueNumeric}},
	"GSETPEN":             {arguments: []knownValueType{knownValueNumeric, knownValueNumeric}},
	"SPRITEDISPOSE":       {arguments: []knownValueType{knownValueString}},
	"SPRITECREATED":       {arguments: []knownValueType{knownValueString}},
	"SPRITEWIDTH":         {arguments: []knownValueType{knownValueString}},
	"SPRITEHEIGHT":        {arguments: []knownValueType{knownValueString}},
	"SPRITEPOSX":          {arguments: []knownValueType{knownValueString}},
	"SPRITEPOSY":          {arguments: []knownValueType{knownValueString}},
	"SPRITEMOVE":          {arguments: []knownValueType{knownValueString, knownValueNumeric, knownValueNumeric}},
	"SPRITESETPOS":        {arguments: []knownValueType{knownValueString, knownValueNumeric, knownValueNumeric}},
	"SPRITEGETCOLOR":      {arguments: []knownValueType{knownValueString, knownValueNumeric, knownValueNumeric}},
	"SPRITEANIMECREATE":   {arguments: []knownValueType{knownValueString, knownValueNumeric, knownValueNumeric}},
	"SPRITEANIMEADDFRAME": {arguments: []knownValueType{knownValueString, knownValueNumeric, knownValueNumeric, knownValueNumeric, knownValueNumeric, knownValueNumeric, knownValueNumeric, knownValueNumeric, knownValueNumeric}},
	"CBGREMOVERANGE":      {arguments: []knownValueType{knownValueNumeric, knownValueNumeric}},
	"CBGSETG":             {arguments: []knownValueType{knownValueNumeric, knownValueNumeric, knownValueNumeric, knownValueNumeric}},
	"CBGSETBMAPG":         {arguments: []knownValueType{knownValueNumeric}},
	"GSAVE":               {arguments: []knownValueType{knownValueNumeric, knownValueNumeric}},
	"GLOAD":               {arguments: []knownValueType{knownValueNumeric, knownValueNumeric}},
	"GCLEAR":              {arguments: []knownValueType{knownValueNumeric, knownValueNumeric}},
	"GCREATEFROMFILE":     {arguments: []knownValueType{knownValueNumeric, knownValueString}},
	"CBGSETSPRITE":        {arguments: []knownValueType{knownValueString, knownValueNumeric, knownValueNumeric, knownValueNumeric}},
	"CBGSETBUTTONSPRITE": {variants: [][]knownValueType{
		{knownValueNumeric, knownValueString, knownValueString, knownValueNumeric, knownValueNumeric, knownValueNumeric},
		{knownValueNumeric, knownValueString, knownValueString, knownValueNumeric, knownValueNumeric, knownValueNumeric, knownValueString},
	}},
	"GDRAWSPRITE": {matrix: methodMatrixSprite},
	"GDRAWG":      {matrix: methodMatrixGraphics},
	"SPRITECREATE": {variants: [][]knownValueType{
		{knownValueString, knownValueNumeric},
		{knownValueString, knownValueNumeric, knownValueNumeric, knownValueNumeric, knownValueNumeric, knownValueNumeric},
	}},
	"ABS":                 {arguments: []knownValueType{knownValueNumeric}},
	"SQRT":                {arguments: []knownValueType{knownValueNumeric}},
	"CBRT":                {arguments: []knownValueType{knownValueNumeric}},
	"LOG":                 {arguments: []knownValueType{knownValueNumeric}},
	"LOG10":               {arguments: []knownValueType{knownValueNumeric}},
	"EXPONENT":            {arguments: []knownValueType{knownValueNumeric}},
	"SIGN":                {arguments: []knownValueType{knownValueNumeric}},
	"LIMIT":               {arguments: []knownValueType{knownValueNumeric, knownValueNumeric, knownValueNumeric}},
	"INRANGE":             {arguments: []knownValueType{knownValueNumeric, knownValueNumeric, knownValueNumeric}},
	"STRLENS":             {arguments: []knownValueType{knownValueString}},
	"STRLENSU":            {arguments: []knownValueType{knownValueString}},
	"STRCOUNT":            {arguments: []knownValueType{knownValueString, knownValueString}},
	"TOINT":               {arguments: []knownValueType{knownValueString}},
	"TOUPPER":             {arguments: []knownValueType{knownValueString}},
	"TOLOWER":             {arguments: []knownValueType{knownValueString}},
	"TOHALF":              {arguments: []knownValueType{knownValueString}},
	"TOFULL":              {arguments: []knownValueType{knownValueString}},
	"REPLACE":             {arguments: []knownValueType{knownValueString, knownValueString, knownValueString}},
	"UNICODE":             {arguments: []knownValueType{knownValueNumeric}},
	"UNICODEBYTE":         {arguments: []knownValueType{knownValueString}},
	"CONVERT":             {arguments: []knownValueType{knownValueNumeric, knownValueNumeric}},
	"ISNUMERIC":           {arguments: []knownValueType{knownValueString}},
	"ESCAPE":              {arguments: []knownValueType{knownValueString}},
	"CHARATU":             {arguments: []knownValueType{knownValueString, knownValueNumeric}},
	"HTML_TOPLAINTEXT":    {arguments: []knownValueType{knownValueString}},
	"HTML_ESCAPE":         {arguments: []knownValueType{knownValueString}},
	"CHKFONT":             {arguments: []knownValueType{knownValueString}},
	"CHKDATA":             {arguments: []knownValueType{knownValueNumeric}},
	"CHKCHARADATA":        {arguments: []knownValueType{knownValueString}},
	"BARSTR":              {arguments: []knownValueType{knownValueNumeric, knownValueNumeric, knownValueNumeric}},
	"COLOR_FROMNAME":      {arguments: []knownValueType{knownValueString}},
	"COLOR_FROMRGB":       {arguments: []knownValueType{knownValueNumeric, knownValueNumeric, knownValueNumeric}},
	"GETBIT":              {minimum: 2, validation: methodValidationGetBit},
	"GETNUMB":             {arguments: []knownValueType{knownValueString, knownValueString}},
	"GETPALAMLV":          {arguments: []knownValueType{knownValueNumeric, knownValueNumeric}},
	"GETEXPLV":            {arguments: []knownValueType{knownValueNumeric, knownValueNumeric}},
	"GETLINESTR":          {arguments: []knownValueType{knownValueString}},
	"STRFORM":             {arguments: []knownValueType{knownValueString}},
	"GETCONFIG":           {arguments: []knownValueType{knownValueString}},
	"GETCONFIGS":          {arguments: []knownValueType{knownValueString}},
	"HTML_POPPRINTINGSTR": {},
	"GETKEY":              {arguments: []knownValueType{knownValueNumeric}},
	"GETKEYTRIGGERED":     {arguments: []knownValueType{knownValueNumeric}},
	"SETANIMETIMER":       {arguments: []knownValueType{knownValueNumeric}},
	"SUBSTRING":           {arguments: []knownValueType{knownValueString, knownValueNumeric, knownValueNumeric}, minimum: 1},
	"SUBSTRINGU":          {arguments: []knownValueType{knownValueString, knownValueNumeric, knownValueNumeric}, minimum: 1},
	"STRFIND":             {arguments: []knownValueType{knownValueString, knownValueString, knownValueNumeric}, minimum: 2},
	"STRFINDU":            {arguments: []knownValueType{knownValueString, knownValueString, knownValueNumeric}, minimum: 2},
	"TOSTR":               {arguments: []knownValueType{knownValueNumeric, knownValueString}, minimum: 1},
	"MONEYSTR":            {arguments: []knownValueType{knownValueNumeric, knownValueString}, minimum: 1},
	"SAVETEXT":            {arguments: []knownValueType{knownValueString, knownValueNumeric, knownValueNumeric, knownValueNumeric}, minimum: 2, rejectNulls: true},
	"LOADTEXT":            {arguments: []knownValueType{knownValueNumeric, knownValueNumeric, knownValueNumeric}, minimum: 1, rejectNulls: true},
	"GETNUM":              {minimum: 2, validation: methodValidationGetNum},
	"FINDELEMENT":         {minimum: 2, validation: methodValidationFindElement},
	"FINDLASTELEMENT":     {minimum: 2, validation: methodValidationFindElement},
	"ARRAYMSORT":          {minimum: 2, validation: methodValidationArrayMultiSort},
	"SUMARRAY":            {minimum: 1, validation: methodValidationSumArray},
	"SUMCARRAY":           {minimum: 1, validation: methodValidationSumArray},
	"MAXARRAY":            {minimum: 1, validation: methodValidationArrayExtrema},
	"MAXCARRAY":           {minimum: 1, validation: methodValidationArrayExtrema},
	"MINARRAY":            {minimum: 1, validation: methodValidationArrayExtrema},
	"MINCARRAY":           {minimum: 1, validation: methodValidationArrayExtrema},
	"MATCH":               {minimum: 2, validation: methodValidationMatch},
	"CMATCH":              {minimum: 2, validation: methodValidationMatch},
	"INRANGEARRAY":        {minimum: 3, validation: methodValidationInRangeArray},
	"INRANGECARRAY":       {minimum: 3, validation: methodValidationInRangeArray},
	"STRJOIN":             {minimum: 1, validation: methodValidationStrJoin},
	"CSVNAME":             {arguments: []knownValueType{knownValueNumeric, knownValueNumeric}, minimum: 1},
	"CSVCALLNAME":         {arguments: []knownValueType{knownValueNumeric, knownValueNumeric}, minimum: 1},
	"CSVNICKNAME":         {arguments: []knownValueType{knownValueNumeric, knownValueNumeric}, minimum: 1},
	"CSVMASTERNAME":       {arguments: []knownValueType{knownValueNumeric, knownValueNumeric}, minimum: 1},
	"CSVCSTR":             {arguments: []knownValueType{knownValueNumeric, knownValueNumeric, knownValueNumeric}, minimum: 2},
	"CSVBASE":             {arguments: []knownValueType{knownValueNumeric, knownValueNumeric, knownValueNumeric}, minimum: 2},
	"CSVABL":              {arguments: []knownValueType{knownValueNumeric, knownValueNumeric, knownValueNumeric}, minimum: 2},
	"CSVMARK":             {arguments: []knownValueType{knownValueNumeric, knownValueNumeric, knownValueNumeric}, minimum: 2},
	"CSVEXP":              {arguments: []knownValueType{knownValueNumeric, knownValueNumeric, knownValueNumeric}, minimum: 2},
	"CSVRELATION":         {arguments: []knownValueType{knownValueNumeric, knownValueNumeric, knownValueNumeric}, minimum: 2},
	"CSVTALENT":           {arguments: []knownValueType{knownValueNumeric, knownValueNumeric, knownValueNumeric}, minimum: 2},
	"CSVCFLAG":            {arguments: []knownValueType{knownValueNumeric, knownValueNumeric, knownValueNumeric}, minimum: 2},
	"CSVEQUIP":            {arguments: []knownValueType{knownValueNumeric, knownValueNumeric, knownValueNumeric}, minimum: 2},
	"CSVJUEL":             {arguments: []knownValueType{knownValueNumeric, knownValueNumeric, knownValueNumeric}, minimum: 2},
	"EXISTCSV":            {arguments: []knownValueType{knownValueNumeric, knownValueNumeric}, minimum: 1},
	"FINDCHARA":           {minimum: 2, validation: methodValidationFindChara},
	"FINDLASTCHARA":       {minimum: 2, validation: methodValidationFindChara},
	"FIND_CHARADATA":      {arguments: []knownValueType{knownValueString}, minimumSet: true},
}

var modernMethodSignatures = map[string]methodSignature{
	"GETNUM":                {minimum: 2, validation: methodValidationGetNum},
	"GCLEAR":                {variants: [][]knownValueType{{knownValueNumeric, knownValueNumeric}, {knownValueNumeric, knownValueNumeric, knownValueNumeric, knownValueNumeric, knownValueNumeric, knownValueNumeric}}},
	"GCREATEFROMFILE":       {arguments: []knownValueType{knownValueNumeric, knownValueString, knownValueNumeric}, minimum: 2, rejectNulls: true},
	"CBGSETBUTTONSPRITE":    {variants: [][]knownValueType{{knownValueNumeric, knownValueString, knownValueString, knownValueNumeric, knownValueNumeric, knownValueNumeric}, {knownValueNumeric, knownValueString, knownValueString, knownValueNumeric, knownValueNumeric, knownValueNumeric, knownValueString}}},
	"GDRAWTEXT":             {arguments: []knownValueType{knownValueNumeric, knownValueString, knownValueNumeric, knownValueNumeric}, minimum: 2, validation: methodValidationGraphicsDrawText},
	"GGETTEXTSIZE":          {arguments: []knownValueType{knownValueString, knownValueString, knownValueNumeric, knownValueNumeric}, minimum: 3, rejectNulls: true},
	"GDRAWGWITHROTATE":      {arguments: []knownValueType{knownValueNumeric, knownValueNumeric, knownValueNumeric, knownValueNumeric, knownValueNumeric}, minimum: 3, validation: methodValidationGraphicsDrawGWithRotate},
	"GDRAWLINE":             {arguments: []knownValueType{knownValueNumeric, knownValueNumeric, knownValueNumeric, knownValueNumeric, knownValueNumeric}},
	"GDASHSTYLE":            {arguments: []knownValueType{knownValueNumeric, knownValueNumeric, knownValueNumeric}},
	"SETTEXTBOX":            {arguments: []knownValueType{knownValueString}},
	"HOTKEY_STATE":          {arguments: []knownValueType{knownValueNumeric, knownValueNumeric}, minimum: 2},
	"GDRAWSPRITE":           {matrix: methodMatrixSprite},
	"GDRAWG":                {matrix: methodMatrixGraphics, matrixNullable: true},
	"MOUSEB":                {},
	"CBGCLEAR":              {},
	"CBGCLEARBUTTON":        {},
	"CBGREMOVEBMAP":         {},
	"GETMEMORYUSAGE":        {},
	"CLEARMEMORY":           {},
	"GETTEXTBOX":            {},
	"GETDOINGFUNCTION":      {},
	"HTML_STRINGLEN":        {arguments: []knownValueType{knownValueString, knownValueNumeric}, minimum: 1, rejectNulls: true},
	"HTML_SUBSTRING":        {arguments: []knownValueType{knownValueString, knownValueNumeric}},
	"HTML_STRINGLINES":      {arguments: []knownValueType{knownValueString, knownValueNumeric}},
	"EXISTFILE":             {arguments: []knownValueType{knownValueString}},
	"EXISTVAR":              {arguments: []knownValueType{knownValueString}},
	"ISDEFINED":             {arguments: []knownValueType{knownValueString}},
	"GETVAR":                {arguments: []knownValueType{knownValueString}},
	"GETVARS":               {arguments: []knownValueType{knownValueString}},
	"EXISTSOUND":            {arguments: []knownValueType{knownValueString}},
	"EXISTFUNCTION":         {arguments: []knownValueType{knownValueString, knownValueNumeric}, minimum: 1, rejectNulls: true},
	"GETDISPLAYLINE":        {arguments: []knownValueType{knownValueNumeric}},
	"FLOWINPUT":             {arguments: []knownValueType{knownValueNumeric, knownValueNumeric, knownValueNumeric, knownValueNumeric}, minimum: 1, rejectNulls: true},
	"FLOWINPUTS":            {arguments: []knownValueType{knownValueNumeric, knownValueString}, minimum: 1, rejectNulls: true},
	"OUTPUTLOG":             {arguments: []knownValueType{knownValueString, knownValueNumeric}, minimumSet: true, rejectNulls: true},
	"REGEXPMATCH":           {minimum: 2, validation: methodValidationRegexpMatch},
	"EXISTMETH":             {arguments: []knownValueType{knownValueString}},
	"BITMAP_CACHE_ENABLE":   {arguments: []knownValueType{knownValueNumeric}},
	"HOTKEY_STATE_INIT":     {arguments: []knownValueType{knownValueNumeric}},
	"MAP_CREATE":            {arguments: []knownValueType{knownValueString}},
	"MAP_EXIST":             {arguments: []knownValueType{knownValueString}},
	"MAP_RELEASE":           {arguments: []knownValueType{knownValueString}},
	"MAP_GET":               {arguments: []knownValueType{knownValueString, knownValueString}},
	"MAP_CLEAR":             {arguments: []knownValueType{knownValueString}},
	"MAP_SIZE":              {arguments: []knownValueType{knownValueString}},
	"MAP_HAS":               {arguments: []knownValueType{knownValueString, knownValueString}},
	"MAP_SET":               {arguments: []knownValueType{knownValueString, knownValueString, knownValueString}},
	"MAP_REMOVE":            {arguments: []knownValueType{knownValueString, knownValueString}},
	"MAP_TOXML":             {arguments: []knownValueType{knownValueString}},
	"MAP_FROMXML":           {arguments: []knownValueType{knownValueString, knownValueString}},
	"DT_CREATE":             {arguments: []knownValueType{knownValueString}},
	"DT_EXIST":              {arguments: []knownValueType{knownValueString}},
	"DT_RELEASE":            {arguments: []knownValueType{knownValueString}},
	"DT_NOCASE":             {arguments: []knownValueType{knownValueString, knownValueNumeric}},
	"DT_CLEAR":              {arguments: []knownValueType{knownValueString}},
	"DT_COLUMN_EXIST":       {arguments: []knownValueType{knownValueString, knownValueString}},
	"DT_COLUMN_REMOVE":      {arguments: []knownValueType{knownValueString, knownValueString}},
	"DT_COLUMN_LENGTH":      {arguments: []knownValueType{knownValueString}},
	"DT_ROW_LENGTH":         {arguments: []knownValueType{knownValueString}},
	"DT_CELL_GET":           {arguments: []knownValueType{knownValueString, knownValueNumeric, knownValueString, knownValueNumeric}, minimum: 3, rejectNulls: true},
	"DT_CELL_ISNULL":        {arguments: []knownValueType{knownValueString, knownValueNumeric, knownValueString, knownValueNumeric}, minimum: 3, rejectNulls: true},
	"DT_CELL_GETS":          {arguments: []knownValueType{knownValueString, knownValueNumeric, knownValueString, knownValueNumeric}, minimum: 3, rejectNulls: true},
	"DT_FROMXML":            {arguments: []knownValueType{knownValueString, knownValueString, knownValueString}},
	"SAVETEXT":              {arguments: []knownValueType{knownValueString, knownValueUnknown, knownValueNumeric, knownValueNumeric}, minimum: 2, rejectNulls: true},
	"LOADTEXT":              {arguments: []knownValueType{knownValueUnknown, knownValueNumeric, knownValueNumeric}, minimum: 1, rejectNulls: true},
	"SETVAR":                {arguments: []knownValueType{knownValueString, knownValueUnknown}},
	"VARSETEX":              {arguments: []knownValueType{knownValueString, knownValueUnknown, knownValueNumeric, knownValueNumeric, knownValueNumeric}, minimum: 2, rejectNulls: true},
	"ENUMFILES":             {minimum: 1, validation: methodValidationEnumFiles},
	"DT_COLUMN_ADD":         {arguments: []knownValueType{knownValueString, knownValueString, knownValueUnknown, knownValueNumeric}, minimum: 2, rejectNulls: true},
	"DT_ROW_ADD":            {minimum: 1, validation: methodValidationDataTableRowAdd},
	"DT_ROW_REMOVE":         {minimum: 2, validation: methodValidationDataTableRowRemove},
	"DT_CELL_SET":           {arguments: []knownValueType{knownValueString, knownValueNumeric, knownValueString, knownValueUnknown, knownValueNumeric}, minimum: 3, rejectNulls: true, nullable: []bool{false, false, false, true}},
	"DT_SELECT":             {minimum: 1, validation: methodValidationDataTableSelect},
	"XML_GET":               {minimum: 2, validation: methodValidationXMLGet},
	"XML_GET_BYNAME":        {minimum: 2, validation: methodValidationXMLGet},
	"XML_ADDNODE":           {minimum: 3, validation: methodValidationXMLAddNode},
	"XML_ADDNODE_BYNAME":    {minimum: 3, validation: methodValidationXMLAddNode},
	"XML_REMOVENODE":        {minimum: 2, validation: methodValidationXMLRemoveNode},
	"XML_REMOVENODE_BYNAME": {minimum: 2, validation: methodValidationXMLRemoveNode},
	"DT_TOXML":              {minimum: 1, validation: methodValidationDataTableToXML},
	"MOVETEXTBOX":           {arguments: []knownValueType{knownValueNumeric, knownValueNumeric, knownValueNumeric}},
	"RESUMETEXTBOX":         {arguments: []knownValueType{}},
	"GETMETH":               {minimum: 1, validation: methodValidationDynamicMethod},
	"GETMETHS":              {minimum: 1, validation: methodValidationDynamicMethod},
	"GGETFONT":              {arguments: []knownValueType{knownValueNumeric}},
	"GGETFONTSIZE":          {arguments: []knownValueType{knownValueNumeric}},
	"GGETFONTSTYLE":         {arguments: []knownValueType{knownValueNumeric}},
	"GGETBRUSH":             {arguments: []knownValueType{knownValueNumeric}},
	"GGETPEN":               {arguments: []knownValueType{knownValueNumeric}},
	"GGETPENWIDTH":          {arguments: []knownValueType{knownValueNumeric}},
	"SPRITEDISPOSEALL":      {arguments: []knownValueType{knownValueNumeric}},
	"GSETFONT": {variants: [][]knownValueType{
		{knownValueNumeric, knownValueString, knownValueNumeric},
		{knownValueNumeric, knownValueString, knownValueNumeric, knownValueNumeric},
	}},
	"GSETPEN": {arguments: []knownValueType{knownValueNumeric, knownValueNumeric, knownValueNumeric}},
}

func (p *formatter) methodSignature(command string) (methodSignature, bool) {
	if p.dialect == EMEE {
		if signature, ok := modernMethodSignatures[command]; ok {
			return signature, true
		}
	}
	if signature, ok := simpleMethodSignatures[command]; ok {
		return signature, true
	}
	return methodSignature{}, false
}

func expressionNeedsDeferral(tokens []exprToken, dialect Dialect) bool {
	return commandMacroPossibleForDialect(tokens, dialect) || containsExpressionMarker(tokens) || containsExpressionForm(tokens)
}

func (p *formatter) methodArgumentParts(start, end int) ([]exprToken, []syntaxSpan, error) {
	tokens, err := p.lexSyntaxRange(start, end)
	if err != nil {
		return nil, nil, err
	}
	parts := p.splitTopLevel(p.src, start, end, ',')
	if len(parts) > 0 {
		left, right := extraFieldBoundsWithSpace(p.src, parts[len(parts)-1], p.allowFullwidthSpace)
		if left == right {
			parts = parts[:len(parts)-1]
		}
	}
	return tokens, parts, nil
}

func (p *formatter) simpleMethodArgumentsValid(command string, start, end int) (int, string, bool) {
	signature, registered := p.methodSignature(command)
	if !registered || !p.expressionParseEnabled() {
		return 0, "", true
	}
	if signature.matrix != methodMatrixNone {
		if p.trimInputLeft(start, end) == end {
			return start, "method argument count does not match", false
		}
		tokens, parts, err := p.methodArgumentParts(start, end)
		if err != nil {
			return 0, "", true
		}
		if containsExpressionMarker(tokens) || containsExpressionForm(tokens) {
			return 0, "", true
		}
		if commandMacroPossibleForDialect(tokens, p.dialect) {
			return 0, "", true
		}
		return p.methodMatrixArgumentsValid(signature.matrix, signature.matrixNullable, parts, start)
	}
	minimum, optional := signature.minimum, signature.minimum > 0 || signature.minimumSet
	if !optional {
		minimum = len(signature.arguments)
	}
	if p.trimInputLeft(start, end) == end {
		if signature.variadic != knownValueUnknown || signature.sameAsFirst {
			return start, "method requires an argument", false
		}
		if len(signature.variants) != 0 {
			return p.methodVariantsValid(signature.variants, nil, start)
		}
		if signature.validation != methodValidationNone && minimum > 0 {
			return start, "method requires an argument", false
		}
		if len(signature.arguments) == 0 || minimum == 0 {
			return 0, "", true
		}
		return start, "method requires an argument", false
	}
	tokens, parts, err := p.methodArgumentParts(start, end)
	if err != nil {
		return 0, "", true
	}
	if signature.variadic != knownValueUnknown || signature.sameAsFirst {
		return p.methodVariadicValid(signature, parts, start)
	}
	if commandMacroPossibleForDialect(tokens, p.dialect) {
		return 0, "", true
	}
	for _, token := range tokens {
		if token.kind == exprMarker || token.kind == exprForm {
			return 0, "", true
		}
	}
	if signature.validation != methodValidationNone {
		switch signature.validation {
		case methodValidationDataTableRowAdd:
			return p.dataTableRowAddArgumentsValid(parts, start)
		case methodValidationDataTableRowRemove:
			return p.dataTableRowRemoveArgumentsValid(parts, start)
		case methodValidationDataTableToXML:
			return p.dataTableToXMLArgumentsValid(parts, start)
		case methodValidationEnumFiles:
			return p.enumFilesArgumentsValid(parts, start)
		case methodValidationDynamicMethod:
			return p.dynamicMethodArgumentsValid(command, parts, start)
		case methodValidationRand:
			return p.randArgumentsValid(parts, start)
		case methodValidationGetNum:
			return p.getNumArgumentsValid(parts, start)
		case methodValidationFindElement:
			return p.findElementArgumentsValid(parts, start)
		case methodValidationArrayMultiSort:
			return p.arrayMultiSortArgumentsValid(parts, start)
		case methodValidationSumArray:
			return p.sumArrayArgumentsValid(command, parts, start)
		case methodValidationMatch:
			return p.matchArgumentsValid(command, parts, start)
		case methodValidationInRangeArray:
			return p.inRangeArrayArgumentsValid(command, parts, start)
		case methodValidationStrJoin:
			return p.strJoinArgumentsValid(parts, start)
		case methodValidationRegexpMatch:
			return p.regexpMatchArgumentsValid(parts, start)
		case methodValidationGetBit:
			return p.getBitArgumentsValid(parts, start)
		case methodValidationArrayExtrema:
			return p.arrayExtremaArgumentsValid(command, parts, start)
		case methodValidationDataTableSelect:
			return p.dataTableSelectArgumentsValid(parts, start)
		case methodValidationXMLGet:
			return p.xmlGetArgumentsValid(parts, start)
		case methodValidationXMLAddNode:
			return p.xmlAddNodeArgumentsValid(command, parts, start)
		case methodValidationXMLRemoveNode:
			return p.xmlRemoveNodeArgumentsValid(command, parts, start)
		case methodValidationFindChara:
			return p.findCharaArgumentsValid(parts, start)
		case methodValidationGraphicsDrawText:
			return p.graphicsDrawTextArgumentsValid(parts, start)
		case methodValidationGraphicsDrawGWithRotate:
			return p.graphicsDrawGWithRotateArgumentsValid(parts, start)
		}
	}
	if command == "REPLACE" && p.dialect == EMEE && len(parts) == 4 {
		return p.replaceArgumentsValid(parts)
	}
	if len(signature.variants) != 0 {
		return p.methodVariantsValid(signature.variants, parts, start)
	}
	return p.methodVariantValid(signature.arguments, minimum, parts, start, signature.rejectNulls, signature.nullable)
}

func (p *formatter) findCharaArgumentsValid(parts []syntaxSpan, start int) (int, string, bool) {
	if len(parts) < 2 || len(parts) > 4 {
		return start, "method argument count does not match", false
	}
	if offset, message, valid := p.methodReferenceRoleValid(parts[0], methodReferenceCharacter, true); !valid {
		return offset, message, false
	}
	if offset, message, valid := p.methodReferenceSameTypeValid(parts[1], p.methodReferenceValueType(parts[0]), true); !valid {
		return offset, message, false
	}
	for _, field := range parts[2:] {
		if offset, message, valid := p.methodFieldTypeValid(field, knownValueNumeric, false); !valid {
			return offset, message, false
		}
	}
	return 0, "", true
}

func (p *formatter) graphicsDrawTextArgumentsValid(parts []syntaxSpan, start int) (int, string, bool) {
	if len(parts) < 2 || len(parts) > 4 {
		return start, "method argument count does not match", false
	}
	if offset, message, valid := p.methodFieldTypeValid(parts[0], knownValueNumeric, true); !valid {
		return offset, message, false
	}
	if offset, message, valid := p.methodFieldTypeValid(parts[1], knownValueString, true); !valid {
		return offset, message, false
	}
	if len(parts) == 3 {
		return p.methodFieldTypeValid(parts[2], knownValueNumeric, false)
	}
	for _, field := range parts[2:] {
		if offset, message, valid := p.methodFieldTypeValid(field, knownValueNumeric, true); !valid {
			return offset, message, false
		}
	}
	return 0, "", true
}

func (p *formatter) graphicsDrawGWithRotateArgumentsValid(parts []syntaxSpan, start int) (int, string, bool) {
	if len(parts) != 3 && len(parts) != 5 {
		return start, "method argument count does not match", false
	}
	for _, field := range parts {
		if offset, message, valid := p.methodFieldTypeValid(field, knownValueNumeric, true); !valid {
			return offset, message, false
		}
	}
	return 0, "", true
}

func (p *formatter) methodVariantsValid(variants [][]knownValueType, parts []syntaxSpan, start int) (int, string, bool) {
	matched := false
	var firstOffset int
	var firstMessage string
	for _, arguments := range variants {
		if len(arguments) != len(parts) {
			continue
		}
		matched = true
		offset, message, valid := p.methodVariantValid(arguments, len(arguments), parts, start, true, nil)
		if valid {
			return 0, "", true
		}
		if firstMessage == "" {
			firstOffset, firstMessage = offset, message
		}
	}
	if matched {
		return firstOffset, firstMessage, false
	}
	return start, "method argument count does not match", false
}

func (p *formatter) methodVariadicValid(signature methodSignature, parts []syntaxSpan, start int) (int, string, bool) {
	uncertain := false
	first := knownValueUnknown
	for n, field := range parts {
		left, right := extraFieldBoundsWithSpace(p.src, field, p.allowFullwidthSpace)
		if left == right {
			if uncertain {
				return 0, "", true
			}
			return left, "method contains an omitted argument", false
		}
		tokens, err := p.lexSyntaxRange(left, right)
		if err != nil || expressionNeedsDeferral(tokens, p.dialect) {
			uncertain = true
			continue
		}
		if !uncertain && builtinScalarIndexesMissing(tokens, p.dialect) {
			return left, "scalar variable requires indexes", false
		}
		actual := knownExpressionTypeForDialect(tokens, p.dialect)
		if actual == knownValueUnknown {
			uncertain = true
			continue
		}
		if signature.sameAsFirst {
			if n == 0 {
				first = actual
				continue
			}
			if first != knownValueUnknown && actual != first {
				if uncertain {
					return 0, "", true
				}
				return left, "method argument has an incompatible type", false
			}
			continue
		}
		if !commandTypeMatches(signature.variadic, actual) {
			if uncertain {
				return 0, "", true
			}
			return left, "method argument has an incompatible type", false
		}
	}
	if len(parts) < signature.minimum {
		if uncertain {
			return 0, "", true
		}
		return start, "method argument count does not match", false
	}
	if uncertain {
		return 0, "", true
	}
	return 0, "", true
}

func (p *formatter) methodMatrixArgumentsValid(kind methodMatrixKind, nullable bool, parts []syntaxSpan, start int) (int, string, bool) {
	if kind == methodMatrixSprite {
		if len(parts) != 2 && len(parts) != 4 && len(parts) != 6 && len(parts) != 7 {
			return start, "method argument count does not match", false
		}
		expected := []knownValueType{knownValueNumeric, knownValueString}
		for len(expected) < len(parts) && len(expected) < 6 {
			expected = append(expected, knownValueNumeric, knownValueNumeric)
		}
		if len(parts) == 7 {
			expected = append(expected, knownValueUnknown)
		}
		return p.methodMatrixFieldsValid(expected, nullable, parts, start)
	}
	if len(parts) != 10 && len(parts) != 11 {
		return start, "method argument count does not match", false
	}
	expected := make([]knownValueType, 10)
	for i := range expected {
		expected[i] = knownValueNumeric
	}
	if len(parts) == 11 {
		expected = append(expected, knownValueUnknown)
	}
	return p.methodMatrixFieldsValid(expected, nullable, parts, start)
}

func (p *formatter) methodMatrixFieldsValid(expected []knownValueType, nullable bool, parts []syntaxSpan, start int) (int, string, bool) {
	for n, field := range parts {
		left, right := extraFieldBoundsWithSpace(p.src, field, p.allowFullwidthSpace)
		if left == right {
			if nullable && n == len(parts)-1 && expected[n] == knownValueUnknown {
				return 0, "", true
			}
			return left, "method contains an omitted argument", false
		}
		tokens, err := p.lexSyntaxRange(left, right)
		if err != nil || expressionNeedsDeferral(tokens, p.dialect) {
			return 0, "", true
		}
		if expected[n] == knownValueUnknown {
			if matrixVariableKnown, matrixValid := p.methodMatrixVariableValid(tokens); matrixVariableKnown {
				if !matrixValid {
					return left, "method argument has an incompatible type", false
				}
			}
			continue
		}
		if builtinScalarIndexesMissing(tokens, p.dialect) {
			return left, "scalar variable requires indexes", false
		}
		actual := knownExpressionTypeForDialect(tokens, p.dialect)
		if actual == knownValueUnknown {
			continue
		}
		if !commandTypeMatches(expected[n], actual) {
			return left, "method argument has an incompatible type", false
		}
	}
	return 0, "", true
}

func (p *formatter) getBitArgumentsValid(parts []syntaxSpan, start int) (int, string, bool) {
	if len(parts) != 2 {
		return start, "method argument count does not match", false
	}
	for _, field := range parts {
		if offset, message, valid := p.methodFieldTypeValid(field, knownValueNumeric, true); !valid {
			return offset, message, false
		}
	}
	left, right := extraFieldBoundsWithSpace(p.src, parts[1], p.allowFullwidthSpace)
	tokens, err := p.lexSyntaxRange(left, right)
	if err != nil {
		return 0, "", true
	}
	value, kind := directIntegerLiteral(tokens)
	if kind != knownValueNumeric {
		return 0, "", true
	}
	if value < 0 || value > 63 {
		return left, "bit index literal is outside 0..63", false
	}
	return 0, "", true
}

func (p *formatter) methodMatrixVariableValid(tokens []exprToken) (bool, bool) {
	name, ok := matrixBuiltinName(tokens)
	if !ok {
		if knownExpressionTypeForDialect(tokens, p.dialect) != knownValueUnknown {
			return true, false
		}
		return false, false
	}
	info, known := builtinVariableInfoByName(name, p.dialect)
	if !known {
		return false, false
	}
	return matrixBuiltinIndexesValid(tokens, info)
}

func matrixBuiltinName(tokens []exprToken) (string, bool) {
	tokens = stripExpressionParens(tokens)
	if len(tokens) > 0 && tokens[0].kind == exprOperator && string(tokens[0].text) == "+" {
		tokens = stripExpressionParens(tokens[1:])
	}
	if len(tokens) == 0 || tokens[0].kind != exprIdentifier {
		return "", false
	}
	if len(tokens) == 1 {
		return string(tokens[0].text), true
	}
	if string(tokens[1].text) != ":" {
		return "", false
	}
	return string(tokens[0].text), true
}

func matrixBuiltinIndexesValid(tokens []exprToken, info builtinVariable) (bool, bool) {
	if info.value != knownValueNumeric || (info.rank != 2 && info.rank != 3) {
		return true, false
	}
	tokens = stripExpressionParens(tokens)
	if len(tokens) == 1 {
		return true, true
	}
	if len(tokens) < 3 || string(tokens[1].text) != ":" {
		return false, false
	}
	start, depth, indexes := 2, 0, 0
	for i := 2; i <= len(tokens); i++ {
		boundary := i == len(tokens)
		if i < len(tokens) {
			switch string(tokens[i].text) {
			case "(":
				depth++
			case ")":
				if depth > 0 {
					depth--
				}
			case ":":
				if depth == 0 {
					boundary = true
				}
			}
		}
		if !boundary {
			continue
		}
		if start == i {
			return true, false
		}
		if _, kind := directIntegerLiteral(tokens[start:i]); kind != knownValueNumeric {
			return false, false
		}
		indexes++
		start = i + 1
	}
	if indexes == int(info.rank) {
		return true, !info.character
	}
	if info.character && indexes == int(info.rank)+1 {
		return true, true
	}
	return true, false
}

func (p *formatter) methodVariantValid(arguments []knownValueType, minimum int, parts []syntaxSpan, start int, rejectNulls bool, nullable []bool) (int, string, bool) {
	if len(parts) < minimum || len(parts) > len(arguments) {
		return start, "method argument count does not match", false
	}
	for n, field := range parts {
		left, right := extraFieldBoundsWithSpace(p.src, field, p.allowFullwidthSpace)
		if left == right {
			if n >= minimum {
				if rejectNulls && (n >= len(nullable) || !nullable[n]) {
					return left, "method contains an omitted argument", false
				}
				continue
			}
			return left, "method contains an omitted argument", false
		}
		tokens, err := p.lexSyntaxRange(left, right)
		if err != nil {
			return 0, "", true
		}
		if expressionNeedsDeferral(tokens, p.dialect) {
			return 0, "", true
		}
		if builtinScalarIndexesMissing(tokens, p.dialect) {
			return left, "scalar variable requires indexes", false
		}
		if !commandTypeMatches(arguments[n], knownExpressionTypeForDialect(tokens, p.dialect)) {
			return left, "method argument has an incompatible type", false
		}
	}
	return 0, "", true
}

func (p *formatter) dataTableRowAddArgumentsValid(parts []syntaxSpan, start int) (int, string, bool) {
	if len(parts) == 0 {
		return start, "method requires an argument", false
	}
	if offset, message, valid := p.methodFieldTypeValid(parts[0], knownValueString, true); !valid {
		return offset, message, false
	}
	if len(parts) == 1 {
		return 0, "", true
	}
	if len(parts) == 4 {
		if offset, message, valid := p.methodFieldTypeValid(parts[3], knownValueNumeric, true); !valid {
			return offset, message, false
		}
		for n, field := range parts[1:3] {
			left, right := extraFieldBoundsWithSpace(p.src, field, p.allowFullwidthSpace)
			if left == right {
				return left, "method contains an omitted argument", false
			}
			tokens, err := p.lexSyntaxRange(left, right)
			if err != nil {
				return 0, "", true
			}
			expected := knownValueUnknown
			if n == 0 {
				expected = knownValueString
			}
			if known, valid := p.methodReferenceFieldValid(tokens, expected, true); known && !valid {
				return left, "method argument has an incompatible type", false
			}
		}
		return 0, "", true
	}
	if (len(parts)-1)%2 != 0 {
		return start, "method argument count does not match", false
	}
	for n := 1; n < len(parts); n += 2 {
		if offset, message, valid := p.methodFieldTypeValid(parts[n], knownValueString, true); !valid {
			return offset, message, false
		}
		if offset, message, valid := p.methodFieldTypeValid(parts[n+1], knownValueUnknown, false); !valid {
			return offset, message, false
		}
	}
	return 0, "", true
}

func (p *formatter) dataTableRowRemoveArgumentsValid(parts []syntaxSpan, start int) (int, string, bool) {
	if len(parts) != 2 && len(parts) != 3 {
		return start, "method argument count does not match", false
	}
	if offset, message, valid := p.methodFieldTypeValid(parts[0], knownValueString, true); !valid {
		return offset, message, false
	}
	if len(parts) == 2 {
		return p.methodFieldTypeValid(parts[1], knownValueNumeric, true)
	}
	if offset, message, valid := p.methodReferenceArgumentValid(parts[1], knownValueNumeric, true, true); !valid {
		return offset, message, false
	}
	return p.methodFieldTypeValid(parts[2], knownValueNumeric, true)
}

func (p *formatter) dataTableToXMLArgumentsValid(parts []syntaxSpan, start int) (int, string, bool) {
	if len(parts) != 1 && len(parts) != 2 {
		return start, "method argument count does not match", false
	}
	if offset, message, valid := p.methodFieldTypeValid(parts[0], knownValueString, true); !valid {
		return offset, message, false
	}
	if len(parts) == 2 {
		return p.methodReferenceArgumentValid(parts[1], knownValueString, true, true)
	}
	return 0, "", true
}

func (p *formatter) enumFilesArgumentsValid(parts []syntaxSpan, start int) (int, string, bool) {
	if len(parts) == 0 || len(parts) > 4 {
		return start, "method argument count does not match", false
	}
	if offset, message, valid := p.methodFieldTypeValid(parts[0], knownValueString, true); !valid {
		return offset, message, false
	}
	if len(parts) > 1 {
		if offset, message, valid := p.methodFieldTypeValid(parts[1], knownValueString, true); !valid {
			return offset, message, false
		}
	}
	if len(parts) > 2 {
		if offset, message, valid := p.methodFieldTypeValid(parts[2], knownValueNumeric, true); !valid {
			return offset, message, false
		}
	}
	if len(parts) > 3 {
		return p.methodReferenceArgumentValid(parts[3], knownValueString, true, true)
	}
	return 0, "", true
}

func (p *formatter) dynamicMethodArgumentsValid(command string, parts []syntaxSpan, start int) (int, string, bool) {
	if len(parts) == 0 {
		return start, "method requires an argument", false
	}
	if offset, message, valid := p.methodFieldTypeValid(parts[0], knownValueString, true); !valid {
		return offset, message, false
	}
	if len(parts) < 2 {
		return 0, "", true
	}
	want := knownValueNumeric
	if command == "GETMETHS" {
		want = knownValueString
	}
	return p.methodFieldTypeValid(parts[1], want, false)
}

type methodReferenceRole uint8

const (
	methodReferenceAny methodReferenceRole = iota
	methodReferenceOneDimensional
	methodReferenceArray
	methodReferenceNumericArray
	methodReferenceNumericOneDimensional
	methodReferenceCharacter
	methodReferenceCharacterNumeric
	methodReferenceCharacterNumericOneDimensional
	methodReferenceMutableArray
	methodReferenceMutableOneDimensional
	methodReferenceMutableNumericScalar
	methodReferenceMutableNumericOneDimensional
	methodReferenceMutableStringOneDimensional
	methodReferenceMutableString
)

func (p *formatter) methodReferenceInfo(tokens []exprToken) (builtinVariable, bool) {
	tokens = stripExpressionParens(tokens)
	if len(tokens) > 0 && tokens[0].kind == exprOperator && string(tokens[0].text) == "+" {
		tokens = stripExpressionParens(tokens[1:])
	}
	if !variableArgumentTokens(tokens) || len(tokens) == 0 || tokens[0].kind != exprIdentifier {
		return builtinVariable{}, false
	}
	if len(tokens) > 1 && string(tokens[1].text) == "@" {
		return builtinVariable{}, false
	}
	name := string(tokens[0].text)
	if reservedBuiltinMethod(name) {
		return builtinVariable{}, false
	}
	info, known := builtinVariableInfoByName(name, p.dialect)
	if !known {
		return builtinVariable{}, false
	}
	return info, true
}

func (p *formatter) methodReferenceRoleValid(field syntaxSpan, role methodReferenceRole, required bool) (int, string, bool) {
	left, right := extraFieldBoundsWithSpace(p.src, field, p.allowFullwidthSpace)
	if left == right {
		if required {
			return left, "method contains an omitted argument", false
		}
		return 0, "", true
	}
	tokens, err := p.lexSyntaxRange(left, right)
	if err != nil || expressionNeedsDeferral(tokens, p.dialect) {
		return 0, "", true
	}
	if _, valid, proven := indexedBuiltinType(stripExpressionParens(tokens), p.dialect); proven && !valid {
		return left, "method argument has an incompatible type", false
	}
	if role == methodReferenceMutableNumericScalar && builtinScalarIndexesMissing(tokens, p.dialect) {
		return left, "scalar variable requires indexes", false
	}
	info, known := p.methodReferenceInfo(tokens)
	if !known {
		trimmed := stripExpressionParens(tokens)
		if len(trimmed) > 0 && trimmed[0].kind == exprIdentifier && (len(trimmed) < 2 || string(trimmed[1].text) != "@") {
			if _, builtin := builtinVariableInfoByName(string(trimmed[0].text), p.dialect); builtin && !variableArgumentTokens(tokens) {
				return left, "method argument has an incompatible type", false
			}
		}
		if knownExpressionTypeForDialect(tokens, p.dialect) != knownValueUnknown {
			return left, "method argument has an incompatible type", false
		}
		return 0, "", true
	}
	valid := true
	switch role {
	case methodReferenceOneDimensional:
		valid = info.rank == 1
	case methodReferenceArray:
		valid = info.rank >= 1
	case methodReferenceNumericArray:
		valid = info.value == knownValueNumeric && info.rank >= 1
	case methodReferenceNumericOneDimensional:
		valid = info.value == knownValueNumeric && info.rank == 1
	case methodReferenceCharacter:
		valid = info.character
	case methodReferenceCharacterNumeric:
		valid = info.character && info.value == knownValueNumeric
	case methodReferenceCharacterNumericOneDimensional:
		valid = info.character && info.value == knownValueNumeric && info.rank == 1
	case methodReferenceMutableArray:
		valid = !info.character && !info.readOnly && info.rank >= 1
	case methodReferenceMutableOneDimensional:
		valid = !info.character && !info.readOnly && info.rank == 1
	case methodReferenceMutableNumericScalar:
		valid = info.value == knownValueNumeric && !info.readOnly
	case methodReferenceMutableNumericOneDimensional:
		valid = !info.character && info.value == knownValueNumeric && !info.readOnly && info.rank == 1
	case methodReferenceMutableStringOneDimensional:
		valid = !info.character && info.value == knownValueString && !info.readOnly && info.rank == 1
	case methodReferenceMutableString:
		valid = info.value == knownValueString && !info.readOnly
	}
	if !valid {
		return left, "method argument has an incompatible type", false
	}
	return 0, "", true
}

func (p *formatter) methodReferenceSameTypeValid(field syntaxSpan, want knownValueType, required bool) (int, string, bool) {
	left, right := extraFieldBoundsWithSpace(p.src, field, p.allowFullwidthSpace)
	if left == right {
		if required {
			return left, "method contains an omitted argument", false
		}
		return 0, "", true
	}
	tokens, err := p.lexSyntaxRange(left, right)
	if err != nil || expressionNeedsDeferral(tokens, p.dialect) {
		return 0, "", true
	}
	if builtinScalarIndexesMissing(tokens, p.dialect) {
		return left, "scalar variable requires indexes", false
	}
	actual := knownExpressionTypeForDialect(tokens, p.dialect)
	if want != knownValueUnknown && actual != knownValueUnknown && actual != want {
		return left, "method argument has an incompatible type", false
	}
	return 0, "", true
}

func (p *formatter) randArgumentsValid(parts []syntaxSpan, start int) (int, string, bool) {
	if len(parts) < 1 || len(parts) > 2 {
		return start, "method argument count does not match", false
	}
	if len(parts) == 2 {
		if left, right := extraFieldBoundsWithSpace(p.src, parts[0], p.allowFullwidthSpace); left == right {
			secondLeft, secondRight := extraFieldBoundsWithSpace(p.src, parts[1], p.allowFullwidthSpace)
			if secondLeft != secondRight {
				if tokens, err := p.lexSyntaxRange(secondLeft, secondRight); err == nil && bareRandReference(tokens) {
					return secondLeft, "method argument has an incompatible type", false
				}
			}
			return p.methodFieldTypeValid(parts[1], knownValueNumeric, true)
		}
	}
	for _, field := range parts {
		left, right := extraFieldBoundsWithSpace(p.src, field, p.allowFullwidthSpace)
		if left != right {
			tokens, err := p.lexSyntaxRange(left, right)
			if err == nil && bareRandReference(tokens) {
				return left, "method argument has an incompatible type", false
			}
		}
		if offset, message, valid := p.methodFieldTypeValid(field, knownValueNumeric, true); !valid {
			return offset, message, false
		}
	}
	return 0, "", true
}

func bareRandReference(tokens []exprToken) bool {
	return len(tokens) > 0 && tokens[0].kind == exprIdentifier && string(tokens[0].text) == "RAND" && (len(tokens) == 1 || string(tokens[1].text) != "(")
}

func (p *formatter) getNumArgumentsValid(parts []syntaxSpan, start int) (int, string, bool) {
	maximum := 2
	if p.dialect == EMEE {
		maximum = 3
	}
	if len(parts) < 2 || len(parts) > maximum {
		return start, "method argument count does not match", false
	}
	if offset, message, valid := p.methodReferenceRoleValid(parts[0], methodReferenceAny, true); !valid {
		return offset, message, false
	}
	if offset, message, valid := p.methodFieldTypeValid(parts[1], knownValueString, true); !valid {
		return offset, message, false
	}
	if len(parts) == 3 {
		return p.methodFieldTypeValid(parts[2], knownValueNumeric, true)
	}
	return 0, "", true
}

func (p *formatter) findElementArgumentsValid(parts []syntaxSpan, start int) (int, string, bool) {
	return p.methodReferenceSameTypeNumericTailValid(parts, start, 5, methodReferenceOneDimensional)
}

func (p *formatter) methodReferenceValueType(field syntaxSpan) knownValueType {
	left, right := extraFieldBoundsWithSpace(p.src, field, p.allowFullwidthSpace)
	if left == right {
		return knownValueUnknown
	}
	tokens, err := p.lexSyntaxRange(left, right)
	if err != nil || expressionNeedsDeferral(tokens, p.dialect) {
		return knownValueUnknown
	}
	if info, known := p.methodReferenceInfo(tokens); known {
		return info.value
	}
	return knownExpressionTypeForDialect(tokens, p.dialect)
}

func (p *formatter) arrayMultiSortArgumentsValid(parts []syntaxSpan, start int) (int, string, bool) {
	if len(parts) < 2 {
		return start, "method argument count does not match", false
	}
	for n, field := range parts {
		role := methodReferenceMutableArray
		if n == 0 {
			role = methodReferenceMutableOneDimensional
		} else if offset, message, valid := p.emueraArrayMultiSortSecondaryValid(field); !valid {
			return offset, message, false
		}
		if offset, message, valid := p.methodReferenceRoleValid(field, role, true); !valid {
			return offset, message, false
		}
	}
	return 0, "", true
}

func (p *formatter) emueraArrayMultiSortSecondaryValid(field syntaxSpan) (int, string, bool) {
	if p.dialect != Emuera1824 {
		return 0, "", true
	}
	left, right := extraFieldBoundsWithSpace(p.src, field, p.allowFullwidthSpace)
	if left == right {
		return 0, "", true
	}
	tokens, err := p.lexSyntaxRange(left, right)
	if err != nil || expressionNeedsDeferral(tokens, p.dialect) {
		return 0, "", true
	}
	info, known := builtinVariableRoleInfo(tokens, p.dialect)
	if known && info.rank == 3 {
		return left, "method argument has an incompatible type", false
	}
	return 0, "", true
}

func (p *formatter) sumArrayArgumentsValid(command string, parts []syntaxSpan, start int) (int, string, bool) {
	role := methodReferenceNumericArray
	if command == "SUMCARRAY" {
		role = methodReferenceCharacterNumeric
	}
	return p.methodReferenceNumericTailValid(parts, start, role)
}

func (p *formatter) arrayExtremaArgumentsValid(command string, parts []syntaxSpan, start int) (int, string, bool) {
	role := methodReferenceNumericOneDimensional
	if strings.HasSuffix(command, "CARRAY") {
		role = methodReferenceCharacterNumericOneDimensional
	}
	return p.methodReferenceNumericTailValid(parts, start, role)
}

func (p *formatter) matchArgumentsValid(command string, parts []syntaxSpan, start int) (int, string, bool) {
	role := methodReferenceOneDimensional
	if command == "CMATCH" {
		role = methodReferenceCharacter
	}
	return p.methodReferenceSameTypeNumericTailValid(parts, start, 4, role)
}

func (p *formatter) methodReferenceNumericTailValid(parts []syntaxSpan, start int, role methodReferenceRole) (int, string, bool) {
	if len(parts) < 1 || len(parts) > 3 {
		return start, "method argument count does not match", false
	}
	if offset, message, valid := p.methodReferenceRoleValid(parts[0], role, true); !valid {
		return offset, message, false
	}
	for _, field := range parts[1:] {
		if offset, message, valid := p.methodFieldTypeValid(field, knownValueNumeric, false); !valid {
			return offset, message, false
		}
	}
	return 0, "", true
}

func (p *formatter) methodReferenceSameTypeNumericTailValid(parts []syntaxSpan, start, maximum int, role methodReferenceRole) (int, string, bool) {
	if len(parts) < 2 || len(parts) > maximum {
		return start, "method argument count does not match", false
	}
	if offset, message, valid := p.methodReferenceRoleValid(parts[0], role, true); !valid {
		return offset, message, false
	}
	if offset, message, valid := p.methodReferenceSameTypeValid(parts[1], p.methodReferenceValueType(parts[0]), true); !valid {
		return offset, message, false
	}
	for _, field := range parts[2:] {
		if offset, message, valid := p.methodFieldTypeValid(field, knownValueNumeric, false); !valid {
			return offset, message, false
		}
	}
	return 0, "", true
}

func (p *formatter) inRangeArrayArgumentsValid(command string, parts []syntaxSpan, start int) (int, string, bool) {
	maximum := 5
	if p.dialect == Emuera1824 {
		maximum = 6
	}
	if len(parts) < 3 || len(parts) > maximum {
		return start, "method argument count does not match", false
	}
	role := methodReferenceNumericOneDimensional
	if command == "INRANGECARRAY" {
		role = methodReferenceCharacterNumeric
		if p.dialect == EMEE {
			role = methodReferenceCharacterNumericOneDimensional
		}
	}
	if offset, message, valid := p.methodReferenceRoleValid(parts[0], role, true); !valid {
		return offset, message, false
	}
	for n, field := range parts[1:] {
		if p.dialect == Emuera1824 && n >= 4 {
			continue
		}
		required := n < 2
		if offset, message, valid := p.methodFieldTypeValid(field, knownValueNumeric, required); !valid {
			return offset, message, false
		}
	}
	return 0, "", true
}

func (p *formatter) strJoinArgumentsValid(parts []syntaxSpan, start int) (int, string, bool) {
	if len(parts) < 1 || len(parts) > 4 {
		return start, "method argument count does not match", false
	}
	if offset, message, valid := p.methodReferenceRoleValid(parts[0], methodReferenceArray, true); !valid {
		return offset, message, false
	}
	if offset, message, valid := p.strJoinCharacterRangeValid(parts); !valid {
		return offset, message, false
	}
	if len(parts) > 1 {
		if offset, message, valid := p.methodFieldTypeValid(parts[1], knownValueString, false); !valid {
			return offset, message, false
		}
	}
	if len(parts) > 2 {
		for _, field := range parts[2:] {
			if offset, message, valid := p.methodFieldTypeValid(field, knownValueNumeric, false); !valid {
				return offset, message, false
			}
		}
	}
	return 0, "", true
}

func (p *formatter) strJoinCharacterRangeValid(parts []syntaxSpan) (int, string, bool) {
	left, right := extraFieldBoundsWithSpace(p.src, parts[0], p.allowFullwidthSpace)
	if left == right {
		return 0, "", true
	}
	tokens, err := p.lexSyntaxRange(left, right)
	if err != nil || expressionNeedsDeferral(tokens, p.dialect) {
		return 0, "", true
	}
	normalized := stripExpressionParens(tokens)
	if len(normalized) > 0 && normalized[0].kind == exprOperator && string(normalized[0].text) == "+" {
		normalized = stripExpressionParens(normalized[1:])
	}
	info, known := p.methodReferenceInfo(normalized)
	if !known || !info.character {
		return 0, "", true
	}
	if len(normalized) > 1 && string(normalized[1].text) == ":" {
		if _, proven := builtinVariableRoleInfo(normalized, p.dialect); !proven {
			return 0, "", true
		}
	}
	if info.value == knownValueString {
		return left, "method argument has an incompatible type", false
	}
	if len(parts) < 4 {
		return 0, "", true
	}
	lengthLeft, lengthRight := extraFieldBoundsWithSpace(p.src, parts[3], p.allowFullwidthSpace)
	if lengthLeft == lengthRight {
		return 0, "", true
	}
	lengthTokens, err := p.lexSyntaxRange(lengthLeft, lengthRight)
	if err != nil || expressionNeedsDeferral(lengthTokens, p.dialect) {
		return 0, "", true
	}
	length, known := constantIntegerValue(lengthTokens)
	if !known || length <= 0 {
		return 0, "", true
	}
	return left, "method argument has an incompatible type", false
}

func (p *formatter) regexpMatchArgumentsValid(parts []syntaxSpan, start int) (int, string, bool) {
	if len(parts) < 2 || len(parts) > 4 {
		return start, "method argument count does not match", false
	}
	for _, field := range parts[:2] {
		if offset, message, valid := p.methodFieldTypeValid(field, knownValueString, true); !valid {
			return offset, message, false
		}
	}
	if len(parts) == 2 {
		return 0, "", true
	}
	if len(parts) == 3 {
		return p.methodFieldTypeValid(parts[2], knownValueNumeric, true)
	}
	if offset, message, valid := p.methodReferenceRoleValid(parts[2], methodReferenceMutableNumericScalar, true); !valid {
		return offset, message, false
	}
	return p.methodReferenceRoleValid(parts[3], methodReferenceMutableStringOneDimensional, true)
}

func (p *formatter) dataTableSelectArgumentsValid(parts []syntaxSpan, start int) (int, string, bool) {
	if len(parts) < 1 || len(parts) > 4 {
		return start, "method argument count does not match", false
	}
	if offset, message, valid := p.methodFieldTypeValid(parts[0], knownValueString, true); !valid {
		return offset, message, false
	}
	for _, field := range parts[1:min(len(parts), 3)] {
		if offset, message, valid := p.methodFieldTypeValid(field, knownValueString, false); !valid {
			return offset, message, false
		}
	}
	if len(parts) == 4 {
		return p.methodReferenceRoleValid(parts[3], methodReferenceMutableNumericOneDimensional, true)
	}
	return 0, "", true
}

func (p *formatter) xmlGetArgumentsValid(parts []syntaxSpan, start int) (int, string, bool) {
	if len(parts) < 2 || len(parts) > 4 {
		return start, "method argument count does not match", false
	}
	if offset, message, valid := p.methodFieldTypeValid(parts[0], knownValueUnknown, true); !valid {
		return offset, message, false
	}
	if offset, message, valid := p.methodFieldTypeValid(parts[1], knownValueString, true); !valid {
		return offset, message, false
	}
	if len(parts) >= 3 {
		left, right := extraFieldBoundsWithSpace(p.src, parts[2], p.allowFullwidthSpace)
		if left == right {
			return left, "method contains an omitted argument", false
		}
		tokens, err := p.lexSyntaxRange(left, right)
		if err != nil || expressionNeedsDeferral(tokens, p.dialect) {
			return 0, "", true
		}
		actual := knownExpressionTypeForDialect(tokens, p.dialect)
		if actual == knownValueNumeric {
			if offset, message, valid := p.methodFieldTypeValid(parts[2], knownValueNumeric, true); !valid {
				return offset, message, false
			}
		} else if offset, message, valid := p.methodReferenceRoleValid(parts[2], methodReferenceMutableStringOneDimensional, true); !valid {
			return offset, message, false
		}
	}
	if len(parts) == 4 {
		return p.methodFieldTypeValid(parts[3], knownValueNumeric, true)
	}
	return 0, "", true
}

func (p *formatter) xmlTargetArgumentsValid(field syntaxSpan, byName bool) (int, string, bool) {
	if byName {
		return p.methodFieldTypeValid(field, knownValueString, true)
	}
	left, right := extraFieldBoundsWithSpace(p.src, field, p.allowFullwidthSpace)
	if left == right {
		return left, "method contains an omitted argument", false
	}
	tokens, err := p.lexSyntaxRange(left, right)
	if err != nil || expressionNeedsDeferral(tokens, p.dialect) {
		return 0, "", true
	}
	if knownExpressionTypeForDialect(tokens, p.dialect) == knownValueNumeric {
		return p.methodFieldTypeValid(field, knownValueNumeric, true)
	}
	return p.methodReferenceRoleValid(field, methodReferenceMutableString, true)
}

func (p *formatter) xmlAddNodeArgumentsValid(command string, parts []syntaxSpan, start int) (int, string, bool) {
	if len(parts) < 3 || len(parts) > 5 {
		return start, "method argument count does not match", false
	}
	if offset, message, valid := p.xmlTargetArgumentsValid(parts[0], strings.HasSuffix(command, "_BYNAME")); !valid {
		return offset, message, false
	}
	for _, field := range parts[1:3] {
		if offset, message, valid := p.methodFieldTypeValid(field, knownValueString, true); !valid {
			return offset, message, false
		}
	}
	for _, field := range parts[3:] {
		if offset, message, valid := p.methodFieldTypeValid(field, knownValueNumeric, true); !valid {
			return offset, message, false
		}
	}
	return 0, "", true
}

func (p *formatter) xmlRemoveNodeArgumentsValid(command string, parts []syntaxSpan, start int) (int, string, bool) {
	if len(parts) < 2 || len(parts) > 3 {
		return start, "method argument count does not match", false
	}
	if offset, message, valid := p.xmlTargetArgumentsValid(parts[0], strings.HasSuffix(command, "_BYNAME")); !valid {
		return offset, message, false
	}
	if offset, message, valid := p.methodFieldTypeValid(parts[1], knownValueString, true); !valid {
		return offset, message, false
	}
	if len(parts) == 3 {
		return p.methodFieldTypeValid(parts[2], knownValueNumeric, true)
	}
	return 0, "", true
}

func (p *formatter) methodFieldTypeValid(field syntaxSpan, expected knownValueType, required bool) (int, string, bool) {
	left, right := extraFieldBoundsWithSpace(p.src, field, p.allowFullwidthSpace)
	if left == right {
		if required {
			return left, "method contains an omitted argument", false
		}
		return 0, "", true
	}
	tokens, err := p.lexSyntaxRange(left, right)
	if err != nil {
		return 0, "", true
	}
	if builtinScalarIndexesMissing(tokens, p.dialect) {
		return left, "scalar variable requires indexes", false
	}
	actual := knownExpressionTypeForDialect(tokens, p.dialect)
	if actual != knownValueUnknown && !commandTypeMatches(expected, actual) {
		return left, "method argument has an incompatible type", false
	}
	return 0, "", true
}

func (p *formatter) methodReferenceArgumentValid(field syntaxSpan, expected knownValueType, required bool, requireArray bool) (int, string, bool) {
	left, right := extraFieldBoundsWithSpace(p.src, field, p.allowFullwidthSpace)
	if left == right {
		if required {
			return left, "method contains an omitted argument", false
		}
		return 0, "", true
	}
	tokens, err := p.lexSyntaxRange(left, right)
	if err != nil {
		return 0, "", true
	}
	if known, valid := p.methodReferenceFieldValid(tokens, expected, requireArray); known && !valid {
		return left, "method argument has an incompatible type", false
	}
	return 0, "", true
}

func (p *formatter) methodReferenceFieldValid(tokens []exprToken, expected knownValueType, requireArray bool) (bool, bool) {
	tokens = stripExpressionParens(tokens)
	if info, known := builtinVariableRoleInfo(tokens, p.dialect); known {
		if builtinVariableReadOnly(tokens, p.dialect) {
			return true, false
		}
		return true, !info.character && (!requireArray || info.rank == 1) && (expected == knownValueUnknown || info.value == expected)
	}
	if knownExpressionTypeForDialect(tokens, p.dialect) != knownValueUnknown {
		return true, false
	}
	return false, false
}

func (p *formatter) replaceArgumentsValid(parts []syntaxSpan) (int, string, bool) {
	mode, modeKnown := int64(0), false
	for n, field := range parts {
		left, right := extraFieldBoundsWithSpace(p.src, field, p.allowFullwidthSpace)
		if left == right {
			return left, "method contains an omitted argument", false
		}
		tokens, err := p.lexSyntaxRange(left, right)
		if err != nil || expressionNeedsDeferral(tokens, p.dialect) {
			return 0, "", true
		}
		if builtinScalarIndexesMissing(tokens, p.dialect) {
			return left, "scalar variable requires indexes", false
		}
		expected := knownValueNumeric
		if n < 3 {
			expected = knownValueString
		}
		if !commandTypeMatches(expected, knownExpressionTypeForDialect(tokens, p.dialect)) {
			return left, "method argument has an incompatible type", false
		}
		if n == 3 {
			mode, modeKnown = constantIntegerValue(tokens)
		}
	}
	if modeKnown && int32(mode) == 1 {
		return p.methodReferenceRoleValid(parts[2], methodReferenceMutableStringOneDimensional, true)
	}
	return 0, "", true
}

func containsExpressionMarker(tokens []exprToken) bool {
	for _, token := range tokens {
		if token.kind == exprMarker {
			return true
		}
	}
	return false
}

func containsExpressionForm(tokens []exprToken) bool {
	for _, token := range tokens {
		if token.kind == exprForm {
			return true
		}
	}
	return false
}
