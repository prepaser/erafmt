package cli

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"unicode/utf8"
)

type encodingKind uint8

const (
	encodingUTF8 encodingKind = iota
	encodingUTF16LE
	encodingUTF16BE
	encodingCP932
)

type inputEncodingKind uint8

const (
	inputEncodingAuto inputEncodingKind = iota
	inputEncodingCP932
	inputEncodingCP932Fallback
)

type sourceDocument struct {
	text     []byte
	encoding encodingKind
	bom      []byte
	raw      []byte
}

var (
	errUTF32       = errors.New("UTF-32 is not supported")
	errBadEncoding = errors.New("input is not valid UTF-8 or BOM-marked UTF-16")
)

func decodeSourceWithEncoding(data []byte, requested inputEncodingKind) (sourceDocument, error) {
	raw := data
	if len(data) >= 4 && (bytes.Equal(data[:4], []byte{0xFF, 0xFE, 0x00, 0x00}) || bytes.Equal(data[:4], []byte{0x00, 0x00, 0xFE, 0xFF})) {
		return sourceDocument{}, errUTF32
	}
	if len(data) >= 2 && bytes.Equal(data[:2], []byte{0xFF, 0xFE}) {
		return decodeUTF16(data[2:], encodingUTF16LE, data[:2], raw)
	}
	if len(data) >= 2 && bytes.Equal(data[:2], []byte{0xFE, 0xFF}) {
		return decodeUTF16(data[2:], encodingUTF16BE, data[:2], raw)
	}
	var bom []byte
	if len(data) >= 3 && bytes.Equal(data[:3], []byte{0xEF, 0xBB, 0xBF}) {
		bom = data[:3]
		data = data[3:]
		if !utf8.Valid(data) {
			return sourceDocument{}, errBadEncoding
		}
		return sourceDocument{text: data, encoding: encodingUTF8, bom: bom, raw: raw}, nil
	}
	if requested == inputEncodingCP932 {
		text, err := decodeCP932(data)
		if err != nil {
			return sourceDocument{}, err
		}
		return sourceDocument{text: text, encoding: encodingCP932, raw: raw}, nil
	}
	if utf8.Valid(data) {
		return sourceDocument{text: data, encoding: encodingUTF8, bom: bom, raw: raw}, nil
	}
	if requested == inputEncodingCP932Fallback {
		text, err := decodeCP932(data)
		if err != nil {
			return sourceDocument{}, err
		}
		return sourceDocument{text: text, encoding: encodingCP932, raw: raw}, nil
	}
	return sourceDocument{}, errBadEncoding
}

func decodeUTF16(data []byte, enc encodingKind, bom, raw []byte) (sourceDocument, error) {
	if len(data)%2 != 0 {
		return sourceDocument{}, fmt.Errorf("invalid UTF-16 byte length")
	}
	utf8Bytes := 0
	const maxInt = int(^uint(0) >> 1)
	for i := 0; i < len(data); i += 2 {
		w := utf16Word(data[i:], enc)
		width := 0
		switch {
		case w >= 0xD800 && w <= 0xDBFF:
			if i+2 >= len(data) {
				return sourceDocument{}, fmt.Errorf("invalid UTF-16 surrogate pair")
			}
			next := utf16Word(data[i+2:], enc)
			if next < 0xDC00 || next > 0xDFFF {
				return sourceDocument{}, fmt.Errorf("invalid UTF-16 surrogate pair")
			}
			width = 4
			i += 2
		case w >= 0xDC00 && w <= 0xDFFF:
			return sourceDocument{}, fmt.Errorf("invalid UTF-16 surrogate pair")
		case w < 0x80:
			width = 1
		case w < 0x800:
			width = 2
		default:
			width = 3
		}
		if utf8Bytes > maxInt-width {
			return sourceDocument{}, fmt.Errorf("invalid UTF-16 decoded length")
		}
		utf8Bytes += width
	}
	text := make([]byte, utf8Bytes)
	pos := 0
	for i := 0; i < len(data); i += 2 {
		w := utf16Word(data[i:], enc)
		if w >= 0xD800 && w <= 0xDBFF {
			next := utf16Word(data[i+2:], enc)
			r := rune(0x10000 + ((uint32(w) - 0xD800) << 10) + (uint32(next) - 0xDC00))
			pos += utf8.EncodeRune(text[pos:], r)
			i += 2
			continue
		}
		pos += utf8.EncodeRune(text[pos:], rune(w))
	}
	return sourceDocument{text: text, encoding: enc, bom: bom, raw: raw}, nil
}

func utf16Word(data []byte, enc encodingKind) uint16 {
	if enc == encodingUTF16LE {
		return binary.LittleEndian.Uint16(data)
	}
	return binary.BigEndian.Uint16(data)
}

func encodeOutput(doc sourceDocument, text []byte, utf8Output bool) ([]byte, error) {
	if !utf8.Valid(text) {
		return nil, errBadEncoding
	}
	if doc.raw != nil && bytes.Equal(text, doc.text) {
		if !utf8Output && doc.encoding != encodingCP932 {
			return doc.raw, nil
		}
		if utf8Output && doc.encoding == encodingUTF8 && bytes.Equal(doc.bom, []byte{0xEF, 0xBB, 0xBF}) {
			return doc.raw, nil
		}
	}
	if utf8Output {
		doc.encoding = encodingUTF8
		doc.bom = []byte{0xEF, 0xBB, 0xBF}
	}
	return encodeSource(doc, text)
}

func encodeSource(doc sourceDocument, text []byte) ([]byte, error) {
	if !utf8.Valid(text) {
		return nil, errBadEncoding
	}
	switch doc.encoding {
	case encodingUTF8:
		out := make([]byte, 0, len(doc.bom)+len(text))
		out = append(out, doc.bom...)
		return append(out, text...), nil
	case encodingUTF16LE, encodingUTF16BE:
		if doc.encoding == encodingUTF16LE && bytes.Equal(doc.bom, []byte{0xff, 0xfe}) && len(text) != 0 && text[0] == 0 {
			return nil, fmt.Errorf("UTF-16LE output would be interpreted as UTF-32; use --utf8")
		}
		body, err := encodeUTF16(text, doc.encoding)
		if err != nil {
			return nil, err
		}
		out := append([]byte(nil), doc.bom...)
		return append(out, body...), nil
	case encodingCP932:
		return nil, errCP932Output
	default:
		return nil, errBadEncoding
	}
}

func encodeUTF16(text []byte, enc encodingKind) ([]byte, error) {
	if !utf8.Valid(text) {
		return nil, errBadEncoding
	}
	units := 0
	for i := 0; i < len(text); {
		r, size := utf8.DecodeRune(text[i:])
		if r > 0xFFFF {
			units += 2
		} else {
			units++
		}
		i += size
	}
	out := make([]byte, units*2)
	pos := 0
	for i := 0; i < len(text); {
		r, size := utf8.DecodeRune(text[i:])
		if r > 0xFFFF {
			r -= 0x10000
			putUTF16Word(out[pos:], uint16(0xD800+(r>>10)), enc)
			putUTF16Word(out[pos+2:], uint16(0xDC00+(r&0x3FF)), enc)
			pos += 4
		} else {
			putUTF16Word(out[pos:], uint16(r), enc)
			pos += 2
		}
		i += size
	}
	return out, nil
}

func putUTF16Word(dst []byte, word uint16, enc encodingKind) {
	if enc == encodingUTF16LE {
		binary.LittleEndian.PutUint16(dst, word)
		return
	}
	binary.BigEndian.PutUint16(dst, word)
}
