package erabasic

import "testing"

func TestBuiltinMethodDialectSets(t *testing.T) {
	if len(builtinMethodMasks) != 266 {
		t.Fatalf("method map size=%d, want 266", len(builtinMethodMasks))
	}
	for _, name := range []string{"SUBSTRING", "CHKFONT", "SETVAR", "DT_ROW_ADD"} {
		if !ReservedBuiltinMethod(name) {
			t.Fatalf("method %q is missing", name)
		}
	}
	if methodDialect(builtinMethodMasks["SETVAR"], CommonDialect) {
		t.Fatal("EM+EE-only SETVAR is enabled in common dialect")
	}
	if !methodDialect(builtinMethodMasks["SETVAR"], EMEE) {
		t.Fatal("SETVAR is disabled in EM+EE dialect")
	}
	if methodDialect(builtinMethodMasks["SETVAR"], Emuera1824) {
		t.Fatal("SETVAR is enabled in Emuera 1.824 dialect")
	}
}
