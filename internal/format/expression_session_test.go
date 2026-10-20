package format

import (
	"bytes"
	"strconv"
	"strings"
	"testing"
)

func BenchmarkFormIslands(b *testing.B) {
	for _, fields := range []int{8000, 32000} {
		b.Run(strconv.Itoa(fields), func(b *testing.B) {
			source := []byte("@F\nPRINTFORM " + strings.Repeat("{1+2}", fields) + "\n")
			options := Options{Select: []RuleID{BinarySpacing}}
			for b.Loop() {
				result := FormatWithOptions(source, ERB, options)
				if len(result.Diagnostics) != 0 || len(result.Changes) == 0 {
					b.Fatalf("diagnostics=%+v changes=%d", result.Diagnostics, len(result.Changes))
				}
			}
		})
	}
}

func TestExpressionSessionFormattedCacheIsolation(t *testing.T) {
	session := newExpressionSessionWithSpace([]byte(`A+B`), true)
	session.skipFormTypes = true
	session.builtinTypes = true
	session.dialect = EMEE

	if _, err := session.lexRange(expressionSourceOriginal, 0, 3, 0); err != nil {
		t.Fatal(err)
	}
	originalCacheSize := len(session.lexed)

	formatted := []byte(`A + B`)
	session.setSource(expressionSourceFormatted, formatted)
	if session.formatted == nil {
		t.Fatal("formatted source has no isolated session")
	}
	if !session.formatted.allowFullwidthSpace || !session.formatted.skipFormTypes || !session.formatted.builtinTypes || session.formatted.dialect != EMEE {
		t.Fatal("formatted session did not inherit parser settings")
	}
	tokens, err := session.lexRange(expressionSourceFormatted, 0, len(formatted), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(tokens) != 3 || len(session.lexed) != originalCacheSize || len(session.formatted.lexed) != 1 {
		t.Fatalf("cache ownership = parent %d, formatted %d, tokens %d", len(session.lexed), len(session.formatted.lexed), len(tokens))
	}

	if got, ok := session.source(expressionSourceFormatted, 0, len(formatted)); !ok || !bytes.Equal(got, formatted) {
		t.Fatalf("formatted source = %q, %v", got, ok)
	}
	if !session.validUTF8(expressionSourceFormatted) {
		t.Fatal("valid formatted UTF-8 was rejected")
	}

	session.setSource(expressionSourceFormatted, []byte(`[[A]]`))
	if !session.hasAlias(expressionSourceFormatted, 0, 5) {
		t.Fatal("formatted alias was not checked by the isolated session")
	}

	session.setSource(expressionSourceFormatted, []byte{0xff})
	if session.validUTF8(expressionSourceFormatted) {
		t.Fatal("invalid formatted UTF-8 was accepted")
	}
}

func TestCommandFieldTokenCacheIsolation(t *testing.T) {
	for _, src := range []string{"RESULT + 1", "RESULT ;!; + 1"} {
		session := newExpressionSession([]byte(src))
		tokens, err := session.lexRange(expressionSourceOriginal, 0, len(src), 0)
		if err != nil {
			t.Fatal(err)
		}
		fields := stripCommandMarkers(tokens)
		fields[0].kind = exprString
		if strings.Contains(src, ";!;") && tokens[0].kind != exprIdentifier {
			t.Fatalf("filtering changed the original marked tokens: %q", src)
		}
		cached, err := session.lexRange(expressionSourceOriginal, 0, len(src), 0)
		if err != nil {
			t.Fatal(err)
		}
		if cached[0].kind != exprIdentifier {
			t.Fatalf("filtered tokens changed the session cache: %q", src)
		}
	}
}
