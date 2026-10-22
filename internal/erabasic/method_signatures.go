package erabasic

import "strings"

type MethodSignature struct {
	Arguments      []KnownValueType
	Variants       [][]KnownValueType
	Variadic       KnownValueType
	Minimum        int
	MinimumSet     bool
	SameAsFirst    bool
	Matrix         MethodMatrixKind
	MatrixNullable bool
	Validation     methodValidation
	RejectNulls    bool
	Nullable       []bool
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

type MethodMatrixKind uint8

const (
	methodMatrixNone MethodMatrixKind = iota
	methodMatrixSprite
	methodMatrixGraphics
)

var simpleMethodSignatures = map[string]MethodSignature{
	"RAND":                {Minimum: 1, Validation: methodValidationRand},
	"MIN":                 {Variadic: KnownValueNumeric, Minimum: 1},
	"MAX":                 {Variadic: KnownValueNumeric, Minimum: 1},
	"GROUPMATCH":          {SameAsFirst: true, Minimum: 2},
	"NOSAMES":             {SameAsFirst: true, Minimum: 2},
	"ALLSAMES":            {SameAsFirst: true, Minimum: 2},
	"GETCHARA":            {Arguments: []KnownValueType{KnownValueNumeric, KnownValueNumeric}, Minimum: 1},
	"GETSPCHARA":          {Arguments: []KnownValueType{KnownValueNumeric}},
	"HTML_GETPRINTEDSTR":  {Arguments: []KnownValueType{KnownValueNumeric}, MinimumSet: true, RejectNulls: true},
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
	"GCREATED":            {Arguments: []KnownValueType{KnownValueNumeric}},
	"GWIDTH":              {Arguments: []KnownValueType{KnownValueNumeric}},
	"GHEIGHT":             {Arguments: []KnownValueType{KnownValueNumeric}},
	"GCREATE":             {Arguments: []KnownValueType{KnownValueNumeric, KnownValueNumeric, KnownValueNumeric}},
	"GDISPOSE":            {Arguments: []KnownValueType{KnownValueNumeric}},
	"GGETCOLOR":           {Arguments: []KnownValueType{KnownValueNumeric, KnownValueNumeric, KnownValueNumeric}},
	"GSETCOLOR":           {Arguments: []KnownValueType{KnownValueNumeric, KnownValueNumeric, KnownValueNumeric, KnownValueNumeric}},
	"GFILLRECTANGLE":      {Arguments: []KnownValueType{KnownValueNumeric, KnownValueNumeric, KnownValueNumeric, KnownValueNumeric, KnownValueNumeric}},
	"GDRAWGWITHMASK":      {Arguments: []KnownValueType{KnownValueNumeric, KnownValueNumeric, KnownValueNumeric, KnownValueNumeric, KnownValueNumeric}},
	"GSETBRUSH":           {Arguments: []KnownValueType{KnownValueNumeric, KnownValueNumeric}},
	"GSETFONT":            {Arguments: []KnownValueType{KnownValueNumeric, KnownValueString, KnownValueNumeric}},
	"GSETPEN":             {Arguments: []KnownValueType{KnownValueNumeric, KnownValueNumeric}},
	"SPRITEDISPOSE":       {Arguments: []KnownValueType{KnownValueString}},
	"SPRITECREATED":       {Arguments: []KnownValueType{KnownValueString}},
	"SPRITEWIDTH":         {Arguments: []KnownValueType{KnownValueString}},
	"SPRITEHEIGHT":        {Arguments: []KnownValueType{KnownValueString}},
	"SPRITEPOSX":          {Arguments: []KnownValueType{KnownValueString}},
	"SPRITEPOSY":          {Arguments: []KnownValueType{KnownValueString}},
	"SPRITEMOVE":          {Arguments: []KnownValueType{KnownValueString, KnownValueNumeric, KnownValueNumeric}},
	"SPRITESETPOS":        {Arguments: []KnownValueType{KnownValueString, KnownValueNumeric, KnownValueNumeric}},
	"SPRITEGETCOLOR":      {Arguments: []KnownValueType{KnownValueString, KnownValueNumeric, KnownValueNumeric}},
	"SPRITEANIMECREATE":   {Arguments: []KnownValueType{KnownValueString, KnownValueNumeric, KnownValueNumeric}},
	"SPRITEANIMEADDFRAME": {Arguments: []KnownValueType{KnownValueString, KnownValueNumeric, KnownValueNumeric, KnownValueNumeric, KnownValueNumeric, KnownValueNumeric, KnownValueNumeric, KnownValueNumeric, KnownValueNumeric}},
	"CBGREMOVERANGE":      {Arguments: []KnownValueType{KnownValueNumeric, KnownValueNumeric}},
	"CBGSETG":             {Arguments: []KnownValueType{KnownValueNumeric, KnownValueNumeric, KnownValueNumeric, KnownValueNumeric}},
	"CBGSETBMAPG":         {Arguments: []KnownValueType{KnownValueNumeric}},
	"GSAVE":               {Arguments: []KnownValueType{KnownValueNumeric, KnownValueNumeric}},
	"GLOAD":               {Arguments: []KnownValueType{KnownValueNumeric, KnownValueNumeric}},
	"GCLEAR":              {Arguments: []KnownValueType{KnownValueNumeric, KnownValueNumeric}},
	"GCREATEFROMFILE":     {Arguments: []KnownValueType{KnownValueNumeric, KnownValueString}},
	"CBGSETSPRITE":        {Arguments: []KnownValueType{KnownValueString, KnownValueNumeric, KnownValueNumeric, KnownValueNumeric}},
	"CBGSETBUTTONSPRITE": {Variants: [][]KnownValueType{
		{KnownValueNumeric, KnownValueString, KnownValueString, KnownValueNumeric, KnownValueNumeric, KnownValueNumeric},
		{KnownValueNumeric, KnownValueString, KnownValueString, KnownValueNumeric, KnownValueNumeric, KnownValueNumeric, KnownValueString},
	}},
	"GDRAWSPRITE":         {Matrix: methodMatrixSprite},
	"GDRAWG":              {Matrix: methodMatrixGraphics},
	"SPRITECREATE":        {Variants: [][]KnownValueType{{KnownValueString, KnownValueNumeric}, {KnownValueString, KnownValueNumeric, KnownValueNumeric, KnownValueNumeric, KnownValueNumeric, KnownValueNumeric}}},
	"ABS":                 {Arguments: []KnownValueType{KnownValueNumeric}},
	"SQRT":                {Arguments: []KnownValueType{KnownValueNumeric}},
	"CBRT":                {Arguments: []KnownValueType{KnownValueNumeric}},
	"LOG":                 {Arguments: []KnownValueType{KnownValueNumeric}},
	"LOG10":               {Arguments: []KnownValueType{KnownValueNumeric}},
	"EXPONENT":            {Arguments: []KnownValueType{KnownValueNumeric}},
	"SIGN":                {Arguments: []KnownValueType{KnownValueNumeric}},
	"LIMIT":               {Arguments: []KnownValueType{KnownValueNumeric, KnownValueNumeric, KnownValueNumeric}},
	"INRANGE":             {Arguments: []KnownValueType{KnownValueNumeric, KnownValueNumeric, KnownValueNumeric}},
	"STRLENS":             {Arguments: []KnownValueType{KnownValueString}},
	"STRLENSU":            {Arguments: []KnownValueType{KnownValueString}},
	"STRCOUNT":            {Arguments: []KnownValueType{KnownValueString, KnownValueString}},
	"TOINT":               {Arguments: []KnownValueType{KnownValueString}},
	"TOUPPER":             {Arguments: []KnownValueType{KnownValueString}},
	"TOLOWER":             {Arguments: []KnownValueType{KnownValueString}},
	"TOHALF":              {Arguments: []KnownValueType{KnownValueString}},
	"TOFULL":              {Arguments: []KnownValueType{KnownValueString}},
	"REPLACE":             {Arguments: []KnownValueType{KnownValueString, KnownValueString, KnownValueString}},
	"UNICODE":             {Arguments: []KnownValueType{KnownValueNumeric}},
	"UNICODEBYTE":         {Arguments: []KnownValueType{KnownValueString}},
	"CONVERT":             {Arguments: []KnownValueType{KnownValueNumeric, KnownValueNumeric}},
	"ISNUMERIC":           {Arguments: []KnownValueType{KnownValueString}},
	"ESCAPE":              {Arguments: []KnownValueType{KnownValueString}},
	"CHARATU":             {Arguments: []KnownValueType{KnownValueString, KnownValueNumeric}},
	"HTML_TOPLAINTEXT":    {Arguments: []KnownValueType{KnownValueString}},
	"HTML_ESCAPE":         {Arguments: []KnownValueType{KnownValueString}},
	"CHKFONT":             {Arguments: []KnownValueType{KnownValueString}},
	"CHKDATA":             {Arguments: []KnownValueType{KnownValueNumeric}},
	"CHKCHARADATA":        {Arguments: []KnownValueType{KnownValueString}},
	"BARSTR":              {Arguments: []KnownValueType{KnownValueNumeric, KnownValueNumeric, KnownValueNumeric}},
	"COLOR_FROMNAME":      {Arguments: []KnownValueType{KnownValueString}},
	"COLOR_FROMRGB":       {Arguments: []KnownValueType{KnownValueNumeric, KnownValueNumeric, KnownValueNumeric}},
	"GETBIT":              {Minimum: 2, Validation: methodValidationGetBit},
	"GETNUMB":             {Arguments: []KnownValueType{KnownValueString, KnownValueString}},
	"GETPALAMLV":          {Arguments: []KnownValueType{KnownValueNumeric, KnownValueNumeric}},
	"GETEXPLV":            {Arguments: []KnownValueType{KnownValueNumeric, KnownValueNumeric}},
	"GETLINESTR":          {Arguments: []KnownValueType{KnownValueString}},
	"STRFORM":             {Arguments: []KnownValueType{KnownValueString}},
	"GETCONFIG":           {Arguments: []KnownValueType{KnownValueString}},
	"GETCONFIGS":          {Arguments: []KnownValueType{KnownValueString}},
	"HTML_POPPRINTINGSTR": {},
	"GETKEY":              {Arguments: []KnownValueType{KnownValueNumeric}},
	"GETKEYTRIGGERED":     {Arguments: []KnownValueType{KnownValueNumeric}},
	"SETANIMETIMER":       {Arguments: []KnownValueType{KnownValueNumeric}},
	"SUBSTRING":           {Arguments: []KnownValueType{KnownValueString, KnownValueNumeric, KnownValueNumeric}, Minimum: 1},
	"SUBSTRINGU":          {Arguments: []KnownValueType{KnownValueString, KnownValueNumeric, KnownValueNumeric}, Minimum: 1},
	"STRFIND":             {Arguments: []KnownValueType{KnownValueString, KnownValueString, KnownValueNumeric}, Minimum: 2},
	"STRFINDU":            {Arguments: []KnownValueType{KnownValueString, KnownValueString, KnownValueNumeric}, Minimum: 2},
	"TOSTR":               {Arguments: []KnownValueType{KnownValueNumeric, KnownValueString}, Minimum: 1},
	"MONEYSTR":            {Arguments: []KnownValueType{KnownValueNumeric, KnownValueString}, Minimum: 1},
	"SAVETEXT":            {Arguments: []KnownValueType{KnownValueString, KnownValueNumeric, KnownValueNumeric, KnownValueNumeric}, Minimum: 2, RejectNulls: true},
	"LOADTEXT":            {Arguments: []KnownValueType{KnownValueNumeric, KnownValueNumeric, KnownValueNumeric}, Minimum: 1, RejectNulls: true},
	"GETNUM":              {Minimum: 2, Validation: methodValidationGetNum},
	"FINDELEMENT":         {Minimum: 2, Validation: methodValidationFindElement},
	"FINDLASTELEMENT":     {Minimum: 2, Validation: methodValidationFindElement},
	"ARRAYMSORT":          {Minimum: 2, Validation: methodValidationArrayMultiSort},
	"SUMARRAY":            {Minimum: 1, Validation: methodValidationSumArray},
	"SUMCARRAY":           {Minimum: 1, Validation: methodValidationSumArray},
	"MAXARRAY":            {Minimum: 1, Validation: methodValidationArrayExtrema},
	"MAXCARRAY":           {Minimum: 1, Validation: methodValidationArrayExtrema},
	"MINARRAY":            {Minimum: 1, Validation: methodValidationArrayExtrema},
	"MINCARRAY":           {Minimum: 1, Validation: methodValidationArrayExtrema},
	"MATCH":               {Minimum: 2, Validation: methodValidationMatch},
	"CMATCH":              {Minimum: 2, Validation: methodValidationMatch},
	"INRANGEARRAY":        {Minimum: 3, Validation: methodValidationInRangeArray},
	"INRANGECARRAY":       {Minimum: 3, Validation: methodValidationInRangeArray},
	"STRJOIN":             {Minimum: 1, Validation: methodValidationStrJoin},
	"CSVNAME":             {Arguments: []KnownValueType{KnownValueNumeric, KnownValueNumeric}, Minimum: 1},
	"CSVCALLNAME":         {Arguments: []KnownValueType{KnownValueNumeric, KnownValueNumeric}, Minimum: 1},
	"CSVNICKNAME":         {Arguments: []KnownValueType{KnownValueNumeric, KnownValueNumeric}, Minimum: 1},
	"CSVMASTERNAME":       {Arguments: []KnownValueType{KnownValueNumeric, KnownValueNumeric}, Minimum: 1},
	"CSVCSTR":             {Arguments: []KnownValueType{KnownValueNumeric, KnownValueNumeric, KnownValueNumeric}, Minimum: 2},
	"CSVBASE":             {Arguments: []KnownValueType{KnownValueNumeric, KnownValueNumeric, KnownValueNumeric}, Minimum: 2},
	"CSVABL":              {Arguments: []KnownValueType{KnownValueNumeric, KnownValueNumeric, KnownValueNumeric}, Minimum: 2},
	"CSVMARK":             {Arguments: []KnownValueType{KnownValueNumeric, KnownValueNumeric, KnownValueNumeric}, Minimum: 2},
	"CSVEXP":              {Arguments: []KnownValueType{KnownValueNumeric, KnownValueNumeric, KnownValueNumeric}, Minimum: 2},
	"CSVRELATION":         {Arguments: []KnownValueType{KnownValueNumeric, KnownValueNumeric, KnownValueNumeric}, Minimum: 2},
	"CSVTALENT":           {Arguments: []KnownValueType{KnownValueNumeric, KnownValueNumeric, KnownValueNumeric}, Minimum: 2},
	"CSVCFLAG":            {Arguments: []KnownValueType{KnownValueNumeric, KnownValueNumeric, KnownValueNumeric}, Minimum: 2},
	"CSVEQUIP":            {Arguments: []KnownValueType{KnownValueNumeric, KnownValueNumeric, KnownValueNumeric}, Minimum: 2},
	"CSVJUEL":             {Arguments: []KnownValueType{KnownValueNumeric, KnownValueNumeric, KnownValueNumeric}, Minimum: 2},
	"EXISTCSV":            {Arguments: []KnownValueType{KnownValueNumeric, KnownValueNumeric}, Minimum: 1},
	"FINDCHARA":           {Minimum: 2, Validation: methodValidationFindChara},
	"FINDLASTCHARA":       {Minimum: 2, Validation: methodValidationFindChara},
	"FIND_CHARADATA":      {Arguments: []KnownValueType{KnownValueString}, MinimumSet: true},
}

var modernMethodSignatures = map[string]MethodSignature{
	"GETNUM":          {Minimum: 2, Validation: methodValidationGetNum},
	"GCLEAR":          {Variants: [][]KnownValueType{{KnownValueNumeric, KnownValueNumeric}, {KnownValueNumeric, KnownValueNumeric, KnownValueNumeric, KnownValueNumeric, KnownValueNumeric, KnownValueNumeric}}},
	"GCREATEFROMFILE": {Arguments: []KnownValueType{KnownValueNumeric, KnownValueString, KnownValueNumeric}, Minimum: 2, RejectNulls: true},
	"CBGSETBUTTONSPRITE": {Variants: [][]KnownValueType{
		{KnownValueNumeric, KnownValueString, KnownValueString, KnownValueNumeric, KnownValueNumeric, KnownValueNumeric},
		{KnownValueNumeric, KnownValueString, KnownValueString, KnownValueNumeric, KnownValueNumeric, KnownValueNumeric, KnownValueString},
	}},
	"GDRAWTEXT":             {Arguments: []KnownValueType{KnownValueNumeric, KnownValueString, KnownValueNumeric, KnownValueNumeric}, Minimum: 2, Validation: methodValidationGraphicsDrawText},
	"GGETTEXTSIZE":          {Arguments: []KnownValueType{KnownValueString, KnownValueString, KnownValueNumeric, KnownValueNumeric}, Minimum: 3, RejectNulls: true},
	"GDRAWGWITHROTATE":      {Arguments: []KnownValueType{KnownValueNumeric, KnownValueNumeric, KnownValueNumeric, KnownValueNumeric, KnownValueNumeric}, Minimum: 3, Validation: methodValidationGraphicsDrawGWithRotate},
	"GDRAWLINE":             {Arguments: []KnownValueType{KnownValueNumeric, KnownValueNumeric, KnownValueNumeric, KnownValueNumeric, KnownValueNumeric}},
	"GDASHSTYLE":            {Arguments: []KnownValueType{KnownValueNumeric, KnownValueNumeric, KnownValueNumeric}},
	"SETTEXTBOX":            {Arguments: []KnownValueType{KnownValueString}},
	"HOTKEY_STATE":          {Arguments: []KnownValueType{KnownValueNumeric, KnownValueNumeric}, Minimum: 2},
	"GDRAWSPRITE":           {Matrix: methodMatrixSprite},
	"GDRAWG":                {Matrix: methodMatrixGraphics, MatrixNullable: true},
	"MOUSEB":                {},
	"CBGCLEAR":              {},
	"CBGCLEARBUTTON":        {},
	"CBGREMOVEBMAP":         {},
	"GETMEMORYUSAGE":        {},
	"CLEARMEMORY":           {},
	"GETTEXTBOX":            {},
	"GETDOINGFUNCTION":      {},
	"HTML_STRINGLEN":        {Arguments: []KnownValueType{KnownValueString, KnownValueNumeric}, Minimum: 1, RejectNulls: true},
	"HTML_SUBSTRING":        {Arguments: []KnownValueType{KnownValueString, KnownValueNumeric}},
	"HTML_STRINGLINES":      {Arguments: []KnownValueType{KnownValueString, KnownValueNumeric}},
	"EXISTFILE":             {Arguments: []KnownValueType{KnownValueString}},
	"EXISTVAR":              {Arguments: []KnownValueType{KnownValueString}},
	"ISDEFINED":             {Arguments: []KnownValueType{KnownValueString}},
	"GETVAR":                {Arguments: []KnownValueType{KnownValueString}},
	"GETVARS":               {Arguments: []KnownValueType{KnownValueString}},
	"EXISTSOUND":            {Arguments: []KnownValueType{KnownValueString}},
	"EXISTFUNCTION":         {Arguments: []KnownValueType{KnownValueString, KnownValueNumeric}, Minimum: 1, RejectNulls: true},
	"GETDISPLAYLINE":        {Arguments: []KnownValueType{KnownValueNumeric}},
	"FLOWINPUT":             {Arguments: []KnownValueType{KnownValueNumeric, KnownValueNumeric, KnownValueNumeric, KnownValueNumeric}, Minimum: 1, RejectNulls: true},
	"FLOWINPUTS":            {Arguments: []KnownValueType{KnownValueNumeric, KnownValueString}, Minimum: 1, RejectNulls: true},
	"OUTPUTLOG":             {Arguments: []KnownValueType{KnownValueString, KnownValueNumeric}, MinimumSet: true, RejectNulls: true},
	"REGEXPMATCH":           {Minimum: 2, Validation: methodValidationRegexpMatch},
	"EXISTMETH":             {Arguments: []KnownValueType{KnownValueString}},
	"BITMAP_CACHE_ENABLE":   {Arguments: []KnownValueType{KnownValueNumeric}},
	"HOTKEY_STATE_INIT":     {Arguments: []KnownValueType{KnownValueNumeric}},
	"MAP_CREATE":            {Arguments: []KnownValueType{KnownValueString}},
	"MAP_EXIST":             {Arguments: []KnownValueType{KnownValueString}},
	"MAP_RELEASE":           {Arguments: []KnownValueType{KnownValueString}},
	"MAP_GET":               {Arguments: []KnownValueType{KnownValueString, KnownValueString}},
	"MAP_CLEAR":             {Arguments: []KnownValueType{KnownValueString}},
	"MAP_SIZE":              {Arguments: []KnownValueType{KnownValueString}},
	"MAP_HAS":               {Arguments: []KnownValueType{KnownValueString, KnownValueString}},
	"MAP_SET":               {Arguments: []KnownValueType{KnownValueString, KnownValueString, KnownValueString}},
	"MAP_REMOVE":            {Arguments: []KnownValueType{KnownValueString, KnownValueString}},
	"MAP_TOXML":             {Arguments: []KnownValueType{KnownValueString}},
	"MAP_FROMXML":           {Arguments: []KnownValueType{KnownValueString, KnownValueString}},
	"DT_CREATE":             {Arguments: []KnownValueType{KnownValueString}},
	"DT_EXIST":              {Arguments: []KnownValueType{KnownValueString}},
	"DT_RELEASE":            {Arguments: []KnownValueType{KnownValueString}},
	"DT_NOCASE":             {Arguments: []KnownValueType{KnownValueString, KnownValueNumeric}},
	"DT_CLEAR":              {Arguments: []KnownValueType{KnownValueString}},
	"DT_COLUMN_EXIST":       {Arguments: []KnownValueType{KnownValueString, KnownValueString}},
	"DT_COLUMN_REMOVE":      {Arguments: []KnownValueType{KnownValueString, KnownValueString}},
	"DT_COLUMN_LENGTH":      {Arguments: []KnownValueType{KnownValueString}},
	"DT_ROW_LENGTH":         {Arguments: []KnownValueType{KnownValueString}},
	"DT_CELL_GET":           {Arguments: []KnownValueType{KnownValueString, KnownValueNumeric, KnownValueString, KnownValueNumeric}, Minimum: 3, RejectNulls: true},
	"DT_CELL_ISNULL":        {Arguments: []KnownValueType{KnownValueString, KnownValueNumeric, KnownValueString, KnownValueNumeric}, Minimum: 3, RejectNulls: true},
	"DT_CELL_GETS":          {Arguments: []KnownValueType{KnownValueString, KnownValueNumeric, KnownValueString, KnownValueNumeric}, Minimum: 3, RejectNulls: true},
	"DT_FROMXML":            {Arguments: []KnownValueType{KnownValueString, KnownValueString, KnownValueString}},
	"SAVETEXT":              {Arguments: []KnownValueType{KnownValueString, KnownValueUnknown, KnownValueNumeric, KnownValueNumeric}, Minimum: 2, RejectNulls: true},
	"LOADTEXT":              {Arguments: []KnownValueType{KnownValueUnknown, KnownValueNumeric, KnownValueNumeric}, Minimum: 1, RejectNulls: true},
	"SETVAR":                {Arguments: []KnownValueType{KnownValueString, KnownValueUnknown}},
	"VARSETEX":              {Arguments: []KnownValueType{KnownValueString, KnownValueUnknown, KnownValueNumeric, KnownValueNumeric, KnownValueNumeric}, Minimum: 2, RejectNulls: true},
	"ENUMFILES":             {Minimum: 1, Validation: methodValidationEnumFiles},
	"DT_COLUMN_ADD":         {Arguments: []KnownValueType{KnownValueString, KnownValueString, KnownValueUnknown, KnownValueNumeric}, Minimum: 2, RejectNulls: true},
	"DT_ROW_ADD":            {Minimum: 1, Validation: methodValidationDataTableRowAdd},
	"DT_ROW_REMOVE":         {Minimum: 2, Validation: methodValidationDataTableRowRemove},
	"DT_CELL_SET":           {Arguments: []KnownValueType{KnownValueString, KnownValueNumeric, KnownValueString, KnownValueUnknown, KnownValueNumeric}, Minimum: 3, RejectNulls: true, Nullable: []bool{false, false, false, true}},
	"DT_SELECT":             {Minimum: 1, Validation: methodValidationDataTableSelect},
	"XML_GET":               {Minimum: 2, Validation: methodValidationXMLGet},
	"XML_GET_BYNAME":        {Minimum: 2, Validation: methodValidationXMLGet},
	"XML_ADDNODE":           {Minimum: 3, Validation: methodValidationXMLAddNode},
	"XML_ADDNODE_BYNAME":    {Minimum: 3, Validation: methodValidationXMLAddNode},
	"XML_REMOVENODE":        {Minimum: 2, Validation: methodValidationXMLRemoveNode},
	"XML_REMOVENODE_BYNAME": {Minimum: 2, Validation: methodValidationXMLRemoveNode},
	"DT_TOXML":              {Minimum: 1, Validation: methodValidationDataTableToXML},
	"MOVETEXTBOX":           {Arguments: []KnownValueType{KnownValueNumeric, KnownValueNumeric, KnownValueNumeric}},
	"RESUMETEXTBOX":         {Arguments: []KnownValueType{}},
	"GETMETH":               {Minimum: 1, Validation: methodValidationDynamicMethod},
	"GETMETHS":              {Minimum: 1, Validation: methodValidationDynamicMethod},
	"GGETFONT":              {Arguments: []KnownValueType{KnownValueNumeric}},
	"GGETFONTSIZE":          {Arguments: []KnownValueType{KnownValueNumeric}},
	"GGETFONTSTYLE":         {Arguments: []KnownValueType{KnownValueNumeric}},
	"GGETBRUSH":             {Arguments: []KnownValueType{KnownValueNumeric}},
	"GGETPEN":               {Arguments: []KnownValueType{KnownValueNumeric}},
	"GGETPENWIDTH":          {Arguments: []KnownValueType{KnownValueNumeric}},
	"SPRITEDISPOSEALL":      {Arguments: []KnownValueType{KnownValueNumeric}},
	"GSETFONT":              {Variants: [][]KnownValueType{{KnownValueNumeric, KnownValueString, KnownValueNumeric}, {KnownValueNumeric, KnownValueString, KnownValueNumeric, KnownValueNumeric}}},
	"GSETPEN":               {Arguments: []KnownValueType{KnownValueNumeric, KnownValueNumeric, KnownValueNumeric}},
}

func (p *SemanticContext) MethodSignature(command string) (MethodSignature, bool) {
	if p.Dialect == EMEE {
		if signature, ok := modernMethodSignatures[command]; ok {
			return signature, true
		}
	}
	if signature, ok := simpleMethodSignatures[command]; ok {
		return signature, true
	}
	return MethodSignature{}, false
}

func ExpressionNeedsDeferral(tokens []ExprToken, dialect Dialect) bool {
	return CommandMacroPossibleForDialect(tokens, dialect) || containsExpressionMarker(tokens) || containsExpressionForm(tokens)
}

func (p *SemanticContext) MethodArgumentParts(start, end int) ([]ExprToken, []SyntaxSpan, error) {
	tokens, err := p.LexSyntaxRange(start, end)
	if err != nil {
		return nil, nil, err
	}
	parts := p.SplitTopLevel(p.Src, start, end, ',')
	if len(parts) > 0 {
		left, right := ExtraFieldBoundsWithSpace(p.Src, parts[len(parts)-1], p.AllowFullwidthSpace)
		if left == right {
			parts = parts[:len(parts)-1]
		}
	}
	return tokens, parts, nil
}

func (p *SemanticContext) SimpleMethodArgumentsValid(command string, start, end int) (int, string, bool) {
	signature, registered := p.MethodSignature(command)
	if !registered || !p.expressionParseEnabled() {
		return 0, "", true
	}
	if signature.Matrix != methodMatrixNone {
		if TrimInputLeft(p.Src, start, end, p.AllowFullwidthSpace) == end {
			return start, "method argument count does not match", false
		}
		tokens, parts, err := p.MethodArgumentParts(start, end)
		if err != nil {
			return 0, "", true
		}
		if containsExpressionMarker(tokens) || containsExpressionForm(tokens) {
			return 0, "", true
		}
		if CommandMacroPossibleForDialect(tokens, p.Dialect) {
			return 0, "", true
		}
		return p.MethodMatrixArgumentsValid(signature.Matrix, signature.MatrixNullable, parts, start)
	}
	minimum, optional := signature.Minimum, signature.Minimum > 0 || signature.MinimumSet
	if !optional {
		minimum = len(signature.Arguments)
	}
	if TrimInputLeft(p.Src, start, end, p.AllowFullwidthSpace) == end {
		if signature.Variadic != KnownValueUnknown || signature.SameAsFirst {
			return start, "method requires an argument", false
		}
		if len(signature.Variants) != 0 {
			return p.MethodVariantsValid(signature.Variants, nil, start)
		}
		if signature.Validation != methodValidationNone && minimum > 0 {
			return start, "method requires an argument", false
		}
		if len(signature.Arguments) == 0 || minimum == 0 {
			return 0, "", true
		}
		return start, "method requires an argument", false
	}
	tokens, parts, err := p.MethodArgumentParts(start, end)
	if err != nil {
		return 0, "", true
	}
	if signature.Variadic != KnownValueUnknown || signature.SameAsFirst {
		return p.MethodVariadicValid(signature, parts, start)
	}
	if CommandMacroPossibleForDialect(tokens, p.Dialect) {
		return 0, "", true
	}
	for _, token := range tokens {
		if token.Kind == ExprMarker || token.Kind == ExprForm {
			return 0, "", true
		}
	}
	if signature.Validation != methodValidationNone {
		switch signature.Validation {
		case methodValidationDataTableRowAdd:
			return p.DataTableRowAddArgumentsValid(parts, start)
		case methodValidationDataTableRowRemove:
			return p.DataTableRowRemoveArgumentsValid(parts, start)
		case methodValidationDataTableToXML:
			return p.DataTableToXMLArgumentsValid(parts, start)
		case methodValidationEnumFiles:
			return p.EnumFilesArgumentsValid(parts, start)
		case methodValidationDynamicMethod:
			return p.DynamicMethodArgumentsValid(command, parts, start)
		case methodValidationRand:
			return p.RandArgumentsValid(parts, start)
		case methodValidationGetNum:
			return p.GetNumArgumentsValid(parts, start)
		case methodValidationFindElement:
			return p.FindElementArgumentsValid(parts, start)
		case methodValidationArrayMultiSort:
			return p.ArrayMultiSortArgumentsValid(parts, start)
		case methodValidationSumArray:
			return p.SumArrayArgumentsValid(command, parts, start)
		case methodValidationMatch:
			return p.MatchArgumentsValid(command, parts, start)
		case methodValidationInRangeArray:
			return p.InRangeArrayArgumentsValid(command, parts, start)
		case methodValidationStrJoin:
			return p.StrJoinArgumentsValid(parts, start)
		case methodValidationRegexpMatch:
			return p.RegexpMatchArgumentsValid(parts, start)
		case methodValidationGetBit:
			return p.GetBitArgumentsValid(parts, start)
		case methodValidationArrayExtrema:
			return p.ArrayExtremaArgumentsValid(command, parts, start)
		case methodValidationDataTableSelect:
			return p.DataTableSelectArgumentsValid(parts, start)
		case methodValidationXMLGet:
			return p.XmlGetArgumentsValid(parts, start)
		case methodValidationXMLAddNode:
			return p.XmlAddNodeArgumentsValid(command, parts, start)
		case methodValidationXMLRemoveNode:
			return p.XmlRemoveNodeArgumentsValid(command, parts, start)
		case methodValidationFindChara:
			return p.FindCharaArgumentsValid(parts, start)
		case methodValidationGraphicsDrawText:
			return p.GraphicsDrawTextArgumentsValid(parts, start)
		case methodValidationGraphicsDrawGWithRotate:
			return p.GraphicsDrawGWithRotateArgumentsValid(parts, start)
		}
	}
	if command == "REPLACE" && p.Dialect == EMEE && len(parts) == 4 {
		return p.ReplaceArgumentsValid(parts)
	}
	if len(signature.Variants) != 0 {
		return p.MethodVariantsValid(signature.Variants, parts, start)
	}
	return p.MethodVariantValid(signature.Arguments, minimum, parts, start, signature.RejectNulls, signature.Nullable)
}

func (p *SemanticContext) FindCharaArgumentsValid(parts []SyntaxSpan, start int) (int, string, bool) {
	if len(parts) < 2 || len(parts) > 4 {
		return start, "method argument count does not match", false
	}
	if offset, message, valid := p.MethodReferenceRoleValid(parts[0], methodReferenceCharacter, true); !valid {
		return offset, message, false
	}
	if offset, message, valid := p.MethodReferenceSameTypeValid(parts[1], p.MethodReferenceValueType(parts[0]), true); !valid {
		return offset, message, false
	}
	for _, field := range parts[2:] {
		if offset, message, valid := p.MethodFieldTypeValid(field, KnownValueNumeric, false); !valid {
			return offset, message, false
		}
	}
	return 0, "", true
}

func (p *SemanticContext) GraphicsDrawTextArgumentsValid(parts []SyntaxSpan, start int) (int, string, bool) {
	if len(parts) < 2 || len(parts) > 4 {
		return start, "method argument count does not match", false
	}
	if offset, message, valid := p.MethodFieldTypeValid(parts[0], KnownValueNumeric, true); !valid {
		return offset, message, false
	}
	if offset, message, valid := p.MethodFieldTypeValid(parts[1], KnownValueString, true); !valid {
		return offset, message, false
	}
	if len(parts) == 3 {
		return p.MethodFieldTypeValid(parts[2], KnownValueNumeric, false)
	}
	for _, field := range parts[2:] {
		if offset, message, valid := p.MethodFieldTypeValid(field, KnownValueNumeric, true); !valid {
			return offset, message, false
		}
	}
	return 0, "", true
}

func (p *SemanticContext) GraphicsDrawGWithRotateArgumentsValid(parts []SyntaxSpan, start int) (int, string, bool) {
	if len(parts) != 3 && len(parts) != 5 {
		return start, "method argument count does not match", false
	}
	for _, field := range parts {
		if offset, message, valid := p.MethodFieldTypeValid(field, KnownValueNumeric, true); !valid {
			return offset, message, false
		}
	}
	return 0, "", true
}

func (p *SemanticContext) MethodVariantsValid(variants [][]KnownValueType, parts []SyntaxSpan, start int) (int, string, bool) {
	matched := false
	var firstOffset int
	var firstMessage string
	for _, arguments := range variants {
		if len(arguments) != len(parts) {
			continue
		}
		matched = true
		offset, message, valid := p.MethodVariantValid(arguments, len(arguments), parts, start, true, nil)
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

func (p *SemanticContext) MethodVariadicValid(signature MethodSignature, parts []SyntaxSpan, start int) (int, string, bool) {
	uncertain := false
	first := KnownValueUnknown
	for n, field := range parts {
		left, right := ExtraFieldBoundsWithSpace(p.Src, field, p.AllowFullwidthSpace)
		if left == right {
			if uncertain {
				return 0, "", true
			}
			return left, "method contains an omitted argument", false
		}
		tokens, err := p.LexSyntaxRange(left, right)
		if err != nil || ExpressionNeedsDeferral(tokens, p.Dialect) {
			uncertain = true
			continue
		}
		if !uncertain && BuiltinScalarIndexesMissing(tokens, p.Dialect) {
			return left, "scalar variable requires indexes", false
		}
		actual := KnownExpressionTypeForDialect(tokens, p.Dialect)
		if actual == KnownValueUnknown {
			uncertain = true
			continue
		}
		if signature.SameAsFirst {
			if n == 0 {
				first = actual
				continue
			}
			if first != KnownValueUnknown && actual != first {
				if uncertain {
					return 0, "", true
				}
				return left, "method argument has an incompatible type", false
			}
			continue
		}
		if !CommandTypeMatches(signature.Variadic, actual) {
			if uncertain {
				return 0, "", true
			}
			return left, "method argument has an incompatible type", false
		}
	}
	if len(parts) < signature.Minimum {
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

func (p *SemanticContext) MethodMatrixArgumentsValid(kind MethodMatrixKind, nullable bool, parts []SyntaxSpan, start int) (int, string, bool) {
	if kind == methodMatrixSprite {
		if len(parts) != 2 && len(parts) != 4 && len(parts) != 6 && len(parts) != 7 {
			return start, "method argument count does not match", false
		}
		expected := []KnownValueType{KnownValueNumeric, KnownValueString}
		for len(expected) < len(parts) && len(expected) < 6 {
			expected = append(expected, KnownValueNumeric, KnownValueNumeric)
		}
		if len(parts) == 7 {
			expected = append(expected, KnownValueUnknown)
		}
		return p.MethodMatrixFieldsValid(expected, nullable, parts, start)
	}
	if len(parts) != 10 && len(parts) != 11 {
		return start, "method argument count does not match", false
	}
	expected := make([]KnownValueType, 10)
	for i := range expected {
		expected[i] = KnownValueNumeric
	}
	if len(parts) == 11 {
		expected = append(expected, KnownValueUnknown)
	}
	return p.MethodMatrixFieldsValid(expected, nullable, parts, start)
}

func (p *SemanticContext) MethodMatrixFieldsValid(expected []KnownValueType, nullable bool, parts []SyntaxSpan, start int) (int, string, bool) {
	for n, field := range parts {
		left, right := ExtraFieldBoundsWithSpace(p.Src, field, p.AllowFullwidthSpace)
		if left == right {
			if nullable && n == len(parts)-1 && expected[n] == KnownValueUnknown {
				return 0, "", true
			}
			return left, "method contains an omitted argument", false
		}
		tokens, err := p.LexSyntaxRange(left, right)
		if err != nil || ExpressionNeedsDeferral(tokens, p.Dialect) {
			return 0, "", true
		}
		if expected[n] == KnownValueUnknown {
			if matrixVariableKnown, matrixValid := p.MethodMatrixVariableValid(tokens); matrixVariableKnown {
				if !matrixValid {
					return left, "method argument has an incompatible type", false
				}
			}
			continue
		}
		if BuiltinScalarIndexesMissing(tokens, p.Dialect) {
			return left, "scalar variable requires indexes", false
		}
		actual := KnownExpressionTypeForDialect(tokens, p.Dialect)
		if actual == KnownValueUnknown {
			continue
		}
		if !CommandTypeMatches(expected[n], actual) {
			return left, "method argument has an incompatible type", false
		}
	}
	return 0, "", true
}

func (p *SemanticContext) GetBitArgumentsValid(parts []SyntaxSpan, start int) (int, string, bool) {
	if len(parts) != 2 {
		return start, "method argument count does not match", false
	}
	for _, field := range parts {
		if offset, message, valid := p.MethodFieldTypeValid(field, KnownValueNumeric, true); !valid {
			return offset, message, false
		}
	}
	left, right := ExtraFieldBoundsWithSpace(p.Src, parts[1], p.AllowFullwidthSpace)
	tokens, err := p.LexSyntaxRange(left, right)
	if err != nil {
		return 0, "", true
	}
	value, kind := DirectIntegerLiteral(tokens)
	if kind != KnownValueNumeric {
		return 0, "", true
	}
	if value < 0 || value > 63 {
		return left, "bit index literal is outside 0..63", false
	}
	return 0, "", true
}

func (p *SemanticContext) MethodMatrixVariableValid(tokens []ExprToken) (bool, bool) {
	name, ok := matrixBuiltinName(tokens)
	if !ok {
		if KnownExpressionTypeForDialect(tokens, p.Dialect) != KnownValueUnknown {
			return true, false
		}
		return false, false
	}
	info, known := BuiltinVariableInfoByName(name, p.Dialect)
	if !known {
		return false, false
	}
	return matrixBuiltinIndexesValid(tokens, info)
}

func matrixBuiltinName(tokens []ExprToken) (string, bool) {
	tokens = stripExpressionParens(tokens)
	if len(tokens) > 0 && tokens[0].Kind == ExprOperator && string(tokens[0].Text) == "+" {
		tokens = stripExpressionParens(tokens[1:])
	}
	if len(tokens) == 0 || tokens[0].Kind != ExprIdentifier {
		return "", false
	}
	if len(tokens) == 1 {
		return string(tokens[0].Text), true
	}
	if string(tokens[1].Text) != ":" {
		return "", false
	}
	return string(tokens[0].Text), true
}

func matrixBuiltinIndexesValid(tokens []ExprToken, info BuiltinVariable) (bool, bool) {
	if info.Value != KnownValueNumeric || (info.Rank != 2 && info.Rank != 3) {
		return true, false
	}
	tokens = stripExpressionParens(tokens)
	if len(tokens) == 1 {
		return true, true
	}
	if len(tokens) < 3 || string(tokens[1].Text) != ":" {
		return false, false
	}
	start, depth, indexes := 2, 0, 0
	for i := 2; i <= len(tokens); i++ {
		boundary := i == len(tokens)
		if i < len(tokens) {
			switch string(tokens[i].Text) {
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
		if _, kind := DirectIntegerLiteral(tokens[start:i]); kind != KnownValueNumeric {
			return false, false
		}
		indexes++
		start = i + 1
	}
	if indexes == int(info.Rank) {
		return true, !info.Character
	}
	if info.Character && indexes == int(info.Rank)+1 {
		return true, true
	}
	return true, false
}

func (p *SemanticContext) MethodVariantValid(arguments []KnownValueType, minimum int, parts []SyntaxSpan, start int, rejectNulls bool, nullable []bool) (int, string, bool) {
	if len(parts) < minimum || len(parts) > len(arguments) {
		return start, "method argument count does not match", false
	}
	for n, field := range parts {
		left, right := ExtraFieldBoundsWithSpace(p.Src, field, p.AllowFullwidthSpace)
		if left == right {
			if n >= minimum {
				if rejectNulls && (n >= len(nullable) || !nullable[n]) {
					return left, "method contains an omitted argument", false
				}
				continue
			}
			return left, "method contains an omitted argument", false
		}
		tokens, err := p.LexSyntaxRange(left, right)
		if err != nil {
			return 0, "", true
		}
		if ExpressionNeedsDeferral(tokens, p.Dialect) {
			return 0, "", true
		}
		if BuiltinScalarIndexesMissing(tokens, p.Dialect) {
			return left, "scalar variable requires indexes", false
		}
		if !CommandTypeMatches(arguments[n], KnownExpressionTypeForDialect(tokens, p.Dialect)) {
			return left, "method argument has an incompatible type", false
		}
	}
	return 0, "", true
}

func (p *SemanticContext) DataTableRowAddArgumentsValid(parts []SyntaxSpan, start int) (int, string, bool) {
	if len(parts) == 0 {
		return start, "method requires an argument", false
	}
	if offset, message, valid := p.MethodFieldTypeValid(parts[0], KnownValueString, true); !valid {
		return offset, message, false
	}
	if len(parts) == 1 {
		return 0, "", true
	}
	if len(parts) == 4 {
		if offset, message, valid := p.MethodFieldTypeValid(parts[3], KnownValueNumeric, true); !valid {
			return offset, message, false
		}
		for n, field := range parts[1:3] {
			left, right := ExtraFieldBoundsWithSpace(p.Src, field, p.AllowFullwidthSpace)
			if left == right {
				return left, "method contains an omitted argument", false
			}
			tokens, err := p.LexSyntaxRange(left, right)
			if err != nil {
				return 0, "", true
			}
			expected := KnownValueUnknown
			if n == 0 {
				expected = KnownValueString
			}
			if known, valid := p.MethodReferenceFieldValid(tokens, expected, true); known && !valid {
				return left, "method argument has an incompatible type", false
			}
		}
		return 0, "", true
	}
	if (len(parts)-1)%2 != 0 {
		return start, "method argument count does not match", false
	}
	for n := 1; n < len(parts); n += 2 {
		if offset, message, valid := p.MethodFieldTypeValid(parts[n], KnownValueString, true); !valid {
			return offset, message, false
		}
		if offset, message, valid := p.MethodFieldTypeValid(parts[n+1], KnownValueUnknown, false); !valid {
			return offset, message, false
		}
	}
	return 0, "", true
}

func (p *SemanticContext) DataTableRowRemoveArgumentsValid(parts []SyntaxSpan, start int) (int, string, bool) {
	if len(parts) != 2 && len(parts) != 3 {
		return start, "method argument count does not match", false
	}
	if offset, message, valid := p.MethodFieldTypeValid(parts[0], KnownValueString, true); !valid {
		return offset, message, false
	}
	if len(parts) == 2 {
		return p.MethodFieldTypeValid(parts[1], KnownValueNumeric, true)
	}
	if offset, message, valid := p.MethodReferenceArgumentValid(parts[1], KnownValueNumeric, true, true); !valid {
		return offset, message, false
	}
	return p.MethodFieldTypeValid(parts[2], KnownValueNumeric, true)
}

func (p *SemanticContext) DataTableToXMLArgumentsValid(parts []SyntaxSpan, start int) (int, string, bool) {
	if len(parts) != 1 && len(parts) != 2 {
		return start, "method argument count does not match", false
	}
	if offset, message, valid := p.MethodFieldTypeValid(parts[0], KnownValueString, true); !valid {
		return offset, message, false
	}
	if len(parts) == 2 {
		return p.MethodReferenceArgumentValid(parts[1], KnownValueString, true, true)
	}
	return 0, "", true
}

func (p *SemanticContext) EnumFilesArgumentsValid(parts []SyntaxSpan, start int) (int, string, bool) {
	if len(parts) == 0 || len(parts) > 4 {
		return start, "method argument count does not match", false
	}
	if offset, message, valid := p.MethodFieldTypeValid(parts[0], KnownValueString, true); !valid {
		return offset, message, false
	}
	if len(parts) > 1 {
		if offset, message, valid := p.MethodFieldTypeValid(parts[1], KnownValueString, true); !valid {
			return offset, message, false
		}
	}
	if len(parts) > 2 {
		if offset, message, valid := p.MethodFieldTypeValid(parts[2], KnownValueNumeric, true); !valid {
			return offset, message, false
		}
	}
	if len(parts) > 3 {
		return p.MethodReferenceArgumentValid(parts[3], KnownValueString, true, true)
	}
	return 0, "", true
}

func (p *SemanticContext) DynamicMethodArgumentsValid(command string, parts []SyntaxSpan, start int) (int, string, bool) {
	if len(parts) == 0 {
		return start, "method requires an argument", false
	}
	if offset, message, valid := p.MethodFieldTypeValid(parts[0], KnownValueString, true); !valid {
		return offset, message, false
	}
	if len(parts) < 2 {
		return 0, "", true
	}
	want := KnownValueNumeric
	if command == "GETMETHS" {
		want = KnownValueString
	}
	return p.MethodFieldTypeValid(parts[1], want, false)
}

type MethodReferenceRole uint8

const (
	methodReferenceAny MethodReferenceRole = iota
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

func (p *SemanticContext) MethodReferenceInfo(tokens []ExprToken) (BuiltinVariable, bool) {
	tokens = stripExpressionParens(tokens)
	if len(tokens) > 0 && tokens[0].Kind == ExprOperator && string(tokens[0].Text) == "+" {
		tokens = stripExpressionParens(tokens[1:])
	}
	if !VariableArgumentTokens(tokens) || len(tokens) == 0 || tokens[0].Kind != ExprIdentifier {
		return BuiltinVariable{}, false
	}
	if len(tokens) > 1 && string(tokens[1].Text) == "@" {
		return BuiltinVariable{}, false
	}
	name := string(tokens[0].Text)
	if ReservedBuiltinMethod(name) {
		return BuiltinVariable{}, false
	}
	info, known := BuiltinVariableInfoByName(name, p.Dialect)
	if !known {
		return BuiltinVariable{}, false
	}
	return info, true
}

func (p *SemanticContext) MethodReferenceRoleValid(field SyntaxSpan, role MethodReferenceRole, required bool) (int, string, bool) {
	left, right := ExtraFieldBoundsWithSpace(p.Src, field, p.AllowFullwidthSpace)
	if left == right {
		if required {
			return left, "method contains an omitted argument", false
		}
		return 0, "", true
	}
	tokens, err := p.LexSyntaxRange(left, right)
	if err != nil || ExpressionNeedsDeferral(tokens, p.Dialect) {
		return 0, "", true
	}
	if _, valid, proven := indexedBuiltinType(stripExpressionParens(tokens), p.Dialect); proven && !valid {
		return left, "method argument has an incompatible type", false
	}
	if role == methodReferenceMutableNumericScalar && BuiltinScalarIndexesMissing(tokens, p.Dialect) {
		return left, "scalar variable requires indexes", false
	}
	info, known := p.MethodReferenceInfo(tokens)
	if !known {
		trimmed := stripExpressionParens(tokens)
		if len(trimmed) > 0 && trimmed[0].Kind == ExprIdentifier && (len(trimmed) < 2 || string(trimmed[1].Text) != "@") {
			if _, builtin := BuiltinVariableInfoByName(string(trimmed[0].Text), p.Dialect); builtin && !VariableArgumentTokens(tokens) {
				return left, "method argument has an incompatible type", false
			}
		}
		if KnownExpressionTypeForDialect(tokens, p.Dialect) != KnownValueUnknown {
			return left, "method argument has an incompatible type", false
		}
		return 0, "", true
	}
	valid := true
	switch role {
	case methodReferenceOneDimensional:
		valid = info.Rank == 1
	case methodReferenceArray:
		valid = info.Rank >= 1
	case methodReferenceNumericArray:
		valid = info.Value == KnownValueNumeric && info.Rank >= 1
	case methodReferenceNumericOneDimensional:
		valid = info.Value == KnownValueNumeric && info.Rank == 1
	case methodReferenceCharacter:
		valid = info.Character
	case methodReferenceCharacterNumeric:
		valid = info.Character && info.Value == KnownValueNumeric
	case methodReferenceCharacterNumericOneDimensional:
		valid = info.Character && info.Value == KnownValueNumeric && info.Rank == 1
	case methodReferenceMutableArray:
		valid = !info.Character && !info.ReadOnly && info.Rank >= 1
	case methodReferenceMutableOneDimensional:
		valid = !info.Character && !info.ReadOnly && info.Rank == 1
	case methodReferenceMutableNumericScalar:
		valid = info.Value == KnownValueNumeric && !info.ReadOnly
	case methodReferenceMutableNumericOneDimensional:
		valid = !info.Character && info.Value == KnownValueNumeric && !info.ReadOnly && info.Rank == 1
	case methodReferenceMutableStringOneDimensional:
		valid = !info.Character && info.Value == KnownValueString && !info.ReadOnly && info.Rank == 1
	case methodReferenceMutableString:
		valid = info.Value == KnownValueString && !info.ReadOnly
	}
	if !valid {
		return left, "method argument has an incompatible type", false
	}
	return 0, "", true
}

func (p *SemanticContext) MethodReferenceSameTypeValid(field SyntaxSpan, want KnownValueType, required bool) (int, string, bool) {
	left, right := ExtraFieldBoundsWithSpace(p.Src, field, p.AllowFullwidthSpace)
	if left == right {
		if required {
			return left, "method contains an omitted argument", false
		}
		return 0, "", true
	}
	tokens, err := p.LexSyntaxRange(left, right)
	if err != nil || ExpressionNeedsDeferral(tokens, p.Dialect) {
		return 0, "", true
	}
	if BuiltinScalarIndexesMissing(tokens, p.Dialect) {
		return left, "scalar variable requires indexes", false
	}
	actual := KnownExpressionTypeForDialect(tokens, p.Dialect)
	if want != KnownValueUnknown && actual != KnownValueUnknown && actual != want {
		return left, "method argument has an incompatible type", false
	}
	return 0, "", true
}

func (p *SemanticContext) RandArgumentsValid(parts []SyntaxSpan, start int) (int, string, bool) {
	if len(parts) < 1 || len(parts) > 2 {
		return start, "method argument count does not match", false
	}
	if len(parts) == 2 {
		if left, right := ExtraFieldBoundsWithSpace(p.Src, parts[0], p.AllowFullwidthSpace); left == right {
			secondLeft, secondRight := ExtraFieldBoundsWithSpace(p.Src, parts[1], p.AllowFullwidthSpace)
			if secondLeft != secondRight {
				if tokens, err := p.LexSyntaxRange(secondLeft, secondRight); err == nil && bareRandReference(tokens) {
					return secondLeft, "method argument has an incompatible type", false
				}
			}
			return p.MethodFieldTypeValid(parts[1], KnownValueNumeric, true)
		}
	}
	for _, field := range parts {
		left, right := ExtraFieldBoundsWithSpace(p.Src, field, p.AllowFullwidthSpace)
		if left != right {
			tokens, err := p.LexSyntaxRange(left, right)
			if err == nil && bareRandReference(tokens) {
				return left, "method argument has an incompatible type", false
			}
		}
		if offset, message, valid := p.MethodFieldTypeValid(field, KnownValueNumeric, true); !valid {
			return offset, message, false
		}
	}
	return 0, "", true
}

func bareRandReference(tokens []ExprToken) bool {
	return len(tokens) > 0 && tokens[0].Kind == ExprIdentifier && string(tokens[0].Text) == "RAND" && (len(tokens) == 1 || string(tokens[1].Text) != "(")
}

func (p *SemanticContext) GetNumArgumentsValid(parts []SyntaxSpan, start int) (int, string, bool) {
	maximum := 2
	if p.Dialect == EMEE {
		maximum = 3
	}
	if len(parts) < 2 || len(parts) > maximum {
		return start, "method argument count does not match", false
	}
	if offset, message, valid := p.MethodReferenceRoleValid(parts[0], methodReferenceAny, true); !valid {
		return offset, message, false
	}
	if offset, message, valid := p.MethodFieldTypeValid(parts[1], KnownValueString, true); !valid {
		return offset, message, false
	}
	if len(parts) == 3 {
		return p.MethodFieldTypeValid(parts[2], KnownValueNumeric, true)
	}
	return 0, "", true
}

func (p *SemanticContext) FindElementArgumentsValid(parts []SyntaxSpan, start int) (int, string, bool) {
	return p.MethodReferenceSameTypeNumericTailValid(parts, start, 5, methodReferenceOneDimensional)
}

func (p *SemanticContext) MethodReferenceValueType(field SyntaxSpan) KnownValueType {
	left, right := ExtraFieldBoundsWithSpace(p.Src, field, p.AllowFullwidthSpace)
	if left == right {
		return KnownValueUnknown
	}
	tokens, err := p.LexSyntaxRange(left, right)
	if err != nil || ExpressionNeedsDeferral(tokens, p.Dialect) {
		return KnownValueUnknown
	}
	if info, known := p.MethodReferenceInfo(tokens); known {
		return info.Value
	}
	return KnownExpressionTypeForDialect(tokens, p.Dialect)
}

func (p *SemanticContext) ArrayMultiSortArgumentsValid(parts []SyntaxSpan, start int) (int, string, bool) {
	if len(parts) < 2 {
		return start, "method argument count does not match", false
	}
	for n, field := range parts {
		role := methodReferenceMutableArray
		if n == 0 {
			role = methodReferenceMutableOneDimensional
		} else if offset, message, valid := p.EmueraArrayMultiSortSecondaryValid(field); !valid {
			return offset, message, false
		}
		if offset, message, valid := p.MethodReferenceRoleValid(field, role, true); !valid {
			return offset, message, false
		}
	}
	return 0, "", true
}

func (p *SemanticContext) EmueraArrayMultiSortSecondaryValid(field SyntaxSpan) (int, string, bool) {
	if p.Dialect != Emuera1824 {
		return 0, "", true
	}
	left, right := ExtraFieldBoundsWithSpace(p.Src, field, p.AllowFullwidthSpace)
	if left == right {
		return 0, "", true
	}
	tokens, err := p.LexSyntaxRange(left, right)
	if err != nil || ExpressionNeedsDeferral(tokens, p.Dialect) {
		return 0, "", true
	}
	info, known := BuiltinVariableRoleInfo(tokens, p.Dialect)
	if known && info.Rank == 3 {
		return left, "method argument has an incompatible type", false
	}
	return 0, "", true
}

func (p *SemanticContext) SumArrayArgumentsValid(command string, parts []SyntaxSpan, start int) (int, string, bool) {
	role := methodReferenceNumericArray
	if command == "SUMCARRAY" {
		role = methodReferenceCharacterNumeric
	}
	return p.MethodReferenceNumericTailValid(parts, start, role)
}

func (p *SemanticContext) ArrayExtremaArgumentsValid(command string, parts []SyntaxSpan, start int) (int, string, bool) {
	role := methodReferenceNumericOneDimensional
	if strings.HasSuffix(command, "CARRAY") {
		role = methodReferenceCharacterNumericOneDimensional
	}
	return p.MethodReferenceNumericTailValid(parts, start, role)
}

func (p *SemanticContext) MatchArgumentsValid(command string, parts []SyntaxSpan, start int) (int, string, bool) {
	role := methodReferenceOneDimensional
	if command == "CMATCH" {
		role = methodReferenceCharacter
	}
	return p.MethodReferenceSameTypeNumericTailValid(parts, start, 4, role)
}

func (p *SemanticContext) MethodReferenceNumericTailValid(parts []SyntaxSpan, start int, role MethodReferenceRole) (int, string, bool) {
	if len(parts) < 1 || len(parts) > 3 {
		return start, "method argument count does not match", false
	}
	if offset, message, valid := p.MethodReferenceRoleValid(parts[0], role, true); !valid {
		return offset, message, false
	}
	for _, field := range parts[1:] {
		if offset, message, valid := p.MethodFieldTypeValid(field, KnownValueNumeric, false); !valid {
			return offset, message, false
		}
	}
	return 0, "", true
}

func (p *SemanticContext) MethodReferenceSameTypeNumericTailValid(parts []SyntaxSpan, start, maximum int, role MethodReferenceRole) (int, string, bool) {
	if len(parts) < 2 || len(parts) > maximum {
		return start, "method argument count does not match", false
	}
	if offset, message, valid := p.MethodReferenceRoleValid(parts[0], role, true); !valid {
		return offset, message, false
	}
	if offset, message, valid := p.MethodReferenceSameTypeValid(parts[1], p.MethodReferenceValueType(parts[0]), true); !valid {
		return offset, message, false
	}
	for _, field := range parts[2:] {
		if offset, message, valid := p.MethodFieldTypeValid(field, KnownValueNumeric, false); !valid {
			return offset, message, false
		}
	}
	return 0, "", true
}

func (p *SemanticContext) InRangeArrayArgumentsValid(command string, parts []SyntaxSpan, start int) (int, string, bool) {
	maximum := 5
	if p.Dialect == Emuera1824 {
		maximum = 6
	}
	if len(parts) < 3 || len(parts) > maximum {
		return start, "method argument count does not match", false
	}
	role := methodReferenceNumericOneDimensional
	if command == "INRANGECARRAY" {
		role = methodReferenceCharacterNumeric
		if p.Dialect == EMEE {
			role = methodReferenceCharacterNumericOneDimensional
		}
	}
	if offset, message, valid := p.MethodReferenceRoleValid(parts[0], role, true); !valid {
		return offset, message, false
	}
	for n, field := range parts[1:] {
		if p.Dialect == Emuera1824 && n >= 4 {
			continue
		}
		required := n < 2
		if offset, message, valid := p.MethodFieldTypeValid(field, KnownValueNumeric, required); !valid {
			return offset, message, false
		}
	}
	return 0, "", true
}

func (p *SemanticContext) StrJoinArgumentsValid(parts []SyntaxSpan, start int) (int, string, bool) {
	if len(parts) < 1 || len(parts) > 4 {
		return start, "method argument count does not match", false
	}
	if offset, message, valid := p.MethodReferenceRoleValid(parts[0], methodReferenceArray, true); !valid {
		return offset, message, false
	}
	if offset, message, valid := p.StrJoinCharacterRangeValid(parts); !valid {
		return offset, message, false
	}
	if len(parts) > 1 {
		if offset, message, valid := p.MethodFieldTypeValid(parts[1], KnownValueString, false); !valid {
			return offset, message, false
		}
	}
	if len(parts) > 2 {
		for _, field := range parts[2:] {
			if offset, message, valid := p.MethodFieldTypeValid(field, KnownValueNumeric, false); !valid {
				return offset, message, false
			}
		}
	}
	return 0, "", true
}

func (p *SemanticContext) StrJoinCharacterRangeValid(parts []SyntaxSpan) (int, string, bool) {
	left, right := ExtraFieldBoundsWithSpace(p.Src, parts[0], p.AllowFullwidthSpace)
	if left == right {
		return 0, "", true
	}
	tokens, err := p.LexSyntaxRange(left, right)
	if err != nil || ExpressionNeedsDeferral(tokens, p.Dialect) {
		return 0, "", true
	}
	normalized := stripExpressionParens(tokens)
	if len(normalized) > 0 && normalized[0].Kind == ExprOperator && string(normalized[0].Text) == "+" {
		normalized = stripExpressionParens(normalized[1:])
	}
	info, known := p.MethodReferenceInfo(normalized)
	if !known || !info.Character {
		return 0, "", true
	}
	if len(normalized) > 1 && string(normalized[1].Text) == ":" {
		if _, proven := BuiltinVariableRoleInfo(normalized, p.Dialect); !proven {
			return 0, "", true
		}
	}
	if info.Value == KnownValueString {
		return left, "method argument has an incompatible type", false
	}
	if len(parts) < 4 {
		return 0, "", true
	}
	lengthLeft, lengthRight := ExtraFieldBoundsWithSpace(p.Src, parts[3], p.AllowFullwidthSpace)
	if lengthLeft == lengthRight {
		return 0, "", true
	}
	lengthTokens, err := p.LexSyntaxRange(lengthLeft, lengthRight)
	if err != nil || ExpressionNeedsDeferral(lengthTokens, p.Dialect) {
		return 0, "", true
	}
	length, known := constantIntegerValue(lengthTokens)
	if !known || length <= 0 {
		return 0, "", true
	}
	return left, "method argument has an incompatible type", false
}

func (p *SemanticContext) RegexpMatchArgumentsValid(parts []SyntaxSpan, start int) (int, string, bool) {
	if len(parts) < 2 || len(parts) > 4 {
		return start, "method argument count does not match", false
	}
	for _, field := range parts[:2] {
		if offset, message, valid := p.MethodFieldTypeValid(field, KnownValueString, true); !valid {
			return offset, message, false
		}
	}
	if len(parts) == 2 {
		return 0, "", true
	}
	if len(parts) == 3 {
		return p.MethodFieldTypeValid(parts[2], KnownValueNumeric, true)
	}
	if offset, message, valid := p.MethodReferenceRoleValid(parts[2], methodReferenceMutableNumericScalar, true); !valid {
		return offset, message, false
	}
	return p.MethodReferenceRoleValid(parts[3], methodReferenceMutableStringOneDimensional, true)
}

func (p *SemanticContext) DataTableSelectArgumentsValid(parts []SyntaxSpan, start int) (int, string, bool) {
	if len(parts) < 1 || len(parts) > 4 {
		return start, "method argument count does not match", false
	}
	if offset, message, valid := p.MethodFieldTypeValid(parts[0], KnownValueString, true); !valid {
		return offset, message, false
	}
	for _, field := range parts[1:min(len(parts), 3)] {
		if offset, message, valid := p.MethodFieldTypeValid(field, KnownValueString, false); !valid {
			return offset, message, false
		}
	}
	if len(parts) == 4 {
		return p.MethodReferenceRoleValid(parts[3], methodReferenceMutableNumericOneDimensional, true)
	}
	return 0, "", true
}

func (p *SemanticContext) XmlGetArgumentsValid(parts []SyntaxSpan, start int) (int, string, bool) {
	if len(parts) < 2 || len(parts) > 4 {
		return start, "method argument count does not match", false
	}
	if offset, message, valid := p.MethodFieldTypeValid(parts[0], KnownValueUnknown, true); !valid {
		return offset, message, false
	}
	if offset, message, valid := p.MethodFieldTypeValid(parts[1], KnownValueString, true); !valid {
		return offset, message, false
	}
	if len(parts) >= 3 {
		left, right := ExtraFieldBoundsWithSpace(p.Src, parts[2], p.AllowFullwidthSpace)
		if left == right {
			return left, "method contains an omitted argument", false
		}
		tokens, err := p.LexSyntaxRange(left, right)
		if err != nil || ExpressionNeedsDeferral(tokens, p.Dialect) {
			return 0, "", true
		}
		actual := KnownExpressionTypeForDialect(tokens, p.Dialect)
		if actual == KnownValueNumeric {
			if offset, message, valid := p.MethodFieldTypeValid(parts[2], KnownValueNumeric, true); !valid {
				return offset, message, false
			}
		} else if offset, message, valid := p.MethodReferenceRoleValid(parts[2], methodReferenceMutableStringOneDimensional, true); !valid {
			return offset, message, false
		}
	}
	if len(parts) == 4 {
		return p.MethodFieldTypeValid(parts[3], KnownValueNumeric, true)
	}
	return 0, "", true
}

func (p *SemanticContext) XmlTargetArgumentsValid(field SyntaxSpan, byName bool) (int, string, bool) {
	if byName {
		return p.MethodFieldTypeValid(field, KnownValueString, true)
	}
	left, right := ExtraFieldBoundsWithSpace(p.Src, field, p.AllowFullwidthSpace)
	if left == right {
		return left, "method contains an omitted argument", false
	}
	tokens, err := p.LexSyntaxRange(left, right)
	if err != nil || ExpressionNeedsDeferral(tokens, p.Dialect) {
		return 0, "", true
	}
	if KnownExpressionTypeForDialect(tokens, p.Dialect) == KnownValueNumeric {
		return p.MethodFieldTypeValid(field, KnownValueNumeric, true)
	}
	return p.MethodReferenceRoleValid(field, methodReferenceMutableString, true)
}

func (p *SemanticContext) XmlAddNodeArgumentsValid(command string, parts []SyntaxSpan, start int) (int, string, bool) {
	if len(parts) < 3 || len(parts) > 5 {
		return start, "method argument count does not match", false
	}
	if offset, message, valid := p.XmlTargetArgumentsValid(parts[0], strings.HasSuffix(command, "_BYNAME")); !valid {
		return offset, message, false
	}
	for _, field := range parts[1:3] {
		if offset, message, valid := p.MethodFieldTypeValid(field, KnownValueString, true); !valid {
			return offset, message, false
		}
	}
	for _, field := range parts[3:] {
		if offset, message, valid := p.MethodFieldTypeValid(field, KnownValueNumeric, true); !valid {
			return offset, message, false
		}
	}
	return 0, "", true
}

func (p *SemanticContext) XmlRemoveNodeArgumentsValid(command string, parts []SyntaxSpan, start int) (int, string, bool) {
	if len(parts) < 2 || len(parts) > 3 {
		return start, "method argument count does not match", false
	}
	if offset, message, valid := p.XmlTargetArgumentsValid(parts[0], strings.HasSuffix(command, "_BYNAME")); !valid {
		return offset, message, false
	}
	if offset, message, valid := p.MethodFieldTypeValid(parts[1], KnownValueString, true); !valid {
		return offset, message, false
	}
	if len(parts) == 3 {
		return p.MethodFieldTypeValid(parts[2], KnownValueNumeric, true)
	}
	return 0, "", true
}

func (p *SemanticContext) MethodFieldTypeValid(field SyntaxSpan, expected KnownValueType, required bool) (int, string, bool) {
	left, right := ExtraFieldBoundsWithSpace(p.Src, field, p.AllowFullwidthSpace)
	if left == right {
		if required {
			return left, "method contains an omitted argument", false
		}
		return 0, "", true
	}
	tokens, err := p.LexSyntaxRange(left, right)
	if err != nil {
		return 0, "", true
	}
	if BuiltinScalarIndexesMissing(tokens, p.Dialect) {
		return left, "scalar variable requires indexes", false
	}
	actual := KnownExpressionTypeForDialect(tokens, p.Dialect)
	if actual != KnownValueUnknown && !CommandTypeMatches(expected, actual) {
		return left, "method argument has an incompatible type", false
	}
	return 0, "", true
}

func (p *SemanticContext) MethodReferenceArgumentValid(field SyntaxSpan, expected KnownValueType, required bool, requireArray bool) (int, string, bool) {
	left, right := ExtraFieldBoundsWithSpace(p.Src, field, p.AllowFullwidthSpace)
	if left == right {
		if required {
			return left, "method contains an omitted argument", false
		}
		return 0, "", true
	}
	tokens, err := p.LexSyntaxRange(left, right)
	if err != nil {
		return 0, "", true
	}
	if known, valid := p.MethodReferenceFieldValid(tokens, expected, requireArray); known && !valid {
		return left, "method argument has an incompatible type", false
	}
	return 0, "", true
}

func (p *SemanticContext) MethodReferenceFieldValid(tokens []ExprToken, expected KnownValueType, requireArray bool) (bool, bool) {
	tokens = stripExpressionParens(tokens)
	if info, known := BuiltinVariableRoleInfo(tokens, p.Dialect); known {
		if BuiltinVariableReadOnly(tokens, p.Dialect) {
			return true, false
		}
		return true, !info.Character && (!requireArray || info.Rank == 1) && (expected == KnownValueUnknown || info.Value == expected)
	}
	if KnownExpressionTypeForDialect(tokens, p.Dialect) != KnownValueUnknown {
		return true, false
	}
	return false, false
}

func (p *SemanticContext) ReplaceArgumentsValid(parts []SyntaxSpan) (int, string, bool) {
	mode, modeKnown := int64(0), false
	for n, field := range parts {
		left, right := ExtraFieldBoundsWithSpace(p.Src, field, p.AllowFullwidthSpace)
		if left == right {
			return left, "method contains an omitted argument", false
		}
		tokens, err := p.LexSyntaxRange(left, right)
		if err != nil || ExpressionNeedsDeferral(tokens, p.Dialect) {
			return 0, "", true
		}
		if BuiltinScalarIndexesMissing(tokens, p.Dialect) {
			return left, "scalar variable requires indexes", false
		}
		expected := KnownValueNumeric
		if n < 3 {
			expected = KnownValueString
		}
		if !CommandTypeMatches(expected, KnownExpressionTypeForDialect(tokens, p.Dialect)) {
			return left, "method argument has an incompatible type", false
		}
		if n == 3 {
			mode, modeKnown = constantIntegerValue(tokens)
		}
	}
	if modeKnown && int32(mode) == 1 {
		return p.MethodReferenceRoleValid(parts[2], methodReferenceMutableStringOneDimensional, true)
	}
	return 0, "", true
}

func containsExpressionMarker(tokens []ExprToken) bool {
	for _, token := range tokens {
		if token.Kind == ExprMarker {
			return true
		}
	}
	return false
}

func containsExpressionForm(tokens []ExprToken) bool {
	for _, token := range tokens {
		if token.Kind == ExprForm {
			return true
		}
	}
	return false
}
