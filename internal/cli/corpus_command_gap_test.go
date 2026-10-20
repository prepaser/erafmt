package cli

import (
	"bytes"
	"testing"

	"github.com/prepaser/erafmt/internal/format"
)

func TestSourcePreservationAllowsFullwidthCommandGrammar(t *testing.T) {
	separator := " "
	for _, test := range []struct {
		name, statement string
		dialect         format.Dialect
	}{
		{"resetcolor", "RESETCOLOR　", format.CommonDialect},
		{"outputlog", "OUTPUTLOG　", format.CommonDialect},
		{"drawline", "DRAWLINE　", format.CommonDialect},
		{"clearbgimage", "CLEARBGIMAGE　", format.EMEE},
		{"trycallform", "TRYCALLFORM　TARGET(A)", format.CommonDialect},
		{"input", "INPUT　A", format.CommonDialect},
		{"binput", "BINPUT　A", format.EMEE},
		{"printv", "PRINTV　A", format.CommonDialect},
		{"printvn", "PRINTVN　A", format.EMEE},
		{"await", "AWAIT　1", format.EMEE},
		{"playsound", "PLAYSOUND　\"foo\"", format.EMEE},
		{"breakbutton", "BREAKBUTTON　1", format.EMEE},
	} {
		for _, continued := range []bool{false, true} {
			name := test.name
			source := []byte("@F\n" + test.statement + "\n")
			options := format.Options{Dialect: test.dialect, AllowFullwidthSpace: true}
			preservation := sourcePreservationOptions{dialect: test.dialect, allowFullwidthSpace: true}
			if continued {
				name += "/continuation"
				source = []byte("@F\n{\n" + test.statement + "\n}\n")
				options.ContinuationSeparator = &separator
				preservation.continuationSeparator = &separator
			}
			t.Run(name, func(t *testing.T) {
				result := format.FormatWithOptions(source, format.ERB, options)
				if len(result.Diagnostics) != 0 {
					t.Fatalf("diagnostics: %+v\noutput=%q", result.Diagnostics, result.Output)
				}
				if err := checkSourcePreservedOptions(source, result.Output, preservation); err != nil {
					t.Fatalf("preservation: %v\noutput=%q", err, result.Output)
				}
			})
		}
	}
}

func TestSourcePreservationAllowsPrintVCommentFormatting(t *testing.T) {
	separator := " "
	for _, continued := range []bool{false, true} {
		source := []byte("@F\nPRINTV A;note\n")
		options := format.Options{Dialect: format.CommonDialect}
		preservation := sourcePreservationOptions{dialect: format.CommonDialect}
		if continued {
			source = []byte("@F\n{\nPRINTV A;note\n}\n")
			options.ContinuationSeparator = &separator
			preservation.continuationSeparator = &separator
		}
		result := format.FormatWithOptions(source, format.ERB, options)
		if err := checkSourcePreservedOptions(source, result.Output, preservation); err != nil {
			t.Fatalf("continued=%v preservation: %v\noutput=%q", continued, err, result.Output)
		}
	}
}

func TestSourcePreservationRejectsPrintVPayloadMutations(t *testing.T) {
	separator := " "
	for _, test := range []struct {
		name, source, mutation, replacement string
		continued                           bool
	}{
		{"quoted payload", "@F\nPRINTV \"literal\";note\n", "literal", "changed", false},
		{"raw payload", "@F\nPRINTV raw literal;note\n", "raw literal", "changed", false},
		{"marked row", "@F\n;!; PRINTV A;note\n", ";!;", ";#;", false},
		{"continued payload", "@F\n{\nPRINTV \"literal\";note\n}\n", "literal", "changed", true},
		{"continued row", "@F\n{\nPRINTV A;note\nraw\n}\n", "raw\n", "changed", true},
		{"continued quoted fullwidth", "@F\n{\nPRINTV　\"literal　text\";note\n}\n", "literal　text", "literal text", true},
		{"continued raw fullwidth", "@F\n{\nPRINTV A\nhello　world\n}\n", "hello　world", "hello world", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			options := format.Options{Dialect: format.CommonDialect}
			preservation := sourcePreservationOptions{dialect: format.CommonDialect}
			if test.continued {
				options.ContinuationSeparator = &separator
				preservation.continuationSeparator = &separator
			}
			result := format.FormatWithOptions([]byte(test.source), format.ERB, options)
			mutated := bytes.Replace(result.Output, []byte(test.mutation), []byte(test.replacement), 1)
			if bytes.Equal(mutated, result.Output) {
				t.Fatalf("mutation %q was not applied to output %q", test.mutation, result.Output)
			}
			if err := checkSourcePreservedOptions([]byte(test.source), mutated, preservation); err == nil {
				t.Fatalf("accepted mutation: output=%q", mutated)
			}
		})
	}
}
