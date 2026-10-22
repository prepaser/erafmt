package erabasic

import (
	"bytes"
	"testing"
)

func TestFullwidthSpaceHelpers(t *testing.T) {
	src := []byte("\u3000\tvalue\u3000")
	if got := TrimInput(src, false); !bytes.Equal(got, src) {
		t.Fatalf("trimInput(default) = %q, want unchanged", got)
	}
	if got := TrimInput(src, true); string(got) != "value" {
		t.Fatalf("trimInput(allow) = %q, want value", got)
	}
	got, _, _, _ := headOfWithSpace([]byte("\u3000;#;\u3000RETURN 1"), true)
	if string(got) != "RETURN 1" {
		t.Fatalf("headOfWithSpace = %q, want RETURN 1", got)
	}
	if command, end := CommandOfWithSpace([]byte("RETURN\u30001"), true); command != "RETURN" || end != len("RETURN") {
		t.Fatalf("commandOfWithSpace = %q, %d", command, end)
	}
}

func TestFullwidthHeaderSpaceOption(t *testing.T) {
	valid := []string{"@\u3000F", "@F\u3000(ARG:0)", "@F(ARG:0\u3000+ 1)", "@F(1)\u3000; note"}
	for _, source := range valid {
		src := []byte(source)
		if got := validateHeader(src); got.Status != headerUnsupported {
			t.Errorf("validateHeader(%q) = %v %q, want unsupported", source, got.Status, got.Message)
		}
		session := NewExpressionSessionWithSpace(src, true)
		if got := validateHeaderWithSession(src, session); got.Status != headerValid {
			t.Errorf("validateHeaderWithSession(%q) = %v %q, want valid", source, got.Status, got.Message)
		}
	}
}
