package erabasic

import "testing"

func TestContinuationRowLimitBoundary(t *testing.T) {
	if continuationContentLimitExceeded(maxContinuationSegments - 1) {
		t.Fatal("last allowed continuation row was rejected")
	}
	if !continuationContentLimitExceeded(maxContinuationSegments) {
		t.Fatal("first continuation row beyond the limit was accepted")
	}
}
