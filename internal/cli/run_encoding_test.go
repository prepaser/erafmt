package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRunUTF8Stdin(t *testing.T) {
	for _, doc := range []sourceDocument{
		{encoding: encodingUTF8},
		{encoding: encodingUTF8, bom: []byte{0xEF, 0xBB, 0xBF}},
		{encoding: encodingUTF16LE, bom: []byte{0xFF, 0xFE}},
		{encoding: encodingUTF16BE, bom: []byte{0xFE, 0xFF}},
	} {
		for _, text := range [][]byte{nil, []byte("@F\r\nPRINTL text α € 😀\r\n")} {
			want := append([]byte{0xEF, 0xBB, 0xBF}, text...)
			input, err := encodeSource(doc, text)
			if err != nil {
				t.Fatal(err)
			}
			var out, diagnostics bytes.Buffer
			if code := Run([]string{"--select=none"}, bytes.NewReader(input), &out, &diagnostics); code != 0 || !bytes.Equal(out.Bytes(), input) {
				t.Fatalf("default output changed: code=%d output=%x diagnostics=%q", code, out.Bytes(), diagnostics.String())
			}
			out.Reset()
			diagnostics.Reset()
			if code := Run([]string{"--utf8", "--select=none", "--explain"}, bytes.NewReader(input), &out, &diagnostics); code != 0 || !bytes.Equal(out.Bytes(), want) || diagnostics.Len() != 0 {
				t.Fatalf("UTF-8 output: code=%d output=%x want=%x diagnostics=%q", code, out.Bytes(), want, diagnostics.String())
			}
			out.Reset()
			wantCode := 0
			wantOut := ""
			if !bytes.Equal(input, want) {
				wantCode = 1
				wantOut = "<stdin>\n"
			}
			if code := Run([]string{"--utf8", "--select=none", "--check"}, bytes.NewReader(input), &out, &diagnostics); code != wantCode || out.String() != wantOut {
				t.Fatalf("UTF-8 check: code=%d want=%d output=%q diagnostics=%q", code, wantCode, out.String(), diagnostics.String())
			}
		}
	}
}

func TestRunUTF8Files(t *testing.T) {
	root := t.TempDir()
	text := []byte(";text α € 😀\r\n")
	want := append([]byte{0xEF, 0xBB, 0xBF}, text...)
	for _, tc := range []struct {
		name string
		doc  sourceDocument
	}{
		{"utf8.ERB", sourceDocument{encoding: encodingUTF8, bom: []byte{0xEF, 0xBB, 0xBF}}},
		{"utf8-plain.ERB", sourceDocument{encoding: encodingUTF8}},
		{"utf16le.ERB", sourceDocument{encoding: encodingUTF16LE, bom: []byte{0xFF, 0xFE}}},
		{"utf16be.ERH", sourceDocument{encoding: encodingUTF16BE, bom: []byte{0xFE, 0xFF}}},
	} {
		path := filepath.Join(root, tc.name)
		input, err := encodeSource(tc.doc, text)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, input, 0o644); err != nil {
			t.Fatal(err)
		}
		var out, diagnostics bytes.Buffer
		wantCode := 1
		wantOut := path + "\n"
		if bytes.Equal(input, want) {
			wantCode = 0
			wantOut = ""
		}
		if code := Run([]string{"--utf8", "--select=none", "--check", path}, bytes.NewReader(nil), &out, &diagnostics); code != wantCode || out.String() != wantOut {
			t.Fatalf("file check: code=%d output=%q diagnostics=%q", code, out.String(), diagnostics.String())
		}
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, input) {
			t.Fatalf("check modified file: output=%x err=%v", got, err)
		}
	}
	var out, diagnostics bytes.Buffer
	if code := Run([]string{"--utf8", "--select=none", "-l", "-w", root}, bytes.NewReader(nil), &out, &diagnostics); code != 0 {
		t.Fatalf("directory write: code=%d diagnostics=%q", code, diagnostics.String())
	}
	wantList := ""
	for _, name := range []string{"utf16be.ERH", "utf16le.ERB", "utf8-plain.ERB", "utf8.ERB"} {
		path := filepath.Join(root, name)
		if name != "utf8.ERB" {
			wantList += path + "\n"
		}
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("UTF-8 write %s: output=%x err=%v", path, got, err)
		}
	}
	if out.String() != wantList {
		t.Fatalf("write list=%q want=%q", out.String(), wantList)
	}
	out.Reset()
	if code := Run([]string{"--utf8", "--select=none", "--check", root}, bytes.NewReader(nil), &out, &diagnostics); code != 0 || out.Len() != 0 {
		t.Fatalf("UTF-8 files are not unchanged: code=%d output=%q diagnostics=%q", code, out.String(), diagnostics.String())
	}
}

func TestRunUTF8ErrorsDoNotWrite(t *testing.T) {
	badSyntax, err := encodeSource(sourceDocument{encoding: encodingUTF16LE, bom: []byte{0xFF, 0xFE}}, []byte("@F\nENDIF\n"))
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range [][]byte{{0xFF, 0xFE, 0x00}, badSyntax} {
		path := filepath.Join(t.TempDir(), "error.ERB")
		if err := os.WriteFile(path, input, 0o644); err != nil {
			t.Fatal(err)
		}
		var out, diagnostics bytes.Buffer
		if code := Run([]string{"--utf8", "-w", path}, bytes.NewReader(nil), &out, &diagnostics); code != 2 {
			t.Fatalf("error write: code=%d diagnostics=%q", code, diagnostics.String())
		}
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, input) || out.Len() != 0 {
			t.Fatalf("error modified file: output=%x stdout=%q err=%v", got, out.String(), err)
		}
	}
	var out, diagnostics bytes.Buffer
	if code := Run([]string{"--utf8", "--rules"}, unreadableInput{}, &out, &diagnostics); code != 2 || out.Len() != 0 || bytes.Contains(diagnostics.Bytes(), []byte("closed")) {
		t.Fatalf("UTF-8 with rules read input: code=%d stdout=%q diagnostics=%q", code, out.String(), diagnostics.String())
	}
}

func TestRunUTF8WarningPreprocessorWrite(t *testing.T) {
	text := []byte("@F\r\n[SKIPEND]\r\nPRINTL text €  % raw ;!;  \r\nRETURN 1+2")
	want := append([]byte{0xEF, 0xBB, 0xBF}, []byte("@F\n[SKIPEND]\nPRINTL text €  % raw ;!;  \nRETURN 1+2\n")...)
	for _, doc := range []sourceDocument{
		{encoding: encodingUTF16LE, bom: []byte{0xFF, 0xFE}},
		{encoding: encodingUTF16BE, bom: []byte{0xFE, 0xFF}},
	} {
		input, err := encodeSource(doc, text)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), "warning.ERB")
		if err := os.WriteFile(path, input, 0o644); err != nil {
			t.Fatal(err)
		}
		var out, diagnostics bytes.Buffer
		if code := Run([]string{"--utf8", "-w", path}, bytes.NewReader(nil), &out, &diagnostics); code != 2 || out.Len() != 0 || !bytes.Contains(diagnostics.Bytes(), []byte("invalid_structure")) {
			t.Fatalf("warning write: code=%d stdout=%q diagnostics=%q", code, out.String(), diagnostics.String())
		}
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("warning changed code or payload: output=%x want=%x err=%v", got, want, err)
		}
	}
}
