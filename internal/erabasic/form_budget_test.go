package erabasic

import (
	"strings"
	"testing"
)

func nestedFormSource(depth int) []byte {
	source := `@"x { A+1 }"`
	for i := 1; i < depth; i++ {
		source = `@"x { ` + source + ` }"`
	}
	return []byte(source)
}

func TestFormFingerprintDepthBoundary(t *testing.T) {
	if _, err := formFingerprint(nestedFormSource(MaxFormDepth)); err != nil {
		t.Fatalf("depth limit rejected valid FORM: %v", err)
	}
	if _, err := formFingerprint(nestedFormSource(MaxFormDepth + 1)); err == nil {
		t.Fatal("FORM nesting beyond the resource limit was accepted")
	}
}

func TestFormFingerprintWorkLimit(t *testing.T) {
	input := []byte(`@"` + strings.Repeat("x", maxFormFingerprintBytes) + `"`)
	if _, err := formFingerprint(input); err == nil {
		t.Fatal("FORM fingerprint work beyond the resource limit was accepted")
	}
}
