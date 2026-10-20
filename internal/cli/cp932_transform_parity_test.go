package cli

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"
	"unicode/utf8"

	"golang.org/x/text/encoding/japanese"
)

func TestDecodeCP932StrictTransformParity(t *testing.T) {
	compare := func(name string, data []byte) {
		t.Helper()
		want, wantErr := decodeCP932StrictBaseline(data)
		got, gotErr := decodeCP932Strict(data)
		if (wantErr == nil) != (gotErr == nil) || (wantErr != nil && wantErr.Error() != gotErr.Error()) || !bytes.Equal(want, got) {
			t.Fatalf("%s: got %x, %v; want %x, %v", name, got, gotErr, want, wantErr)
		}
	}
	for value := 0; value <= 0xff; value++ {
		compare(fmt.Sprintf("single-%02x", value), []byte{byte(value)})
	}
	for lead := 0; lead <= 0xff; lead++ {
		for trail := 0; trail <= 0xff; trail++ {
			compare(fmt.Sprintf("pair-%02x-%02x", lead, trail), []byte{byte(lead), byte(trail)})
		}
	}
	rng := rand.New(rand.NewSource(73))
	for i := 0; i < 40000; i++ {
		data := make([]byte, rng.Intn(128))
		_, _ = rng.Read(data)
		compare(fmt.Sprintf("random-%d", i), data)
	}
}

func decodeCP932StrictBaseline(data []byte) ([]byte, error) {
	decoder := japanese.ShiftJIS.NewDecoder()
	out := make([]byte, 0, len(data))
	for i := 0; i < len(data); {
		b := data[i]
		switch {
		case b < 0x80:
			out = append(out, b)
			i++
		case b == 0xa0 || b >= 0xfd:
			r := rune(0xf8f0)
			if b >= 0xfd {
				r += rune(b - 0xfd + 1)
			}
			out = utf8.AppendRune(out, r)
			i++
		case isCP932Lead(b):
			if i+1 >= len(data) || !isCP932Trail(data[i+1]) {
				return nil, fmt.Errorf("%w at byte %d", errCP932Bytes, i)
			}
			if b >= 0xf0 && b <= 0xf9 {
				trail := int(data[i+1]) - 0x40
				if data[i+1] > 0x7f {
					trail--
				}
				out = utf8.AppendRune(out, rune(0xe000+int(b-0xf0)*188+trail))
				i += 2
				continue
			}
			chunk, err := decoder.Bytes(data[i : i+2])
			if err != nil || bytes.Contains(chunk, []byte("\xef\xbf\xbd")) {
				return nil, fmt.Errorf("%w at byte %d", errCP932Bytes, i)
			}
			out = append(out, chunk...)
			i += 2
		default:
			chunk, err := decoder.Bytes(data[i : i+1])
			if err != nil || bytes.Contains(chunk, []byte("\xef\xbf\xbd")) {
				return nil, fmt.Errorf("%w at byte %d", errCP932Bytes, i)
			}
			out = append(out, chunk...)
			i++
		}
	}
	return out, nil
}
