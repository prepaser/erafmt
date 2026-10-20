package cli

import (
	"bytes"
	"errors"
	"fmt"
	"unicode/utf8"

	"golang.org/x/text/encoding/japanese"
)

var (
	errCP932Output = errors.New("CP932 input requires --utf8 output")
	errCP932Bytes  = errors.New("input is not valid CP932")
)

func decodeCP932(data []byte) ([]byte, error) {
	text, err := japanese.ShiftJIS.NewDecoder().Bytes(data)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errCP932Bytes, err)
	}
	if !bytes.Contains(text, []byte("\xef\xbf\xbd")) {
		return text, nil
	}
	return decodeCP932Strict(data)
}

func decodeCP932Strict(data []byte) ([]byte, error) {
	decoder := japanese.ShiftJIS.NewDecoder()
	out := make([]byte, 0, len(data))
	var buf [utf8.UTFMax]byte
	for i := 0; i < len(data); {
		b := data[i]
		switch {
		case b < 0x80:
			out = append(out, b)
			i++
		case b == 0xA0 || b >= 0xFD:
			r := rune(0xF8F0)
			if b >= 0xFD {
				r += rune(b - 0xFD + 1)
			}
			out = utf8.AppendRune(out, r)
			i++
		case isCP932Lead(b):
			if i+1 >= len(data) || !isCP932Trail(data[i+1]) {
				return nil, fmt.Errorf("%w at byte %d", errCP932Bytes, i)
			}
			if b >= 0xF0 && b <= 0xF9 {
				trail := int(data[i+1]) - 0x40
				if data[i+1] > 0x7F {
					trail--
				}
				out = utf8.AppendRune(out, rune(0xE000+int(b-0xF0)*188+trail))
				i += 2
				continue
			}
			nDst, nSrc, err := decoder.Transform(buf[:], data[i:i+2], true)
			chunk := buf[:nDst]
			if err != nil || nSrc != 2 || bytes.Contains(chunk, []byte("\xef\xbf\xbd")) {
				return nil, fmt.Errorf("%w at byte %d", errCP932Bytes, i)
			}
			out = append(out, chunk...)
			i += 2
		default:
			nDst, nSrc, err := decoder.Transform(buf[:], data[i:i+1], true)
			chunk := buf[:nDst]
			if err != nil || nSrc != 1 || bytes.Contains(chunk, []byte("\xef\xbf\xbd")) {
				return nil, fmt.Errorf("%w at byte %d", errCP932Bytes, i)
			}
			out = append(out, chunk...)
			i++
		}
	}
	return out, nil
}

func isCP932Lead(b byte) bool {
	return b >= 0x81 && b <= 0x9F || b >= 0xE0 && b <= 0xFC
}

func isCP932Trail(b byte) bool {
	return b >= 0x40 && b <= 0xFC && b != 0x7F
}
