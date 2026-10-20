package format

import (
	"strings"
	"testing"
)

func TestTryGotoListRejectsFuncArguments(t *testing.T) {
	var state state
	_, message := state.instruction("TRYGOTOLIST", 1)
	if message != "" || state.frames[0].listCommand != "TRYGOTOLIST" {
		t.Fatalf("list frame=%+v message=%q", state.frames, message)
	}
	for _, tc := range []struct {
		name string
		src  string
		bad  bool
	}{
		{name: "plain", src: "@F\nTRYGOTOLIST\nFUNC TARGET\nENDFUNC\n"},
		{name: "subname", src: "@F\nTRYGOTOLIST\nFUNC TARGET[1]\nENDFUNC\n", bad: true},
		{name: "row args", src: "@F\nTRYGOTOLIST\nFUNC TARGET,1\nENDFUNC\n", bad: true},
		{name: "parenthesized args", src: "@F\nTRYGOTOLIST\nFUNC TARGET(1)\nENDFUNC\n", bad: true},
		{name: "empty parentheses", src: "@F\nTRYGOTOLIST\nFUNC TARGET()\nENDFUNC\n"},
		{name: "empty subnames", src: "@F\nTRYGOTOLIST\nFUNC TARGET[]\nENDFUNC\n"},
		{name: "args after empty subnames", src: "@F\nTRYGOTOLIST\nFUNC TARGET[](1)\nENDFUNC\n", bad: true},
		{name: "empty row args", src: "@F\nTRYGOTOLIST\nFUNC TARGET,\nENDFUNC\n"},
		{name: "macro args", src: "@F\nTRYGOTOLIST\nFUNC TARGET(EFAUDIT_ARGS)\nENDFUNC\n"},
		{name: "marked args", src: "@F\nTRYGOTOLIST\nFUNC TARGET(;!;1)\nENDFUNC\n"},
		{name: "other list", src: "@F\nTRYCALLLIST\nFUNC TARGET[1],1\nENDFUNC\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := FormatWithOptions([]byte(tc.src), ERB, commandOptions())
			if tc.bad {
				if len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "unsupported_syntax" || !strings.Contains(r.Diagnostics[0].Message, "TRYGOTOLIST") {
					t.Fatalf("expected TRY GOTO diagnostic: %+v", r.Diagnostics)
				}
				return
			}
			if len(r.Diagnostics) != 0 {
				t.Fatalf("unexpected diagnostics: %+v", r.Diagnostics)
			}
		})
	}
}

func TestTryGotoListVariantMergesConservatively(t *testing.T) {
	src := "@F\n[IF X]\nTRYGOTOLIST\n[ELSE]\nTRYCALLLIST\n[ENDIF]\nFUNC TARGET[1],1\nENDFUNC\n"
	r := FormatWithOptions([]byte(src), ERB, commandOptions())
	if len(r.Diagnostics) != 0 {
		t.Fatalf("variant-dependent call was diagnosed: %+v", r.Diagnostics)
	}
}

func TestSharpAttributeWarnings(t *testing.T) {
	src := "@EVENTTRAIN\n#SINGLE\n#SINGLE\n#PRI\n#LATER\n#ONLY\n#SINGLE\n@F\n#FUNCTION\n#FUNCTION\n#FUNCTIONS\n@G\n#LOCALSIZE 1\n#LOCALSIZE 2\n"
	r := FormatWithOptions([]byte(src), ERB, commandOptions())
	want := []string{"duplicate_attribute", "conflicting_attribute", "duplicate_local_size", "duplicate_declaration", "unsupported_declaration"}
	seen := make(map[string]bool)
	for _, diagnostic := range r.Diagnostics {
		seen[diagnostic.Code] = true
	}
	for _, code := range want {
		if !seen[code] {
			t.Fatalf("diagnostics=%+v, missing code %q", r.Diagnostics, code)
		}
	}
}

func TestSystemFunctionsCannotDeclareMethods(t *testing.T) {
	for _, name := range []string{"EVENTTRAIN", "SHOW_STATUS", "COM12", "COM_ABLE3", "ABLUP4"} {
		r := FormatWithOptions([]byte("@"+name+"\n#FUNCTION\n"), ERB, commandOptions())
		if len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "unsupported_declaration" {
			t.Fatalf("%s diagnostics=%+v output=%q", name, r.Diagnostics, r.Output)
		}
	}
	for _, name := range []string{"eventtrain", "COMx", "ABLUP"} {
		r := FormatWithOptions([]byte("@"+name+"\n#FUNCTION\n"), ERB, commandOptions())
		if len(r.Diagnostics) != 0 {
			t.Fatalf("uncertain system %s diagnostics=%+v output=%q", name, r.Diagnostics, r.Output)
		}
	}
}

func TestSharpMethodsRejectBMPDigitLeadingFunctions(t *testing.T) {
	for _, name := range []string{"１２３", "١ABC"} {
		for _, declaration := range []string{"#FUNCTION", "#FUNCTIONS"} {
			src := "@" + name + "\n" + declaration + "\nRETURNF 1\n"
			r := FormatWithOptions([]byte(src), ERB, commandOptions())
			if len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "unsupported_declaration" || r.Diagnostics[0].Severity != Unsupported {
				t.Fatalf("name=%q declaration=%q diagnostics=%+v output=%q", name, declaration, r.Diagnostics, r.Output)
			}
		}
	}
	for _, name := range []string{"𐒠ABC", "𝟙ABC", "１２３"} {
		src := "@" + name + "\nRETURN 1\n"
		r := FormatWithOptions([]byte(src), ERB, commandOptions())
		if len(r.Diagnostics) != 0 {
			t.Fatalf("ordinary name=%q diagnostics=%+v output=%q", name, r.Diagnostics, r.Output)
		}
	}
	for _, name := range []string{"𐒠ABC", "𝟙ABC"} {
		src := "@" + name + "\n#FUNCTION\nRETURNF 1\n"
		r := FormatWithOptions([]byte(src), ERB, commandOptions())
		if len(r.Diagnostics) != 0 {
			t.Fatalf("supplementary name=%q diagnostics=%+v output=%q", name, r.Diagnostics, r.Output)
		}
	}
}

func TestMethodReturnModes(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
		want string
	}{
		{"ordinary returnf", "@F\nRETURNF 1\n", "RETURNF can only be used in #FUNCTION"},
		{"numeric return", "@F\n#FUNCTION\nRETURN 0\n", "RETURN cannot be used in #FUNCTION"},
		{"numeric form", "@F\n#FUNCTION\nRETURNFORM 0\n", "RETURNFORM cannot be used in #FUNCTION"},
		{"numeric string", "@F\n#FUNCTION\nRETURNF \"x\"\n", "#FUNCTION requires a numeric RETURNF expression"},
		{"string numeric", "@F\n#FUNCTIONS\nRETURNF 1\n", "#FUNCTIONS requires a string RETURNF expression"},
		{"numeric builtin variable", "@F\n#FUNCTION\nRETURNF RESULT\n", ""},
		{"string builtin variable", "@F\n#FUNCTIONS\nRETURNF CSTR\n", ""},
		{"numeric string variable", "@F\n#FUNCTION\nRETURNF CSTR\n", "#FUNCTION requires a numeric RETURNF expression"},
		{"unknown method call", "@F\n#FUNCTIONS\nRETURNF UNKNOWN()\n", ""},
		{"ordinary without header", "RETURNF 1\n", ""},
		{"marked declaration", "@F\n;#;#FUNCTION\nRETURNF 1\n", ""},
		{"commented string method", "@F\n#FUNCTIONS ; note\nRETURNF \"x\"\n", ""},
		{"caret comment method", "@F\n#FUNCTIONS ;^; note\nRETURNF \"x\"\n", ""},
		{"ignored method tail", "@F\n#FUNCTIONS 1\nRETURNF \"x\"\n", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := FormatWithOptions([]byte(tc.src), ERB, commandOptions())
			found := ""
			for _, d := range r.Diagnostics {
				if strings.Contains(d.Message, "RETURN") || strings.Contains(d.Message, "#FUNCTION") {
					found = d.Message
					break
				}
			}
			if tc.want == "" {
				if found != "" {
					t.Fatalf("unexpected return diagnostic=%q all=%+v", found, r.Diagnostics)
				}
				return
			}
			if found != tc.want {
				t.Fatalf("return diagnostic=%q want=%q all=%+v", found, tc.want, r.Diagnostics)
			}
		})
	}
}

func TestUnknownMethodModeResetsAtNextFunction(t *testing.T) {
	src := "@F\n#FUNCTIONS ; note\nRETURNF \"x\"\n@G\nRETURNF 1\n"
	r := FormatWithOptions([]byte(src), ERB, commandOptions())
	if len(r.Diagnostics) != 1 || r.Diagnostics[0].Line != 5 || r.Diagnostics[0].Message != "RETURNF can only be used in #FUNCTION" {
		t.Fatalf("diagnostics=%+v", r.Diagnostics)
	}
}

func TestMethodReturnModesAllowFullwidthSpace(t *testing.T) {
	for _, tc := range []struct {
		src  string
		want string
	}{
		{"@F\n#FUNCTIONS\u3000\nRETURNF \"x\"\n", ""},
		{"@F\n#FUNCTION\u3000\nRETURNF 1\n", ""},
		{"@F\n#FUNCTION\u3000\nRETURN 0\n", "RETURN cannot be used in #FUNCTION"},
	} {
		options := commandOptions()
		options.AllowFullwidthSpace = true
		r := FormatWithOptions([]byte(tc.src), ERB, options)
		if tc.want == "" {
			if len(r.Diagnostics) != 0 {
				t.Fatalf("source=%q diagnostics=%+v", tc.src, r.Diagnostics)
			}
		} else if len(r.Diagnostics) != 1 || r.Diagnostics[0].Message != tc.want {
			t.Fatalf("source=%q diagnostics=%+v want=%q", tc.src, r.Diagnostics, tc.want)
		}
	}
}

func TestIgnoredLocalSizeDoesNotEstablishDuplicate(t *testing.T) {
	for _, value := range []string{"", "0", "2147483647", "UNKNOWN"} {
		src := "@F\n#LOCALSIZE " + value + "\n#LOCALSIZE 1\n"
		r := FormatWithOptions([]byte(src), ERB, commandOptions())
		for _, d := range r.Diagnostics {
			if d.Code == "duplicate_local_size" {
				t.Fatalf("ignored or unknown size %q: %+v", value, r.Diagnostics)
			}
		}
	}
	r := FormatWithOptions([]byte("@EVENTTRAIN\n#LOCALSIZE 1\n#LOCALSIZE 2\n"), ERB, commandOptions())
	if len(r.Diagnostics) != 2 || r.Diagnostics[0].Code != "event_local_size" || r.Diagnostics[1].Code != "event_local_size" {
		t.Fatalf("event sizes: %+v", r.Diagnostics)
	}
}

func TestLocalSizeConstantExpressionsEstablishRangeAndDuplicateState(t *testing.T) {
	r := FormatWithOptions([]byte("@F\n#LOCALSIZE 1+2\n#LOCALSIZE 3\n"), ERB, commandOptions())
	if len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "duplicate_local_size" {
		t.Fatalf("diagnostics=%+v output=%q", r.Diagnostics, r.Output)
	}
	r = FormatWithOptions([]byte("@F\n#LOCALSIZE 0+0\n"), ERB, commandOptions())
	if len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "suspicious_local_size" {
		t.Fatalf("diagnostics=%+v output=%q", r.Diagnostics, r.Output)
	}
}

func TestEventSharpAttributesAreTracked(t *testing.T) {
	src := "@EVENTTRAIN\n#SINGLE\n#SINGLE\n#LOCALSIZE\nRETURN 0\n"
	r := FormatWithOptions([]byte(src), ERB, commandOptions())
	if len(r.Diagnostics) != 2 {
		t.Fatalf("diagnostics=%+v", r.Diagnostics)
	}
	for _, d := range r.Diagnostics {
		if d.Code != "duplicate_attribute" && d.Code != "missing_local_size" {
			t.Fatalf("diagnostics=%+v", r.Diagnostics)
		}
	}
}

func TestNonEventSharpAttributesAreIgnoredIndividually(t *testing.T) {
	src := "@F\n#SINGLE\n#SINGLE\n#PRI\n#LATER\n#ONLY\n"
	r := FormatWithOptions([]byte(src), ERB, commandOptions())
	if len(r.Diagnostics) != 5 {
		t.Fatalf("diagnostics=%+v", r.Diagnostics)
	}
	for _, diagnostic := range r.Diagnostics {
		if diagnostic.Code != "event_attribute_ignored" {
			t.Fatalf("diagnostics=%+v", r.Diagnostics)
		}
	}
}

func TestEventRecognitionUsesRuntimeNames(t *testing.T) {
	r := FormatWithOptions([]byte("@EVENT_HELPER\n#LOCALSIZE 1+2\n"), ERB, commandOptions())
	if len(r.Diagnostics) != 0 || !strings.Contains(string(r.Output), "#LOCALSIZE 1 + 2") {
		t.Fatalf("user function: %+v", r)
	}
	r = FormatWithOptions([]byte("@eventtrain\n#LOCALSIZE \"ignored\"\n#SINGLE\n#SINGLE\n"), ERB, commandOptions())
	if len(r.Diagnostics) != 0 || !strings.Contains(string(r.Output), "#LOCALSIZE \"ignored\"") {
		t.Fatalf("configuration-dependent event: %+v", r)
	}
}

func TestUncertainSharpAttributesDeferLaterConflicts(t *testing.T) {
	for _, attr := range []string{";#;#ONLY", "#only"} {
		src := "@EVENTTRAIN\n#SINGLE\n" + attr + "\n#SINGLE\n"
		r := FormatWithOptions([]byte(src), ERB, commandOptions())
		for _, d := range r.Diagnostics {
			if d.Code == "duplicate_attribute" || d.Code == "conflicting_attribute" {
				t.Fatalf("uncertain attribute %q: %+v", attr, r.Diagnostics)
			}
		}
	}
}
