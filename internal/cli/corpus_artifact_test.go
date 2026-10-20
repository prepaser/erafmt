package cli

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func rejectCorpusArtifactOverlap(kind, output string, targets []discoveredTarget) error {
	if output == "" {
		return nil
	}

	outputInfo, outputErr := os.Stat(output)
	outputMissing := os.IsNotExist(outputErr)
	if outputErr != nil && !outputMissing {
		return fmt.Errorf("cannot inspect %s %q: %w", kind, output, outputErr)
	}
	if !outputMissing && !outputInfo.Mode().IsRegular() {
		return fmt.Errorf("cannot inspect %s %q: not a regular file", kind, output)
	}
	var outputState fileState
	var outputStateErr error
	if !outputMissing {
		outputState, outputStateErr = corpusArtifactState(output)
	}
	for _, target := range targets {
		sourceInfo, err := os.Stat(target.path)
		if err != nil {
			return fmt.Errorf("cannot inspect audited source %q for %s: %w", target.path, kind, err)
		}
		if !sourceInfo.Mode().IsRegular() {
			return fmt.Errorf("cannot inspect audited source %q for %s: not a regular file", target.path, kind)
		}
		if outputMissing {
			continue
		}
		if outputStateErr == nil && target.state.sameIdentity(outputState) {
			return fmt.Errorf("%s %q identifies audited source %q", kind, output, target.path)
		}
		if os.SameFile(outputInfo, sourceInfo) {
			return fmt.Errorf("%s %q identifies audited source %q", kind, output, target.path)
		}
	}
	return nil
}

func configuredCorpusTargets(initial []discoveredTarget) ([]discoveredTarget, error) {
	targets := append([]discoveredTarget(nil), initial...)
	seen := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		seen[target.path] = struct{}{}
	}
	for _, envName := range []string{"ERAFMT_CORPUS", "ERAFMT_CORPUS_UTF8"} {
		root := os.Getenv(envName)
		if root == "" {
			continue
		}
		found, errs := discoverTargets([]string{root})
		if len(errs) != 0 {
			return nil, fmt.Errorf("cannot inspect %s=%q: %v", envName, root, errs)
		}
		for _, target := range found {
			if _, ok := seen[target.path]; ok {
				continue
			}
			seen[target.path] = struct{}{}
			targets = append(targets, target)
		}
	}
	return targets, nil
}

func corpusArtifactState(path string) (fileState, error) {
	file, err := os.Open(path)
	if err != nil {
		return fileState{}, err
	}
	defer file.Close()
	return stateFromFile(file)
}

func TestCorpusArtifactsRejectSourceAliases(t *testing.T) {
	for _, test := range []struct {
		name      string
		testName  string
		envName   string
		makeAlias func(t *testing.T, source, dir string) string
	}{
		{name: "report-direct", testName: "TestCorpus", envName: "ERAFMT_CORPUS_REPORT", makeAlias: func(_ *testing.T, source, _ string) string { return source }},
		{name: "report-symlink", testName: "TestCorpus", envName: "ERAFMT_CORPUS_REPORT", makeAlias: makeCorpusSymlink},
		{name: "report-hardlink", testName: "TestCorpus", envName: "ERAFMT_CORPUS_REPORT", makeAlias: makeCorpusHardlink},
		{name: "receipt-direct", testName: "TestInputProfileRepresentatives", envName: "ERAFMT_PROFILE_RECEIPT", makeAlias: func(_ *testing.T, source, _ string) string { return source }},
		{name: "receipt-symlink", testName: "TestInputProfileRepresentatives", envName: "ERAFMT_PROFILE_RECEIPT", makeAlias: makeCorpusSymlink},
		{name: "receipt-hardlink", testName: "TestInputProfileRepresentatives", envName: "ERAFMT_PROFILE_RECEIPT", makeAlias: makeCorpusHardlink},
		{name: "encoding-receipt-direct", testName: "TestInputProfileEncodingErrorRepresentatives", envName: "ERAFMT_PROFILE_ENCODING_ERROR_RECEIPT", makeAlias: func(_ *testing.T, source, _ string) string { return source }},
		{name: "encoding-receipt-symlink", testName: "TestInputProfileEncodingErrorRepresentatives", envName: "ERAFMT_PROFILE_ENCODING_ERROR_RECEIPT", makeAlias: makeCorpusSymlink},
		{name: "encoding-receipt-hardlink", testName: "TestInputProfileEncodingErrorRepresentatives", envName: "ERAFMT_PROFILE_ENCODING_ERROR_RECEIPT", makeAlias: makeCorpusHardlink},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "source.ERB")
			original := []byte("@F\nRETURN 1+2\n")
			if err := os.WriteFile(source, original, 0o640); err != nil {
				t.Fatal(err)
			}
			artifact := test.makeAlias(t, source, t.TempDir())
			cmd := exec.Command(os.Args[0], "-test.run=^"+test.testName+"$", "-test.v")
			keys := []string{"ERAFMT_CORPUS", "ERAFMT_CORPUS_UTF8", "ERAFMT_CORPUS_REPORT", "ERAFMT_PROFILE_RECEIPT", "ERAFMT_PROFILE_ENCODING_ERROR_RECEIPT", "ERAFMT_CORPUS_DIALECT", "ERAFMT_CORPUS_INPUT_ENCODING", "ERAFMT_CORPUS_ALLOW_FULLWIDTH_SPACE", "ERAFMT_CORPUS_CONTINUATION_SEPARATOR"}
			cmd.Env = filteredTestEnv(keys...)
			cmd.Env = append(cmd.Env, "ERAFMT_CORPUS="+root, test.envName+"="+artifact, "ERAFMT_CORPUS_DIALECT=common", "ERAFMT_CORPUS_INPUT_ENCODING=auto")
			if test.testName == "TestInputProfileRepresentatives" {
				cmd.Env = append(cmd.Env, "ERAFMT_CORPUS_ALLOW_FULLWIDTH_SPACE=false")
			}
			out, err := cmd.CombinedOutput()
			if err == nil || !bytes.Contains(out, []byte("identifies audited source")) {
				t.Fatalf("alias was not rejected: err=%v output=%s", err, out)
			}
			got, err := os.ReadFile(source)
			if err != nil || !bytes.Equal(got, original) {
				t.Fatalf("source changed after rejected artifact: %v", err)
			}
		})
	}
}

func makeCorpusSymlink(t *testing.T, source, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "artifact.json")
	if err := os.Symlink(source, path); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("symlink creation unavailable: %v", err)
		}
		t.Fatal(err)
	}
	return path
}

func makeCorpusHardlink(t *testing.T, source, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "artifact.json")
	if err := os.Link(source, path); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("hardlink creation unavailable: %v", err)
		}
		t.Fatal(err)
	}
	return path
}

func TestCorpusArtifactsAllowDistinctSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation is not portable on Windows")
	}
	root := t.TempDir()
	source := filepath.Join(root, "source.ERB")
	if err := os.WriteFile(source, []byte("@F\nRETURN 1\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	targetDir := t.TempDir()
	target := filepath.Join(targetDir, "report.json")
	if err := os.WriteFile(target, []byte("old\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "report-link.json")
	if err := os.Symlink(target, output); err != nil {
		t.Fatal(err)
	}
	targets, errs := discoverTargets([]string{root})
	if len(errs) != 0 || len(targets) != 1 {
		t.Fatalf("targets=%#v errors=%v", targets, errs)
	}
	if err := rejectCorpusArtifactOverlap("corpus report", output, targets); err != nil {
		t.Fatalf("distinct symlink rejected: %v", err)
	}
	if err := os.WriteFile(output, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(target); err != nil || !bytes.Equal(got, []byte("{}\n")) {
		t.Fatalf("distinct symlink target=%q err=%v", got, err)
	}
}

func TestCorpusArtifactsAllowNewArtifactWithStableSources(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.ERB")
	if err := os.WriteFile(source, []byte("@F\nRETURN 1\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	targets, errs := discoverTargets([]string{root})
	if len(errs) != 0 || len(targets) != 1 {
		t.Fatalf("targets=%#v errors=%v", targets, errs)
	}
	output := filepath.Join(t.TempDir(), "report.json")
	if err := rejectCorpusArtifactOverlap("corpus report", output, targets); err != nil {
		t.Fatalf("new artifact rejected: %v", err)
	}
}

func TestCorpusArtifactsRejectRemovedSourceOutputs(t *testing.T) {
	for _, test := range []struct {
		name  string
		setup func(t *testing.T, source, dir string) string
	}{
		{name: "direct", setup: func(_ *testing.T, source, _ string) string { return source }},
		{name: "dangling-symlink", setup: func(t *testing.T, source, dir string) string {
			output := filepath.Join(dir, "report.json")
			if err := os.Symlink(source, output); err != nil {
				if runtime.GOOS == "windows" {
					t.Skipf("symlink creation unavailable: %v", err)
				}
				t.Fatal(err)
			}
			return output
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "source.ERB")
			if err := os.WriteFile(source, []byte("@F\nRETURN 1\n"), 0o640); err != nil {
				t.Fatal(err)
			}
			targets, errs := discoverTargets([]string{root})
			if len(errs) != 0 || len(targets) != 1 {
				t.Fatalf("targets=%#v errors=%v", targets, errs)
			}
			output := test.setup(t, source, t.TempDir())
			if err := os.Remove(source); err != nil {
				t.Fatal(err)
			}
			if err := rejectCorpusArtifactOverlap("corpus report", output, targets); err == nil {
				t.Fatal("removed source output was accepted")
			}
		})
	}
}

func TestCorpusArtifactsRejectConfiguredRootAliases(t *testing.T) {
	root := t.TempDir()
	utf8Root := t.TempDir()
	for _, dir := range []string{root, utf8Root} {
		if err := os.WriteFile(filepath.Join(dir, "source.ERB"), []byte("@F\nRETURN 1+2\n"), 0o640); err != nil {
			t.Fatal(err)
		}
	}
	source := filepath.Join(utf8Root, "source.ERB")
	for _, test := range []struct {
		name     string
		testName string
		envName  string
	}{
		{name: "report", testName: "TestCorpus", envName: "ERAFMT_CORPUS_REPORT"},
		{name: "profile", testName: "TestInputProfileRepresentatives", envName: "ERAFMT_PROFILE_RECEIPT"},
		{name: "encoding", testName: "TestInputProfileEncodingErrorRepresentatives", envName: "ERAFMT_PROFILE_ENCODING_ERROR_RECEIPT"},
	} {
		t.Run(test.name, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^"+test.testName+"$", "-test.v")
			keys := []string{"ERAFMT_CORPUS", "ERAFMT_CORPUS_UTF8", "ERAFMT_CORPUS_REPORT", "ERAFMT_PROFILE_RECEIPT", "ERAFMT_PROFILE_ENCODING_ERROR_RECEIPT", "ERAFMT_CORPUS_DIALECT", "ERAFMT_CORPUS_INPUT_ENCODING", "ERAFMT_CORPUS_ALLOW_FULLWIDTH_SPACE", "ERAFMT_CORPUS_CONTINUATION_SEPARATOR"}
			cmd.Env = filteredTestEnv(keys...)
			cmd.Env = append(cmd.Env, "ERAFMT_CORPUS="+root, "ERAFMT_CORPUS_UTF8="+utf8Root, test.envName+"="+source, "ERAFMT_CORPUS_DIALECT=common", "ERAFMT_CORPUS_INPUT_ENCODING=auto")
			if test.testName == "TestInputProfileRepresentatives" {
				cmd.Env = append(cmd.Env, "ERAFMT_CORPUS_ALLOW_FULLWIDTH_SPACE=false")
			}
			out, err := cmd.CombinedOutput()
			if err == nil || !bytes.Contains(out, []byte("identifies audited source")) {
				t.Fatalf("configured UTF-8 root alias was not rejected: err=%v output=%s", err, out)
			}
		})
	}
}

func TestCorpusArtifactsRecheckCurrentSourceIdentity(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("replacement identity test uses Unix rename semantics")
	}
	for _, test := range []struct {
		name      string
		makeAlias func(t *testing.T, source, dir string) string
	}{
		{name: "direct", makeAlias: func(_ *testing.T, source, _ string) string { return source }},
		{name: "symlink", makeAlias: makeCorpusSymlink},
		{name: "hardlink", makeAlias: makeCorpusHardlink},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "source.ERB")
			if err := os.WriteFile(source, []byte("old\n"), 0o640); err != nil {
				t.Fatal(err)
			}
			targets, errs := discoverTargets([]string{root})
			if len(errs) != 0 || len(targets) != 1 {
				t.Fatalf("targets=%#v errors=%v", targets, errs)
			}
			replacement := filepath.Join(t.TempDir(), "replacement.ERB")
			if err := os.WriteFile(replacement, []byte("new\n"), 0o640); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(replacement, source); err != nil {
				t.Fatal(err)
			}
			output := test.makeAlias(t, source, t.TempDir())
			if err := rejectCorpusArtifactOverlap("corpus report", output, targets); err == nil {
				t.Fatal("current source alias was accepted after replacement")
			}
		})
	}
}

func TestCorpusJSONPreservesInvalidGameAndPathBytes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows paths cannot contain invalid UTF-8 bytes")
	}
	root := filepath.Join(t.TempDir(), string([]byte{0xfd}), "corpus")
	if err := os.MkdirAll(root, 0o750); err != nil {
		t.Fatal(err)
	}
	gameNames := [][]byte{{'g', 0xff}, {'g', 0xfe}}
	for _, name := range gameNames {
		path := filepath.Join(root, string(name), "source.ERB")
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("@F\nFUTURECOMMAND A+B\n"), 0o640); err != nil {
			t.Fatal(err)
		}
	}
	reportPath := filepath.Join(t.TempDir(), "report.json")
	cmd := exec.Command(os.Args[0], "-test.run=^TestCorpus$", "-test.v")
	cmd.Env = filteredTestEnv("ERAFMT_CORPUS", "ERAFMT_CORPUS_REPORT", "ERAFMT_CORPUS_DIALECT", "ERAFMT_CORPUS_INPUT_ENCODING", "ERAFMT_CORPUS_ALLOW_FULLWIDTH_SPACE", "ERAFMT_CORPUS_CONTINUATION_SEPARATOR")
	cmd.Env = append(cmd.Env, "ERAFMT_CORPUS="+root, "ERAFMT_CORPUS_REPORT="+reportPath, "ERAFMT_CORPUS_DIALECT=common", "ERAFMT_CORPUS_INPUT_ENCODING=auto")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("corpus audit failed: %v\n%s", err, out)
	}
	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatal(err)
	}
	var report struct {
		Root, RootBytes string
		Totals          corpusCounts
		Games           []struct {
			Name, NameBytes string
			Counts          corpusCounts
		}
		DiagnosticCategories []struct {
			Representative struct{ Path, PathBytes string }
		}
	}
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Games) != len(gameNames) {
		t.Fatalf("games=%d want %d: %s", len(report.Games), len(gameNames), data)
	}
	rootBytes, err := base64.StdEncoding.DecodeString(report.RootBytes)
	if err != nil || !bytes.Equal(rootBytes, []byte(root)) {
		t.Fatalf("root identity=%q decode=%x err=%v want=%x", report.RootBytes, rootBytes, err, []byte(root))
	}
	seen := make(map[string]bool)
	totalFiles := 0
	for _, game := range report.Games {
		raw, err := base64.StdEncoding.DecodeString(game.NameBytes)
		if err != nil || len(raw) == 0 || !strings.Contains(game.Name, "\ufffd") {
			t.Fatalf("game identity=%+v decode=%v", game, err)
		}
		seen[string(raw)] = true
		totalFiles += game.Counts.Files
	}
	if totalFiles != report.Totals.Files {
		t.Fatalf("game files=%d totals=%d", totalFiles, report.Totals.Files)
	}
	for _, want := range gameNames {
		if !seen[string(want)] {
			t.Fatalf("missing game identity %x in %s", want, data)
		}
	}
	if len(report.DiagnosticCategories) == 0 {
		t.Fatalf("diagnostic representatives missing: %s", data)
	}
	pathBytes := report.DiagnosticCategories[0].Representative.PathBytes
	decodedPath, err := base64.StdEncoding.DecodeString(pathBytes)
	if err != nil || !bytes.Contains(decodedPath, []byte{0xff}) && !bytes.Contains(decodedPath, []byte{0xfe}) {
		t.Fatalf("diagnostic path identity=%q decode=%x err=%v", pathBytes, decodedPath, err)
	}
}

func TestProfileReceiptPreservesInvalidRelativePathBytes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows paths cannot contain invalid UTF-8 bytes")
	}
	root := t.TempDir()
	name := []byte{'g', 0xff}
	path := filepath.Join(root, string(name), "source.ERB")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("@F\nRETURN 1+2\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	receiptPath := filepath.Join(t.TempDir(), "receipt.json")
	cmd := exec.Command(os.Args[0], "-test.run=^TestInputProfileRepresentatives$", "-test.v")
	cmd.Env = filteredTestEnv("ERAFMT_CORPUS", "ERAFMT_PROFILE_RECEIPT", "ERAFMT_CORPUS_DIALECT", "ERAFMT_CORPUS_INPUT_ENCODING", "ERAFMT_CORPUS_ALLOW_FULLWIDTH_SPACE", "ERAFMT_CORPUS_CONTINUATION_SEPARATOR")
	cmd.Env = append(cmd.Env, "ERAFMT_CORPUS="+root+string(filepath.Separator), "ERAFMT_PROFILE_RECEIPT="+receiptPath, "ERAFMT_CORPUS_DIALECT=common", "ERAFMT_CORPUS_INPUT_ENCODING=auto")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("profile audit failed: %v\n%s", err, out)
	}
	data, err := os.ReadFile(receiptPath)
	if err != nil {
		t.Fatal(err)
	}
	var receipts []struct{ RelativePath, RelativePathBytes string }
	if err := json.Unmarshal(data, &receipts); err != nil {
		t.Fatal(err)
	}
	for _, receipt := range receipts {
		if receipt.RelativePathBytes == "" {
			continue
		}
		raw, err := base64.StdEncoding.DecodeString(receipt.RelativePathBytes)
		if err != nil || !bytes.Equal(raw, append(append([]byte{}, name...), []byte("/source.ERB")...)) {
			t.Fatalf("relative path identity=%+v decode=%x err=%v", receipt, raw, err)
		}
		return
	}
	t.Fatalf("relative path bytes missing: %s", data)
}
