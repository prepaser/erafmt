package erabasic

import "testing"

func TestContinuationDiagnosticSegmentBoundaries(t *testing.T) {
	segments := []ContinuationSegment{
		{LogicalStart: 0, LogicalEnd: 2, SourceStart: 10, SourceEnd: 12},
		{LogicalStart: 2, LogicalEnd: 4, SourceStart: 20, SourceEnd: 22},
		{LogicalStart: 6, LogicalEnd: 6, SourceStart: 30, SourceEnd: 30},
		{LogicalStart: 8, LogicalEnd: 10, SourceStart: 40, SourceEnd: 42},
	}
	for _, tc := range []struct{ offset, segment, local int }{
		{offset: -1, segment: 0, local: -1},
		{offset: 0, segment: 0, local: 0},
		{offset: 1, segment: 0, local: 1},
		{offset: 2, segment: 1, local: 0},
		{offset: 4, segment: 1, local: 2},
		{offset: 5, segment: 1, local: 2},
		{offset: 6, segment: 2, local: 0},
		{offset: 7, segment: 2, local: 0},
		{offset: 8, segment: 3, local: 0},
		{offset: 10, segment: 3, local: 2},
		{offset: 99, segment: 3, local: 2},
	} {
		segment, local := ContinuationDiagnosticSegment(tc.offset, segments)
		if segment != tc.segment || local != tc.local {
			t.Errorf("offset=%d got=(%d,%d), want=(%d,%d)", tc.offset, segment, local, tc.segment, tc.local)
		}
	}
	if segment, local := ContinuationDiagnosticSegment(0, nil); segment != -1 || local != 0 {
		t.Fatalf("empty segments got=(%d,%d), want=(-1,0)", segment, local)
	}
	duplicateEnds := []ContinuationSegment{{LogicalStart: 0, LogicalEnd: 0, SourceStart: 10, SourceEnd: 10}, {LogicalStart: 0, LogicalEnd: 0, SourceStart: 20, SourceEnd: 20}, {LogicalStart: 0, LogicalEnd: 1, SourceStart: 30, SourceEnd: 31}}
	for _, tc := range []struct{ offset, segment, local int }{{offset: 0, segment: 2, local: 0}, {offset: 1, segment: 2, local: 1}, {offset: 2, segment: 2, local: 1}} {
		segment, local := ContinuationDiagnosticSegment(tc.offset, duplicateEnds)
		if segment != tc.segment || local != tc.local {
			t.Errorf("duplicate ends offset=%d got=(%d,%d), want=(%d,%d)", tc.offset, segment, local, tc.segment, tc.local)
		}
	}
	allEmpty := duplicateEnds[:2]
	if segment, local := ContinuationDiagnosticSegment(0, allEmpty); segment != 0 || local != 0 {
		t.Fatalf("all-empty start got=(%d,%d), want=(0,0)", segment, local)
	}
	if segment, local := ContinuationDiagnosticSegment(1, allEmpty); segment != 1 || local != 0 {
		t.Fatalf("all-empty tail got=(%d,%d), want=(1,0)", segment, local)
	}
}
