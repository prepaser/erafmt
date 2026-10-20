package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/prepaser/erafmt/internal/format"
)

func TestPreservationOracleUnavailableCommandGaps(t *testing.T) {
	commands := []string{"PRINTN", "PRINTVN", "PRINTSN", "PRINTFORMN", "PRINTFORMSN", "CALLSHARP", "FORCE_QUIT", "SKIPLOG", "TRYCALLF"}
	for _, dialect := range []format.Dialect{format.CommonDialect, format.Emuera1824} {
		for _, fullwidth := range []bool{false, true} {
			for _, command := range commands {
				source := []byte("@F\n" + command + "  A#.\n")
				result := format.FormatWithOptions(source, format.ERB, format.Options{Dialect: dialect, AllowFullwidthSpace: fullwidth})
				if !bytes.Equal(result.Output, source) {
					t.Fatalf("unavailable command %s changed source=%q output=%q", command, source, result.Output)
				}
				options := sourcePreservationOptions{dialect: dialect, allowFullwidthSpace: fullwidth}
				if err := checkSourcePreservedOptions(source, result.Output, options); err != nil {
					t.Fatal(err)
				}
				changed := []byte(strings.Replace(string(source), command+"  ", command+" ", 1))
				if err := checkSourcePreservedOptions(source, changed, options); err == nil {
					t.Fatalf("dialect=%d fullwidth=%v accepted unavailable prefix %q", dialect, fullwidth, changed)
				}
			}
		}
	}
	for _, command := range []string{"PRINTVN", "PRINTSN", "PRINTFORMSN"} {
		source := []byte("@F\n" + command + "  0 0\n")
		options := format.Options{Dialect: format.EMEE, AllowFullwidthSpace: true}
		result := format.FormatWithOptions(source, format.ERB, options)
		if err := checkSourcePreservedOptions(source, result.Output, sourcePreservationOptions{dialect: options.Dialect, allowFullwidthSpace: true}); err != nil {
			t.Fatalf("available command %s output=%q: %v", command, result.Output, err)
		}
	}
}

func TestPreservationOracleUnavailableNameAssignments(t *testing.T) {
	for _, name := range []string{"PRINTN", "PRINTVN", "PRINTSN", "PRINTFORMN", "PRINTFORMSN"} {
		for _, dialect := range []format.Dialect{format.CommonDialect, format.Emuera1824} {
			for _, fullwidth := range []bool{false, true} {
				source := []byte("@F\n#DIM " + name + ",1\n" + name + "+=1+2\n")
				options := format.Options{Dialect: dialect, AllowFullwidthSpace: fullwidth}
				result := format.FormatWithOptions(source, format.ERB, options)
				if len(result.Diagnostics) != 0 {
					t.Fatalf("source=%q diagnostics=%+v", source, result.Diagnostics)
				}
				if err := checkSourcePreservedOptions(source, result.Output, sourcePreservationOptions{dialect: dialect, allowFullwidthSpace: fullwidth}); err != nil {
					t.Fatalf("source=%q output=%q: %v", source, result.Output, err)
				}
			}
		}
	}
}

func TestPreservationOraclePrintCommandOwnership(t *testing.T) {
	commands := []struct{ name, availableBody string }{
		{"PRINTN", "literal +=1+2"}, {"PRINTVN", "1+2"},
		{"PRINTSN", `"x"+"y"`}, {"PRINTFORMN", "literal {1+2}"},
		{"PRINTFORMSN", `"literal {1+2}"`},
	}
	separator := " "
	for _, dialect := range []format.Dialect{format.CommonDialect, format.Emuera1824, format.EMEE} {
		for _, fullwidth := range []bool{false, true} {
			for _, continued := range []bool{false, true} {
				options := format.Options{Dialect: dialect, AllowFullwidthSpace: fullwidth}
				proof := sourcePreservationOptions{dialect: dialect, allowFullwidthSpace: fullwidth}
				prefix, suffix := "@F\n", "\n"
				if continued {
					prefix, suffix = prefix+"{\n", suffix+"}\n"
					options.ContinuationSeparator = &separator
					proof.continuationSeparator = &separator
				}
				for _, command := range commands {
					body := command.name + " +=1+2"
					source := []byte(prefix + body + suffix)
					result := format.FormatWithOptions(source, format.ERB, options)
					if err := checkSourcePreservedOptions(source, result.Output, proof); err != nil {
						t.Fatalf("dialect=%d fullwidth=%v continued=%v source=%q output=%q: %v", dialect, fullwidth, continued, source, result.Output, err)
					}
					records := preservedRecordsOptions(source, proof)
					kind := records[len(records)-1].kind
					if continued {
						kind, _, _ = continuationLogicalFingerprint([]byte(body), dialect, fullwidth)
					}
					if kind == "assignment" {
						t.Fatalf("dialect=%d fullwidth=%v continued=%v command treated as assignment: %q", dialect, fullwidth, continued, body)
					}
					if dialect == format.EMEE {
						source = []byte(prefix + command.name + "  " + command.availableBody + suffix)
					} else {
						if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "unsupported_command" || !bytes.Contains(result.Output, []byte(body+"\n")) {
							t.Fatalf("source=%q output=%q diagnostics=%v", source, result.Output, result.Diagnostics)
						}
						for _, changed := range []string{
							strings.Replace(body, " ", "  ", 1),
							strings.Replace(body, "+=", "+= ", 1),
							strings.Replace(body, "1+2", "1 + 2", 1),
						} {
							if checkSourcePreservedOptions(source, []byte(prefix+changed+suffix), proof) == nil {
								t.Fatalf("dialect=%d fullwidth=%v continued=%v accepted unavailable mutation: %q", dialect, fullwidth, continued, changed)
							}
						}
						source = []byte(prefix + command.name + "+=1+2" + suffix)
					}
					result = format.FormatWithOptions(source, format.ERB, options)
					if len(result.Diagnostics) != 0 {
						t.Fatalf("valid control source=%q diagnostics=%v", source, result.Diagnostics)
					}
					if err := checkSourcePreservedOptions(source, result.Output, proof); err != nil {
						t.Fatalf("source=%q output=%q: %v", source, result.Output, err)
					}
				}
			}
		}
	}
}

func TestPreservationOracleContinuedFormLiteralCommandGap(t *testing.T) {
	separator := " "
	for _, command := range []struct {
		name    string
		dialect format.Dialect
	}{
		{"PRINTFORMS", format.CommonDialect}, {"STRFORM", format.CommonDialect},
		{"PRINTFORMSN", format.EMEE},
	} {
		for _, fullwidth := range []bool{false, true} {
			source := []byte("@F\n{\n" + command.name + `  "literal  {1+2}"` + "\n}\n")
			result := format.FormatWithOptions(source, format.ERB, format.Options{Dialect: command.dialect, AllowFullwidthSpace: fullwidth, ContinuationSeparator: &separator})
			if len(result.Diagnostics) != 0 {
				t.Fatalf("source=%q diagnostics=%v", source, result.Diagnostics)
			}
			proof := sourcePreservationOptions{dialect: command.dialect, allowFullwidthSpace: fullwidth, continuationSeparator: &separator}
			if err := checkSourcePreservedOptions(source, result.Output, proof); err != nil {
				t.Fatalf("source=%q output=%q: %v", source, result.Output, err)
			}
			changed := bytes.Replace(result.Output, []byte("literal  "), []byte("literal "), 1)
			if bytes.Equal(changed, result.Output) || checkSourcePreservedOptions(source, changed, proof) == nil {
				t.Fatalf("source=%q accepted literal mutation=%q", source, changed)
			}
		}
	}
}

func TestPreservationOracleAliasStatementOwnership(t *testing.T) {
	statements := []string{
		"RETURN  [[x]]+0", "PRINTV  [[x]]+0", "IF  [[x]]+0\nENDIF",
		"CALLFORM  F([[x]]+0)", `RETURN  "[[x]]"+UNKNOWN`,
		"PRINTV  'literal [[x]]", "PRINTFORM  literal [[x]] { 1+2 }",
		`PRINTFORMS  "literal [[x]] { 1+2 }"`, "#DIM AliasVar,[[x]]+0",
		"UNKNOWN=[[x]]+0", "RETURN  [[unterminated+0",
	}
	for _, statement := range statements {
		for _, fullwidth := range []bool{false, true} {
			source := []byte("@F\n" + statement + "\n")
			result := format.FormatWithOptions(source, format.ERB, format.Options{AllowFullwidthSpace: fullwidth})
			options := sourcePreservationOptions{allowFullwidthSpace: fullwidth}
			if err := checkSourcePreservedOptions(source, result.Output, options); err != nil {
				t.Fatalf("source=%q output=%q: %v", source, result.Output, err)
			}
			mutations := []string{strings.Replace(statement, "+0", " +0", 1), strings.Replace(statement, "  ", " ", 1), strings.Replace(statement, "[[x]]", "[[y]]", 1)}
			for _, mutation := range mutations {
				if mutation == statement {
					continue
				}
				changed := []byte("@F\n" + mutation + "\n")
				if err := checkSourcePreservedOptions(source, changed, options); err == nil {
					t.Fatalf("source=%q accepted alias mutation=%q", source, changed)
				}
			}
		}
	}
}

func TestPreservationOracleRawPrintVFieldProof(t *testing.T) {
	for _, command := range []string{"PRINTV", "PRINTVN"} {
		for _, fullwidth := range []bool{false, true} {
			options := format.Options{Dialect: format.EMEE, AllowFullwidthSpace: fullwidth}
			preservation := sourcePreservationOptions{dialect: options.Dialect, allowFullwidthSpace: fullwidth}
			for _, body := range []string{
				"'raw',A#.", "A#.,'raw", "'raw',A+.", "A+.,'raw",
				"'literal (; ,A#.", "'raw\\, literal,A#.", "'raw',A#.\x00suffix",
			} {
				source := []byte("@F\n" + command + " " + body + "\n")
				result := format.FormatWithOptions(source, format.ERB, options)
				if len(result.Diagnostics) == 0 || !bytes.Contains(result.Output, []byte(command+" "+body)) {
					t.Fatalf("body=%q output=%q diagnostics=%v", body, result.Output, result.Diagnostics)
				}
				if err := checkSourcePreservedOptions(source, result.Output, preservation); err != nil {
					t.Fatal(err)
				}
				changed := bytes.Replace(source, []byte(","), []byte(", "), 1)
				if err := checkSourcePreservedOptions(source, changed, preservation); err == nil {
					t.Fatalf("body=%q accepted mutation=%q", body, changed)
				}
			}
			for _, body := range []string{
				"'raw',A+B", "'raw　(; ,A+B", "UNKNOWN,'raw',A#.", "'raw',UNKNOWN,A#.",
				"'first\\, literal  ,F('nested; raw  ,A+B),C+D", "ABS('raw,A#.)", "'raw\x00,A#.", "A+B,'raw\x00,A+B",
			} {
				source := []byte("@F\n" + command + " " + body + "\n")
				result := format.FormatWithOptions(source, format.ERB, options)
				if len(result.Diagnostics) != 0 {
					t.Fatalf("body=%q diagnostics=%v", body, result.Diagnostics)
				}
				if err := checkSourcePreservedOptions(source, result.Output, preservation); err != nil {
					t.Fatalf("body=%q output=%q: %v", body, result.Output, err)
				}
				if bytes.IndexByte(source, 0) >= 0 {
					changed := bytes.Replace(source, []byte("\x00,"), []byte("\x00, "), 1)
					if checkSourcePreservedOptions(source, changed, preservation) == nil {
						t.Fatalf("body=%q accepted ignored suffix mutation", body)
					}
				}
				changed := bytes.Replace(source, []byte("raw"), []byte("RAW"), 1)
				if !bytes.Equal(source, changed) && checkSourcePreservedOptions(source, changed, preservation) == nil {
					t.Fatalf("body=%q accepted raw payload mutation", body)
				}
			}
		}
	}
}
