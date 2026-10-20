package cli

import "testing"

func TestCorpusVerifierAcceptsRuntimeIdentifierBytes(t *testing.T) {
	for _, name := range []string{"`USR", "USR`", "\x01USR", "USR\x01", "\x07USR", "USR\x07", "\x1fUSR", "USR\x1f", "\x7fUSR", "USR\x7f", "US\x00R"} {
		if !assignmentLHS([]byte(name)) {
			t.Errorf("assignmentLHS rejected runtime identifier %q", name)
		}
		original := append([]byte("@F\n"), append([]byte(name), []byte("=1\n")...)...)
		formatted := append([]byte("@F\n    "), append([]byte(name), []byte(" = 1\n")...)...)
		if err := checkSourcePreserved(original, formatted); err != nil {
			t.Errorf("verifier rejected runtime identifier %q: %v", name, err)
		}
	}
	for _, source := range []string{"\x00USR = 1", "USR\n = 1", "USR\r = 1", "USR + 1"} {
		if assignmentLHS([]byte(source)) {
			t.Errorf("assignmentLHS accepted invalid identifier boundary %q", source)
		}
	}
}

func TestCorpusVerifierAcceptsScopedIndexWhitespace(t *testing.T) {
	for _, source := range []string{
		"CFLAG:(LOCAL @ F):0",
		"RESULT:(LOCAL\t@\tF):0",
		"CFLAG:(LOCAL@F):0",
	} {
		if !assignmentLHS([]byte(source)) {
			t.Errorf("assignmentLHS rejected scoped index %q", source)
		}
	}
}

func TestCorpusVerifierPreservesNULIncrementSuffix(t *testing.T) {
	original := []byte("@F\nUSER++\x00suffix\n")
	formatted := []byte("@F\n    USER ++\x00suffix\n")
	if err := checkSourcePreserved(original, formatted); err != nil {
		t.Fatalf("NUL suffix rejected: %v", err)
	}
	embedded := []byte("@F\nUS\x00ER++\n")
	embeddedFormatted := []byte("@F\n    US\x00ER ++\n")
	if err := checkSourcePreserved(embedded, embeddedFormatted); err != nil {
		t.Fatalf("embedded NUL identifier rejected: %v", err)
	}
	quoted := []byte("@F\nUSER:\"x++\x00suffix\"++\n")
	quotedFormatted := []byte("@F\n    USER: \"x++\x00suffix\" ++\n")
	if err := checkSourcePreserved(quoted, quotedFormatted); err != nil {
		t.Fatalf("quoted index NUL was truncated: %v", err)
	}
	mutated := []byte("@F\n    USER ++\x00changed\n")
	if err := checkSourcePreserved(original, mutated); err == nil {
		t.Fatal("NUL suffix mutation was accepted")
	}
	withComment := []byte("@F\nUSER++\x00suffix; raw\n")
	withCommentFormatted := []byte("@F\n    USER ++\x00suffix; raw\n")
	if err := checkSourcePreserved(withComment, withCommentFormatted); err != nil {
		t.Fatalf("NUL suffix with comment rejected: %v", err)
	}
	if err := checkSourcePreserved(withComment, []byte("@F\n    USER ++\x00suffix;  raw\n")); err == nil {
		t.Fatal("postfix NUL suffix comment mutation was accepted")
	}
	prefixEmbedded := []byte("@F\n++US\x00ER; raw\n")
	prefixEmbeddedFormatted := []byte("@F\n    ++US\x00ER; raw\n")
	if err := checkSourcePreserved(prefixEmbedded, prefixEmbeddedFormatted); err != nil {
		t.Fatalf("prefix embedded NUL identifier rejected: %v", err)
	}
	if err := checkSourcePreserved(prefixEmbedded, []byte("@F\n    ++US \x00ER; raw\n")); err == nil {
		t.Fatal("prefix embedded NUL spacing mutation was accepted")
	}
	for _, pair := range [][2][]byte{
		{[]byte("@F\n++US \x00suffix; raw\n"), []byte("@F\n    ++ US \x00suffix; raw\n")},
		{[]byte("@F\n++US:0\x00suffix\n"), []byte("@F\n    ++ US:0 \x00suffix\n")},
		{[]byte("@F\n++US:(0)\x00suffix\n"), []byte("@F\n    ++ US:(0) \x00suffix\n")},
	} {
		if err := checkSourcePreserved(pair[0], pair[1]); err != nil {
			t.Fatalf("prefix NUL boundary rejected: %v", err)
		}
	}
	boundary := []byte("@F\n++US \x00suffix; raw\n")
	if err := checkSourcePreserved(boundary, []byte("@F\n    ++ US \x00suffix;  raw\n")); err == nil {
		t.Fatal("NUL suffix comment mutation was accepted")
	}
}
