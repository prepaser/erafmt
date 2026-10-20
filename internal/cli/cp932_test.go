package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDecodeCP932OverridesBOMlessUTF8(t *testing.T) {
	auto, err := decodeSourceWithEncoding([]byte{0xC2, 0xA3}, inputEncodingAuto)
	if err != nil || string(auto.text) != "£" || auto.encoding != encodingUTF8 {
		t.Fatalf("auto = %#v, %v", auto, err)
	}
	cp932, err := decodeSourceWithEncoding([]byte{0xC2, 0xA3}, inputEncodingCP932)
	if err != nil || string(cp932.text) != "ﾂ｣" || cp932.encoding != encodingCP932 {
		t.Fatalf("cp932 = %#v, %v", cp932, err)
	}
}

func TestDecodeCP932FallbackPrefersValidUTF8(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want string
		kind encodingKind
	}{
		{name: "utf8", data: []byte("日本語"), want: "日本語", kind: encodingUTF8},
		{name: "ambiguous cp932", data: []byte{0xC2, 0xA3}, want: "£", kind: encodingUTF8},
		{name: "legacy cp932", data: []byte{0x93, 0xFA, 0x96, 0x7B, 0x8C, 0xEA}, want: "日本語", kind: encodingCP932},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc, err := decodeSourceWithEncoding(tc.data, inputEncodingCP932Fallback)
			if err != nil || doc.encoding != tc.kind || string(doc.text) != tc.want {
				t.Fatalf("doc = %#v, err = %v", doc, err)
			}
		})
	}
}

func TestDecodeCP932BOMIsAuthoritative(t *testing.T) {
	utf8Input := append([]byte{0xEF, 0xBB, 0xBF}, []byte("£")...)
	doc, err := decodeSourceWithEncoding(utf8Input, inputEncodingCP932)
	if err != nil || string(doc.text) != "£" || !bytes.Equal(doc.bom, utf8Input[:3]) || doc.encoding != encodingUTF8 {
		t.Fatalf("UTF-8 BOM = %#v, %v", doc, err)
	}
	utf16Input, err := encodeSource(sourceDocument{encoding: encodingUTF16LE, bom: []byte{0xFF, 0xFE}}, []byte("£"))
	if err != nil {
		t.Fatal(err)
	}
	doc, err = decodeSourceWithEncoding(utf16Input, inputEncodingCP932)
	if err != nil || string(doc.text) != "£" || !bytes.Equal(doc.bom, utf16Input[:2]) || doc.encoding != encodingUTF16LE {
		t.Fatalf("UTF-16 BOM = %#v, %v", doc, err)
	}
}

func TestDecodeCP932FallbackRejectsMalformedAuthoritativeInput(t *testing.T) {
	for _, data := range [][]byte{
		{0xEF, 0xBB, 0xBF, 0xC3, 0x28},
		{0xFF, 0xFE, 0x00, 0x00},
		{0x00, 0x00, 0xFE, 0xFF},
	} {
		if _, err := decodeSourceWithEncoding(data, inputEncodingCP932Fallback); err == nil {
			t.Fatalf("decodeSourceWithEncoding(%x) unexpectedly succeeded", data)
		}
	}
}

func TestDecodeCP932StrictExtensions(t *testing.T) {
	doc, err := decodeSourceWithEncoding([]byte{0xF0, 0x40, 0xF9, 0xFC, 0xA0, 0xFD, 0xFE, 0xFF}, inputEncodingCP932)
	if err != nil {
		t.Fatal(err)
	}
	want := "\ue000\ue757\uf8f0\uf8f1\uf8f2\uf8f3"
	if string(doc.text) != want {
		t.Fatalf("decoded extensions = %U, want %U", []rune(string(doc.text)), []rune(want))
	}
}

func TestDecodeCP932ByteShapes(t *testing.T) {
	for value := 0; value <= 0xFF; value++ {
		b := byte(value)
		_, err := decodeSourceWithEncoding([]byte{b}, inputEncodingCP932)
		if isCP932Lead(b) {
			if err == nil {
				t.Fatalf("lead byte %02x accepted without a trail", b)
			}
			continue
		}
		if b >= 0xA1 && b <= 0xDF || b < 0x80 || b == 0x80 || b == 0xA0 || b >= 0xFD {
			if err != nil {
				t.Fatalf("valid single byte %02x rejected: %v", b, err)
			}
		}
	}
	for _, pair := range [][]byte{{0x81, 0x40}, {0x82, 0xA0}, {0xE0, 0x40}, {0xFA, 0x40}, {0xF0, 0x40}, {0xF9, 0xFC}} {
		if _, err := decodeSourceWithEncoding(pair, inputEncodingCP932); err != nil {
			t.Fatalf("valid pair %x rejected: %v", pair, err)
		}
	}
	for _, pair := range [][]byte{{0x81, 0x30}, {0x81, 0x7F}, {0x81, 0xFD}, {0xE0, 0x39}} {
		if _, err := decodeSourceWithEncoding(pair, inputEncodingCP932); err == nil {
			t.Fatalf("invalid pair %x accepted", pair)
		}
	}
}

func TestDecodeCP932RejectsInvalidBytes(t *testing.T) {
	for _, data := range [][]byte{
		{0x81},
		{0x81, 0x7F},
		{0x81, 0xFF},
		{0xEF, 0xBB, 0xBF, 0xC3, 0x28},
		{0xFF, 0xFE, 0x00},
		{0xFF, 0xFE, 0x00, 0x00},
	} {
		if _, err := decodeSourceWithEncoding(data, inputEncodingCP932); err == nil {
			t.Fatalf("decodeSourceWithEncoding(%x) unexpectedly succeeded", data)
		}
	}
	if _, err := decodeCP932([]byte{0x81}); !errors.Is(err, errCP932Bytes) {
		t.Fatalf("invalid CP932 error = %v", err)
	}
}

func TestCP932OutputRequiresUTF8(t *testing.T) {
	doc, err := decodeSourceWithEncoding([]byte{0x82, 0xA0}, inputEncodingCP932)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := encodeOutput(doc, doc.text, false); !errors.Is(err, errCP932Output) {
		t.Fatalf("legacy output error = %v", err)
	}
	out, err := encodeOutput(doc, doc.text, true)
	if err != nil || !bytes.Equal(out, append([]byte{0xEF, 0xBB, 0xBF}, doc.text...)) {
		t.Fatalf("UTF-8 output = %x, %v", out, err)
	}
}

func TestRunCP932Stdin(t *testing.T) {
	input := []byte("PRINTL \x82\xA0\r\n")
	want := append([]byte{0xEF, 0xBB, 0xBF}, []byte("PRINTL あ\n")...)
	var out, diagnostics bytes.Buffer
	if code := Run([]string{"--input-encoding=cp932", "--utf8"}, bytes.NewReader(input), &out, &diagnostics); code != 0 {
		t.Fatalf("Run() = %d, diagnostics %q", code, diagnostics.String())
	}
	if !bytes.Equal(out.Bytes(), want) {
		t.Fatalf("output = %x, want %x", out.Bytes(), want)
	}
}

func TestRunCP932FilesConvertAtomically(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.ERB")
	input := []byte("PRINTL \x82\xA0\r\n")
	if err := os.WriteFile(path, input, 0o644); err != nil {
		t.Fatal(err)
	}
	var out, diagnostics bytes.Buffer
	if code := Run([]string{"--input-encoding=cp932", "--utf8", "--check", path}, strings.NewReader(""), &out, &diagnostics); code != 1 || out.String() != path+"\n" {
		t.Fatalf("check = %d, output %q, diagnostics %q", code, out.String(), diagnostics.String())
	}
	if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, input) {
		t.Fatalf("check modified file: %x, %v", got, err)
	}
	out.Reset()
	diagnostics.Reset()
	if code := Run([]string{"--input-encoding=cp932", "--utf8", "-w", path}, strings.NewReader(""), &out, &diagnostics); code != 0 {
		t.Fatalf("write = %d, diagnostics %q", code, diagnostics.String())
	}
	want := append([]byte{0xEF, 0xBB, 0xBF}, []byte("PRINTL あ\n")...)
	if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, want) {
		t.Fatalf("written file = %x, want %x, err=%v", got, want, err)
	}
}

func TestRunCP932RequiresUTF8BeforeInput(t *testing.T) {
	var out, diagnostics bytes.Buffer
	if code := Run([]string{"--input-encoding=cp932", "--check"}, unreadableInput{}, &out, &diagnostics); code != 2 {
		t.Fatalf("missing --utf8 code = %d, diagnostics %q", code, diagnostics.String())
	}
	if !strings.Contains(diagnostics.String(), "requires --utf8") {
		t.Fatalf("missing --utf8 diagnostic = %q", diagnostics.String())
	}
	out.Reset()
	diagnostics.Reset()
	if code := Run([]string{"--input-encoding=cp932-fallback", "--check"}, unreadableInput{}, &out, &diagnostics); code != 2 {
		t.Fatalf("fallback missing --utf8 code = %d, diagnostics %q", code, diagnostics.String())
	}
	if !strings.Contains(diagnostics.String(), "requires --utf8") {
		t.Fatalf("fallback missing --utf8 diagnostic = %q", diagnostics.String())
	}
	out.Reset()
	diagnostics.Reset()
	if code := Run([]string{"--input-encoding=cp932", "--utf8", "--rules"}, unreadableInput{}, &out, &diagnostics); code != 2 || out.Len() != 0 {
		t.Fatalf("rules conflict code = %d, output %q, diagnostics %q", code, out.String(), diagnostics.String())
	}
}
