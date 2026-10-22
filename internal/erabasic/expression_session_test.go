package erabasic

import (
	"bytes"
	"strings"
	"testing"
)

func TestExpressionSessionFormattedCacheIsolation(t *testing.T) {
	session := NewExpressionSessionWithSpace([]byte(`A+B`), true)
	session.SkipFormTypes = true
	session.BuiltinTypes = true
	session.Dialect = EMEE
	if _, err := session.LexRange(ExpressionSourceOriginal, 0, 3, 0); err != nil {
		t.Fatal(err)
	}
	originalCacheSize := len(session.Lexed)
	formatted := []byte(`A + B`)
	session.SetSource(ExpressionSourceFormatted, formatted)
	if session.Formatted == nil {
		t.Fatal("formatted source has no isolated session")
	}
	if !session.Formatted.AllowFullwidthSpace || !session.Formatted.SkipFormTypes || !session.Formatted.BuiltinTypes || session.Formatted.Dialect != EMEE {
		t.Fatal("formatted session did not inherit parser settings")
	}
	tokens, err := session.LexRange(ExpressionSourceFormatted, 0, len(formatted), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(tokens) != 3 || len(session.Lexed) != originalCacheSize || len(session.Formatted.Lexed) != 1 {
		t.Fatalf("cache ownership = parent %d, formatted %d, tokens %d", len(session.Lexed), len(session.Formatted.Lexed), len(tokens))
	}
	if got, ok := session.source(ExpressionSourceFormatted, 0, len(formatted)); !ok || !bytes.Equal(got, formatted) {
		t.Fatalf("formatted source = %q, %v", got, ok)
	}
	if !session.ValidUTF8(ExpressionSourceFormatted) {
		t.Fatal("valid formatted UTF-8 was rejected")
	}
	session.SetSource(ExpressionSourceFormatted, []byte(`[[A]]`))
	if !session.HasAlias(ExpressionSourceFormatted, 0, 5) {
		t.Fatal("formatted alias was not checked by the isolated session")
	}
	session.SetSource(ExpressionSourceFormatted, []byte{0xff})
	if session.ValidUTF8(ExpressionSourceFormatted) {
		t.Fatal("invalid formatted UTF-8 was accepted")
	}
}

func TestCommandFieldTokenCacheIsolation(t *testing.T) {
	for _, src := range []string{"RESULT + 1", "RESULT ;!; + 1"} {
		session := NewExpressionSession([]byte(src))
		tokens, err := session.LexRange(ExpressionSourceOriginal, 0, len(src), 0)
		if err != nil {
			t.Fatal(err)
		}
		fields := StripCommandMarkers(tokens)
		fields[0].Kind = exprString
		if strings.Contains(src, ";!;") && tokens[0].Kind != ExprIdentifier {
			t.Fatalf("filtering changed the original marked tokens: %q", src)
		}
		cached, err := session.LexRange(ExpressionSourceOriginal, 0, len(src), 0)
		if err != nil {
			t.Fatal(err)
		}
		if cached[0].Kind != ExprIdentifier {
			t.Fatalf("filtered tokens changed the session cache: %q", src)
		}
	}
}
