package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"

	"github.com/prepaser/erafmt/internal/format"
)

func TestInputProfileRepresentatives(t *testing.T) {
	root := os.Getenv("ERAFMT_CORPUS")
	if root == "" {
		t.Skip("set ERAFMT_CORPUS")
	}
	if os.Getenv("ERAFMT_PROFILE_RECEIPT") == "" {
		t.Skip("set ERAFMT_PROFILE_RECEIPT to audit representative input/output profiles")
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
	var continuationSeparator *string
	if value, ok := os.LookupEnv("ERAFMT_CORPUS_CONTINUATION_SEPARATOR"); ok {
		if err := validateContinuationSeparator(value); err != nil {
			t.Fatal(err)
		}
		continuationSeparator = &value
	}
	paths, errs := discoverTargets([]string{root})
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	output := os.Getenv("ERAFMT_PROFILE_RECEIPT")
	protected := paths
	if output != "" {
		protected, err = configuredCorpusTargets(protected)
		if err != nil {
			t.Fatal(err)
		}
		if err := rejectCorpusArtifactOverlap("profile receipt", output, protected); err != nil {
			t.Fatal(err)
		}
	}
	if len(paths) == 0 {
		writeProfileReceipt(t, []struct{}{}, protected)
		t.Fatal("profile audit found no ERB or ERH target files")
	}
	type sample struct {
		path            string
		data            []byte
		want            []byte
		diagnosticClass string
		changed         bool
	}
	type encodingErrorSample struct {
		path, category, decodeError string
		data                        []byte
	}
	type profileKey struct {
		encoding        encodingKind
		bom             bool
		status          string
		diagnosticClass string
		changed         bool
	}
	type receipt struct {
		Dialect, Input, Encoding, Status, DiagnosticClass, RelativePath, Output string
		RelativePathBytes                                                       string `json:",omitempty"`
		FirstDiagnosticClass, SecondDiagnosticClass                             string
		BOM                                                                     bool
		SelectNone                                                              bool
		AllowFullwidthSpace                                                     bool
		ContinuationSeparator                                                   *string
		Changed                                                                 bool
		FailureCategory                                                         string `json:",omitempty"`
		DecodeError                                                             string `json:",omitempty"`
		FirstCode, SecondCode                                                   int
		Stable                                                                  bool
	}
	var receipts []receipt
	encodingErrors := make(map[string]encodingErrorSample)
	for _, selectNone := range []bool{true, false} {
		options := format.Options{Dialect: dialect, AllowFullwidthSpace: allowFullwidthSpace, ContinuationSeparator: continuationSeparator}
		if selectNone {
			options.Select = []format.RuleID{}
		}
		samples := make(map[profileKey]sample)
		for _, target := range paths {
			data, err := os.ReadFile(target.path)
			if err != nil {
				t.Fatal(err)
			}
			doc, err := decodeSourceWithEncoding(data, inputEncoding)
			if err != nil {
				if selectNone {
					category := decoderFailureCategory(err)
					if category == "" {
						t.Fatalf("%s has unclassified decoder error: %v", target.path, err)
					}
					key := inputEncodingName + "/" + category
					if _, ok := encodingErrors[key]; !ok {
						encodingErrors[key] = encodingErrorSample{path: target.path, category: category, decodeError: err.Error(), data: data}
					}
				}
				continue
			}
			result := format.FormatWithOptions(doc.text, kindForPath(target.path), options)
			status, diagnosticClass := profileDiagnosticClass(result.Diagnostics)
			want := append([]byte(nil), data...)
			changed := false
			if status != "fatal" {
				encoded, encodeErr := encodeOutput(doc, result.Output, true)
				if encodeErr != nil {
					t.Fatalf("%s output encoding: %v", target.path, encodeErr)
				}
				want = encoded
				changed = !bytes.Equal(data, encoded)
			}
			key := profileKey{encoding: doc.encoding, bom: len(doc.bom) != 0, status: status, diagnosticClass: diagnosticClass, changed: changed}
			if _, ok := samples[key]; !ok {
				samples[key] = sample{path: target.path, data: data, want: want, diagnosticClass: diagnosticClass, changed: changed}
			}
		}
		keys := make([]profileKey, 0, len(samples))
		for key := range samples {
			keys = append(keys, key)
		}
		sort.Slice(keys, func(i, j int) bool {
			if keys[i].encoding != keys[j].encoding {
				return keys[i].encoding < keys[j].encoding
			}
			if keys[i].bom != keys[j].bom {
				return !keys[i].bom
			}
			if keys[i].status != keys[j].status {
				return keys[i].status < keys[j].status
			}
			if keys[i].diagnosticClass != keys[j].diagnosticClass {
				return keys[i].diagnosticClass < keys[j].diagnosticClass
			}
			return !keys[i].changed && keys[j].changed
		})
		for _, key := range keys {
			tc := samples[key]
			path := filepath.Join(t.TempDir(), filepath.Base(tc.path))
			if err := os.WriteFile(path, tc.data, 0o640); err != nil {
				t.Fatal(err)
			}
			args := []string{"--input-encoding=" + inputEncodingName, "--utf8", "--dialect=" + reportDialect(dialect), "--diagnostic-format=json"}
			if selectNone {
				args = append(args, "--select=none")
			}
			if continuationSeparator != nil {
				args = append(args, "--continuation-separator="+*continuationSeparator)
			}
			if allowFullwidthSpace {
				args = append(args, "--allow-fullwidth-space")
			}
			args = append(args, "-w", path)
			var out, diagnostics bytes.Buffer
			first := Run(args, bytes.NewReader(nil), &out, &diagnostics)
			firstStatus, firstClass := profileCLIStatus(t, diagnostics.Bytes())
			wantCode := 0
			if key.status != "complete" {
				wantCode = 2
			}
			if first != wantCode || firstStatus != key.status {
				t.Fatalf("%s/%t status=%s want=%s", encodingName(key.encoding), key.bom, firstStatus, key.status)
			}
			if firstClass != tc.diagnosticClass {
				t.Fatalf("%s/%t/%s diagnostic class=%s want=%s", encodingName(key.encoding), key.bom, key.status, firstClass, tc.diagnosticClass)
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, tc.want) {
				t.Fatalf("%s/%t/%s output bytes differ from the selected profile", encodingName(key.encoding), key.bom, key.status)
			}
			if key.status == "fatal" {
				if first != 2 {
					t.Fatalf("%s/%t/%s fatal output: code=%d output=%x diagnostics=%q", encodingName(key.encoding), key.bom, key.status, first, got, diagnostics.String())
				}
			} else {
				decoded, decodeErr := decodeSource(got)
				if decodeErr != nil || decoded.encoding != encodingUTF8 || !bytes.Equal(decoded.bom, []byte{0xef, 0xbb, 0xbf}) {
					t.Fatalf("%s/%t/%s output profile: decode=%v doc=%#v", encodingName(key.encoding), key.bom, key.status, decodeErr, decoded)
				}
			}
			if tc.changed != !bytes.Equal(got, tc.data) {
				t.Fatalf("%s/%t/%s changed=%t want=%t", encodingName(key.encoding), key.bom, key.status, !bytes.Equal(got, tc.data), tc.changed)
			}
			out.Reset()
			diagnostics.Reset()
			second := Run(args, bytes.NewReader(nil), &out, &diagnostics)
			secondStatus, secondClass := profileCLIStatus(t, diagnostics.Bytes())
			if secondStatus != firstStatus {
				t.Fatalf("%s/%t status changed: %s to %s", encodingName(key.encoding), key.bom, firstStatus, secondStatus)
			}
			if secondClass != firstClass {
				t.Fatalf("%s/%t/%s diagnostic class changed: %s to %s", encodingName(key.encoding), key.bom, key.status, firstClass, secondClass)
			}
			again, err := os.ReadFile(path)
			if second != first || err != nil || !bytes.Equal(got, again) {
				t.Fatalf("%s/%t/%s output is not stable: %v", encodingName(key.encoding), key.bom, key.status, err)
			}
			sourceAfter, err := os.ReadFile(tc.path)
			if err != nil || !bytes.Equal(sourceAfter, tc.data) {
				t.Fatalf("source corpus file changed: %s", tc.path)
			}
			output := "unchanged-fatal"
			if key.status != "fatal" {
				output = "utf8+bom"
			}
			relativePath, err := profileReceiptRelativePath(root, tc.path)
			if err != nil {
				t.Fatal(err)
			}
			receipts = append(receipts, receipt{
				Dialect: reportDialect(dialect), Input: inputEncodingName, Encoding: encodingName(key.encoding), Status: key.status, DiagnosticClass: tc.diagnosticClass,
				RelativePath: relativePath, RelativePathBytes: jsonPathBytes(relativePath), Output: output,
				FirstDiagnosticClass: firstClass, SecondDiagnosticClass: secondClass, BOM: key.bom, SelectNone: selectNone,
				AllowFullwidthSpace: allowFullwidthSpace, ContinuationSeparator: continuationSeparator,
				Changed: tc.changed, FirstCode: first, SecondCode: second, Stable: true,
			})
		}
	}
	errorKeys := make([]string, 0, len(encodingErrors))
	for key := range encodingErrors {
		errorKeys = append(errorKeys, key)
	}
	sort.Strings(errorKeys)
	for _, key := range errorKeys {
		tc := encodingErrors[key]
		path := filepath.Join(t.TempDir(), filepath.Base(tc.path))
		if err := os.WriteFile(path, tc.data, 0o640); err != nil {
			t.Fatal(err)
		}
		args := []string{"--input-encoding=" + inputEncodingName, "--utf8", "--dialect=" + reportDialect(dialect), "--diagnostic-format=json"}
		if continuationSeparator != nil {
			args = append(args, "--continuation-separator="+*continuationSeparator)
		}
		if allowFullwidthSpace {
			args = append(args, "--allow-fullwidth-space")
		}
		args = append(args, "-w", path)
		firstCode, firstEvent, firstBytes := runEncodingError(t, args, path, tc.data)
		secondCode, secondEvent, secondBytes := runEncodingError(t, args, path, tc.data)
		if firstCode != 2 || secondCode != 2 || firstEvent.Severity != "error" || firstEvent.Code != "io" || secondEvent.Severity != "error" || secondEvent.Code != "io" {
			t.Fatalf("%s encoding error: codes=%d,%d diagnostics=%#v,%#v", key, firstCode, secondCode, firstEvent, secondEvent)
		}
		sourceAfter, err := os.ReadFile(tc.path)
		if err != nil || !bytes.Equal(sourceAfter, tc.data) {
			t.Fatalf("source corpus file changed: %s", tc.path)
		}
		relativePath, err := profileReceiptRelativePath(root, tc.path)
		if err != nil {
			t.Fatal(err)
		}
		receipts = append(receipts, receipt{
			Dialect: reportDialect(dialect), Input: inputEncodingName, Encoding: "unknown", Status: "encoding-error", DiagnosticClass: "error",
			RelativePath: relativePath, RelativePathBytes: jsonPathBytes(relativePath), Output: "unchanged-encoding-error",
			FirstDiagnosticClass: "error", SecondDiagnosticClass: "error", BOM: inputHasBOM(tc.data), SelectNone: false,
			AllowFullwidthSpace: allowFullwidthSpace, ContinuationSeparator: continuationSeparator,
			Changed: false, FailureCategory: tc.category, DecodeError: tc.decodeError, FirstCode: firstCode, SecondCode: secondCode,
			Stable: bytes.Equal(firstBytes, secondBytes),
		})
	}
	writeProfileReceipt(t, receipts, protected)
	if len(receipts) == 0 {
		t.Fatal("profile audit produced no representatives")
	}
	t.Logf("profile representatives: %d", len(receipts))
}

func writeProfileReceipt(t *testing.T, receipts any, targets []discoveredTarget) {
	t.Helper()
	output := os.Getenv("ERAFMT_PROFILE_RECEIPT")
	if output == "" {
		return
	}
	data, err := json.MarshalIndent(receipts, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	protected, err := configuredCorpusTargets(targets)
	if err != nil {
		t.Fatal(err)
	}
	if err := rejectCorpusArtifactOverlap("profile receipt", output, protected); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(output, append(data, '\n'), 0o640); err != nil {
		t.Fatal(err)
	}
}

func profileReceiptRelativePath(root, path string) (string, error) {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return "", fmt.Errorf("relative path %q from %q: %w", path, root, err)
	}
	return filepath.ToSlash(relative), nil
}

func TestInputProfileRepresentativesIncludesEncodingErrors(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "valid.ERB"), []byte("@F\nRETURN 1\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "warning.ERB"), []byte("@F\nREPEAT 1-1\nREND\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "invalid.ERB"), []byte{0xff, 0xfe, 0xfd}, 0o640); err != nil {
		t.Fatal(err)
	}
	receiptPath := filepath.Join(t.TempDir(), "profile.json")
	t.Setenv("ERAFMT_CORPUS", root+string(filepath.Separator))
	t.Setenv("ERAFMT_PROFILE_RECEIPT", receiptPath)
	t.Setenv("ERAFMT_CORPUS_DIALECT", "common")
	t.Setenv("ERAFMT_CORPUS_INPUT_ENCODING", "auto")
	t.Setenv("ERAFMT_CORPUS_ALLOW_FULLWIDTH_SPACE", "false")
	t.Setenv("ERAFMT_CORPUS_CONTINUATION_SEPARATOR", " ")
	TestInputProfileRepresentatives(t)

	data, err := os.ReadFile(receiptPath)
	if err != nil {
		t.Fatal(err)
	}
	var receipts []struct {
		Status, RelativePath, FailureCategory, DecodeError, Output, DiagnosticClass string
		FirstDiagnosticClass, SecondDiagnosticClass                                 string
		FirstCode, SecondCode                                                       int
		Changed                                                                     bool
	}
	if err := json.Unmarshal(data, &receipts); err != nil {
		t.Fatal(err)
	}
	if len(receipts) != 4 {
		t.Fatalf("representative count=%d want 4: %s", len(receipts), data)
	}
	var found, warningFound bool
	for _, receipt := range receipts {
		if receipt.RelativePath == "warning.ERB" {
			warningFound = true
			if receipt.Status != "complete" || receipt.DiagnosticClass != "warning" || receipt.FirstDiagnosticClass != "warning" || receipt.SecondDiagnosticClass != "warning" || !receipt.Changed {
				t.Fatalf("warning representative=%+v", receipt)
			}
		}
		if receipt.Status != "encoding-error" {
			continue
		}
		found = true
		if receipt.RelativePath != "invalid.ERB" || receipt.FailureCategory == "" || receipt.DecodeError == "" || receipt.Output != "unchanged-encoding-error" || receipt.FirstCode != 2 || receipt.SecondCode != 2 || receipt.FirstDiagnosticClass != "error" || receipt.SecondDiagnosticClass != "error" {
			t.Fatalf("encoding-error receipt=%+v", receipt)
		}
	}
	if !found {
		t.Fatalf("encoding-error representative missing: %s", data)
	}
	if !warningFound {
		t.Fatalf("warning representative missing: %s", data)
	}
}

func TestInputProfileRepresentativesTracksReplacement(t *testing.T) {
	root := t.TempDir()
	clean := []byte{0xef, 0xbb, 0xbf, '@', 'F', '\n', ' ', ' ', ' ', ' ', 'R', 'E', 'T', 'U', 'R', 'N', ' ', '1', '\n'}
	if err := os.WriteFile(filepath.Join(root, "clean.ERB"), clean, 0o640); err != nil {
		t.Fatal(err)
	}
	dirty := []byte{0xef, 0xbb, 0xbf, '@', 'F', '\n', 'R', 'E', 'T', 'U', 'R', 'N', ' ', '1', '+', '2', '\n'}
	if err := os.WriteFile(filepath.Join(root, "dirty.ERB"), dirty, 0o640); err != nil {
		t.Fatal(err)
	}
	receiptPath := filepath.Join(t.TempDir(), "profile.json")
	t.Setenv("ERAFMT_CORPUS", root)
	t.Setenv("ERAFMT_PROFILE_RECEIPT", receiptPath)
	t.Setenv("ERAFMT_CORPUS_DIALECT", "common")
	t.Setenv("ERAFMT_CORPUS_INPUT_ENCODING", "auto")
	TestInputProfileRepresentatives(t)
	data, err := os.ReadFile(receiptPath)
	if err != nil {
		t.Fatal(err)
	}
	var receipts []struct {
		RelativePath string
		Changed      bool
	}
	if err := json.Unmarshal(data, &receipts); err != nil {
		t.Fatal(err)
	}
	var dirtyChanged, cleanUnchanged bool
	for _, receipt := range receipts {
		switch receipt.RelativePath {
		case "dirty.ERB":
			dirtyChanged = dirtyChanged || receipt.Changed
		case "clean.ERB":
			cleanUnchanged = cleanUnchanged || !receipt.Changed
		}
	}
	if !dirtyChanged || !cleanUnchanged {
		t.Fatalf("replacement coverage missing: %s", data)
	}
}

func TestInputProfileRepresentativesRejectsEmptyTargetDirectory(t *testing.T) {
	root := t.TempDir()
	receiptPath := filepath.Join(t.TempDir(), "profile.json")
	cmd := exec.Command(os.Args[0], "-test.run=^TestInputProfileRepresentatives$", "-test.v")
	cmd.Env = filteredTestEnv("ERAFMT_CORPUS", "ERAFMT_PROFILE_RECEIPT", "ERAFMT_CORPUS_DIALECT", "ERAFMT_CORPUS_INPUT_ENCODING", "ERAFMT_CORPUS_ALLOW_FULLWIDTH_SPACE", "ERAFMT_CORPUS_CONTINUATION_SEPARATOR")
	cmd.Env = append(cmd.Env, "ERAFMT_CORPUS="+root, "ERAFMT_PROFILE_RECEIPT="+receiptPath, "ERAFMT_CORPUS_DIALECT=common", "ERAFMT_CORPUS_INPUT_ENCODING=auto")
	out, err := cmd.CombinedOutput()
	if err == nil || !bytes.Contains(out, []byte("profile audit found no ERB or ERH target files")) {
		t.Fatalf("empty profile audit result: err=%v output=%s", err, out)
	}
	data, err := os.ReadFile(receiptPath)
	if err != nil || !bytes.Equal(bytes.TrimSpace(data), []byte("[]")) {
		t.Fatalf("empty profile receipt=%q err=%v", data, err)
	}
}

func profileDiagnosticClass(diagnostics []format.Diagnostic) (status, class string) {
	status, class = "complete", "none"
	for _, diagnostic := range diagnostics {
		var stop bool
		status, class, stop = profileDiagnosticState(status, class, string(diagnostic.Severity))
		if stop {
			return status, class
		}
	}
	return status, class
}

func profileCLIStatus(t *testing.T, data []byte) (status, class string) {
	t.Helper()
	status, class = "complete", "none"
	for _, line := range bytes.Split(bytes.TrimSpace(data), []byte{'\n'}) {
		if len(line) == 0 {
			continue
		}
		var event diagnosticEvent
		if err := json.Unmarshal(line, &event); err != nil {
			t.Fatalf("invalid CLI diagnostic JSON %q: %v", line, err)
		}
		if event.Type != "diagnostic" {
			continue
		}
		var stop bool
		status, class, stop = profileDiagnosticState(status, class, event.Severity)
		if stop {
			return status, class
		}
	}
	return status, class
}

func profileDiagnosticState(status, class, severity string) (string, string, bool) {
	switch severity {
	case string(format.Error):
		return "fatal", "error", true
	case string(format.Unsupported):
		return "partial", "unsupported", false
	case string(format.Warning):
		if class == "none" {
			class = "warning"
		}
	}
	return status, class, false
}

func encodingName(kind encodingKind) string {
	switch kind {
	case encodingUTF8:
		return "utf8"
	case encodingUTF16LE:
		return "utf16le"
	case encodingUTF16BE:
		return "utf16be"
	case encodingCP932:
		return "cp932"
	default:
		return "unknown"
	}
}

func inputHasBOM(data []byte) bool {
	return bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}) || bytes.HasPrefix(data, []byte{0xff, 0xfe}) || bytes.HasPrefix(data, []byte{0xfe, 0xff}) || bytes.HasPrefix(data, []byte{0x00, 0x00, 0xfe, 0xff})
}
