package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/prepaser/erafmt/internal/format"
)

func TestCorpus(t *testing.T) {
	root := os.Getenv("ERAFMT_CORPUS")
	if root == "" {
		t.Skip("set ERAFMT_CORPUS to audit a local game collection without writes")
	}
	dialect, err := corpusDialect()
	if err != nil {
		t.Fatal(err)
	}
	inputEncoding, inputEncodingName, err := corpusInputEncoding()
	if err != nil {
		t.Fatal(err)
	}
	allowFullwidthSpace, err := corpusAllowFullwidthSpace()
	if err != nil {
		t.Fatal(err)
	}
	paths, errors := discoverTargets([]string{root})
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	if len(paths) == 0 {
		t.Fatal("corpus audit found no ERB or ERH target files")
	}
	output := os.Getenv("ERAFMT_CORPUS_REPORT")
	protected := paths
	if output != "" {
		protected, err = configuredCorpusTargets(protected)
		if err != nil {
			t.Fatal(err)
		}
		if err := rejectCorpusArtifactOverlap("corpus report", output, protected); err != nil {
			t.Fatal(err)
		}
	}
	var continuationSeparator *string
	if value, ok := os.LookupEnv("ERAFMT_CORPUS_CONTINUATION_SEPARATOR"); ok {
		if err := validateContinuationSeparator(value); err != nil {
			t.Fatal(err)
		}
		continuationSeparator = &value
	}
	report := struct {
		Root                                                      string
		RootBytes                                                 string `json:",omitempty"`
		Started, Finished, Dialect, InputEncoding, OutputEncoding string
		AllowFullwidthSpace                                       bool
		EffectiveOptions                                          struct {
			ContinuationSeparator *string
		}
		Totals                               corpusCounts
		Games                                []corpusGameReport
		ReadFailures                         []corpusReadFailure
		Diagnostics                          map[string]int
		DiagnosticCategories                 []corpusDiagnosticCategory
		DiagnosticCategoryOccurrencesOmitted int
		Corrections                          map[format.RuleID]int
		Samples                              []string
	}{Root: root, RootBytes: jsonPathBytes(root), Started: time.Now().UTC().Format(time.RFC3339), Dialect: reportDialect(dialect), InputEncoding: inputEncodingName, OutputEncoding: corpusOutputEncoding(inputEncoding), AllowFullwidthSpace: allowFullwidthSpace, EffectiveOptions: struct {
		ContinuationSeparator *string
	}{ContinuationSeparator: continuationSeparator}, Diagnostics: make(map[string]int), Corrections: make(map[format.RuleID]int)}
	gameCounts := make(map[string]*corpusCounts)
	diagnosticGroups := corpusDiagnosticAccumulator{}
	options := format.Options{Dialect: dialect, AllowFullwidthSpace: allowFullwidthSpace}
	preservationOptions := sourcePreservationOptions{dialect: dialect, allowFullwidthSpace: allowFullwidthSpace}
	if continuationSeparator != nil {
		options.ContinuationSeparator = continuationSeparator
		preservationOptions.continuationSeparator = continuationSeparator
	}
	for i, target := range paths {
		path := target.path
		rel, _ := filepath.Rel(root, path)
		game := strings.SplitN(rel, string(filepath.Separator), 2)[0]
		if gameCounts[game] == nil {
			gameCounts[game] = &corpusCounts{}
		}
		counts := gameCounts[game]
		counts.Files++
		original, err := os.ReadFile(path)
		if err != nil {
			counts.ReadErrors++
			report.ReadFailures = append(report.ReadFailures, corpusReadFailure{Path: path, PathBytes: jsonPathBytes(path), Error: err.Error()})
			continue
		}
		counts.Read++
		doc, err := decodeSourceWithEncoding(original, inputEncoding)
		if err != nil {
			counts.EncodingErrors++
			continue
		}
		counts.Decoded++
		verified := true
		kind := kindForPath(path)
		result := format.FormatWithOptions(doc.text, kind, options)
		out, diagnostics := result.Output, result.Diagnostics
		for _, change := range result.Changes {
			report.Corrections[change.Rule]++
		}
		fatal, partial := false, false
		for _, d := range diagnostics {
			fatal = fatal || d.Severity == format.Error
			partial = partial || d.Severity == format.Unsupported
			if d.Severity == format.Error && d.Code == "invalid_edits" {
				t.Errorf("%s:%d:%d invalid_edits: %s", path, d.Line, d.Column, d.Message)
				verified = false
			}
			report.Diagnostics[string(d.Severity)+"/"+d.Code]++
			diagnosticGroups.add(rel, d)
			if len(report.Samples) < 20 {
				report.Samples = append(report.Samples, fmt.Sprintf("%s:%d:%d %s/%s: %s", rel, d.Line, d.Column, d.Severity, d.Code, d.Message))
			}
		}
		if fatal {
			counts.SyntaxErrors++
			if !bytes.Equal(out, doc.text) || len(result.Changes) != 0 {
				t.Fatalf("fatal diagnostic changed %s", path)
			}
		} else if partial {
			counts.Partial++
		}
		if !fatal {
			if err := validateCorpusOutput(out); err != nil {
				t.Errorf("%s: %v", path, err)
				verified = false
			}
		}
		again := format.FormatWithOptions(out, kind, options)
		if err := validateCorpusReentry(result, again); err != nil {
			t.Errorf("%s: %v", path, err)
			verified = false
		}
		if err := checkSourcePreservedOptions(doc.text, out, preservationOptions); err != nil {
			t.Errorf("%s: %v", path, err)
			verified = false
		}
		encoded := original
		if !fatal {
			encoded, err = encodeCorpusOutput(doc, out, inputEncoding)
			if err != nil {
				t.Fatal(path, err)
			}
		}
		decodeProfile := inputEncodingAuto
		if fatal {
			decodeProfile = inputEncoding
		}
		redecoded, err := decodeSourceWithEncoding(encoded, decodeProfile)
		if err != nil || !bytes.Equal(redecoded.text, out) || ((fatal || inputEncoding == inputEncodingAuto) && (redecoded.encoding != doc.encoding || !bytes.Equal(redecoded.bom, doc.bom))) {
			t.Fatalf("encoding changed: %s", path)
		}
		if bytes.Equal(original, encoded) {
			counts.Unchanged++
		} else {
			counts.Changed++
		}
		if verified {
			counts.Verified++
		}
		if (i+1)%10000 == 0 {
			t.Logf("audited %d/%d files", i+1, len(paths))
		}
	}
	report.DiagnosticCategories, report.DiagnosticCategoryOccurrencesOmitted = diagnosticGroups.snapshot()
	gameNames := make([]string, 0, len(gameCounts))
	for game := range gameCounts {
		gameNames = append(gameNames, game)
	}
	sort.Slice(gameNames, func(i, j int) bool {
		return bytes.Compare([]byte(gameNames[i]), []byte(gameNames[j])) < 0
	})
	for _, game := range gameNames {
		c := gameCounts[game]
		report.Games = append(report.Games, corpusGameReport{Name: game, NameBytes: jsonPathBytes(game), Counts: *c})
		report.Totals.Files += c.Files
		report.Totals.Read += c.Read
		report.Totals.Decoded += c.Decoded
		report.Totals.Verified += c.Verified
		report.Totals.Changed += c.Changed
		report.Totals.Unchanged += c.Unchanged
		report.Totals.Partial += c.Partial
		report.Totals.SyntaxErrors += c.SyntaxErrors
		report.Totals.EncodingErrors += c.EncodingErrors
		report.Totals.ReadErrors += c.ReadErrors
	}
	report.Finished = time.Now().UTC().Format(time.RFC3339)
	if output != "" {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		protected, err = configuredCorpusTargets(protected)
		if err != nil {
			t.Fatal(err)
		}
		if err := rejectCorpusArtifactOverlap("corpus report", output, protected); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(output, append(data, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("totals: %+v", report.Totals)
	if len(report.ReadFailures) != 0 {
		failures := make([]string, 0, len(report.ReadFailures))
		for _, failure := range report.ReadFailures {
			failures = append(failures, fmt.Sprintf("%s: %s", failure.Path, failure.Error))
		}
		t.Fatalf("corpus audit encountered %d read errors: %s", len(failures), strings.Join(failures, "; "))
	}
}

func corpusOutputEncoding(input inputEncodingKind) string {
	if input == inputEncodingCP932 || input == inputEncodingCP932Fallback {
		return "utf8-bom"
	}
	return "preserve"
}

func encodeCorpusOutput(doc sourceDocument, text []byte, input inputEncodingKind) ([]byte, error) {
	if input == inputEncodingCP932 || input == inputEncodingCP932Fallback {
		return encodeOutput(doc, text, true)
	}
	return encodeSource(doc, text)
}

func validateCorpusOutput(output []byte) error {
	if bytes.Contains(output, []byte{'\r'}) {
		return fmt.Errorf("nonfatal output contains CR")
	}
	if len(output) != 0 && output[len(output)-1] != '\n' {
		return fmt.Errorf("nonfatal nonempty output lacks final LF")
	}
	return nil
}

func validateCorpusReentry(first, second format.Result) error {
	if !bytes.Equal(first.Output, second.Output) || len(second.Changes) != 0 {
		return fmt.Errorf("formatting is not idempotent")
	}
	categories := make(map[corpusDiagnosticCategoryKey]int)
	for _, diagnostic := range first.Diagnostics {
		categories[corpusDiagnosticCategoryKey{diagnostic.Severity, diagnostic.Code}]++
	}
	for _, diagnostic := range second.Diagnostics {
		categories[corpusDiagnosticCategoryKey{diagnostic.Severity, diagnostic.Code}]--
	}
	for category, count := range categories {
		if count != 0 {
			return fmt.Errorf("second pass changed diagnostic category %s/%s", category.Severity, category.Code)
		}
	}
	return nil
}

func TestCorpusRejectsEmptyTargetDirectory(t *testing.T) {
	root := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestCorpus$")
	cmd.Env = filteredTestEnv("ERAFMT_CORPUS", "ERAFMT_CORPUS_REPORT", "ERAFMT_PROFILE_RECEIPT", "ERAFMT_CORPUS_UTF8", "ERAFMT_CORPUS_DIALECT", "ERAFMT_CORPUS_INPUT_ENCODING", "ERAFMT_CORPUS_ALLOW_FULLWIDTH_SPACE", "ERAFMT_CORPUS_CONTINUATION_SEPARATOR")
	cmd.Env = append(cmd.Env, "ERAFMT_CORPUS="+root, "ERAFMT_CORPUS_INPUT_ENCODING=auto", "ERAFMT_CORPUS_DIALECT=common")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("empty corpus audit unexpectedly passed")
	}
	if !bytes.Contains(out, []byte("corpus audit found no ERB or ERH target files")) {
		t.Fatalf("empty corpus audit failed for an unexpected reason: %v\n%s", err, out)
	}
}

func TestCorpusReportsReadErrors(t *testing.T) {
	root := t.TempDir()
	blocked := filepath.Join(root, "blocked.ERB")
	if err := os.WriteFile(blocked, []byte("@F\nRETURN 1+2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(blocked, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := os.ReadFile(blocked); err == nil {
		t.Skip("current uid can read mode-000 files")
	}
	reportPath := filepath.Join(t.TempDir(), "report.json")
	cmd := exec.Command(os.Args[0], "-test.run=^TestCorpus$", "-test.v")
	cmd.Env = filteredTestEnv("ERAFMT_CORPUS", "ERAFMT_CORPUS_REPORT", "ERAFMT_CORPUS_DIALECT", "ERAFMT_CORPUS_INPUT_ENCODING", "ERAFMT_CORPUS_ALLOW_FULLWIDTH_SPACE", "ERAFMT_CORPUS_CONTINUATION_SEPARATOR")
	cmd.Env = append(cmd.Env, "ERAFMT_CORPUS="+root, "ERAFMT_CORPUS_REPORT="+reportPath, "ERAFMT_CORPUS_DIALECT=common", "ERAFMT_CORPUS_INPUT_ENCODING=auto")
	out, err := cmd.CombinedOutput()
	if err == nil || !bytes.Contains(out, []byte("corpus audit encountered 1 read errors")) || !bytes.Contains(out, []byte("blocked.ERB")) {
		t.Fatalf("read-error audit result: err=%v output=%s", err, out)
	}
	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Totals       corpusCounts
		ReadFailures []corpusReadFailure
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Totals.Files != 1 || got.Totals.Read != 0 || got.Totals.ReadErrors != 1 || len(got.ReadFailures) != 1 || !strings.Contains(got.ReadFailures[0].Path, "blocked.ERB") || got.ReadFailures[0].Error == "" {
		t.Fatalf("read-error report=%s parsed=%+v", data, got)
	}
}

func filteredTestEnv(keys ...string) []string {
	blocked := make(map[string]bool, len(keys))
	for _, key := range keys {
		blocked[key] = true
	}
	env := make([]string, 0, len(os.Environ()))
	for _, value := range os.Environ() {
		name, _, ok := strings.Cut(value, "=")
		if !ok || !blocked[name] {
			env = append(env, value)
		}
	}
	return env
}
