package cli

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/prepaser/erafmt/internal/format"
)

func TestCorpusUTF8Writes(t *testing.T) {
	root := os.Getenv("ERAFMT_CORPUS_UTF8")
	if root == "" {
		t.Skip("set ERAFMT_CORPUS_UTF8 to verify UTF-8 writes using temporary copies")
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
	type sampleKey struct {
		encoding        encodingKind
		bom             bool
		status          string
		diagnosticClass string
		changed         bool
		selectNone      bool
	}
	selected := make(map[sampleKey]int)
	for _, selectNone := range []bool{false, true} {
		options := format.Options{Dialect: dialect, AllowFullwidthSpace: allowFullwidthSpace, ContinuationSeparator: continuationSeparator}
		if selectNone {
			options.Select = []format.RuleID{}
		}
		for _, target := range paths {
			source := target.path
			original, err := os.ReadFile(source)
			if err != nil {
				t.Fatal(err)
			}
			doc, decodeErr := decodeSourceWithEncoding(original, inputEncoding)
			status, diagnosticClass := "fatal", "error"
			want, wantCode := original, 2
			changed := false
			if decodeErr == nil {
				r := format.FormatWithOptions(doc.text, kindForPath(source), options)
				status, diagnosticClass = profileDiagnosticClass(r.Diagnostics)
				fatal := status == "fatal"
				if !fatal {
					if err := checkSourcePreservedOptions(doc.text, r.Output, sourcePreservationOptions{dialect: dialect, continuationSeparator: continuationSeparator, allowFullwidthSpace: allowFullwidthSpace}); err != nil {
						t.Fatal(err)
					}
					want, err = encodeOutput(doc, r.Output, true)
					if err != nil {
						t.Fatal(err)
					}
					changed = !bytes.Equal(original, want)
					wantCode = 0
					if status == "partial" {
						wantCode = 2
					}
				}
			}
			key := sampleKey{encoding: doc.encoding, bom: len(doc.bom) != 0, status: status, diagnosticClass: diagnosticClass, changed: changed, selectNone: selectNone}
			if decodeErr != nil {
				key = sampleKey{status: "encoding-error", diagnosticClass: "error", selectNone: selectNone}
			}
			wantStatus, wantClass := status, diagnosticClass
			if selected[key] >= 2 {
				continue
			}
			selected[key]++
			t.Run(fmt.Sprintf("%d/bom=%t/%s/%s/changed=%t/select-none=%t/%s", key.encoding, key.bom, key.status, key.diagnosticClass, key.changed, selectNone, filepath.Base(source)), func(t *testing.T) {
				args := []string{"--utf8", "--diagnostic-format=json", "--dialect=" + reportDialect(dialect), "-w"}
				if inputEncoding != inputEncodingAuto {
					args = append(args, "--input-encoding="+inputEncodingName)
				}
				if continuationSeparator != nil {
					args = append(args, "--continuation-separator="+*continuationSeparator)
				}
				if allowFullwidthSpace {
					args = append(args, "--allow-fullwidth-space")
				}
				if selectNone {
					args = append(args, "--select=none")
				}
				path := filepath.Join(t.TempDir(), "copy"+filepath.Ext(source))
				if err := os.WriteFile(path, original, 0o640); err != nil {
					t.Fatal(err)
				}
				before, err := os.Stat(path)
				if err != nil {
					t.Fatal(err)
				}
				args = append(args, path)
				var firstGot []byte
				for pass := 0; pass < 2; pass++ {
					var out, diagnostics bytes.Buffer
					if code := Run(args, bytes.NewReader(nil), &out, &diagnostics); code != wantCode || out.Len() != 0 {
						t.Fatalf("selectNone=%t pass=%d code=%d want=%d diagnostics=%q", selectNone, pass, code, wantCode, diagnostics.String())
					}
					gotStatus, gotClass := profileCLIStatus(t, diagnostics.Bytes())
					if gotStatus != wantStatus || gotClass != wantClass {
						t.Fatalf("selectNone=%t pass=%d status=%s/%s want=%s/%s diagnostics=%q", selectNone, pass, gotStatus, gotClass, wantStatus, wantClass, diagnostics.String())
					}
					got, err := os.ReadFile(path)
					if err != nil || !bytes.Equal(got, want) {
						t.Fatalf("selectNone=%t pass=%d output changed or conversion failed: %v", selectNone, pass, err)
					}
					if pass == 0 {
						firstGot = append([]byte(nil), got...)
					} else if !bytes.Equal(got, firstGot) {
						t.Fatalf("selectNone=%t output changed between passes", selectNone)
					}
				}
				after, err := os.Stat(path)
				if err != nil || after.Mode() != before.Mode() {
					t.Fatalf("file mode changed: %v", err)
				}
				if changed != !bytes.Equal(firstGot, original) {
					t.Fatalf("changed=%t want=%t", !bytes.Equal(firstGot, original), changed)
				}
			})
			unchanged, err := os.ReadFile(source)
			if err != nil || !bytes.Equal(unchanged, original) {
				t.Fatalf("source sample changed: %v", err)
			}
		}
	}
	if len(selected) == 0 {
		t.Fatal("no source samples were verified")
	}
	t.Logf("representatives by encoding, diagnostic status, replacement, and selection: %v", selected)
}

func TestCorpusUTF8WritesIncludesWarningRepresentative(t *testing.T) {
	root := t.TempDir()
	clean := []byte{0xef, 0xbb, 0xbf, '@', 'F', '\n', ' ', ' ', ' ', ' ', 'R', 'E', 'T', 'U', 'R', 'N', ' ', '1', '\n'}
	if err := os.WriteFile(filepath.Join(root, "a-clean.ERB"), clean, 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "b-clean.ERB"), clean, 0o640); err != nil {
		t.Fatal(err)
	}
	dirty := []byte{0xef, 0xbb, 0xbf, '@', 'F', '\n', 'R', 'E', 'T', 'U', 'R', 'N', ' ', '1', '+', '2', '\n'}
	if err := os.WriteFile(filepath.Join(root, "z-dirty.ERB"), dirty, 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "warning.ERB"), []byte("@F\nREPEAT 1-1\nREND\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "partial.ERB"), []byte("@F\nFUTURECOMMAND A+B\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "fatal.ERB"), []byte("@F[1]\nRETURN 1\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "decode-error.ERB"), []byte{0xef, 0xbb, 0xbf, 0xc3, 0x28}, 0o640); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestCorpusUTF8Writes$", "-test.v")
	cmd.Env = filteredTestEnv("ERAFMT_CORPUS_UTF8", "ERAFMT_CORPUS_INPUT_ENCODING", "ERAFMT_CORPUS_DIALECT", "ERAFMT_CORPUS_ALLOW_FULLWIDTH_SPACE", "ERAFMT_CORPUS_CONTINUATION_SEPARATOR")
	cmd.Env = append(cmd.Env, "ERAFMT_CORPUS_UTF8="+root, "ERAFMT_CORPUS_INPUT_ENCODING=auto", "ERAFMT_CORPUS_DIALECT=common", "ERAFMT_CORPUS_ALLOW_FULLWIDTH_SPACE=false")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("UTF-8 representative child failed: %v\n%s", err, out)
	}
	if !bytes.Contains(out, []byte("warning.ERB")) {
		t.Fatalf("warning representative was not selected after two clean files:\n%s", out)
	}
	if !bytes.Contains(out, []byte("z-dirty.ERB")) {
		t.Fatalf("changed UTF-8+BOM representative was not selected:\n%s", out)
	}
}
