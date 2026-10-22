package erabasic

import (
	"bytes"
	"strings"
	"testing"
)

func TestExpressionSessionBoundsAndSourceIsolation(t *testing.T) {
	src := []byte(`@"x { A+B }" trailing`)
	session := NewExpressionSession(src)
	limit := strings.Index(string(src), " trailing")
	end, err := session.ScanFormAt(ExpressionSourceOriginal, 0, limit, FormQuoted)
	if err != nil || end != limit {
		t.Fatalf("bounded scan = %d, %v; want %d", end, err, limit)
	}
	largerEnd, err := session.ScanFormAt(ExpressionSourceOriginal, 0, len(src), FormQuoted)
	if err != nil || largerEnd != limit || len(session.Scans) != 1 {
		t.Fatalf("explicit scan reuse = %d, %v, cache size %d", largerEnd, err, len(session.Scans))
	}
	wide := NewExpressionSession(src)
	if end, err := wide.ScanFormAt(ExpressionSourceOriginal, 0, len(src), FormQuoted); err != nil || end != limit {
		t.Fatalf("wide scan = %d, %v; want %d", end, err, limit)
	}
	if end, err := wide.ScanFormAt(ExpressionSourceOriginal, 0, limit, FormQuoted); err != nil || end != limit || len(wide.Scans) != 1 {
		t.Fatalf("narrow explicit reuse = %d, %v, cache size %d", end, err, len(wide.Scans))
	}
	if _, err := session.ScanFormAt(ExpressionSourceOriginal, 0, 3, FormQuoted); err == nil {
		t.Fatal("bounded scan accepted an unterminated FORM")
	}
	first, err := session.FormFingerprintAt(ExpressionSourceOriginal, 0, limit, FormQuoted)
	if err != nil {
		t.Fatal(err)
	}
	session.SetSource(ExpressionSourceFormatted, []byte(`@"x { A+B }"`))
	second, err := session.FormFingerprintAt(ExpressionSourceFormatted, 0, len(session.Sources[ExpressionSourceFormatted]), FormQuoted)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("trivia-only source changed fingerprint: %q vs %q", first, second)
	}
	session.SetSource(ExpressionSourceFormatted, []byte(`@"x { A ;!; B }"`))
	third, err := session.FormFingerprintAt(ExpressionSourceFormatted, 0, len(session.Sources[ExpressionSourceFormatted]), FormQuoted)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(second, third) {
		t.Fatal("formatted source reused a stale fingerprint")
	}
}

func TestExpressionSessionAliasIndexOverlaps(t *testing.T) {
	session := NewExpressionSession([]byte(`[[[`))
	if !session.HasAlias(ExpressionSourceOriginal, 1, 3) {
		t.Fatal("overlapping alias marker was not indexed")
	}
	if session.HasAlias(ExpressionSourceOriginal, 2, 3) {
		t.Fatal("partial alias marker was reported")
	}
}
