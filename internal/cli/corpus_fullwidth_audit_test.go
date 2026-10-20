package cli

import (
	"bytes"
	"testing"

	"github.com/prepaser/erafmt/internal/format"
)

func TestSourcePreservationFullwidthCommandRoles(t *testing.T) {
	separator := ""
	cases := []struct {
		name, source string
		dialect      format.Dialect
		continued    bool
	}{
		{"form-call", "CALLFORM　TARGET(　A　+　B　)", format.CommonDialect, false},
		{"input", "INPUT　A　+　B", format.CommonDialect, false},
		{"binput", "BINPUT　A　+　B", format.EMEE, false},
		{"tinput", "TINPUT　1,　2,　3,　\"x\"", format.CommonDialect, false},
		{"oneinput", "ONEINPUT　A　+　B", format.CommonDialect, false},
		{"printv", "PRINTV　A　+　B;note　x", format.CommonDialect, false},
		{"printvn", "PRINTVN　A　+　B;note", format.EMEE, false},
		{"structured", "PLAYSOUND　\"foo\",　1", format.EMEE, false},
		{"tooltip", "TOOLTIP_SETFONTSIZE　1　+　2", format.EMEE, false},
		{"form-trailing-comment", "PRINTFORMS　\"x　{　A　+　B　}\"　;note", format.CommonDialect, false},
		{"form-any", "SETBGIMAGE　\"x　literal\",　{　A　+　B　}", format.EMEE, false},
		{"empty-continuation", "\nPRINTV　A　+　B", format.CommonDialect, true},
		{"empty-modern-continuation", "\nSETBGIMAGE　\"x\",　{　A　+　B　}", format.EMEE, true},
		{"spanning-printv", "PRINTV　A　+\nB", format.CommonDialect, true},
		{"spanning-input", "INPUT　A　+\nB", format.CommonDialect, true},
		{"spanning-form", "PRINTFORM　x　{　A　+\nB　}", format.CommonDialect, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := []byte("@F\n" + tc.source + "\n")
			options := format.Options{Dialect: tc.dialect, AllowFullwidthSpace: true}
			verification := sourcePreservationOptions{dialect: tc.dialect, allowFullwidthSpace: true}
			if tc.continued {
				source = []byte("@F\n{\n" + tc.source + "\n}\n")
				options.ContinuationSeparator = &separator
				verification.continuationSeparator = &separator
			}
			result := format.FormatWithOptions(source, format.ERB, options)
			for _, diagnostic := range result.Diagnostics {
				if diagnostic.Severity == format.Error {
					t.Fatalf("unexpected diagnostics=%v", result.Diagnostics)
				}
			}
			if err := checkSourcePreservedOptions(source, result.Output, verification); err != nil {
				t.Fatalf("preservation: %v\noutput=%q", err, result.Output)
			}
		})
	}
}

func TestSourcePreservationRejectsFullwidthRawPayloadMutation(t *testing.T) {
	separator := " "
	for _, command := range []string{"PRINT", "PRINTFORM"} {
		for _, continued := range []bool{false, true} {
			name := command + "/direct"
			source := []byte("@F\n" + command + "　　lead　raw\n")
			options := format.Options{AllowFullwidthSpace: true}
			verification := sourcePreservationOptions{allowFullwidthSpace: true}
			if continued {
				name = command + "/continued"
				source = []byte("@F\n{\n" + command + "　　lead　raw\n}\n")
				options.ContinuationSeparator = &separator
				verification.continuationSeparator = &separator
			}
			t.Run(name, func(t *testing.T) {
				result := format.FormatWithOptions(source, format.ERB, options)
				mutated := bytes.Replace(result.Output, []byte("　lead"), []byte(" lead"), 1)
				if bytes.Equal(mutated, result.Output) {
					t.Fatalf("fullwidth payload was not present: %q", result.Output)
				}
				if err := checkSourcePreservedOptions(source, mutated, verification); err == nil {
					t.Fatalf("accepted payload mutation: %q", mutated)
				}
			})
		}
	}
}

func TestSourcePreservationRawCommandFamilies(t *testing.T) {
	commands := []struct {
		name, command string
		dialect       format.Dialect
	}{
		{"print", "PRINT", format.CommonDialect},
		{"printl", "PRINTL", format.CommonDialect},
		{"printn", "PRINTN", format.EMEE},
		{"customdrawline", "CUSTOMDRAWLINE", format.CommonDialect},
		{"begin", "BEGIN", format.CommonDialect},
		{"callevent", "CALLEVENT", format.CommonDialect},
		{"alignment", "ALIGNMENT", format.CommonDialect},
		{"setcolorbyname", "SETCOLORBYNAME", format.CommonDialect},
		{"setbgcolorbyname", "SETBGCOLORBYNAME", format.CommonDialect},
		{"strlen", "STRLEN", format.CommonDialect},
		{"strlenu", "STRLENU", format.CommonDialect},
	}
	for _, tc := range commands {
		for _, continued := range []bool{false, true} {
			name := tc.name + "/direct"
			separator := " "
			source := []byte("@F\n" + tc.command + "　　lead　raw\n")
			options := format.Options{Dialect: tc.dialect, AllowFullwidthSpace: true}
			verification := sourcePreservationOptions{dialect: tc.dialect, allowFullwidthSpace: true}
			if continued {
				name = tc.name + "/continued"
				source = []byte("@F\n{\n" + tc.command + "　　lead　raw\n}\n")
				options.ContinuationSeparator = &separator
				verification.continuationSeparator = &separator
			}
			t.Run(name, func(t *testing.T) {
				result := format.FormatWithOptions(source, format.ERB, options)
				for _, diagnostic := range result.Diagnostics {
					if diagnostic.Severity == format.Error {
						t.Fatalf("unexpected diagnostics=%v", result.Diagnostics)
					}
				}
				if err := checkSourcePreservedOptions(source, result.Output, verification); err != nil {
					t.Fatalf("unmutated output rejected: %v output=%q", err, result.Output)
				}
				mutated := bytes.Replace(result.Output, []byte("　lead"), []byte(" lead"), 1)
				if bytes.Equal(mutated, result.Output) {
					t.Fatalf("payload marker absent from output=%q", result.Output)
				}
				if err := checkSourcePreservedOptions(source, mutated, verification); err == nil {
					t.Fatalf("accepted raw payload mutation: %q", mutated)
				}
			})
		}
	}
}

func TestSourcePreservationRejectsFullwidthLeadingMarkerMutation(t *testing.T) {
	for _, command := range []string{"PRINT", "PRINTFORM", "RETURNFORM", "THROW"} {
		source := []byte("@F\n" + command + "　　;!;　marker　payload\n")
		options := format.Options{AllowFullwidthSpace: true}
		verification := sourcePreservationOptions{allowFullwidthSpace: true}
		t.Run(command, func(t *testing.T) {
			result := format.FormatWithOptions(source, format.ERB, options)
			for _, diagnostic := range result.Diagnostics {
				if diagnostic.Severity == format.Error {
					t.Fatalf("unexpected diagnostics=%v", result.Diagnostics)
				}
			}
			if err := checkSourcePreservedOptions(source, result.Output, verification); err != nil {
				t.Fatalf("unmutated output rejected: %v output=%q", err, result.Output)
			}
			mutated := bytes.Replace(result.Output, []byte("　;!;"), []byte(" ;!;"), 1)
			if bytes.Equal(mutated, result.Output) {
				t.Fatalf("leading marker gap absent from output=%q", result.Output)
			}
			if err := checkSourcePreservedOptions(source, mutated, verification); err == nil {
				t.Fatalf("accepted leading marker payload mutation: %q", mutated)
			}
		})
	}
}

func TestSourcePreservationIgnoredFullwidthSuffixes(t *testing.T) {
	cases := []struct {
		name, source string
		dialect      format.Dialect
	}{
		{"input", "@F\nINPUT　A,　raw　payload\n", format.Emuera1824},
		{"tinput", "@F\nTINPUT　A,　B,　C,　D,　raw　payload\n", format.Emuera1824},
		{"playsound", "@F\nPLAYSOUND　\"foo\",　1,　raw　payload\n", format.EMEE},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			options := format.Options{Dialect: tc.dialect, AllowFullwidthSpace: true}
			verification := sourcePreservationOptions{dialect: tc.dialect, allowFullwidthSpace: true}
			source := []byte(tc.source)
			result := format.FormatWithOptions(source, format.ERB, options)
			for _, diagnostic := range result.Diagnostics {
				if diagnostic.Severity == format.Error {
					t.Fatalf("unexpected diagnostics=%v output=%q", result.Diagnostics, result.Output)
				}
			}
			if err := checkSourcePreservedOptions(source, result.Output, verification); err != nil {
				t.Fatalf("unmutated output rejected: %v output=%q", err, result.Output)
			}
			mutated := bytes.Replace(result.Output, []byte("　raw"), []byte(" raw"), 1)
			if bytes.Equal(mutated, result.Output) {
				t.Fatalf("ignored suffix absent from output=%q", result.Output)
			}
			if err := checkSourcePreservedOptions(source, mutated, verification); err == nil {
				t.Fatalf("accepted ignored suffix mutation: %q", mutated)
			}
		})
	}
}

func TestSourcePreservationIgnoredFullwidthSuffixesContinued(t *testing.T) {
	separator := " "
	cases := []struct {
		name, body string
		dialect    format.Dialect
	}{
		{"input", "INPUT　A,　raw　payload", format.Emuera1824},
		{"tinput", "TINPUT　A,　B,　C,　D,　raw　payload", format.Emuera1824},
		{"playsound", "PLAYSOUND　\"foo\",　1,　raw　payload", format.EMEE},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := []byte("@F\n{\n" + tc.body + "\n}\n")
			options := format.Options{Dialect: tc.dialect, AllowFullwidthSpace: true, ContinuationSeparator: &separator}
			verification := sourcePreservationOptions{dialect: tc.dialect, allowFullwidthSpace: true, continuationSeparator: &separator}
			result := format.FormatWithOptions(source, format.ERB, options)
			for _, diagnostic := range result.Diagnostics {
				if diagnostic.Severity == format.Error {
					t.Fatalf("unexpected diagnostics=%v output=%q", result.Diagnostics, result.Output)
				}
			}
			if err := checkSourcePreservedOptions(source, result.Output, verification); err != nil {
				t.Fatalf("unmutated output rejected: %v output=%q", err, result.Output)
			}
			mutated := bytes.Replace(result.Output, []byte("　raw"), []byte(" raw"), 1)
			if bytes.Equal(mutated, result.Output) {
				t.Fatalf("ignored suffix absent from output=%q", result.Output)
			}
			if checkSourcePreservedOptions(source, mutated, verification) == nil {
				t.Fatalf("accepted continued ignored suffix mutation: %q", mutated)
			}
		})
	}
}

func TestSourcePreservationBoundedCommandSuffixes(t *testing.T) {
	cases := []struct {
		name, command, args string
		dialect             format.Dialect
	}{
		{"inputs", "INPUTS", `"x",　A,　B,　ignored　suffix`, format.EMEE},
		{"binputs", "BINPUTS", `"x",　A,　B,　ignored　suffix`, format.EMEE},
		{"setcolor", "SETCOLOR", `A,　B,　C,　ignored　suffix`, format.CommonDialect},
		{"arraysort", "ARRAYSORT", `A,　FORWARD,　1,　2,　ignored　suffix`, format.CommonDialect},
		{"printbutton", "PRINTBUTTON", `A,　B,　ignored　suffix`, format.CommonDialect},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := []byte("@F\n" + tc.command + "　" + tc.args + "\n")
			options := format.Options{Dialect: tc.dialect, AllowFullwidthSpace: true}
			verification := sourcePreservationOptions{dialect: tc.dialect, allowFullwidthSpace: true}
			result := format.FormatWithOptions(source, format.ERB, options)
			if err := checkSourcePreservedOptions(source, result.Output, verification); err != nil {
				t.Fatalf("unmutated output rejected: %v\noutput=%q", err, result.Output)
			}
			mutated := bytes.Replace(result.Output, []byte("　ignored　suffix"), []byte(" changed suffix"), 1)
			if bytes.Equal(mutated, result.Output) {
				t.Fatalf("opaque suffix absent from output=%q", result.Output)
			}
			if err := checkSourcePreservedOptions(source, mutated, verification); err == nil {
				t.Fatalf("accepted opaque suffix mutation: %q", mutated)
			}
		})
	}
}

func TestSourcePreservationQuotedAndRawPayload(t *testing.T) {
	for _, tc := range []struct {
		name, source, from, to string
	}{
		{"quoted", "@F\nPRINTV　\"literal　text\"\n", "literal　text", "changed text"},
		{"raw", "@F\nPRINT　lead　raw\n", "lead", "changed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := []byte(tc.source)
			options := format.Options{Dialect: format.CommonDialect, AllowFullwidthSpace: true}
			verification := sourcePreservationOptions{dialect: format.CommonDialect, allowFullwidthSpace: true}
			result := format.FormatWithOptions(source, format.ERB, options)
			if err := checkSourcePreservedOptions(source, result.Output, verification); err != nil {
				t.Fatalf("unmutated output rejected: %v\noutput=%q", err, result.Output)
			}
			mutated := bytes.Replace(result.Output, []byte(tc.from), []byte(tc.to), 1)
			if bytes.Equal(mutated, result.Output) {
				t.Fatalf("payload absent from output=%q", result.Output)
			}
			if err := checkSourcePreservedOptions(source, mutated, verification); err == nil {
				t.Fatalf("accepted payload mutation: %q", mutated)
			}
		})
	}
}

func TestSourcePreservationCallFormOpaqueSuffix(t *testing.T) {
	source := []byte("@F\nCALLFORM　TARGET(　A　+　B　)　raw　suffix\n")
	options := format.Options{Dialect: format.CommonDialect, AllowFullwidthSpace: true}
	verification := sourcePreservationOptions{dialect: format.CommonDialect, allowFullwidthSpace: true}
	result := format.FormatWithOptions(source, format.ERB, options)
	if err := checkSourcePreservedOptions(source, result.Output, verification); err != nil {
		t.Fatalf("unmutated output rejected: %v\noutput=%q", err, result.Output)
	}
	for _, replacement := range []string{"changed suffix", "raw changed"} {
		mutated := bytes.Replace(result.Output, []byte("　raw　suffix"), []byte("　"+replacement), 1)
		if bytes.Equal(mutated, result.Output) {
			t.Fatalf("opaque suffix absent from output=%q", result.Output)
		}
		if err := checkSourcePreservedOptions(source, mutated, verification); err == nil {
			t.Fatalf("accepted CALLFORM suffix mutation %q: %q", replacement, mutated)
		}
	}
}

func TestSourcePreservationPrintVContinuationRows(t *testing.T) {
	separator := ""
	for _, tc := range []struct {
		name, body string
		dialect    format.Dialect
	}{
		{"printv", "PRINTV　A　+\nB　+　C", format.CommonDialect},
		{"printv-split-command", "PRINT\nV　A　+\nB　+　C", format.CommonDialect},
		{"printvn", "PRINTVN　A　+\nB　+　C", format.EMEE},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := []byte("@F\n{\n" + tc.body + "\n}\n")
			options := format.Options{Dialect: tc.dialect, AllowFullwidthSpace: true, ContinuationSeparator: &separator}
			verification := sourcePreservationOptions{dialect: tc.dialect, allowFullwidthSpace: true, continuationSeparator: &separator}
			result := format.FormatWithOptions(source, format.ERB, options)
			if err := checkSourcePreservedOptions(source, result.Output, verification); err != nil {
				t.Fatalf("unmutated output rejected: %v\noutput=%q", err, result.Output)
			}
			mutated := bytes.Replace(result.Output, []byte("B + C"), []byte("B + D"), 1)
			if bytes.Equal(mutated, result.Output) {
				t.Fatalf("later-row expression absent from output=%q", result.Output)
			}
			if err := checkSourcePreservedOptions(source, mutated, verification); err == nil {
				t.Fatalf("accepted later-row expression mutation: %q", mutated)
			}
		})
	}
}

func TestSourcePreservationPrintFormsExpressionFallback(t *testing.T) {
	for _, command := range []struct {
		name, command string
		dialect       format.Dialect
	}{
		{"printf", "PRINTFORMS", format.CommonDialect},
		{"printfsn", "PRINTFORMSN", format.EMEE},
	} {
		t.Run(command.name, func(t *testing.T) {
			source := []byte("@F\n" + command.command + "　\"x\"　+　CSTR\n")
			options := format.Options{Dialect: command.dialect, AllowFullwidthSpace: true}
			verification := sourcePreservationOptions{dialect: command.dialect, allowFullwidthSpace: true}
			result := format.FormatWithOptions(source, format.ERB, options)
			if err := checkSourcePreservedOptions(source, result.Output, verification); err != nil {
				t.Fatalf("unmutated output rejected: %v\noutput=%q", err, result.Output)
			}
			for _, tc := range []struct{ from, to string }{{"CSTR", "OTHER"}, {`"x"`, `"y"`}} {
				mutated := bytes.Replace(result.Output, []byte(tc.from), []byte(tc.to), 1)
				if bytes.Equal(mutated, result.Output) {
					t.Fatalf("PRINTFORMS field absent from output=%q", result.Output)
				}
				if err := checkSourcePreservedOptions(source, mutated, verification); err == nil {
					t.Fatalf("accepted PRINTFORMS mutation %q -> %q: %q", tc.from, tc.to, mutated)
				}
			}
		})
	}
}

func TestSourcePreservationQuotedFormCompletion(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		complete     bool
	}{
		{"complete", `@"literal　{　A　+　B　}"`, true},
		{"escaped-terminal-quote", `@"literal\"`, false},
		{"unclosed-island", `@"before %F("x")"`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := []byte(tc.source)
			end, complete := quotedFormEndComplete(source, 0)
			if complete != tc.complete {
				t.Fatalf("completion=%v want=%v end=%d", complete, tc.complete, end)
			}
			if !tc.complete {
				if got := normalizeFullwidthFormAtom(source); !bytes.Equal(got, source) {
					t.Fatalf("normalizer changed incomplete atom: %q -> %q", source, got)
				}
				if got := formAtomFingerprint(source); !bytes.Equal(got, source) {
					t.Fatalf("fingerprint changed incomplete atom: %q -> %q", source, got)
				}
				return
			}
			want := []byte(`@"literal　{ A + B }"`)
			if got := normalizeFullwidthFormAtom(source); !bytes.Equal(got, want) {
				t.Fatalf("complete atom normalization=%q want=%q", got, want)
			}
		})
	}
}
