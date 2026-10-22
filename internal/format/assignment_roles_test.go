package format

import (
	"bytes"

	"testing"
)

func TestImplicitDestinationRoles(t *testing.T) {
	for _, line := range []string{"ITEMPRICE=1", "ITEMPRICE:0+=1", "ITEMPRICE++", "++ITEMPRICE:0", "GAMEBASE_VERSION=1", "RESULTS++", "++RESULTS:0", "ARGS--"} {
		r := FormatWithOptions([]byte("@F\n"+line+"\n"), ERB, Options{})
		if len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "unsupported_syntax" || !bytes.Contains(r.Output, []byte(line+"\n")) {
			t.Fatalf("line=%q output=%q diagnostics=%+v", line, r.Output, r.Diagnostics)
		}
	}
	for _, line := range []string{"RESULT++", "--ARG:0", "LOCAL+=1", `RESULTS*="x"=="y"`, `RESULTS+="x"`, "ITEMPRICE:INDEX=1", "itemprice=1", "USER++", "RESULTS:INDEX++"} {
		r := FormatWithOptions([]byte("@F\n"+line+"\n"), ERB, Options{})
		if len(r.Diagnostics) != 0 {
			t.Fatalf("line=%q diagnostics=%+v", line, r.Diagnostics)
		}
	}
}
