package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUTF16LELeadingNULOutputDoesNotReplaceSource(t *testing.T) {
	original := []byte{0xff, 0xfe, 0x0a, 0x00, 0x00, 0x00}
	path := filepath.Join(t.TempDir(), "leading-nul.ERB")
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}

	var out, diagnostics bytes.Buffer
	if code := Run([]string{"--select=L006", "-w", path}, strings.NewReader(""), &out, &diagnostics); code != 2 {
		t.Fatalf("Run() = %d, want 2; stdout=%x diagnostics=%q", code, out.Bytes(), diagnostics.String())
	}
	if !strings.Contains(diagnostics.String(), "UTF-16LE output would be interpreted as UTF-32") {
		t.Fatalf("diagnostics = %q", diagnostics.String())
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, original) {
		t.Fatalf("source = %x, want unchanged %x", got, original)
	}
}

func TestUTF16LELeadingNULOutputCanUseUTF8(t *testing.T) {
	original := []byte{0xff, 0xfe, 0x0a, 0x00, 0x00, 0x00}
	path := filepath.Join(t.TempDir(), "leading-nul.ERB")
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}

	var out, diagnostics bytes.Buffer
	if code := Run([]string{"--utf8", "--select=L006", "-w", path}, strings.NewReader(""), &out, &diagnostics); code != 0 {
		t.Fatalf("Run() = %d, want 0; stdout=%x diagnostics=%q", code, out.Bytes(), diagnostics.String())
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{0xef, 0xbb, 0xbf, 0x00}
	if !bytes.Equal(got, want) {
		t.Fatalf("source = %x, want %x", got, want)
	}
}

func TestEncodeSourceRejectsAmbiguousUTF16LELeadingNUL(t *testing.T) {
	doc := sourceDocument{encoding: encodingUTF16LE, bom: []byte{0xff, 0xfe}}
	if got, err := encodeSource(doc, []byte{0}); got != nil || err == nil || !strings.Contains(err.Error(), "UTF-16LE output would be interpreted as UTF-32") {
		t.Fatalf("encodeSource() = %x, %v", got, err)
	}

	out, err := encodeSource(sourceDocument{encoding: encodingUTF16BE, bom: []byte{0xfe, 0xff}}, []byte{0})
	if err != nil || !bytes.Equal(out, []byte{0xfe, 0xff, 0x00, 0x00}) {
		t.Fatalf("UTF-16BE output = %x, %v", out, err)
	}
	out, err = encodeOutput(doc, []byte{0}, true)
	if err != nil || !bytes.Equal(out, []byte{0xef, 0xbb, 0xbf, 0x00}) {
		t.Fatalf("UTF-8 output = %x, %v", out, err)
	}
}

func TestContentBOMOutputRemainsStable(t *testing.T) {
	for _, text := range [][]byte{
		[]byte("\n\ufeff@F\nRETURN 0\n"),
		[]byte("\t\ufeffX+=1+2\n"),
		[]byte("\n \ufeffX=1\n"),
		[]byte(" \ufeffX++\n"),
	} {
		for _, doc := range []sourceDocument{
			{encoding: encodingUTF8},
			{encoding: encodingUTF8, bom: []byte{0xef, 0xbb, 0xbf}},
			{encoding: encodingUTF16LE, bom: []byte{0xff, 0xfe}},
			{encoding: encodingUTF16BE, bom: []byte{0xfe, 0xff}},
		} {
			input, err := encodeSource(doc, text)
			if err != nil {
				t.Fatal(err)
			}
			for _, args := range [][]string{nil, {"--utf8"}} {
				var first, second, diagnostics bytes.Buffer
				if code := Run(args, bytes.NewReader(input), &first, &diagnostics); code != 0 {
					t.Fatalf("text=%q encoding=%v args=%v first exit=%d diagnostics=%q", text, doc.encoding, args, code, diagnostics.String())
				}
				if code := Run(args, bytes.NewReader(first.Bytes()), &second, &diagnostics); code != 0 || !bytes.Equal(first.Bytes(), second.Bytes()) {
					t.Fatalf("text=%q encoding=%v args=%v second exit=%d first=%x second=%x diagnostics=%q", text, doc.encoding, args, code, first.Bytes(), second.Bytes(), diagnostics.String())
				}
				decoded, err := decodeSource(first.Bytes())
				if err != nil || bytes.Count(decoded.text, []byte("\ufeff")) != 1 {
					t.Fatalf("text=%q encoding=%v args=%v content=%q error=%v", text, doc.encoding, args, decoded.text, err)
				}
			}
		}
	}
}
