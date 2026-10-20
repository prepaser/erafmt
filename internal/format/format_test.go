package format

import (
	"bytes"
	"testing"
	"unicode/utf8"
)

func TestFormat(t *testing.T) {
	cases := []struct {
		name, input, want string
		kind              Kind
	}{
		{"blocks", "@F\n#DIM USR_X\nIF USR_X>0\nSELECTCASE USR_X\nCASE 1,2\nPRINTL  a;b  \nCASEELSE\nSIF USR_X==0\nRETURN 0\nENDSELECT\nELSE\nSTR:0 = a;b  \nENDIF\n", "@F\n    #DIM USR_X\n\n    IF USR_X > 0\n        SELECTCASE USR_X\n            CASE 1, 2\n                PRINTL  a;b  \n            CASEELSE\n                SIF USR_X == 0\n                    RETURN 0\n        ENDSELECT\n    ELSE\n        STR:0 = a;b  \n    ENDIF\n", ERB},
		{"data_and_functions", "@F\nPRINTDATAKL\nDATALIST\nDATAFORM  %STR%  \nENDLIST\nENDDATA\nTRYCALLLIST\nFUNC F, 1+2\nENDFUNC\n", "@F\n    PRINTDATAKL\n        DATALIST\n            DATAFORM  %STR%  \n        ENDLIST\n    ENDDATA\n    TRYCALLLIST\n        FUNC F, 1 + 2\n    ENDFUNC\n", ERB},
		{"try_and_loops", "@F\nTRYCCALL F\nPRINTL success\nCATCH\nFOR I,0,10\nWHILE I<2\nDO\nPRINTL body\nLOOP I>0\nWEND\nNEXT\nENDCATCH\n", "@F\n    TRYCCALL F\n        PRINTL success\n    CATCH\n        FOR I, 0, 10\n            WHILE I < 2\n                DO\n                    PRINTL body\n                LOOP I > 0\n            WEND\n        NEXT\n    ENDCATCH\n", ERB},
		{"continuation_and_sif", "@F\r\nSIF A>0\n; between\r{\nPRINTFORM  one\r\n\t two  \n  }\rRETURN 0", "@F\n    SIF A > 0\n    ; between\n        {\nPRINTFORM  one\n\t two  \n        }\n    RETURN 0\n", ERB},
		{"headers", "\t#DEFINE M  1+2\r\n #DIMS USR_S, 2 = \"x  \"\n; comment\n\t \r", "#DEFINE M 1+2\n#DIMS USR_S, 2 = \"x  \"\n; comment\n", ERH},
		{"conditional_merge", "@F\n[IF USR_X]\nIF A>1\n[ELSE]\nIF B<2\n[ENDIF]\nRETURN 1\nENDIF\n", "@F\n[IF USR_X]\n    IF A > 1\n[ELSE]\n    IF B < 2\n[ENDIF]\n        RETURN 1\n    ENDIF\n", ERB},
		{"skip", "[SKIPSTART]\n@IGNORED\n{\nIF bad\n[IF USR_X]\n}\n[ENDIF]\n[SKIPEND]\n@F\nRETURN 0\n", "[SKIPSTART]\n@IGNORED\n{\nIF bad\n[IF USR_X]\n}\n[ENDIF]\n[SKIPEND]\n@F\n    RETURN 0\n", ERB},
		{"marker_leaf", "@F\n;!; PRINTL  keep  \n;#; PRINTL debug\n;^; PRINTL ee\n", "@F\n    ;!; PRINTL  keep  \n    ;#; PRINTL debug\n    ;^; PRINTL ee\n", ERB},
		{"empty_continuation_is_not_sif_target", "@F\nSIF A\n{\n; comment\n}\nRETURN 1\n", "@F\n    SIF A\n    {\n; comment\n    }\n        RETURN 1\n", ERB},
		{"continuation_trim_rules", "@F\n{\nPRINTL braces\n{  \n\u3000}\nRETURN 0\n", "@F\n    {\nPRINTL braces\n{  \n    \u3000}\n    RETURN 0\n", ERB},
		{"continued_preprocessor", "@F\n{\n[SKIPSTART]\n}\nIF invalid\n[SKIPEND]\nRETURN 0\n", "@F\n{\n[SKIPSTART]\n}\nIF invalid\n[SKIPEND]\n    RETURN 0\n", ERB},
		{"form_expression", "@F\nIF   STRLENS(@\"x {A+1}\")\nPRINTL x\nENDIF\n", "@F\n    IF STRLENS(@\"x {A + 1}\")\n        PRINTL x\n    ENDIF\n", ERB},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got, diagnostics := Format([]byte(tt.input), tt.kind)
			if len(diagnostics) != 0 {
				t.Fatalf("unexpected diagnostics: %+v", diagnostics)
			}
			if string(got) != tt.want {
				t.Fatalf("got %q\nwant %q", got, tt.want)
			}
			again, _ := Format(got, tt.kind)
			if !bytes.Equal(got, again) {
				t.Fatalf("not idempotent: %q", again)
			}
		})
	}
}

func TestUnsupportedFunctionIsPreserved(t *testing.T) {
	for _, body := range []string{
		";#; IF A\nPRINTL x\n;#; ENDIF\n",
		"　IF A\nENDIF\n",
		"[[OPEN_BLOCK]]\nPRINTL x\nENDIF\n",
		"[UNKNOWN]\nRETURN 0\n",
		"I[[CMD]] A\nENDIF\n",
		"[IF USR_X]\n{\n{\nPRINTL x\n}\n[ENDIF]\n",
	} {
		input := "@BAD\n" + body + "@GOOD\nIF A>0\nRETURN 1\nENDIF\n"
		got, diagnostics := Format([]byte(input), ERB)
		want := "@BAD\n" + body + "@GOOD\n    IF A > 0\n        RETURN 1\n    ENDIF\n"
		if string(got) != want {
			t.Fatalf("got %q\nwant %q\ndiagnostics %+v", got, want, diagnostics)
		}
		if len(diagnostics) == 0 || diagnostics[0].Severity != Unsupported {
			t.Fatalf("expected unsupported diagnostic: %+v", diagnostics)
		}
		again, _ := Format(got, ERB)
		if !bytes.Equal(got, again) {
			t.Fatal("partial result is not idempotent")
		}
	}
}

func TestInvalidStructureNeverChangesFile(t *testing.T) {
	for _, input := range []string{
		"@F\nFOR I,0,1\nWEND\n",
		"@F\nSIF A\nIF B\nENDIF\n",
		"@F\nSIF A\n$LABEL\n",
		"@F\nPRINTDATA\nPRINTDATA\nENDDATA\nENDDATA\n",
		"@F\nDATALIST\nENDLIST\n",
		"@F\nSIF A\n",
		"@F\nIF A\n@G\nRETURN 1\n",
		"@F\n{\nPRINTL x\n",
		"@F\nTRYCCALL F\nENDCATCH\n",
		"@F\nSELECTCASE USR_X\nPRINTL bad\nENDSELECT\n",
		"@F\nSELECTCASE USR_X\nCASE 1\nSIF Y\nCASE 2\nENDSELECT\n",
	} {
		got, diagnostics := Format([]byte(input), ERB)
		if !bytes.Equal(got, []byte(input)) {
			t.Fatalf("invalid input changed: %q", input)
		}
		found := false
		for _, d := range diagnostics {
			found = found || d.Severity == Error
		}
		if !found {
			t.Fatalf("expected error for %q: %+v", input, diagnostics)
		}
	}
}

func TestWarningOnlyPreprocessorFormsPreserveFile(t *testing.T) {
	cases := []struct {
		name  string
		input string
		code  string
	}{
		{"inconsistent function branch state", "@BAD\n[IF USR_X]\nIF A\n[ENDIF]\nRETURN 0\n@GOOD\nRETURN 1+2\n", "unsupported_structure"},
		{"inconsistent branch state", "[IF USR_X]\nIF A\n[ELSE]\nRETURN 0\n[ENDIF]\n@F\nRETURN 1+2\n", "unsupported_structure"},
		{"nested skip", "@F\n[SKIPSTART]\n[SKIPSTART]\n[SKIPEND]\n[SKIPEND]\nRETURN 1+2\n", "invalid_structure"},
		{"unmatched skip end", "@F\n[SKIPEND]\nRETURN 1+2\n", "invalid_structure"},
		{"unmatched endif", "@F\n[ENDIF]\nRETURN 1+2\n", "invalid_structure"},
		{"unmatched branch", "@F\n[ELSEIF USR_X]\nRETURN 1+2\n", "invalid_structure"},
		{"repeated else", "@F\n[IF USR_X]\n[ELSE]\n[ELSE]\n[ENDIF]\nRETURN 1+2\n", "invalid_structure"},
		{"branch after else", "@F\n[IF USR_X]\n[ELSE]\n[ELSEIF Y]\n[ENDIF]\nRETURN 1+2\n", "invalid_structure"},
		{"empty directive", "@F\n[]\nRETURN 1+2\n", "invalid_preprocessor"},
		{"unterminated directive", "@F\n[IF USR_X\nRETURN 1+2\n", "invalid_preprocessor"},
		{"unterminated region", "@F\n[IF USR_X]\nRETURN 1+2\n", "invalid_preprocessor"},
		{"unterminated skip", "[SKIPSTART]\n@F\n", "invalid_preprocessor"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := []byte(tc.input)
			r := FormatWithOptions(input, ERB, Options{Ignore: []RuleID{FinalNewline, LineEndingNormalization}})
			if !bytes.Equal(r.Output, input) || len(r.Changes) != 0 || len(r.Diagnostics) != 1 {
				t.Fatalf("result=%+v", r)
			}
			d := r.Diagnostics[0]
			if d.Severity != Unsupported || d.Code != tc.code {
				t.Fatalf("diagnostic=%+v", d)
			}
		})
	}

	output, diagnostics := Format([]byte("@F\r\n[SKIPEND]\r\nRETURN 1+2"), ERB)
	if string(output) != "@F\n[SKIPEND]\nRETURN 1+2\n" || len(diagnostics) != 1 || diagnostics[0].Severity != Unsupported {
		t.Fatalf("global rules output=%q diagnostics=%+v", output, diagnostics)
	}
}

func TestFatalHeaderAfterWarningPreprocessorIsReported(t *testing.T) {
	for _, input := range []string{
		"[SKIPEND]\r\n@123",
		"@F\r\n[ELSE]\r\nRETURN 0\r\n@123",
	} {
		r := FormatWithOptions([]byte(input), ERB, Options{})
		if string(r.Output) != input || len(r.Changes) != 0 {
			t.Fatalf("input=%q result=%+v", input, r)
		}
		unsupported, invalidHeader := false, false
		for _, d := range r.Diagnostics {
			unsupported = unsupported || d.Severity == Unsupported
			invalidHeader = invalidHeader || d.Severity == Error && d.Code == "invalid_header"
		}
		if !unsupported || !invalidHeader {
			t.Fatalf("input=%q diagnostics=%+v", input, r.Diagnostics)
		}
	}
}

func TestConditionalFunctionBoundaryPreservesFile(t *testing.T) {
	input := []byte("@F\nRETURN 0\n[IF USR_X]\n@G\nRETURN 1\n[ENDIF]\n")
	got, diagnostics := Format(input, ERB)
	if !bytes.Equal(got, input) || len(diagnostics) == 0 || diagnostics[0].Severity != Unsupported {
		t.Fatalf("got %q, %+v", got, diagnostics)
	}
}

func TestSifPartialMetadata(t *testing.T) {
	if !partial("CASE") || !partial("CASEELSE") || !partial("ENDDATA") || partial("BREAK") {
		t.Fatal("PARTIAL classification differs from instruction flags")
	}
}

func FuzzFormat(f *testing.F) {
	f.Add([]byte("\n\ufeff"), byte(0))
	for _, seed := range []string{"", "@F\nIF A>0\nRETURN 1\nENDIF\n", "[IF USR_X]\nSIF A\n[ENDIF]\n", "@F\n{\nPRINTFORM x\n}\n", "　IF USR_X\n", "@F\n;#; IF USR_X\n"} {
		f.Add([]byte(seed), byte(0))
	}
	f.Fuzz(func(t *testing.T, src []byte, k byte) {
		kind := Kind(k % 2)
		out, diagnostics := Format(src, kind)
		if !utf8.Valid(src) {
			if len(diagnostics) == 0 || !bytes.Equal(out, src) {
				t.Fatal("invalid encoding was not preserved")
			}
			return
		}
		again, againDiagnostics := Format(out, kind)
		if hasFatalDiagnostic(diagnostics) != hasFatalDiagnostic(againDiagnostics) {
			t.Fatalf("fatal status changed: %+v -> %+v", diagnostics, againDiagnostics)
		}
		if !bytes.Equal(out, again) {
			t.Fatalf("not idempotent: %q -> %q -> %q", src, out, again)
		}
		content := func(b []byte) []byte {
			return bytes.Map(func(r rune) rune {
				if r == ' ' || r == '\t' || r == '\r' || r == '\n' {
					return -1
				}
				return r
			}, b)
		}
		if !bytes.Equal(content(src), content(out)) {
			t.Fatal("non-trivia content changed")
		}
	})
}
