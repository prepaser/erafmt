package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/prepaser/erafmt/internal/format"
)

func TestFallbackProfileRepresentativeWrites(t *testing.T) {
	utf16, err := encodeSource(sourceDocument{encoding: encodingUTF16LE, bom: []byte{0xff, 0xfe}}, []byte("@F\r\nPRINTL 日本語\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{name: "cp932", data: []byte("@F\r\nPRINTL \x93\xfa\x96\x7b\x8c\xea\r\n")},
		{name: "utf8-bom", data: append([]byte{0xef, 0xbb, 0xbf}, []byte("@F\r\nPRINTL 日本語\r\n")...)},
		{name: "utf16le-bom", data: utf16},
		{name: "warning", data: []byte("@F\r\nREPEAT 1-1\r\nREND\r\n")},
		{name: "partial", data: []byte("@F\r\nFUTURECOMMAND A+B\r\n")},
		{name: "fatal", data: []byte("@F[1]\r\nRETURN 1\r\n")},
		{name: "malformed-utf8-bom", data: []byte{0xef, 0xbb, 0xbf, 0xc3, 0x28}, wantErr: true},
		{name: "utf32-bom", data: []byte{0xff, 0xfe, 0x00, 0x00, 0x41, 0x00, 0x00, 0x00}, wantErr: true},
	}
	for _, tc := range cases {
		for _, selectNone := range []bool{true, false} {
			t.Run(tc.name+"/select-none="+boolName(selectNone), func(t *testing.T) {
				if !tc.wantErr {
				}
				original := append([]byte(nil), tc.data...)
				path := filepath.Join(t.TempDir(), "sample.ERB")
				if err := os.WriteFile(path, original, 0o640); err != nil {
					t.Fatal(err)
				}
				args := []string{"--input-encoding=cp932-fallback", "--utf8", "--diagnostic-format=json", "--continuation-separator= ", "-w"}
				if selectNone {
					args = append(args, "--select=none")
				}
				args = append(args, path)
				var out, diagnostics bytes.Buffer
				code := Run(args, bytes.NewReader(nil), &out, &diagnostics)
				if tc.wantErr {
					for pass := 0; pass < 2; pass++ {
						if pass > 0 {
							out.Reset()
							diagnostics.Reset()
							code = Run(args, bytes.NewReader(nil), &out, &diagnostics)
						}
						status, class := profileCLIStatus(t, diagnostics.Bytes())
						if status != "fatal" || class != "error" {
							t.Fatalf("malformed input pass=%d status=%s/%s want=fatal/error diagnostics=%q", pass, status, class, diagnostics.String())
						}
						if code != 2 || out.Len() != 0 {
							t.Fatalf("malformed input pass=%d code=%d output=%q diagnostics=%q", pass, code, out.String(), diagnostics.String())
						}
						got, readErr := os.ReadFile(path)
						if readErr != nil || !bytes.Equal(got, original) {
							t.Fatalf("malformed input pass=%d changed: %v", pass, readErr)
						}
					}
					return
				}
				separator := " "
				options := format.Options{ContinuationSeparator: &separator}
				if selectNone {
					options.Select = []format.RuleID{}
				}
				doc, err := decodeSourceWithEncoding(original, inputEncodingCP932Fallback)
				if err != nil {
					t.Fatal(err)
				}
				result := format.FormatWithOptions(doc.text, format.ERB, options)
				wantStatus, wantClass := profileDiagnosticClass(result.Diagnostics)
				want := original
				if wantStatus != "fatal" {
					want, err = encodeOutput(doc, result.Output, true)
					if err != nil {
						t.Fatal(err)
					}
				}
				wantCode := 0
				for _, diagnostic := range result.Diagnostics {
					if diagnostic.Severity == format.Error || diagnostic.Severity == format.Unsupported {
						wantCode = 2
					}
				}
				status, class := profileCLIStatus(t, diagnostics.Bytes())
				if status != wantStatus || class != wantClass {
					t.Fatalf("first pass status=%s/%s want=%s/%s diagnostics=%q", status, class, wantStatus, wantClass, diagnostics.String())
				}
				if code != wantCode || out.Len() != 0 {
					t.Fatalf("first pass code=%d want=%d output=%q diagnostics=%q", code, wantCode, out.String(), diagnostics.String())
				}
				got, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(got, want) {
					t.Fatalf("first pass output mismatch: %v", err)
				}
				out.Reset()
				diagnostics.Reset()
				if code := Run(args, bytes.NewReader(nil), &out, &diagnostics); code != wantCode || out.Len() != 0 {
					t.Fatalf("second pass code=%d want=%d output=%q diagnostics=%q", code, wantCode, out.String(), diagnostics.String())
				}
				status, class = profileCLIStatus(t, diagnostics.Bytes())
				if status != wantStatus || class != wantClass {
					t.Fatalf("second pass status=%s/%s want=%s/%s diagnostics=%q", status, class, wantStatus, wantClass, diagnostics.String())
				}
				got, err = os.ReadFile(path)
				if err != nil || !bytes.Equal(got, want) {
					t.Fatalf("second pass output mismatch: %v", err)
				}
			})
		}
	}
}

func TestFallbackProfileContinuationSeparator(t *testing.T) {
	original := []byte("@F\n{\nRETURN 1+2\n}\n")
	path := filepath.Join(t.TempDir(), "continuation.ERB")
	if err := os.WriteFile(path, original, 0o640); err != nil {
		t.Fatal(err)
	}
	args := []string{"--input-encoding=cp932-fallback", "--utf8", "--continuation-separator= ", "-w", path}
	var out, diagnostics bytes.Buffer
	if code := Run(args, bytes.NewReader(nil), &out, &diagnostics); code != 0 || out.Len() != 0 {
		t.Fatalf("run code=%d output=%q diagnostics=%q", code, out.String(), diagnostics.String())
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	separator := " "
	wantResult := format.FormatWithOptions(original, format.ERB, format.Options{ContinuationSeparator: &separator})
	want := append([]byte{0xef, 0xbb, 0xbf}, wantResult.Output...)
	if !bytes.Equal(got, want) {
		t.Fatalf("continuation output=%q want=%q", got, want)
	}
	if code := Run(args, bytes.NewReader(nil), &out, &diagnostics); code != 0 {
		t.Fatalf("second continuation run code=%d diagnostics=%q", code, diagnostics.String())
	}
	again, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(again, want) {
		t.Fatalf("continuation second pass changed output: %v", err)
	}
}

func boolName(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}
