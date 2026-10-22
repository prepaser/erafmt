package format

import (
	"bytes"
	"strings"
	"testing"

	"github.com/prepaser/erafmt/internal/erabasic"
)

func TestConditionalProbesLeaveRawPrintPayloadOpaque(t *testing.T) {
	payload := string(nestedFormSource(erabasic.MaxFormDepth + 1))
	for _, prefix := range []string{"", "+= ", "= ", ":0 = "} {
		for _, input := range []string{"@F\n;#; SIF 1\n;#; PRINT " + prefix + payload + "\nRETURN 1+2\n", "@F\nIF 1\n[IF X]\nELSE\n[ELSE]\n[ENDIF]\nPRINT " + prefix + payload + "\nENDIF\n"} {
			result := FormatWithOptions([]byte(input), ERB, Options{})
			if len(result.Diagnostics) != 0 || !bytes.Contains(result.Output, []byte("PRINT "+prefix+payload)) {
				t.Fatalf("raw payload prefix %q changed result: diagnostics=%+v", prefix, result.Diagnostics)
			}
		}
	}
}

func TestConditionalProbesLeaveRawPrintVPayloadOpaque(t *testing.T) {
	payload := "'" + string(nestedFormSource(erabasic.MaxFormDepth+1))
	for _, input := range []string{"@F\nPRINTV " + payload + "\n", "@F\n;#; SIF 1\n;#; PRINTV " + payload + "\nRETURN 1+2\n", "@F\nIF 1\n[IF X]\nELSE\n[ELSE]\n[ENDIF]\nPRINTV " + payload + "\nENDIF\n"} {
		result := FormatWithOptions([]byte(input), ERB, Options{})
		if len(result.Diagnostics) != 0 || !bytes.Contains(result.Output, []byte("PRINTV "+payload)) {
			t.Fatalf("diagnostics=%+v output changed=%v", result.Diagnostics, !bytes.Contains(result.Output, []byte("PRINTV "+payload)))
		}
	}
}

func TestConditionalProbesLeaveRawPrintVNPayloadOpaque(t *testing.T) {
	payload := "'" + string(nestedFormSource(erabasic.MaxFormDepth+1))
	input := "@F\n;#; SIF 1\n;#; PRINTVN " + payload + "\nRETURN 1+2\n"
	result := FormatWithOptions([]byte(input), ERB, Options{Dialect: EMEE})
	if len(result.Diagnostics) != 0 || !bytes.Contains(result.Output, []byte("PRINTVN "+payload)) {
		t.Fatalf("diagnostics=%+v output changed=%v", result.Diagnostics, !bytes.Contains(result.Output, []byte("PRINTVN "+payload)))
	}
}

func TestConditionalProbesLeaveInactiveDebugFormsOpaque(t *testing.T) {
	payload := string(nestedFormSource(erabasic.MaxFormDepth + 1))
	for _, command := range []string{"DEBUGPRINTFORM", "DEBUGPRINTFORML"} {
		for _, input := range []string{"@F\n;#; SIF 1\n;#; " + command + " " + payload + "\nRETURN 1+2\n", "@F\nIF 1\n[IF X]\nELSE\n[ELSE]\n[ENDIF]\n" + command + " " + payload + "\nENDIF\n"} {
			result := FormatWithOptions([]byte(input), ERB, Options{})
			if len(result.Diagnostics) != 0 || !bytes.Contains(result.Output, []byte(command+" "+payload)) {
				t.Fatalf("%s probe changed result: diagnostics=%+v", command, result.Diagnostics)
			}
		}
	}
}

func TestConditionalConvergenceDeduplicatesCommonStates(t *testing.T) {
	input := []byte("@F\nIF 1\n[IF USR_X]\nELSE\n" + strings.Repeat("[ELSEIF USR_X]\n", 2048) + "[ENDIF]\n" + strings.Repeat("RETURN 0\n", 2048) + "ENDIF\n")
	result := FormatWithOptions(input, ERB, Options{Select: []RuleID{}})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%+v", result.Diagnostics)
	}
}

func TestConditionalConvergenceWorkLimitUsesProbedRow(t *testing.T) {
	input := []byte("@F\nRETURN 0\n")
	p := formatter{DocumentSession: erabasic.DocumentSession{Src: input, Lines: linesOfWithSpace(input, false), ConvergenceWork: erabasic.MaxConvergenceWork}}
	if p.branchesConverge(0, []erabasic.State{{}}) {
		t.Fatal("convergence succeeded beyond the work limit")
	}
	if !p.Aborted || len(p.diagnostics) != 1 {
		t.Fatalf("aborted=%v diagnostics=%+v", p.Aborted, p.diagnostics)
	}
	if p.diagnostics[0].Line != 2 || p.diagnostics[0].Code != "resource_limit" {
		t.Fatalf("diagnostic=%+v, want resource limit on the probed row", p.diagnostics[0])
	}
	if !bytes.Equal(p.Src, input) {
		t.Fatalf("resource exhaustion changed source: %q", p.Src)
	}
}

func TestOptionalSIFPairPreservesPairAndFormatsFollowingCode(t *testing.T) {
	input := []byte("@F\nIF X\n\t;#; SIF A\n\t;#;\tPRINTL  pair  \nPRINTL  after\nENDIF\n")
	want := "@F\n    IF X\n\t;#; SIF A\n\t;#;\tPRINTL  pair  \n        PRINTL  after\n    ENDIF\n"
	r := FormatWithOptions(input, ERB, Options{Select: []RuleID{InstructionIndent}})
	if len(r.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v", r.Diagnostics)
	}
	if string(r.Output) != want {
		t.Fatalf("output = %q, want %q", r.Output, want)
	}
}

func TestOptionalSIFPairIsQuietWithNoRules(t *testing.T) {
	input := []byte("@F\n;#;SIF A\n;#;PRINTL pair\nRETURN 0\n")
	r := FormatWithOptions(input, ERB, Options{Select: []RuleID{}})
	if len(r.Diagnostics) != 0 || len(r.Changes) != 0 || !bytes.Equal(r.Output, input) {
		t.Fatalf("result = %+v, want unchanged quiet result", r)
	}
}

func TestOptionalSIFPairFallsBackWhenItCannotProveThePair(t *testing.T) {
	cases := []struct {
		name string
		body string
		code string
	}{
		{"mismatched marker", ";#;SIF A\n;^;PRINTL x\n", "unsupported_structure"},
		{"nested sif", ";#;SIF A\n;#;SIF B\n", "unsupported_structure"},
		{"end of file", ";#;SIF A\n", "unsupported_structure"},
		{"data context", "PRINTDATA\n;#;SIF A\n;#;PRINTL x\nENDDATA\n", "invalid_structure"},
		{"unknown target", ";#;SIF A\n;#;FUTURE x\n", "unsupported_structure"},
		{"lowercase target", ";#;SIF A\n;#;printl x\n", "unsupported_structure"},
		{"alias condition", ";#;SIF [[A]]\n;#;PRINTL x\n", "unsupported_structure"},
		{"stacked marker", ";#;;#;SIF A\n;#;PRINTL x\n", "unsupported_structure"},
		{"pair in sif target", "SIF A\n;#;SIF B\n;#;PRINTL x\n", "invalid_structure"},
		{"declaration after pair", ";#;SIF A\n;#;PRINTL x\n#DIM X\n", "unsupported_declaration"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			input := []byte("@F\n" + tt.body + "RETURN 0\n")
			r := FormatWithOptions(input, ERB, Options{Select: []RuleID{InstructionIndent}})
			if !bytes.Equal(r.Output, input) {
				t.Fatalf("output changed: %q", r.Output)
			}
			if len(r.Diagnostics) == 0 || r.Diagnostics[0].Code != tt.code {
				t.Fatalf("diagnostics = %+v, want %s", r.Diagnostics, tt.code)
			}
		})
	}
}

func TestOptionalSIFPairAcceptsSetRoles(t *testing.T) {
	cases := []string{
		"VALUE = 1",
		"VALUE == 1",
		"VALUE += 1",
		"VALUE -= 1",
		"VALUE *= 1",
		"VALUE /= 1",
		"VALUE %= 1",
		"VALUE &= 1",
		"VALUE |= 1",
		"VALUE ^= 1",
		"VALUE <<= 1",
		"VALUE >>= 1",
		"VALUE '= 1",
		"VALUE++",
		"VALUE--",
		"++VALUE",
		"--VALUE",
	}
	for _, target := range cases {
		t.Run(target, func(t *testing.T) {
			input := []byte("@F\nIF X\n;#;SIF A\n;#;" + target + "\nRETURN 0\nENDIF\n")
			result := FormatWithOptions(input, ERB, Options{Select: []RuleID{InstructionIndent}})
			want := "@F\n    IF X\n;#;SIF A\n;#;" + target + "\n        RETURN 0\n    ENDIF\n"
			if string(result.Output) != want || len(result.Diagnostics) != 0 {
				t.Fatalf("output=%q diagnostics=%+v, want %q", result.Output, result.Diagnostics, want)
			}
		})
	}
}

func TestOptionalSIFPairSetRoleGuards(t *testing.T) {
	for _, target := range []string{"IF += 1", "RETURN += 1", "SIF += 1", "VALUE[[A]] += 1", "VALUE + 1 = 2"} {
		t.Run(target, func(t *testing.T) {
			input := []byte("@F\nIF X\n;#;SIF A\n;#;" + target + "\nRETURN 0\nENDIF\n")
			result := FormatWithOptions(input, ERB, Options{Select: []RuleID{InstructionIndent}})
			if string(result.Output) != string(input) || len(result.Diagnostics) == 0 || result.Diagnostics[0].Code != "unsupported_structure" {
				t.Fatalf("output=%q diagnostics=%+v, want preserved unsupported input", result.Output, result.Diagnostics)
			}
		})
	}
	input := []byte("@F\nIF X\n;#;;#;SIF A\n;#;VALUE += 1\nRETURN 0\nENDIF\n")
	result := FormatWithOptions(input, ERB, Options{Select: []RuleID{InstructionIndent}})
	if string(result.Output) != string(input) || len(result.Diagnostics) == 0 || result.Diagnostics[0].Code != "unsupported_structure" {
		t.Fatalf("stacked marker output=%q diagnostics=%+v", result.Output, result.Diagnostics)
	}
	input = []byte("@F\nPRINTDATA\n;#;SIF A\n;#;VALUE += 1\nENDDATA\n")
	result = FormatWithOptions(input, ERB, Options{Select: []RuleID{InstructionIndent}})
	if string(result.Output) != string(input) || len(result.Diagnostics) == 0 || result.Diagnostics[0].Code != "invalid_structure" || result.Diagnostics[0].Line != 3 || result.Diagnostics[0].Column != 1 {
		t.Fatalf("data context output=%q diagnostics=%+v", result.Output, result.Diagnostics)
	}
}

func TestOptionalSIFPairAcceptsUnavailableCommandAssignment(t *testing.T) {
	input := []byte("@F\n;#;SIF A\n;#;FORCE_QUIT = 1\nRETURN 0\n")
	result := FormatWithOptions(input, ERB, Options{Dialect: Emuera1824, Select: []RuleID{InstructionIndent}})
	if len(result.Diagnostics) != 0 || string(result.Output) != "@F\n;#;SIF A\n;#;FORCE_QUIT = 1\n    RETURN 0\n" {
		t.Fatalf("unavailable assignment output=%q diagnostics=%+v", result.Output, result.Diagnostics)
	}
}

func TestOptionalSIFPairSetRoleFullwidthAndSelectNone(t *testing.T) {
	input := []byte("@F\nIF X\n;#;SIF\u3000A\n;#;VALUE\u3000+=\u30001\nRETURN 0\nENDIF\n")
	result := FormatWithOptions(input, ERB, Options{AllowFullwidthSpace: true, Select: []RuleID{InstructionIndent}})
	want := "@F\n    IF X\n;#;SIF\u3000A\n;#;VALUE\u3000+=\u30001\n        RETURN 0\n    ENDIF\n"
	if string(result.Output) != want || len(result.Diagnostics) != 0 {
		t.Fatalf("fullwidth output=%q diagnostics=%+v, want %q", result.Output, result.Diagnostics, want)
	}
	unchanged := FormatWithOptions(input, ERB, Options{AllowFullwidthSpace: true, Select: []RuleID{}})
	if !bytes.Equal(unchanged.Output, input) || len(unchanged.Diagnostics) != 0 || len(unchanged.Changes) != 0 {
		t.Fatalf("select-none result=%+v", unchanged)
	}
}

func TestPreprocessorBranchesConvergeAcrossSetRoles(t *testing.T) {
	for _, target := range []string{"RESULT = rawliteral", "RESULT += 1", "RESULT++", "++RESULT"} {
		t.Run(target, func(t *testing.T) {
			input := []byte("@F\n[IF X]\nIF A\nELSE\nRETURN 0\n[ELSE]\nIF B\n[ENDIF]\n" + target + "\nENDIF\n")
			result := FormatWithOptions(input, ERB, Options{Select: []RuleID{InstructionIndent}})
			if len(result.Diagnostics) != 0 {
				t.Fatalf("diagnostics=%+v", result.Diagnostics)
			}
			again := FormatWithOptions(result.Output, ERB, Options{Select: []RuleID{InstructionIndent}})
			if !bytes.Equal(again.Output, result.Output) || len(again.Diagnostics) != 0 || len(again.Changes) != 0 {
				t.Fatalf("not idempotent: %+v", again)
			}
		})
	}
}

func TestPreprocessorBranchesConvergeAcrossCommonPath(t *testing.T) {
	input := []byte("@F\n[IF X]\nIF A\nELSE\nRETURN 0\n[ELSE]\nIF B\n[ENDIF]\nRETURN 1\nENDIF\n")
	want := "@F\n[IF X]\n    IF A\n    ELSE\n        RETURN 0\n[ELSE]\n    IF B\n[ENDIF]\n        RETURN 1\n    ENDIF\n"
	r := FormatWithOptions(input, ERB, Options{Select: []RuleID{InstructionIndent}})
	if string(r.Output) != want || len(r.Diagnostics) != 0 {
		t.Fatalf("output=%q diagnostics=%+v, want %q", r.Output, r.Diagnostics, want)
	}
	again := FormatWithOptions(r.Output, ERB, Options{Select: []RuleID{InstructionIndent}})
	if !bytes.Equal(again.Output, r.Output) || len(again.Diagnostics) != 0 || len(again.Changes) != 0 {
		t.Fatalf("not idempotent: %+v", again)
	}
	input = []byte("@F\n[IF X]\nIF A\nELSE\n[ELSE]\nIF B\n[ENDIF]\n; common\nENDIF\n")
	want = "@F\n[IF X]\n    IF A\n    ELSE\n[ELSE]\n    IF B\n[ENDIF]\n; common\n    ENDIF\n"
	r = FormatWithOptions(input, ERB, Options{Select: []RuleID{InstructionIndent}})
	if string(r.Output) != want || len(r.Diagnostics) != 0 {
		t.Fatalf("comment output=%q diagnostics=%+v, want %q", r.Output, r.Diagnostics, want)
	}
	again = FormatWithOptions(r.Output, ERB, Options{Select: []RuleID{InstructionIndent}})
	if !bytes.Equal(again.Output, r.Output) || len(again.Diagnostics) != 0 || len(again.Changes) != 0 {
		t.Fatalf("comment path is not idempotent: %+v", again)
	}
}

func TestPreprocessorBranchesConvergeRejectsUnsafePath(t *testing.T) {
	cases := []struct {
		name    string
		common  string
		dialect erabasic.Dialect
	}{
		{"pending sif", "SIF B", erabasic.CommonDialect},
		{"alias", "[[ALIAS]]", erabasic.CommonDialect},
		{"marker", ";#; RETURN 1", erabasic.CommonDialect},
		{"directive", "[IF Y]", erabasic.CommonDialect},
		{"label", "$LABEL", erabasic.CommonDialect},
		{"opener", "IF C", erabasic.CommonDialect},
		{"unknown", "FUTURE", erabasic.CommonDialect},
		{"dialect unavailable", "FORCE_QUIT", erabasic.Emuera1824},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := []byte("@F\n[IF X]\nIF A\nELSE\nRETURN 0\n[ELSE]\nIF B\n[ENDIF]\n" + tc.common + "\nRETURN 1\nENDIF\n")
			r := FormatWithOptions(input, ERB, Options{Select: []RuleID{InstructionIndent}, Dialect: Dialect(tc.dialect)})
			if string(r.Output) != string(input) || len(r.Diagnostics) == 0 || r.Diagnostics[0].Code != "unsupported_structure" {
				t.Fatalf("output=%q diagnostics=%+v, want preserved unsupported input", r.Output, r.Diagnostics)
			}
		})
	}
}

func TestPreprocessorBranchesConvergeRejectsNonIFMetadata(t *testing.T) {
	input := []byte("@F\n[IF X]\nSELECTCASE A\nCASE 1\n[ELSE]\nIF B\n[ENDIF]\nRETURN 1\nENDSELECT\n")
	r := FormatWithOptions(input, ERB, Options{Select: []RuleID{InstructionIndent}})
	if string(r.Output) != string(input) || len(r.Diagnostics) == 0 || r.Diagnostics[0].Code != "unsupported_structure" {
		t.Fatalf("output=%q diagnostics=%+v, want preserved unsupported input", r.Output, r.Diagnostics)
	}
}

func TestPreprocessorConditionAcceptsRuntimeIdentifierTokens(t *testing.T) {
	for _, token := range []string{"X", "X1", "1", "αβ_2"} {
		t.Run(token, func(t *testing.T) {
			input := []byte("@F\n[IF " + token + "]\nRETURN 1\n[ENDIF]\nRETURN 0\n")
			result := FormatWithOptions(input, ERB, Options{Select: []RuleID{InstructionIndent}})
			if len(result.Diagnostics) != 0 || !bytes.Contains(result.Output, []byte("[IF "+token+"]\n")) {
				t.Fatalf("output=%q diagnostics=%+v", result.Output, result.Diagnostics)
			}
		})
	}
	input := []byte("@F\n[IF　CONDITION]\nRETURN 1\n[ENDIF]\n")
	result := FormatWithOptions(input, ERB, Options{AllowFullwidthSpace: true, Select: []RuleID{InstructionIndent}})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("fullwidth condition diagnostics=%+v output=%q", result.Diagnostics, result.Output)
	}
}

func TestPreprocessorConditionShapePreservesMalformedDirectives(t *testing.T) {
	for _, condition := range []string{"X+1", "X;comment", `"X"`, "X + 1", "[[ALIAS]]"} {
		t.Run(condition, func(t *testing.T) {
			input := []byte("@F\n[IF " + condition + "]\nRETURN 1\n[ENDIF]\nRETURN 0\n")
			result := FormatWithOptions(input, ERB, Options{Select: []RuleID{InstructionIndent}})
			if !bytes.Equal(result.Output, input) || len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "unsupported_structure" {
				t.Fatalf("output=%q diagnostics=%+v", result.Output, result.Diagnostics)
			}
		})
	}
	input := []byte("@F\n[IF X]\nRETURN 1\n[ELSEIF X+1]\nRETURN 2\n[ENDIF]\nRETURN 0\n")
	result := FormatWithOptions(input, ERB, Options{Select: []RuleID{InstructionIndent}})
	if !bytes.Equal(result.Output, input) || len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "unsupported_structure" {
		t.Fatalf("malformed ELSEIF output=%q diagnostics=%+v", result.Output, result.Diagnostics)
	}
	input = []byte("@F\n[IF　X]\nRETURN 1\n[ENDIF]\n")
	result = FormatWithOptions(input, ERB, Options{Select: []RuleID{InstructionIndent}})
	if !bytes.Equal(result.Output, input) || len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "unsupported_structure" {
		t.Fatalf("disallowed fullwidth condition output=%q diagnostics=%+v", result.Output, result.Diagnostics)
	}
}

func TestPreprocessorAliasStopsBeforeFollowingMalformedHeader(t *testing.T) {
	for _, input := range []string{"@F\n[IF [[ALIAS]]]\n@BAD(\nRETURN 1\n[ENDIF]\n", "@F\n{\n[IF [[ALIAS]]]\n}\n@BAD(\nRETURN 1\n[ENDIF]\n"} {
		result := FormatWithOptions([]byte(input), ERB, Options{})
		if !bytes.Equal(result.Output, []byte(input)) || len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "unsupported_structure" || result.Diagnostics[0].Message != "preprocessor structure depends on aliases" {
			t.Fatalf("output=%q diagnostics=%+v", result.Output, result.Diagnostics)
		}
	}
}

func TestPreprocessorBranchesConvergeDoesNotMergeRuntimeBranch(t *testing.T) {
	for _, command := range []string{"ELSE", "ELSEIF C"} {
		t.Run(command, func(t *testing.T) {
			input := []byte("@F\n[IF X]\nIF A\nELSE\nRETURN 0\n[ELSE]\nIF B\n[ENDIF]\nRETURN 1\n" + command + "\nENDIF\n")
			r := FormatWithOptions(input, ERB, Options{Select: []RuleID{InstructionIndent}})
			if string(r.Output) != string(input) || len(r.Diagnostics) == 0 || r.Diagnostics[0].Severity != Unsupported {
				t.Fatalf("output=%q diagnostics=%+v, want preserved unsupported input", r.Output, r.Diagnostics)
			}
		})
	}
}
