package erabasic

import "testing"

func TestSifPartialMetadata(t *testing.T) {
	if !Partial("CASE") || !Partial("CASEELSE") || !Partial("ENDDATA") || Partial("BREAK") {
		t.Fatal("PARTIAL classification differs from instruction flags")
	}
}
