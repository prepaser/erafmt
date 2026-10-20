package cli

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
)

func TestCodecRoundTrip(t *testing.T) {
	text := []byte("PRINT \"text € 🥕\"\r\nPRINT 2\n")
	for _, tc := range []struct {
		name string
		enc  encodingKind
		bom  []byte
	}{
		{"utf8", encodingUTF8, nil},
		{"utf8 bom", encodingUTF8, []byte{0xEF, 0xBB, 0xBF}},
		{"utf16le", encodingUTF16LE, []byte{0xFF, 0xFE}},
		{"utf16be", encodingUTF16BE, []byte{0xFE, 0xFF}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc := sourceDocument{text: text, encoding: tc.enc, bom: tc.bom}
			encoded, err := encodeSource(doc, text)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := decodeSource(encoded)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(decoded.text, text) || !bytes.Equal(decoded.bom, tc.bom) || decoded.encoding != tc.enc {
				t.Fatalf("decoded document differs: %#v", decoded)
			}
		})
	}
}

func FuzzCodec(f *testing.F) {
	for _, seed := range [][]byte{[]byte("@F\r\n"), {0xff, 0xfe, 'A', 0}, {0xfe, 0xff, 0, 'A'}, {0xff, 0xfe, 0, 0xd8}} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		doc, err := decodeSource(data)
		if err != nil {
			return
		}
		out, err := encodeSource(doc, doc.text)
		if err != nil || !bytes.Equal(data, out) {
			t.Fatal("accepted encoding did not round trip")
		}
		converted, err := encodeOutput(doc, doc.text, true)
		if err != nil {
			t.Fatal(err)
		}
		utf8Doc, err := decodeSource(converted)
		if err != nil || utf8Doc.encoding != encodingUTF8 || !bytes.Equal(utf8Doc.bom, []byte{0xEF, 0xBB, 0xBF}) || !bytes.Equal(utf8Doc.text, doc.text) {
			t.Fatal("UTF-8 conversion changed decoded text")
		}
	})
}

func TestCodecRejectsInvalidInput(t *testing.T) {
	for _, data := range [][]byte{
		{0xC3, 0x28},
		{0xFF, 0xFE, 0x00},
		{0xFF, 0xFE, 0x00, 0xD8},
		{0xFF, 0xFE, 0x00, 0xDC},
		{0xFF, 0xFE, 0x00, 0x00},
	} {
		if _, err := decodeSource(data); err == nil {
			t.Fatalf("decodeSource(%x) unexpectedly succeeded", data)
		}
	}
}

func TestDecodeUTF16Boundaries(t *testing.T) {
	units := []uint16{0x7f, 0, 0x80, 0x7ff, 0x800, 0xd7ff, 0xe000, 0xffff, 0xd800, 0xdc00, 0xdbff, 0xdfff}
	want := "\u007f\u0000\u0080\u07ff\u0800\ud7ff\ue000\uffff\U00010000\U0010ffff"
	for _, tc := range []struct {
		name string
		enc  encodingKind
		bom  []byte
	}{
		{name: "little endian", enc: encodingUTF16LE, bom: []byte{0xff, 0xfe}},
		{name: "big endian", enc: encodingUTF16BE, bom: []byte{0xfe, 0xff}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := make([]byte, len(units)*2)
			for i, unit := range units {
				if tc.enc == encodingUTF16LE {
					binary.LittleEndian.PutUint16(body[i*2:], unit)
				} else {
					binary.BigEndian.PutUint16(body[i*2:], unit)
				}
			}
			input := append(append([]byte(nil), tc.bom...), body...)
			doc, err := decodeSource(input)
			if err != nil {
				t.Fatal(err)
			}
			if string(doc.text) != want || doc.encoding != tc.enc || !bytes.Equal(doc.bom, tc.bom) {
				t.Fatalf("decoded document = %#v, want text %q and encoding %d", doc, want, tc.enc)
			}
		})
	}
}

func TestDecodeUTF16RejectsInvalidSurrogates(t *testing.T) {
	for _, enc := range []encodingKind{encodingUTF16LE, encodingUTF16BE} {
		for _, units := range [][]uint16{{0xd800}, {0xdc00}, {0xd800, 'A'}, {0xd800, 0xdbff}} {
			body := make([]byte, len(units)*2)
			for i, unit := range units {
				if enc == encodingUTF16LE {
					binary.LittleEndian.PutUint16(body[i*2:], unit)
				} else {
					binary.BigEndian.PutUint16(body[i*2:], unit)
				}
			}
			if _, err := decodeUTF16(body, enc, nil, body); err == nil {
				t.Fatalf("decodeUTF16(%d, %x) unexpectedly succeeded", enc, units)
			}
		}
	}
}

func TestEncodeOutputReusesUnchangedRaw(t *testing.T) {
	text := []byte("@F\nRETURN 1\n")
	for _, tc := range []struct {
		name      string
		doc       sourceDocument
		utf8      bool
		wantReuse bool
	}{
		{"utf8", sourceDocument{encoding: encodingUTF8}, false, true},
		{"utf8 bom", sourceDocument{encoding: encodingUTF8, bom: []byte{0xEF, 0xBB, 0xBF}}, false, true},
		{"utf8 bom with utf8", sourceDocument{encoding: encodingUTF8, bom: []byte{0xEF, 0xBB, 0xBF}}, true, true},
		{"utf8 with utf8", sourceDocument{encoding: encodingUTF8}, true, false},
		{"utf16le", sourceDocument{encoding: encodingUTF16LE, bom: []byte{0xFF, 0xFE}}, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input, err := encodeSource(tc.doc, text)
			if err != nil {
				t.Fatal(err)
			}
			doc, err := decodeSource(input)
			if err != nil {
				t.Fatal(err)
			}
			output, err := encodeOutput(doc, doc.text, tc.utf8)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(output, input) && tc.wantReuse {
				t.Fatalf("output = %x, want %x", output, input)
			}
			if len(output) != 0 && (&output[0] == &input[0]) != tc.wantReuse {
				t.Fatalf("raw reuse = %t, want %t", &output[0] == &input[0], tc.wantReuse)
			}
		})
	}
}

func TestEncodeOutputValidatesBeforeRawReuse(t *testing.T) {
	invalid := []byte{0xff}
	doc := sourceDocument{text: invalid, encoding: encodingUTF8, raw: []byte("raw")}
	if _, err := encodeOutput(doc, invalid, false); !errors.Is(err, errBadEncoding) {
		t.Fatalf("encodeOutput error = %v, want %v", err, errBadEncoding)
	}
}

func decodeSource(data []byte) (sourceDocument, error) {
	return decodeSourceWithEncoding(data, inputEncodingAuto)
}

func TestEncodeUTF16BoundaryWords(t *testing.T) {
	boundaryText := string([]rune{0xD7FF, 0xE000, 0xFFFF, 0x10000, 0x10FFFF})
	tests := []struct {
		name  string
		text  []byte
		words []uint16
	}{
		{name: "nil", text: nil, words: nil},
		{name: "empty", text: []byte{}, words: nil},
		{name: "nul", text: []byte{'A', 0}, words: []uint16{0x0041, 0x0000}},
		{name: "boundaries", text: []byte(boundaryText), words: []uint16{0xD7FF, 0xE000, 0xFFFF, 0xD800, 0xDC00, 0xDBFF, 0xDFFF}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, enc := range []encodingKind{encodingUTF16LE, encodingUTF16BE} {
				got, err := encodeUTF16(tc.text, enc)
				if err != nil {
					t.Fatalf("encoding=%d: %v", enc, err)
				}
				want := expectedUTF16Words(tc.words, enc)
				if (got == nil) != (want == nil) || !bytes.Equal(got, want) {
					t.Fatalf("encoding=%d got=%x want=%x", enc, got, want)
				}
			}
		})
	}
}

func expectedUTF16Words(words []uint16, enc encodingKind) []byte {
	out := make([]byte, len(words)*2)
	for i, word := range words {
		if enc == encodingUTF16LE {
			binary.LittleEndian.PutUint16(out[i*2:], word)
		} else {
			binary.BigEndian.PutUint16(out[i*2:], word)
		}
	}
	return out
}

func TestEncodeUTF16RejectsMalformedUTF8(t *testing.T) {
	for _, text := range [][]byte{
		{0x80},
		{0xC2},
		{0xE0, 0x80, 0x80},
		{0xF0, 0x80, 0x80, 0x80},
		{0xED, 0xA0, 0x80},
		{0xF4, 0x90, 0x80, 0x80},
	} {
		for _, enc := range []encodingKind{encodingUTF16LE, encodingUTF16BE} {
			got, err := encodeUTF16(text, enc)
			if !errors.Is(err, errBadEncoding) || got != nil {
				t.Fatalf("encoding=%d input=%x output=%x err=%v", enc, text, got, err)
			}
		}
	}
}
