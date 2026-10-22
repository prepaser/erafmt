package format

import (
	"bytes"

	"testing"
)

func TestERHNamesWithREFPrefixAreNotModifiers(t *testing.T) {
	options := Options{Select: []RuleID{BinarySpacing, CommaSpacing}}
	for _, name := range []string{"REF_VAR", "REF1", "REFαβ", "ref_var"} {
		for _, modifier := range []string{"", "GLOBAL ", "CONST ", "SAVEDATA "} {
			for _, declaration := range []string{"DIM", "DIMS"} {
				initializer := ""
				if modifier == "CONST " {
					initializer = "=1, 2"
					if declaration == "DIMS" {
						initializer = "=\"x\", \"y\""
					}
				}
				src := []byte("#" + declaration + " " + modifier + name + ",1+1" + initializer + "\n#DIM OtherName,1+1\n")
				want := bytes.ReplaceAll(src, []byte(",1+1"), []byte(", 1 + 1"))
				r := FormatWithOptions(src, ERH, options)
				if !bytes.Equal(r.Output, want) || len(r.Diagnostics) != 0 {
					t.Fatalf("source=%q output=%q diagnostics=%+v", src, r.Output, r.Diagnostics)
				}
			}
		}
	}
	for _, declaration := range []string{"DIM REF X", "DIMS REF X", "DIM GLOBAL REF X"} {
		src := []byte("#" + declaration + "\n#DIM OtherName,1+1\n")
		r := FormatWithOptions(src, ERH, options)
		if !bytes.Equal(r.Output, src) || len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "unsupported_declaration" {
			t.Fatalf("REF modifier source=%q result=%+v", src, r)
		}
	}
}
