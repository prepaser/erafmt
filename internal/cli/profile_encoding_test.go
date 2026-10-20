package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type encodingErrorReceipt struct {
	Key, Profile, FailureCategory, Input, DecodeError            string
	FirstSeverity, FirstCodeName, SecondSeverity, SecondCodeName string
	FirstCode, SecondCode                                        int
	FirstUnchanged, SecondUnchanged, Stable                      bool
}

func TestInputProfileEncodingErrorRepresentatives(t *testing.T) {
	output := os.Getenv("ERAFMT_PROFILE_ENCODING_ERROR_RECEIPT")
	var protected []discoveredTarget
	if output != "" {
		targets, err := configuredCorpusTargets(nil)
		if err != nil {
			t.Fatal(err)
		}
		protected = targets
		if err := rejectCorpusArtifactOverlap("profile encoding-error receipt", output, protected); err != nil {
			t.Fatal(err)
		}
	}
	cases := []struct {
		name string
		data []byte
	}{
		{name: "utf8", data: []byte{0xef, 0xbb, 0xbf, 0xc3, 0x28}},
		{name: "utf16", data: []byte{0xff, 0xfe, 0x00}},
		{name: "utf32", data: []byte{0xff, 0xfe, 0x00, 0x00, 0x41, 0x00, 0x00, 0x00}},
		{name: "cp932", data: []byte{0x81, 0x7f}},
	}
	profiles := []struct {
		name string
		kind inputEncodingKind
	}{
		{name: "auto", kind: inputEncodingAuto},
		{name: "cp932", kind: inputEncodingCP932},
		{name: "cp932-fallback", kind: inputEncodingCP932Fallback},
	}
	seen := make(map[string]bool)
	var receipts []encodingErrorReceipt
	for _, profile := range profiles {
		for _, tc := range cases {
			original := append([]byte(nil), tc.data...)
			_, decodeErr := decodeSourceWithEncoding(original, profile.kind)
			if decodeErr == nil {
				continue
			}
			category := decoderFailureCategory(decodeErr)
			if category == "" {
				t.Fatalf("%s/%s has unclassified decoder error: %v", profile.name, tc.name, decodeErr)
			}
			key := profile.name + "/" + category
			if seen[key] {
				continue
			}
			seen[key] = true
			t.Run(key, func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "source.ERB")
				if err := os.WriteFile(path, original, 0o640); err != nil {
					t.Fatal(err)
				}
				args := []string{"--input-encoding=" + profile.name, "--utf8", "--select=none", "--diagnostic-format=json", "-w", path}
				firstCode, firstEvent, firstBytes := runEncodingError(t, args, path, original)
				secondCode, secondEvent, secondBytes := runEncodingError(t, args, path, original)
				if firstCode != 2 || secondCode != 2 {
					t.Fatalf("codes=%d,%d want 2,2", firstCode, secondCode)
				}
				if firstEvent.Severity != "error" || firstEvent.Code != "io" || secondEvent.Severity != "error" || secondEvent.Code != "io" {
					t.Fatalf("diagnostics=%#v,%#v want error/io", firstEvent, secondEvent)
				}
				receipts = append(receipts, encodingErrorReceipt{
					Key: key, Profile: profile.name, FailureCategory: category, Input: tc.name, DecodeError: decodeErr.Error(),
					FirstSeverity: firstEvent.Severity, FirstCodeName: firstEvent.Code, SecondSeverity: secondEvent.Severity, SecondCodeName: secondEvent.Code,
					FirstCode: firstCode, SecondCode: secondCode, FirstUnchanged: bytes.Equal(firstBytes, original), SecondUnchanged: bytes.Equal(secondBytes, original),
					Stable: bytes.Equal(firstBytes, secondBytes),
				})
			})
		}
	}
	if len(receipts) != 11 {
		t.Fatalf("encoding-error representatives=%d want 11: %#v", len(receipts), receipts)
	}
	if output != "" {
		data, err := json.MarshalIndent(receipts, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		protected, err = configuredCorpusTargets(protected)
		if err != nil {
			t.Fatal(err)
		}
		if err := rejectCorpusArtifactOverlap("profile encoding-error receipt", output, protected); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(output, append(data, '\n'), 0o640); err != nil {
			t.Fatal(err)
		}
	}
}

func runEncodingError(t *testing.T, args []string, path string, original []byte) (int, diagnosticEvent, []byte) {
	t.Helper()
	var out, diagnostics bytes.Buffer
	code := Run(args, bytes.NewReader(nil), &out, &diagnostics)
	if out.Len() != 0 {
		t.Fatalf("unexpected stdout=%q", out.String())
	}
	lines := bytes.Split(bytes.TrimSpace(diagnostics.Bytes()), []byte{'\n'})
	if len(lines) != 1 {
		t.Fatalf("diagnostics=%q want one JSON event", diagnostics.String())
	}
	var event diagnosticEvent
	if err := json.Unmarshal(lines[0], &event); err != nil {
		t.Fatalf("invalid diagnostic JSON %q: %v", lines[0], err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, original) {
		t.Fatalf("encoding error changed bytes: %x", got)
	}
	return code, event, got
}

func decoderFailureCategory(err error) string {
	switch {
	case errors.Is(err, errUTF32):
		return "UTF32"
	case errors.Is(err, errCP932Bytes):
		return "CP932"
	case errors.Is(err, errBadEncoding):
		return "UTF8"
	}
	switch {
	case strings.HasPrefix(err.Error(), "invalid UTF-16 "), err.Error() == "UTF-16 round trip failed":
		return "UTF16"
	default:
		return ""
	}
}
