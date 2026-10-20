package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestContinuationVirtualDiagnosticThroughCLI(t *testing.T) {
	text := []byte("@F\n{\nP\n}\n")
	utf16, err := encodeSource(sourceDocument{encoding: encodingUTF16LE, bom: []byte{0xff, 0xfe}}, text)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		src  []byte
		line int
	}{
		{"utf8-buffer-boundary", []byte(";" + strings.Repeat("x", 495) + "\n" + string(text)), 4},
		{"utf16", utf16, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out, diagnostics bytes.Buffer
			code := Run([]string{"--select=none", "--continuation-separator=RINTBUTTONLC　", "--diagnostic-format=json"}, bytes.NewReader(tc.src), &out, &diagnostics)
			if code != 2 || !bytes.Equal(out.Bytes(), tc.src) {
				t.Fatalf("code=%d output=%q diagnostics=%q", code, out.Bytes(), diagnostics.Bytes())
			}
			events := decodeDiagnosticLines(t, diagnostics.Bytes())
			if len(events) != 1 || events[0]["line"] != float64(tc.line) || events[0]["column"] != float64(2) || events[0]["code"] != "unsupported_structure" {
				t.Fatalf("events=%+v want=%d:2 unsupported_structure", events, tc.line)
			}
		})
	}
}
