package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/prepaser/erafmt/internal/format"
)

type runtimeProbeResult struct {
	Accepted  bool   `json:"accepted"`
	Function  string `json:"function"`
	Argument  string `json:"argument"`
	Evidence  string `json:"evidence"`
	ErrorKind string `json:"errorKind"`
	Error     string `json:"error"`
}

type runtimeProbeCase struct {
	name     string
	src      string
	baseOK   bool
	modernOK bool
}

func TestRuntimeConformanceBase(t *testing.T) {
	root := strings.TrimSpace(os.Getenv("ERAFMT_EMUERA1824_ROOT"))
	if root == "" {
		t.Skip("set ERAFMT_EMUERA1824_ROOT to run the base runtime conformance probe")
	}
	if runtime.GOOS != "linux" && runtime.GOOS != "windows" {
		t.Fatalf("base runtime probe is unsupported on %s", runtime.GOOS)
	}
	assembly := buildRuntimeAssembly(t, root)
	checkRuntimeCases(t, assembly, format.Emuera1824, false)
}

func TestRuntimeConformanceModern(t *testing.T) {
	assembly := strings.TrimSpace(os.Getenv("ERAFMT_EMEE_ASSEMBLY"))
	if assembly == "" {
		t.Skip("set ERAFMT_EMEE_ASSEMBLY to run the modern runtime conformance probe")
	}
	if runtime.GOOS != "windows" {
		t.Fatalf("modern runtime probe requires Windows; current platform is %s", runtime.GOOS)
	}
	if _, err := os.Stat(assembly); err != nil {
		t.Fatalf("modern runtime assembly: %v", err)
	}
	checkRuntimeCases(t, assembly, format.EMEE, true)
}

func runtimeCases() []runtimeProbeCase {
	return []runtimeProbeCase{
		{name: "dynamic-operator", src: "RETURN RESULT + RESULT * 2", baseOK: true, modernOK: true},
		{name: "dynamic-index", src: "RETURN RESULT:(RESULT + 1)", baseOK: true, modernOK: true},
		{name: "dynamic-method", src: "RETURN ABS(RESULT + 1)", baseOK: true, modernOK: true},
		{name: "dynamic-form", src: "PRINTFORM raw {RESULT + RESULT}", baseOK: true, modernOK: true},
		{name: "call-escaped-space-comma", src: "TRYCALL F\\ ,1", baseOK: true, modernOK: true},
		{name: "call-escaped-space-arguments", src: "TRYCALL F\\ (1)", baseOK: true, modernOK: true},
		{name: "call-escaped-terminal-space", src: "TRYCALL F\\ ", baseOK: true, modernOK: true},
		{name: "call-escaped-comma", src: `TRYCALL F\,1`, baseOK: true, modernOK: true},
		{name: "call-escaped-parenthesis", src: `TRYCALL F\(1+2)`, baseOK: true, modernOK: true},
		{name: "call-escaped-semicolon", src: `TRYCALL F\;tail`, baseOK: true, modernOK: true},
		{name: "call-escaped-semicolon-arguments", src: `TRYCALL F\;tail(1+2)`, baseOK: true, modernOK: true},
		{name: "call-incomplete-escape", src: "TRYCALL F\\", baseOK: false, modernOK: false},
		{name: "call-escaped-nul", src: "TRYCALL F\\\x00(1+2)", baseOK: false, modernOK: false},
		{name: "form-call-escaped-space", src: "TRYCALLFORM F\\ (1)", baseOK: true, modernOK: true},
		{name: "dynamic-graphics", src: "PRINT_RECT RESULT + 1, RESULT, RESULT, RESULT", baseOK: true, modernOK: true},
		{name: "numeric-before-builtin", src: `RETURN 1A`, baseOK: false, modernOK: false},
		{name: "numeric-before-form", src: `RETURN 1@"x"`, baseOK: false, modernOK: false},
		{name: "graphics-adjacent-operands", src: `PRINT_SPACE 00A`, baseOK: false, modernOK: false},
		{name: "form-assignment-expression", src: `PRINTFORM {RESULT=1}`, baseOK: false, modernOK: false},
		{name: "call-adjacent-operands", src: `CALL F(RESULT 1)`, baseOK: false, modernOK: false},
		{name: "closed-call-parenthesis", src: `RETURN ABS(1)(2)`, baseOK: false, modernOK: false},
		{name: "closed-call-bracket", src: `RETURN ABS(1)[2]`, baseOK: false, modernOK: false},
		{name: "closed-call-index", src: `RETURN ABS(1):2`, baseOK: false, modernOK: false},
		{name: "closed-call-form", src: `RETURN ABS(1)@"x {2+3}"`, baseOK: false, modernOK: false},
		{name: "closed-call-increment", src: `RETURN ABS(1)++`, baseOK: false, modernOK: false},
		{name: "wrapped-closed-call-increment", src: `RETURN (ABS(1))++`, baseOK: false, modernOK: false},
		{name: "closed-call-operator-chain", src: `RETURN ABS(1)+?2#3`, baseOK: false, modernOK: false},
		{name: "closed-call-trailing-unary", src: `RETURN ABS(1)+!`, baseOK: false, modernOK: false},
		{name: "closed-call-colon-chain", src: `RETURN ABS(1)+:0`, baseOK: false, modernOK: false},
		{name: "closed-call-postfix-not", src: `RETURN ABS(1)!`, baseOK: false, modernOK: false},
		{name: "closed-call-missing-operand", src: `RETURN ABS(1)+`, baseOK: false, modernOK: false},
		{name: "compound-increment", src: `RETURN (RESULT+1)++`, baseOK: false, modernOK: false},
		{name: "parenthesized-variable-increment", src: `RETURN (RESULT)++`, baseOK: true, modernOK: true},
		{name: "qualified-global-variable", src: `RETURN RESULT@FUNC`, baseOK: false, modernOK: false},
		{name: "qualified-character-variable", src: `RETURN CFLAG@FUNC`, baseOK: false, modernOK: false},
		{name: "qualified-string-variable", src: `RETURNF NAME@FUNC`, baseOK: false, modernOK: false},
		{name: "getnum-reference", src: `GETNUM RESULT,"x"`, baseOK: true, modernOK: true},
		{name: "getnum-numeric-key", src: `GETNUM RESULT,1`, baseOK: false, modernOK: false},
		{name: "arraymsort-character-reference", src: `ARRAYMSORT RESULT,CFLAG`, baseOK: false, modernOK: false},
		{name: "string-indexed-postfix", src: `RETURN NAME:0++`, baseOK: false, modernOK: false},
		{name: "indexed-string-numeric-role", src: `SETCOLOR NAME:0*2`, baseOK: false, modernOK: false},
		{name: "savevar-fields", src: `SAVEVAR "file","message",RESULT,RESULTS`, baseOK: true, modernOK: true},
		{name: "savevar-literal-tail", src: `SAVEVAR "file","message",1`, baseOK: false, modernOK: false},
		{name: "string-setter", src: `VARSET RESULTS,"x"`, baseOK: true, modernOK: true},
		{name: "character-string-setter", src: `CVARSET CSTR,0,"x"`, baseOK: true, modernOK: true},
		{name: "times-unary-plus-variable", src: "TIMES +RESULT,1.25", baseOK: true, modernOK: true},
		{name: "setter-parenthesized-index", src: "VARSET RESULT:(0+1),1", baseOK: true, modernOK: true},
		{name: "setter-residual-expression", src: "VARSET RESULT:0+1,1", baseOK: false, modernOK: false},
		{name: "return-index-expression", src: "RETURN RESULT:0+1", baseOK: true, modernOK: true},
		{name: "invalid-comparison-operator", src: "RETURN RESULT<>1", baseOK: false, modernOK: false},
		{name: "missing-ternary-question", src: "RETURN RESULT#1", baseOK: false, modernOK: false},
		{name: "missing-ternary-colon", src: "RETURN RESULT?1", baseOK: false, modernOK: false},
		{name: "numeric-array-string-index", src: `RETURN DA:0:"x"`, baseOK: false, modernOK: false},
		{name: "character-string-target-index", src: `RETURN CFLAG:"x":0`, baseOK: false, modernOK: false},
		{name: "caret-comment", src: "RETURN 1+2;^; text", baseOK: true, modernOK: true},
		{name: "button-string-reply", src: `PRINTBUTTON "label","reply"`, baseOK: true, modernOK: true},
		{name: "button-numeric-label", src: "PRINTBUTTON 1,2", baseOK: false, modernOK: false},
		{name: "timed-string-input", src: `TINPUTS 1,"default",0,"timeout"`, baseOK: true, modernOK: true},
		{name: "timed-numeric-input-type", src: `TINPUT 1,"default"`, baseOK: false, modernOK: false},
		{name: "array-string-shift", src: `ARRAYSHIFT RESULTS,1,"x"`, baseOK: true, modernOK: true},
		{name: "array-shift-type", src: "ARRAYSHIFT RESULTS,1,2", baseOK: false, modernOK: false},
		{name: "split-readonly-array", src: `SPLIT "a,b",",",ITEMNAME`, baseOK: false, modernOK: false},
		{name: "split-readonly-count", src: `SPLIT "a,b",",",RESULTS,ITEMPRICE:0`, baseOK: false, modernOK: false},
		{name: "html-readonly-array", src: `HTML_TAGSPLIT "<b>",ITEMNAME`, baseOK: false, modernOK: false},
		{name: "savetext-file-index", src: `SAVETEXT "x",1`, baseOK: true, modernOK: true},
		{name: "savetext-file-path", src: `SAVETEXT "x","file"`, baseOK: false, modernOK: true},
		{name: "loadtext-file-index", src: "LOADTEXT 1", baseOK: true, modernOK: true},
		{name: "loadtext-file-path", src: `LOADTEXT "file"`, baseOK: false, modernOK: true},
		{name: "setter-arithmetic-destination", src: "VARSET RESULT+1,1", baseOK: false, modernOK: false},
		{name: "form-literal-alignment", src: "PRINTFORM {1,2,0}", baseOK: false, modernOK: false},
		{name: "unterminated-quoted-form", src: `RETURNF @"text{1+2}`, baseOK: false, modernOK: false},
		{name: "decimal-overflow", src: "RETURN 9223372036854775808", baseOK: false, modernOK: false},
		{name: "hexadecimal-reinterpretation", src: "RETURN 0xFFFFFFFFFFFFFFFF", baseOK: true, modernOK: true},
		{name: "rounded-exponent-boundary", src: "RETURN 1p63", baseOK: true, modernOK: true},
		{name: "nul-form-boundary", src: "PRINTFORM x\x00{1,,RIGHT}", baseOK: true, modernOK: true},
		{name: "print-space-integer", src: "PRINT_SPACE 1", baseOK: true, modernOK: true},
		{name: "print-space-string-method", src: `PRINT_SPACE STRLENS("abc")`, baseOK: true, modernOK: true},
		{name: "print-space-expression", src: "PRINT_SPACE 1+2", baseOK: true, modernOK: true},
		{name: "print-space-qualified-global", src: "PRINT_SPACE RESULT@FUNC", baseOK: false, modernOK: false},
		{name: "print-space-closed-call-increment", src: "PRINT_SPACE ABS(1)++", baseOK: false, modernOK: false},
		{name: "print-rect-integers", src: "PRINT_RECT 1,2,3,4", baseOK: true, modernOK: true},
		{name: "print-rect-string-method", src: `PRINT_RECT STRLENS("abc"),2,3,4`, baseOK: true, modernOK: true},
		{name: "times-real-coefficient", src: "TIMES RESULT, +01.2500e-02", baseOK: true, modernOK: true},
		{name: "times-warning-coefficient", src: "TIMES RESULT, invalid 1.20", baseOK: true, modernOK: true},
		{name: "outputlog-no-argument", src: "OUTPUTLOG", baseOK: true, modernOK: false},
		{name: "outputlog-method", src: "RESULT = OUTPUTLOG()", baseOK: false, modernOK: true},
		{name: "pixel-unit", src: "PRINT_SPACE 1px", baseOK: false, modernOK: true},
		{name: "incomplete-pixel-unit", src: "PRINT_SPACE 1p", baseOK: false, modernOK: false},
		{name: "incomplete-pixel-expression", src: "PRINT_SPACE 1+px", baseOK: false, modernOK: false},
		{name: "incomplete-expression", src: "PRINT_SPACE 1+", baseOK: false, modernOK: false},
		{name: "string-integer-position", src: `PRINT_SPACE "abc"`, baseOK: false, modernOK: false},
	}
}

func checkRuntimeCases(t *testing.T, assembly string, dialect format.Dialect, modern bool) {
	t.Helper()
	cases := runtimeCases()
	var original []string
	var formatted []string
	for _, tc := range cases {
		original = append(original, tc.src)
		result := format.FormatWithOptions([]byte(tc.src), format.ERB, format.Options{Dialect: dialect})
		for _, diagnostic := range result.Diagnostics {
			if diagnostic.Severity == format.Error {
				t.Fatalf("%s formatter error: %s", tc.name, diagnostic.Message)
			}
		}
		formatted = append(formatted, stripRuntimeLineEnding(string(result.Output)))
	}
	probeDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(probeDir, "csv"), 0o755); err != nil {
		t.Fatalf("create empty CSV directory: %v", err)
	}
	probe := compileRuntimeProbe(t, probeDir, modern)
	originalResults := runRuntimeProbe(t, probe, assembly, filepath.Join(probeDir, "csv"), original)
	formattedResults := runRuntimeProbe(t, probe, assembly, filepath.Join(probeDir, "csv"), formatted)
	if len(originalResults) != len(cases) || len(formattedResults) != len(cases) {
		t.Fatalf("runtime probe returned %d/%d results for %d cases", len(originalResults), len(formattedResults), len(cases))
	}
	for i, tc := range cases {
		before, after := originalResults[i], formattedResults[i]
		expected := tc.baseOK
		if modern {
			expected = tc.modernOK
		}
		if before.Accepted != expected {
			t.Errorf("%s runtime baseline mismatch: accepted=%v expected=%v result=%+v source=%q", tc.name, before.Accepted, expected, before, original[i])
		}
		if before.Accepted != after.Accepted || before.Function != after.Function || before.Argument != after.Argument || before.Evidence != after.Evidence || before.ErrorKind != after.ErrorKind {
			t.Errorf("%s changed runtime result: before=%+v after=%+v source=%q formatted=%q", tc.name, before, after, original[i], formatted[i])
		}
	}
	checkRuntimeEvidence(t, probe, assembly, filepath.Join(probeDir, "csv"))
}

func checkRuntimeEvidence(t *testing.T, probe runtimeProbeCommand, assembly, csvDir string) {
	t.Helper()
	controls := []struct{ name, before, after string }{
		{"operator", "RETURN RESULT+RESULT", "RETURN RESULT-RESULT"},
		{"index", "RETURN RESULT:0", "RETURN RESULT:1"},
		{"variable", "RETURN RESULT:0", "RETURN ITEMPRICE:0"},
		{"literal", "RETURN RESULT+1", "RETURN RESULT+2"},
		{"method", "RETURN ABS(RESULT)", "RETURN SIGN(RESULT)"},
		{"form-operator", "PRINTFORM raw {RESULT+RESULT}", "PRINTFORM raw {RESULT-RESULT}"},
		{"form-literal", "PRINTFORM raw {RESULT+RESULT}", "PRINTFORM other {RESULT+RESULT}"},
		{"unicode-surrogate", "RETURNF UNICODE(0xD800)", "RETURNF UNICODE(0xD801)"},
		{"graphics-operator", "PRINT_RECT RESULT+RESULT,RESULT,RESULT,RESULT", "PRINT_RECT RESULT-RESULT,RESULT,RESULT,RESULT"},
		{"times-index", "TIMES RESULT:0,1.25", "TIMES RESULT:1,1.25"},
		{"times-coefficient", "TIMES RESULT,1.25", "TIMES RESULT,1.5"},
		{"setter-start", "VARSET RESULT,1,0,2", "VARSET RESULT,1,1,2"},
		{"swap-index", "SWAP RESULT:0,RESULT:1", "SWAP RESULT:0,RESULT:2"},
	}
	var sources []string
	for _, control := range controls {
		sources = append(sources, control.before, control.after)
	}
	results := runRuntimeProbe(t, probe, assembly, csvDir, sources)
	if len(results) != len(sources) {
		t.Fatalf("runtime evidence probe returned %d results for %d sources", len(results), len(sources))
	}
	for i, control := range controls {
		t.Run("evidence/"+control.name, func(t *testing.T) {
			before, after := results[2*i], results[2*i+1]
			if !before.Accepted || !after.Accepted {
				t.Fatalf("negative control must parse on both sides: before=%+v after=%+v", before, after)
			}
			if before.Evidence == "" || after.Evidence == "" || before.Evidence == after.Evidence {
				t.Fatalf("runtime evidence missed a structural difference: before=%+v after=%+v", before, after)
			}
		})
	}
	for _, limit := range []struct{ name, source, message string }{
		{"depth", "RETURN " + strings.Repeat("RESULT+(", 32) + "RESULT" + strings.Repeat(")", 32), "traversal limit exceeded"},
		{"items", "RETURN " + strings.Repeat("1,", 64) + "1", "item limit exceeded"},
		{"characters", "PRINT " + strings.Repeat("x", 65536), "character limit exceeded"},
	} {
		t.Run("evidence-limit/"+limit.name, func(t *testing.T) {
			output, errors, err := runtimeProbeOutput(probe, assembly, csvDir, []string{limit.source})
			if err == nil || len(output) != 0 || !strings.Contains(string(errors), "runtime evidence "+limit.message) {
				t.Fatalf("expected explicit evidence limit failure: err=%v output=%q stderr=%s", err, output, errors)
			}
		})
	}
}

func stripRuntimeLineEnding(source string) string {
	if strings.HasSuffix(source, "\r\n") {
		return source[:len(source)-2]
	}
	if strings.HasSuffix(source, "\n") || strings.HasSuffix(source, "\r") {
		return source[:len(source)-1]
	}
	return source
}

func buildRuntimeAssembly(t *testing.T, root string) string {
	t.Helper()
	solution := runtimeSolution(root)
	if solution == "" {
		t.Fatalf("runtime source has no Emuera.sln or Emuera.csproj: %s", root)
	}
	copyRoot := filepath.Join(t.TempDir(), "runtime")
	if err := copyRuntimeTree(filepath.Dir(solution), copyRoot); err != nil {
		t.Fatalf("copy runtime source: %v", err)
	}
	localSolution := filepath.Join(copyRoot, filepath.Base(solution))
	tool := "xbuild"
	args := []string{"/nologo", "/verbosity:minimal", "/p:Configuration=Release", localSolution}
	if runtime.GOOS == "windows" {
		tool = "msbuild"
		args = []string{"/nologo", "/verbosity:minimal", "/p:Configuration=Release", localSolution}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, tool, args...)
	cmd.Dir = copyRoot
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		t.Fatalf("runtime build failed: %v\n%s", err, output.String())
	}
	assembly := findAssembly(copyRoot)
	if assembly == "" {
		t.Fatalf("runtime build produced no Emuera assembly\n%s", output.String())
	}
	return assembly
}

type runtimeProbeCommand struct {
	tool string
	args []string
}

func compileRuntimeProbe(t *testing.T, dir string, modern bool) runtimeProbeCommand {
	t.Helper()
	source := filepath.Join("testdata", "runtime_probe.cs")
	if modern {
		modernDir := filepath.Join(dir, "modern-source")
		if err := os.MkdirAll(modernDir, 0o755); err != nil {
			t.Fatalf("create modern probe source directory: %v", err)
		}
		for _, name := range []string{"runtime_probe.cs", "runtime_probe_modern.csproj"} {
			data, err := os.ReadFile(filepath.Join("testdata", name))
			if err != nil {
				t.Fatalf("read modern probe source %s: %v", name, err)
			}
			if err := os.WriteFile(filepath.Join(modernDir, name), data, 0o644); err != nil {
				t.Fatalf("copy modern probe source %s: %v", name, err)
			}
		}
		outputDir := filepath.Join(dir, "modern-output")
		cmd := exec.Command("dotnet", "build", filepath.Join(modernDir, "runtime_probe_modern.csproj"), "-c", "Release", "-o", outputDir)
		var output bytes.Buffer
		cmd.Stdout = &output
		cmd.Stderr = &output
		if err := cmd.Run(); err != nil {
			t.Fatalf("build modern runtime probe failed: %v\n%s", err, output.String())
		}
		probe := filepath.Join(outputDir, "runtime_probe.dll")
		if _, err := os.Stat(probe); err != nil {
			t.Fatalf("modern runtime probe output: %v\n%s", err, output.String())
		}
		return runtimeProbeCommand{tool: "dotnet", args: []string{probe}}
	}
	out := filepath.Join(dir, "runtime_probe.exe")
	tool := "mcs"
	args := []string{"-nologo", "-optimize+", "-out:" + out, source}
	if runtime.GOOS == "windows" {
		tool = "csc"
		args = []string{"/nologo", "/optimize+", "/out:" + out, source}
	}
	cmd := exec.Command(tool, args...)
	cmd.Dir = filepath.Dir(filepath.Dir(source))
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		t.Fatalf("compile runtime probe failed: %v\n%s", err, output.String())
	}
	if runtime.GOOS == "linux" {
		return runtimeProbeCommand{tool: "mono", args: []string{out}}
	}
	return runtimeProbeCommand{tool: out}
}

func runRuntimeProbe(t *testing.T, probe runtimeProbeCommand, assembly, csvDir string, sources []string) []runtimeProbeResult {
	t.Helper()
	output, errors, err := runtimeProbeOutput(probe, assembly, csvDir, sources)
	if err != nil {
		t.Fatalf("runtime probe failed: %v\n%s", err, errors)
	}
	var results []runtimeProbeResult
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		var result runtimeProbeResult
		if err := json.Unmarshal(scanner.Bytes(), &result); err != nil {
			t.Fatalf("invalid runtime probe JSON %q: %v", scanner.Text(), err)
		}
		results = append(results, result)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("read runtime probe output: %v", err)
	}
	return results
}

func runtimeProbeOutput(probe runtimeProbeCommand, assembly, csvDir string, sources []string) ([]byte, []byte, error) {
	args := append([]string{}, probe.args...)
	args = append(args, assembly, csvDir)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, probe.tool, args...)
	var input bytes.Buffer
	for _, source := range sources {
		input.WriteString(base64.StdEncoding.EncodeToString([]byte(source)))
		input.WriteByte('\n')
	}
	cmd.Stdin = &input
	var output, errors bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &errors
	err := cmd.Run()
	return output.Bytes(), errors.Bytes(), err
}

func runtimeSolution(root string) string {
	for _, candidate := range []string{
		filepath.Join(root, "Emuera.sln"),
		filepath.Join(root, "Emuera", "Emuera.sln"),
		filepath.Join(root, "Emuera", "Emuera.csproj"),
	} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

func findAssembly(root string) string {
	for _, relative := range []string{
		filepath.Join("Emuera", "bin", "Release", "Emuera.exe"),
		filepath.Join("bin", "Release", "Emuera.exe"),
		filepath.Join("Emuera", "bin", "Debug", "Emuera.exe"),
		filepath.Join("bin", "Debug", "Emuera.exe"),
		filepath.Join("out", "Emuera.exe"),
	} {
		candidate := filepath.Join(root, relative)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	var result string
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || result != "" {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if strings.EqualFold(entry.Name(), "Emuera.exe") {
			result = path
		}
		return nil
	})
	return result
}

func copyRuntimeTree(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if hasRuntimeBuildDirectory(relative) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

func hasRuntimeBuildDirectory(path string) bool {
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if part == ".git" || part == "bin" || part == "obj" {
			return true
		}
	}
	return false
}
