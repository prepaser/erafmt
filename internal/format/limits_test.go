package format

import (
	"bytes"
	"strings"
	"testing"
)

func TestResourceLimitPreservesDeepStructure(t *testing.T) {
	var source strings.Builder
	source.WriteString("@F\n")
	for i := 0; i <= maxStructureDepth; i++ {
		source.WriteString("IF 1\n")
	}
	input := []byte(source.String())
	result := FormatWithOptions(input, ERB, Options{})
	assertResourceLimit(t, input, result)
}

func TestResourceLimitPreservesAmplifiedOutput(t *testing.T) {
	var source strings.Builder
	source.WriteString("@F\n")
	for i := 0; i < maxStructureDepth; i++ {
		source.WriteString("IF 1\n")
	}
	for i := 0; i < maxFormattedOutputBytes/(maxStructureDepth*4)+1024; i++ {
		source.WriteString("RETURN 0\n")
	}
	input := []byte(source.String())
	result := FormatWithOptions(input, ERB, Options{})
	assertResourceLimit(t, input, result)
}

func TestResourceLimitPreservesDeepPreprocessor(t *testing.T) {
	var source strings.Builder
	for i := 0; i <= maxPreprocessorDepth; i++ {
		source.WriteString("[IF X]\n")
	}
	input := []byte(source.String())
	result := FormatWithOptions(input, ERB, Options{})
	assertResourceLimit(t, input, result)
}

func TestResourceLimitPreservesLargeStatementSyntax(t *testing.T) {
	input := append([]byte("@F\nRETURN "), bytes.Repeat([]byte("F("), maxSyntaxBytes/2+1)...)
	result := FormatWithOptions(input, ERB, Options{})
	assertResourceLimit(t, input, result)
}

func TestResourceLimitAllowsUnchangedLargeInput(t *testing.T) {
	input := append([]byte("@F\nRETURN 0 "), bytes.Repeat([]byte{'0'}, maxFormattedOutputBytes+1)...)
	result := FormatWithOptions(input, ERB, Options{Select: []RuleID{}})
	if !bytes.Equal(result.Output, input) {
		t.Fatalf("large input changed")
	}
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Code == "resource_limit" {
			t.Fatalf("unchanged large input hit resource limit: %+v", diagnostic)
		}
	}
}

func TestResourceLimitBoundsMetadata(t *testing.T) {
	for _, tc := range []struct {
		name    string
		source  string
		options Options
	}{
		{"lines", strings.Repeat("\r\n", maxMetadataBytes/lineMetadataBytes+1), Options{Select: []RuleID{}}},
		{"edits", "@F\n" + strings.Repeat("RETURN 1+2 \n", 100000), Options{}},
		{"diagnostics", "@F\n" + strings.Repeat("UNKNOWN 0\n", 150000), Options{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := []byte(tc.source)
			assertResourceLimit(t, input, FormatWithOptions(input, ERB, tc.options))
		})
	}
}

func TestMetadataBudgetSharedWithContinuation(t *testing.T) {
	p := formatter{src: []byte("@F\n"), metadata: &metadataBudget{used: maxMetadataBytes - lineMetadataBytes}}
	p.lines = linesOf(p.src)
	q := p.continuationProxy([]byte("RETURN 0"))
	if q.aborted || q.metadata != p.metadata {
		t.Fatal("continuation did not share the available metadata budget")
	}
	q.warning(0, 0, "probe", "warning")
	if !q.aborted || !hasFatalDiagnostic(q.diagnostics) {
		t.Fatal("continuation exceeded its parent's metadata budget")
	}
}

func TestContinuationVerificationMetadataLimit(t *testing.T) {
	separator := ""
	rows := (maxMetadataBytes - 5*lineMetadataBytes - 2*editMetadataBytes) / (lineMetadataBytes + 2*editMetadataBytes)
	input := []byte("@F\n" + strings.Repeat("RETURN 1+2\n", rows) + "{\nRETURN 1+2\n}\n")
	options := Options{Select: []RuleID{BinarySpacing}, ContinuationSeparator: &separator}
	result := FormatWithOptions(input, ERB, options)
	assertResourceLimit(t, input, result)
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Code == "resource_limit" && diagnostic.Line != rows+3 {
			t.Fatalf("verification limit position: %+v", diagnostic)
		}
	}
	assertResourceLimit(t, input, FormatWithOptions(result.Output, ERB, options))
}

func TestColumnMetadataLimitStopsEdit(t *testing.T) {
	input := bytes.Repeat([]byte{'x'}, 8192)
	p := formatter{src: input, lines: linesOf(input), metadata: &metadataBudget{used: maxMetadataBytes - editMetadataBytes}}
	p.addEditUnchecked(0, 5000, 5000, []byte(" "), BinarySpacing)
	if !p.aborted || len(p.edits) != 0 || !hasFatalDiagnostic(p.diagnostics) {
		t.Fatalf("column limit: aborted=%v edits=%d diagnostics=%+v", p.aborted, len(p.edits), p.diagnostics)
	}
}

func TestPreprocessorMetadataLimitUsesSourceOrder(t *testing.T) {
	src := []byte(" [IF X]\n [IF X]\n")
	p := formatter{
		src:      src,
		lines:    linesOf(src),
		rules:    ruleSet{PreprocessorIndent: true},
		metadata: &metadataBudget{used: maxMetadataBytes - editMetadataBytes},
		ppLines:  map[int]int{0: bytes.IndexByte(src[:bytes.IndexByte(src, '\n')], ']'), 1: len(src) - 2},
	}
	p.layoutPreprocessors()
	if !p.aborted || len(p.edits) != 1 || len(p.diagnostics) != 1 {
		t.Fatalf("aborted=%v edits=%d diagnostics=%+v", p.aborted, len(p.edits), p.diagnostics)
	}
	if p.diagnostics[0].Code != "resource_limit" || p.diagnostics[0].Line != 2 {
		t.Fatalf("diagnostics=%+v", p.diagnostics)
	}
}

func TestRemoveLineRuleKeepsEditByteCache(t *testing.T) {
	p := formatter{src: []byte("RETURN 0\nUNKNOWN 0\n"), rules: ruleSet{InstructionIndent: true, BinarySpacing: true}}
	p.lines = linesOf(p.src)
	p.addEdit(0, 0, 0, []byte("    "), InstructionIndent)
	p.statementEdit = len(p.edits)
	p.addEdit(1, 9, 9, []byte("    "), InstructionIndent)
	p.addEdit(1, 17, 17, []byte(" "), BinarySpacing)
	storage := p.edits[:cap(p.edits)]
	p.removeLineRule(1, InstructionIndent)
	if len(p.edits) != 2 || p.editBytes != 5 || p.editBytesEdits != len(p.edits) {
		t.Fatalf("stale rollback cache: edits=%d bytes=%d cached=%d", len(p.edits), p.editBytes, p.editBytesEdits)
	}
	if storage[2].text != nil {
		t.Fatal("compaction retained discarded replacement text")
	}
	p.truncateEdits(1)
	if p.editBytes != 4 || p.editBytesEdits != len(p.edits) {
		t.Fatal("tail rollback invalidated the edit byte cache")
	}
	if storage[1].text != nil {
		t.Fatal("rollback retained discarded replacement text")
	}
	if !p.reserveReplacementBytes(9, 9, 2) || p.editBytes != 6 {
		t.Fatal("replacement accounting changed after rollback")
	}
}

func TestResourceLimitRejectsLargeContinuationSeparator(t *testing.T) {
	separator := strings.Repeat(" ", maxFormattedOutputBytes+1)
	input := []byte("@F\nRETURN 0\n")
	result := FormatWithOptions(input, ERB, Options{ContinuationSeparator: &separator})
	assertResourceLimit(t, input, result)
}

func TestResourceLimitBoundsReplacementWork(t *testing.T) {
	p := formatter{src: make([]byte, maxFormattedOutputBytes+1024)}
	if !p.reserveReplacementBytes(0, 1024, 0) {
		t.Fatal("expected a shrinking edit to fit")
	}
	if !p.reserveReplacementBytes(1024, 2048, 0) {
		t.Fatal("expected another shrinking edit to fit")
	}
	if !p.reserveReplacementBytes(0, 0, maxFormattedOutputBytes) {
		t.Fatal("expected replacement work at the limit to fit")
	}
	if p.reserveReplacementBytes(0, 0, 1) {
		t.Fatal("replacement work beyond the limit was accepted")
	}
}

func TestContinuationRowLimitBoundary(t *testing.T) {
	if continuationContentLimitExceeded(maxContinuationSegments - 1) {
		t.Fatal("last allowed continuation row was rejected")
	}
	if !continuationContentLimitExceeded(maxContinuationSegments) {
		t.Fatal("first continuation row beyond the limit was accepted")
	}
}

func TestParsingLimitsApplyWithoutExpressionRules(t *testing.T) {
	nested := strings.Repeat("F(", maxExpressionDepth+1) + "1" + strings.Repeat(")", maxExpressionDepth+1)
	for _, source := range []string{
		"@F(" + nested + ")\n",
		"@F\n#LOCALSIZE " + nested + "\n",
		"@F\nPRINT_SPACE " + nested + "\n",
		"@F\nTIMES " + nested + ",1\n",
		"@F\nSWAP " + nested + ",A\n",
	} {
		input := []byte(source)
		assertResourceLimit(t, input, FormatWithOptions(input, ERB, Options{Select: []RuleID{}}))
	}
	input := []byte("@F\nPRINT_SPACE " + strings.Repeat("1+", maxExpressionTokens/2) + "1\n")
	assertResourceLimit(t, input, FormatWithOptions(input, ERB, Options{Select: []RuleID{}}))
}

func TestParsingLimitIgnoresCanonicalWhitespace(t *testing.T) {
	input := []byte("@F\nRETURN F(\"" + strings.Repeat("x", 980000) + "\"," + strings.Repeat("1+", 20000) + "1)\n")
	options := Options{Select: []RuleID{BinarySpacing}}
	first := FormatWithOptions(input, ERB, options)
	if len(first.Diagnostics) != 0 || len(first.Output) <= maxSyntaxBytes {
		t.Fatalf("first pass: output=%d diagnostics=%+v", len(first.Output), first.Diagnostics)
	}
	second := FormatWithOptions(first.Output, ERB, options)
	if len(second.Diagnostics) != 0 || len(second.Changes) != 0 || !bytes.Equal(first.Output, second.Output) {
		t.Fatalf("second pass: changes=%d diagnostics=%+v", len(second.Changes), second.Diagnostics)
	}
}

func TestParsingLimitsLeaveOpaquePayloadsAlone(t *testing.T) {
	payload := strings.Repeat("x", maxSyntaxBytes+1)
	for _, command := range []string{"PRINT", "DEBUGPRINTFORM"} {
		input := []byte("@F\n" + command + " " + payload + "\n")
		result := FormatWithOptions(input, ERB, Options{})
		if len(result.Diagnostics) != 0 || !bytes.Contains(result.Output, []byte(command+" "+payload)) {
			t.Fatalf("%s: diagnostics=%+v", command, result.Diagnostics)
		}
	}
	input := []byte("@F\n[UNKNOWN]\nPRINT " + payload + "\n@G\nRETURN 1+2\n")
	result := FormatWithOptions(input, ERB, Options{})
	if !bytes.Contains(result.Output, []byte("PRINT "+payload)) || !bytes.Contains(result.Output, []byte("RETURN 1 + 2")) {
		t.Fatalf("preserved body blocked next function: diagnostics=%+v", result.Diagnostics)
	}
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Code == "resource_limit" {
			t.Fatalf("preserved payload hit limit: %+v", diagnostic)
		}
	}
	input = []byte("[UNKNOWN]\n@G\nRETURN F(" + strings.Repeat("F(", maxExpressionDepth+1) + "1" + strings.Repeat(")", maxExpressionDepth+2) + "\n")
	result = FormatWithOptions(input, ERB, Options{})
	if !bytes.Equal(result.Output, input) || hasFatalDiagnostic(result.Diagnostics) {
		t.Fatalf("preserved file body was parsed: %+v", result.Diagnostics)
	}
}

func TestResourceLimitBoundsPreprocessorSiblingStates(t *testing.T) {
	input := []byte("@F\n" + strings.Repeat("IF 1\n", 128) + "[IF X]\n" + strings.Repeat("[ELSEIF X]\n", 10000) + "[ENDIF]\n" + strings.Repeat("ENDIF\n", 128))
	assertResourceLimit(t, input, FormatWithOptions(input, ERB, Options{Select: []RuleID{}}))
	p := formatter{src: []byte("[IF X]\n[ELSE]\n[ENDIF]\n")}
	p.lines = linesOfWithSpace(p.src, false)
	for i := range p.lines {
		p.directive(i, lBody(p.src, p.lines[i]))
	}
	if p.ppStateUnits != 0 {
		t.Fatalf("closed preprocessor retained %d units", p.ppStateUnits)
	}
}

func TestContinuationResourceLimitUsesPhysicalPosition(t *testing.T) {
	separator := " "
	input := []byte("@F\n{\nRETURN F(\n" + strings.Repeat("F(", maxExpressionDepth) + "1" + strings.Repeat(")", maxExpressionDepth+1) + "\n}\n")
	result := FormatWithOptions(input, ERB, Options{ContinuationSeparator: &separator})
	assertResourceLimit(t, input, result)
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Code == "resource_limit" && diagnostic.Line != 4 {
			t.Fatalf("resource position not mapped: %+v", diagnostic)
		}
	}
}

func TestNestedFormLimitAcrossParsingPaths(t *testing.T) {
	nested := string(nestedFormSource(maxFormDepth + 1))
	for _, source := range []string{
		"@F(" + nested + ")\n",
		"@F\nPRINT_IMG " + nested + "\n",
		"@F\n#LOCALSIZE " + nested + "\n",
		"@F\nINPUTS " + nested + "\n",
	} {
		for _, options := range []Options{{}, {Select: []RuleID{}}} {
			input := []byte(source)
			assertResourceLimit(t, input, FormatWithOptions(input, ERB, options))
		}
	}
	input := []byte("@F\nDEBUGPRINTFORM " + nested + "\n")
	result := FormatWithOptions(input, ERB, Options{})
	if hasFatalDiagnostic(result.Diagnostics) {
		t.Fatalf("inactive debug FORM was parsed: %+v", result.Diagnostics)
	}
}

func TestResourceLimitBoundsFieldSplitting(t *testing.T) {
	fields := strings.Repeat("1,", maxExpressionTokens) + "1"
	for _, source := range []string{
		"@F(" + fields + ")\n",
		"@F\nPRINT_SPACE " + fields + "\n",
		"@F\n#DIM A," + fields + "\n",
		"@F\n#DIM USR_A," + fields + "\n",
		"@F\nPRINTFORM {" + fields + "}\n",
	} {
		for _, options := range []Options{{}, {Select: []RuleID{}}} {
			if strings.Contains(source, "PRINTFORM") && options.Select != nil {
				continue
			}
			input := []byte(source)
			assertResourceLimit(t, input, FormatWithOptions(input, ERB, options))
		}
	}
}

func hasFatalDiagnostic(diagnostics []Diagnostic) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == Error {
			return true
		}
	}
	return false
}

func assertResourceLimit(t *testing.T, input []byte, result Result) {
	t.Helper()
	if !bytes.Equal(result.Output, input) {
		t.Fatalf("resource limit changed output")
	}
	if len(result.Changes) != 0 {
		t.Fatalf("resource limit returned changes: %+v", result.Changes)
	}
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Code == "resource_limit" && diagnostic.Severity == Error {
			return
		}
	}
	t.Fatalf("missing resource_limit diagnostic: %+v", result.Diagnostics)
}

func TestAttachedCommentFilterKeepsEditByteCache(t *testing.T) {
	src := []byte("  ; note\n@F\n")
	p := formatter{
		src:   src,
		lines: linesOf(src),
		rules: ruleSet{CommentIndent: true},
	}
	p.headers = []int{1}
	p.edits = []edit{{start: 0, end: 2, text: []byte("    "), change: Change{Rule: CommentIndent, Line: 1}}}
	p.editBytes = 4
	p.editBytesEdits = 1
	p.layoutAttachedCommentsOnly(layoutInfo{attached: map[int]int{1: 0}})
	if len(p.edits) != 1 || p.editBytes != 0 || p.editBytesEdits != len(p.edits) {
		t.Fatalf("edits=%d bytes=%d cached=%d", len(p.edits), p.editBytes, p.editBytesEdits)
	}
}
