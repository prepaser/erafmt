package format

import (
	"fmt"
	"slices"
)

type RuleID string

const (
	FunctionIndent            RuleID = "I001"
	DeclarationIndent         RuleID = "I002"
	LabelIndent               RuleID = "I003"
	InstructionIndent         RuleID = "I004"
	SIFIndent                 RuleID = "I005"
	CommentIndent             RuleID = "I006"
	ContinuationIndent        RuleID = "I007"
	PreprocessorIndent        RuleID = "I008"
	CommandSpacing            RuleID = "S001"
	BinarySpacing             RuleID = "S002"
	TernarySpacing            RuleID = "S003"
	UnarySpacing              RuleID = "S004"
	CommaSpacing              RuleID = "S005"
	DelimiterSpacing          RuleID = "S006"
	IndexSpacing              RuleID = "S007"
	CaseSpacing               RuleID = "S008"
	TokenSpacing              RuleID = "S009"
	AssignmentLeftSpacing     RuleID = "S010"
	AssignmentRightSpacing    RuleID = "S011"
	DeclarationWordSpacing    RuleID = "S012"
	DeclarationDefaultSpacing RuleID = "S013"
	InlineCommentSpacing      RuleID = "S014"
	CommentPrefixSpacing      RuleID = "S015"
	PreprocessorFieldSpacing  RuleID = "S016"
	FormPadding               RuleID = "S017"
	LabelMarkerSpacing        RuleID = "S018"
	FormConditionSpacing      RuleID = "S019"
	PixelUnitSpacing          RuleID = "S020"
	BlankLineWhitespace       RuleID = "L001"
	CodeTrailingWhitespace    RuleID = "L002"
	CommentTrailingWhitespace RuleID = "L003"
	FunctionSeparation        RuleID = "L004"
	BlankLineRuns             RuleID = "L005"
	FileBoundaryBlankLines    RuleID = "L006"
	DeclarationBodySeparation RuleID = "L007"
	FinalNewline              RuleID = "F001"
	LineEndingNormalization   RuleID = "F002"
)

type Rule struct {
	ID          RuleID
	Name        string
	Description string
}

var ruleCatalog = []Rule{
	{FunctionIndent, "function-indent", "Place function headers at column zero."},
	{DeclarationIndent, "declaration-indent", "Indent ERB declarations inside functions; keep ERH declarations at column zero."},
	{LabelIndent, "label-indent", "Indent labels at the current statement depth."},
	{InstructionIndent, "instruction-indent", "Indent instructions and block boundaries, excluding SIF targets, with four spaces per level."},
	{SIFIndent, "sif-indent", "Indent SIF target instructions one level deeper than the surrounding statements."},
	{CommentIndent, "comment-indent", "Indent standalone comments at the current statement depth."},
	{ContinuationIndent, "continuation-indent", "Indent continuation delimiters with their logical statement."},
	{PreprocessorIndent, "preprocessor-indent", "Place standalone preprocessor directives at column zero."},
	{CommandSpacing, "command-spacing", "Use one space between a selected expression command and its argument."},
	{BinarySpacing, "binary-spacing", "Use one space around binary operators."},
	{TernarySpacing, "ternary-spacing", "Use one space around ternary question-mark and hash operators."},
	{UnarySpacing, "unary-spacing", "Attach unary operators without merging distinct operator tokens."},
	{CommaSpacing, "comma-spacing", "Remove spaces before commas and add one after them, preserving omitted arguments."},
	{DelimiterSpacing, "delimiter-spacing", "Remove spaces inside parentheses and brackets and before opening delimiters."},
	{IndexSpacing, "index-spacing", "Remove spaces around index colons."},
	{CaseSpacing, "case-spacing", "Use one space around CASE IS and TO keywords."},
	{TokenSpacing, "token-spacing", "Normalize remaining expression token gaps, retaining required word separators."},
	{AssignmentLeftSpacing, "assignment-left-spacing", "Use one space before assignment operators."},
	{AssignmentRightSpacing, "assignment-right-spacing", "Use one space after assignment operators, preserving plain assignment payloads."},
	{DeclarationWordSpacing, "declaration-word-spacing", "Use one space between declaration keywords, modifiers, names, and object DEFINE bodies."},
	{DeclarationDefaultSpacing, "declaration-default-spacing", "Use one space around declaration and parameter default equals signs."},
	{InlineCommentSpacing, "inline-comment-spacing", "Use one space before proven code comments."},
	{CommentPrefixSpacing, "comment-prefix-spacing", "Use one space after proven ordinary comment prefixes."},
	{PreprocessorFieldSpacing, "preprocessor-field-spacing", "Normalize field gaps inside trusted preprocessor brackets."},
	{FormPadding, "form-placeholder-padding", "Remove edge padding inside proven FORM placeholders."},
	{LabelMarkerSpacing, "label-marker-spacing", "Remove grammar whitespace between trusted label markers and identifiers."},
	{FormConditionSpacing, "form-condition-spacing", "Use one space after opening FORM conditional markers and before their question marks."},
	{PixelUnitSpacing, "pixel-unit-spacing", "Use one space before proven EMEE pixel unit keywords."},
	{BlankLineWhitespace, "blank-line-whitespace", "Remove grammar whitespace from surviving blank lines."},
	{CodeTrailingWhitespace, "code-trailing-whitespace", "Remove trailing grammar whitespace from proven code statements."},
	{CommentTrailingWhitespace, "comment-trailing-whitespace", "Remove trailing ASCII whitespace from ordinary comments."},
	{FunctionSeparation, "function-separation", "Keep one blank line before each subsequent trusted function and its attached comments."},
	{BlankLineRuns, "blank-line-runs", "Reduce ordinary blank-line runs to one line."},
	{FileBoundaryBlankLines, "file-boundary-blank-lines", "Remove ordinary blank lines at file boundaries."},
	{DeclarationBodySeparation, "declaration-body-separation", "Separate trusted local declarations from the function body with one blank line."},
	{FinalNewline, "final-newline", "Terminate nonempty files with a newline."},
	{LineEndingNormalization, "line-ending-normalization", "Normalize physical line endings to LF or the selected style."},
}

func Rules() []Rule { return slices.Clone(ruleCatalog) }

type Options struct {
	Select                []RuleID
	Ignore                []RuleID
	EOL                   EOL
	Dialect               Dialect
	AllowFullwidthSpace   bool
	ContinuationSeparator *string
}

type Dialect uint8

const (
	CommonDialect Dialect = iota
	Emuera1824
	EMEE
)

type EOL uint8

const (
	DefaultEOL EOL = iota
	LF
	CRLF
)

type Change struct {
	Rule   RuleID
	Line   int
	Column int
}

type Result struct {
	Output      []byte
	Diagnostics []Diagnostic
	Changes     []Change
}

type ruleSet map[RuleID]bool

func resolveRules(options Options) (ruleSet, error) {
	set := make(ruleSet, len(ruleCatalog))
	for _, r := range ruleCatalog {
		set[r.ID] = options.Select == nil
	}
	for _, ids := range [][]RuleID{options.Select, options.Ignore} {
		for _, id := range ids {
			if _, ok := set[id]; !ok {
				return nil, fmt.Errorf("unknown formatting rule %q", id)
			}
		}
	}
	for _, id := range options.Select {
		set[id] = true
	}
	for _, id := range options.Ignore {
		set[id] = false
	}
	return set, nil
}
