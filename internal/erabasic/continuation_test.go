package erabasic

import "testing"

func TestContinuationDiagnosticMapsUnicodeColumn(t *testing.T) {
	segments := []ContinuationSegment{{LogicalStart: 0, LogicalEnd: len([]byte("  😀 +")), Line: 3, SourceStart: 12, SourceEnd: 12 + len([]byte("  😀 +"))}}
	source := []byte("  😀 +")
	if got := ContinuationByteOffset(4, source); got != len([]byte("  😀")) {
		t.Fatalf("offset=%d", got)
	}
	segment, local := ContinuationDiagnosticSegment(len([]byte("  😀")), segments)
	if segment != 0 || local != len([]byte("  😀")) {
		t.Fatalf("segment=%d local=%d", segment, local)
	}
}
