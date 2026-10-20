package format

import (
	"bytes"
	"testing"
)

func TestSpecializedCommandsDeferInlineDebugMarkers(t *testing.T) {
	src := []byte("@F\n" +
		"TIMES RESULT ;#; ,1+2\n" +
		"SPLIT \"a\",\",\",RESULTS;#;,RESULT\n" +
		"ARRAYSORT RESULTS;#;,FORWARD\n")
	r := FormatWithOptions(src, ERB, commandOptions())
	if !bytes.Equal(r.Output, []byte("@F\n    TIMES RESULT ;#; ,1+2\n    SPLIT \"a\",\",\",RESULTS;#;,RESULT\n    ARRAYSORT RESULTS;#;,FORWARD\n")) {
		t.Fatalf("output=%q diagnostics=%+v", r.Output, r.Diagnostics)
	}
	if len(r.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%+v", r.Diagnostics)
	}
}

func TestGraphicsDebugMarkersKeepTrailingBytes(t *testing.T) {
	for _, dialect := range []Dialect{CommonDialect, Emuera1824, EMEE} {
		for _, command := range []string{"PRINT_SPACE", "PRINT_RECT", "PRINT_IMG"} {
			for _, tail := range []string{"  ", "\t", "\u3000"} {
				for _, rules := range [][]RuleID{{CodeTrailingWhitespace}, {CommandSpacing, CodeTrailingWhitespace}} {
					src := []byte(command + "\t1 ;#; +2" + tail + "\n")
					want := src
					if len(rules) > 1 {
						want = []byte(command + " 1 ;#; +2" + tail + "\n")
					}
					opts := Options{Dialect: dialect, AllowFullwidthSpace: true, Select: rules}
					r := FormatWithOptions(src, ERB, opts)
					if !bytes.Equal(r.Output, want) || len(r.Diagnostics) != 0 {
						t.Fatalf("source=%q dialect=%d output=%q diagnostics=%+v", src, dialect, r.Output, r.Diagnostics)
					}
				}
			}
		}
	}

	for _, source := range []string{"PRINT_IMG \";#;\"  \n", "PRINT_IMG @\";#; {1+2}\"  \n"} {
		r := FormatWithOptions([]byte(source), ERB, Options{Select: []RuleID{CodeTrailingWhitespace}})
		want := []byte(source[:len(source)-3] + "\n")
		if !bytes.Equal(r.Output, want) || len(r.Diagnostics) != 0 {
			t.Fatalf("literal source=%q output=%q diagnostics=%+v", source, r.Output, r.Diagnostics)
		}
	}
}

func TestSpecializedCommandsKeepMandatoryMarkerBytes(t *testing.T) {
	src := []byte("@F\nTIMES RESULT;!; , 1\nARRAYSORT RESULTS;!;, FORWARD\n")
	r := FormatWithOptions(src, ERB, commandOptions())
	if !bytes.Contains(r.Output, []byte("RESULT;!;")) || !bytes.Contains(r.Output, []byte("RESULTS;!;")) {
		t.Fatalf("output=%q diagnostics=%+v", r.Output, r.Diagnostics)
	}
	for _, diagnostic := range r.Diagnostics {
		if diagnostic.Severity == Unsupported {
			t.Fatalf("diagnostics=%+v", r.Diagnostics)
		}
	}
}

func TestMandatoryMarkersStillValidateProvenRoles(t *testing.T) {
	for _, body := range []string{
		"SPLIT ;!; RESULT,RESULTS,1",
		"HTML_TAGSPLIT ;!; \"x\",RESULT,1",
		"HTML_TAGSPLIT ;!; RESULT,RESULTS,RESULT",
		"ARRAYSHIFT ;!; NO,1,2",
		"ARRAYSORT ;!; NAME,FORWARD",
		"SORTCHARA ;!; RESULTS,FORWARD",
	} {
		src := []byte("@F\n" + body + "\n")
		r := FormatWithOptions(src, ERB, commandOptions())
		if len(r.Diagnostics) != 1 || r.Diagnostics[0].Severity != Unsupported {
			t.Fatalf("invalid marker body=%q output=%q diagnostics=%+v", body, r.Output, r.Diagnostics)
		}
	}
}

func TestMandatoryMarkersKeepValidSpecializedRoles(t *testing.T) {
	for _, body := range []string{
		"SPLIT ;!; \"a\",\",\",RESULTS,RESULT",
		"HTML_TAGSPLIT ;!; \"a\",RESULTS,RESULT",
		"ARRAYSORT ;!; RESULTS,FORWARD",
		"SORTCHARA ;!; CSTR,FORWARD",
		"TIMES RESULT;!;,1",
		"SWAP RESULT;!;,RESULT",
	} {
		src := []byte("@F\n" + body + "\n")
		r := FormatWithOptions(src, ERB, commandOptions())
		if len(r.Diagnostics) != 0 || !bytes.Contains(r.Output, []byte(";!;")) {
			t.Fatalf("valid marker body=%q output=%q diagnostics=%+v", body, r.Output, r.Diagnostics)
		}
	}
}
