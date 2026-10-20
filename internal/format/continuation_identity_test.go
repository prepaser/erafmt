package format

import (
	"bytes"
	"testing"
)

func TestNilContinuationDefersAmbiguousCommandIdentity(t *testing.T) {
	for _, src := range []string{
		"@F\n#DIM IFOO\n{\nIF\nOO = 1\n}\nRETURN 0\n",
		"@F\n#DIM IFX\n{\nIF\n;comment\n}\nRETURN 0\n",
		"@F\n#DIM DOX\n{\nDO\n}\nRETURN 0\n",
		"@F\n{\nRE\nPEAT 1\n}\nREND\n",
		"@F\n{\nEND\nIF\n}\nRETURN 0\n",
	} {
		result := FormatWithOptions([]byte(src), ERB, Options{})
		if !bytes.Equal(result.Output, []byte(src)) || len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "unsupported_structure" || result.Diagnostics[0].Severity != Unsupported {
			t.Fatalf("source=%q output=%q diagnostics=%+v", src, result.Output, result.Diagnostics)
		}
	}

	separator := ""
	result := FormatWithOptions([]byte("@F\n#DIM IFOO\n{\nIF\nOO = 1\n}\nRETURN 0\n"), ERB, Options{ContinuationSeparator: &separator})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("explicit empty separator changed identity: %+v", result.Diagnostics)
	}
}

func TestNilContinuationDefersLeadingBlankDispatch(t *testing.T) {
	src := "@F\n{\n\nIF 1\n}\nRETURN 0\n"
	result := FormatWithOptions([]byte(src), ERB, Options{})
	if !bytes.Equal(result.Output, []byte(src)) || len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "unsupported_structure" || result.Diagnostics[0].Severity != Unsupported {
		t.Fatalf("output=%q diagnostics=%+v", result.Output, result.Diagnostics)
	}

	separator := ";comment"
	result = FormatWithOptions([]byte(src), ERB, Options{ContinuationSeparator: &separator})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("explicit separator diagnostics=%+v", result.Diagnostics)
	}

	src = "@F\n{\n\nRETURN 0\n}\n@G\nENDIF\n"
	result = FormatWithOptions([]byte(src), ERB, Options{})
	if !bytes.Equal(result.Output, []byte(src)) || len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "unsupported_structure" || result.Diagnostics[0].Severity != Unsupported {
		t.Fatalf("later boundary output=%q diagnostics=%+v", result.Output, result.Diagnostics)
	}

	src = "@F\nIF 1\n{\n\n}\n@G\nRETURN 0\n"
	result = FormatWithOptions([]byte(src), ERB, Options{})
	if !bytes.Equal(result.Output, []byte(src)) || len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "unsupported_structure" || result.Diagnostics[0].Severity != Unsupported {
		t.Fatalf("all blank output=%q diagnostics=%+v", result.Output, result.Diagnostics)
	}
	separator = "ENDIF;comment"
	result = FormatWithOptions([]byte(src), ERB, Options{ContinuationSeparator: &separator})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("all blank explicit separator diagnostics=%+v", result.Diagnostics)
	}
}

func TestNilContinuationDefersAmbiguousFunctionHeader(t *testing.T) {
	for _, src := range []string{
		"@A\n{\n@FO\nO\n}\nRETURN 0\n",
		"@A\n{\n@F(ARG=1p\n63)\n}\nRETURN 0\n",
		"@A\nRETURN 0\n{\n@EVENTFIRST\n}\n#FUNCTION\nRETURNF 1+2\n",
	} {
		result := FormatWithOptions([]byte(src), ERB, Options{})
		if !bytes.Equal(result.Output, []byte(src)) || len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "unsupported_structure" || result.Diagnostics[0].Severity != Unsupported {
			t.Fatalf("source=%q output=%q diagnostics=%+v", src, result.Output, result.Diagnostics)
		}
	}
}

func TestNilContinuationAcceptsSealedParenthesizedHeader(t *testing.T) {
	for _, head := range []string{
		"@B(ARG:0=1);comment",
		"@B((ARG:0=1));comment",
		"@B(ARG:0=F(1));comment",
		"@B(ARG:0=1);!; ;comment",
		"@B(ARG:0=1);#; ;comment",
	} {
		src := []byte("@A\n{\n" + head + "\n}\nRETURN 0\n")
		result := FormatWithOptions(src, ERB, Options{})
		if len(result.Diagnostics) != 0 {
			t.Fatalf("head=%q diagnostics=%+v", head, result.Diagnostics)
		}
		separator := "X"
		result = FormatWithOptions(src, ERB, Options{ContinuationSeparator: &separator})
		if len(result.Diagnostics) != 0 {
			t.Fatalf("head=%q explicit suffix diagnostics=%+v", head, result.Diagnostics)
		}
	}

	for _, head := range []string{"@B(ARG:0=1)", "@B(ARG:0=1);!;"} {
		src := []byte("@A\n{\n" + head + "\n}\nRETURN 0\n")
		result := FormatWithOptions(src, ERB, Options{})
		if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "unsupported_structure" || result.Diagnostics[0].Severity != Unsupported {
			t.Fatalf("unsealed head=%q diagnostics=%+v", head, result.Diagnostics)
		}
	}
}

func TestNilContinuationValidatesSealedHeaderArguments(t *testing.T) {
	for _, head := range []string{
		"@B(F(1));comment",
		"@B(1);comment",
		"@B(\"x\");comment",
		"@B(ITEMPRICE:0);comment",
	} {
		src := []byte("@A\n{\n" + head + "\n}\nRETURN 0\n")
		result := FormatWithOptions(src, ERB, Options{})
		if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "unsupported_syntax" || result.Diagnostics[0].Severity != Unsupported {
			t.Fatalf("head=%q nil diagnostics=%+v output=%q", head, result.Diagnostics, result.Output)
		}
		separator := " "
		explicit := FormatWithOptions(src, ERB, Options{ContinuationSeparator: &separator})
		if len(explicit.Diagnostics) != 1 || explicit.Diagnostics[0].Code != "unsupported_syntax" || explicit.Diagnostics[0].Severity != Unsupported {
			t.Fatalf("head=%q explicit diagnostics=%+v output=%q", head, explicit.Diagnostics, explicit.Output)
		}
		if !bytes.Equal(result.Output, explicit.Output) {
			t.Fatalf("head=%q nil output=%q explicit=%q", head, result.Output, explicit.Output)
		}
	}

	indented := []byte("@A\n{\n  @B(F(1));comment\n}\nRETURN 0\n")
	result := FormatWithOptions(indented, ERB, Options{})
	separator := " "
	explicit := FormatWithOptions(indented, ERB, Options{ContinuationSeparator: &separator})
	if len(result.Diagnostics) != 1 || len(explicit.Diagnostics) != 1 || result.Diagnostics[0] != explicit.Diagnostics[0] {
		t.Fatalf("mapped semantic diagnostic nil=%+v explicit=%+v", result.Diagnostics, explicit.Diagnostics)
	}
	for _, test := range []struct {
		name string
		src  []byte
		wide bool
	}{
		{name: "tab", src: []byte("@A\n{\n\t@B(1);comment\n}\nRETURN 0\n")},
		{name: "fullwidth", src: []byte("@A\n{\n　@B(1);comment\n}\nRETURN 0\n"), wide: true},
		{name: "later-row", src: []byte("@A\n{\n@B(F(1));comment\nRETURN 0\n}\nRETURN 0\n")},
	} {
		options := Options{AllowFullwidthSpace: test.wide}
		result := FormatWithOptions(test.src, ERB, options)
		separator := " "
		explicit := Options{AllowFullwidthSpace: test.wide, ContinuationSeparator: &separator}
		explicitResult := FormatWithOptions(test.src, ERB, explicit)
		if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "unsupported_syntax" || result.Diagnostics[0].Severity != Unsupported {
			t.Fatalf("%s nil diagnostics=%+v", test.name, result.Diagnostics)
		}
		if len(explicitResult.Diagnostics) != 1 || result.Diagnostics[0] != explicitResult.Diagnostics[0] {
			t.Fatalf("%s mapped diagnostics nil=%+v explicit=%+v", test.name, result.Diagnostics, explicitResult.Diagnostics)
		}
	}

	for _, marker := range []string{";!;", ";#;"} {
		src := []byte("@A\n{\n" + marker + "@B(1);comment\n}\nRETURN 0\n")
		result := FormatWithOptions(src, ERB, Options{})
		if !bytes.Equal(result.Output, src) || len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "unsupported_structure" || result.Diagnostics[0].Severity != Unsupported {
			t.Fatalf("marker=%q output=%q diagnostics=%+v", marker, result.Output, result.Diagnostics)
		}
	}

	for _, head := range []string{"@B(ARG:0=F(1));comment", "@B((ARG:0));comment", "@B(ARG:0+1);comment"} {
		src := []byte("@A\n{\n" + head + "\n}\nRETURN 0\n")
		result := FormatWithOptions(src, ERB, Options{})
		if len(result.Diagnostics) != 0 {
			t.Fatalf("valid head=%q diagnostics=%+v", head, result.Diagnostics)
		}
	}
}

func TestNilContinuationDefersHeaderDefaultAndEventContext(t *testing.T) {
	defaultSource := "{\n@F(ARG:0=\n);comment\n}\nRETURN 0\n"
	result := FormatWithOptions([]byte(defaultSource), ERB, Options{})
	if !bytes.Equal(result.Output, []byte(defaultSource)) || len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "unsupported_structure" || result.Diagnostics[0].Severity != Unsupported {
		t.Fatalf("default output=%q diagnostics=%+v", result.Output, result.Diagnostics)
	}
	separator := "1"
	result = FormatWithOptions([]byte(defaultSource), ERB, Options{ContinuationSeparator: &separator})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("default explicit separator diagnostics=%+v", result.Diagnostics)
	}
	commaSource := "{\n@F,ARG:0=\n}\nRETURN 0\n"
	result = FormatWithOptions([]byte(commaSource), ERB, Options{})
	if !bytes.Equal(result.Output, []byte(commaSource)) || len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "unsupported_structure" || result.Diagnostics[0].Severity != Unsupported {
		t.Fatalf("comma output=%q diagnostics=%+v", result.Output, result.Diagnostics)
	}
	separator = "1;comment"
	result = FormatWithOptions([]byte(commaSource), ERB, Options{ContinuationSeparator: &separator})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("comma explicit separator diagnostics=%+v", result.Diagnostics)
	}

	eventSource := "@A\nRETURN 0\n{\n@EVENTFIRST\n}\n#FUNCTION\nRETURNF 1+2\n"
	result = FormatWithOptions([]byte(eventSource), ERB, Options{})
	if !bytes.Equal(result.Output, []byte(eventSource)) || len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "unsupported_structure" || result.Diagnostics[0].Severity != Unsupported {
		t.Fatalf("event output=%q diagnostics=%+v", result.Output, result.Diagnostics)
	}
	separator = "_X"
	result = FormatWithOptions([]byte(eventSource), ERB, Options{ContinuationSeparator: &separator})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("event explicit separator diagnostics=%+v", result.Diagnostics)
	}
}

func TestNilContinuationKeepsKnownHeaderErrors(t *testing.T) {
	for _, src := range []string{
		"@A\n{\n@F(ARG=)\n}\nRETURN 0\n",
		"@A\n{\n@123\n}\nRETURN 0\n",
		"@A\n{\n@F(ARG=1]\n}\nRETURN 0\n",
		"@A\n{\n@F,]\n}\nRETURN 0\n",
	} {
		result := FormatWithOptions([]byte(src), ERB, Options{})
		if !bytes.Equal(result.Output, []byte(src)) || len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "invalid_header" || result.Diagnostics[0].Severity != Error {
			t.Fatalf("source=%q output=%q diagnostics=%+v", src, result.Output, result.Diagnostics)
		}
	}
}

func TestNilContinuationDefersAmbiguousPreprocessorIdentity(t *testing.T) {
	src := "@F\n{\n[IF\nX]\n}\n[ENDIF]\n"
	result := FormatWithOptions([]byte(src), ERB, Options{})
	if !bytes.Equal(result.Output, []byte(src)) || len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "unsupported_structure" || result.Diagnostics[0].Severity != Unsupported {
		t.Fatalf("output=%q diagnostics=%+v", result.Output, result.Diagnostics)
	}
}

func TestNilContinuationDefersMarkerSplitBeforeCommentDispatch(t *testing.T) {
	src := "@F\n{\n;\n!;IF 1\n}\nENDIF\n"
	result := FormatWithOptions([]byte(src), ERB, Options{})
	if !bytes.Equal(result.Output, []byte(src)) || len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "unsupported_structure" || result.Diagnostics[0].Severity != Unsupported {
		t.Fatalf("output=%q diagnostics=%+v", result.Output, result.Diagnostics)
	}
}

func TestNilContinuationDefersIncompleteMarkerPrefix(t *testing.T) {
	for _, src := range []string{
		"@F\n{\n;\n}\nENDIF\nRETURN 0\n",
		"@F\n{\n;!\n}\nENDIF\nRETURN 0\n",
	} {
		result := FormatWithOptions([]byte(src), ERB, Options{})
		if !bytes.Equal(result.Output, []byte(src)) || len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "unsupported_structure" || result.Diagnostics[0].Severity != Unsupported {
			t.Fatalf("source=%q output=%q diagnostics=%+v", src, result.Output, result.Diagnostics)
		}
	}

	separator := "!;IF 1"
	result := FormatWithOptions([]byte("@F\n{\n;\n}\nENDIF\nRETURN 0\n"), ERB, Options{ContinuationSeparator: &separator})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("semicolon explicit separator diagnostics=%+v", result.Diagnostics)
	}
	separator = ";IF 1"
	result = FormatWithOptions([]byte("@F\n{\n;!\n}\nENDIF\nRETURN 0\n"), ERB, Options{ContinuationSeparator: &separator})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("marker explicit separator diagnostics=%+v", result.Diagnostics)
	}
}

func TestNilContinuationTracksPhysicallyDelimitedStructure(t *testing.T) {
	for _, src := range []string{
		"@F\n{\nIF A\n}\nENDIF\n",
		"@F\n{\n[SKIPSTART]\n}\n[SKIPEND]\n",
	} {
		result := FormatWithOptions([]byte(src), ERB, Options{})
		if len(result.Diagnostics) != 0 {
			t.Fatalf("source=%q output=%q diagnostics=%+v", src, result.Output, result.Diagnostics)
		}
	}
}
