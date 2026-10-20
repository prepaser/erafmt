package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/prepaser/erafmt/internal/format"
)

type runner struct {
	out           io.Writer
	reporter      diagnosticReporter
	code          int
	utf8          bool
	pathForm      pathOutputFormat
	inputEncoding inputEncodingKind
}

type pathOutputFormat uint8

const (
	pathOutputText pathOutputFormat = iota
	pathOutputNUL
	pathOutputJSON
)

type optionFlag struct {
	value string
	set   bool
}

func (f *optionFlag) String() string { return f.value }

func (f *optionFlag) Set(value string) error {
	f.value = value
	f.set = true
	return nil
}

func usageFailure(reporter *diagnosticReporter, message string) int {
	_ = reporter.usage(message)
	return 2
}

type fileResult struct {
	original    []byte
	output      []byte
	state       fileState
	unsupported bool
	changes     []format.Change
}

// Run executes the command-line interface and returns the process exit code.
func Run(args []string, in io.Reader, out, errOut io.Writer) int {
	flags := flag.NewFlagSet("erafmt", flag.ContinueOnError)
	var flagOutput bytes.Buffer
	flags.SetOutput(&flagOutput)
	list := flags.Bool("l", false, "list files that would change")
	write := flags.Bool("w", false, "write formatted files")
	check := flags.Bool("check", false, "check whether files are formatted")
	utf8Output := flags.Bool("utf8", false, "output UTF-8 with a BOM")
	inputEncodingFlag := &optionFlag{value: "auto"}
	flags.Var(inputEncodingFlag, "input-encoding", "input encoding (auto, cp932, or cp932-fallback)")
	allowFullwidthSpace := flags.Bool("allow-fullwidth-space", false, "treat fullwidth spaces as grammar whitespace")
	rules := flags.Bool("rules", false, "list formatting rules")
	selectRules := &optionFlag{}
	ignoreRules := &optionFlag{}
	flags.Var(selectRules, "select", "select all, none, or comma-separated rule IDs")
	flags.Var(ignoreRules, "ignore", "exclude comma-separated rule IDs")
	explain := flags.Bool("explain", false, "explain formatting changes")
	eolStyle := &optionFlag{}
	flags.Var(eolStyle, "eol", "line ending style (lf or crlf)")
	dialect := &optionFlag{value: "common"}
	flags.Var(dialect, "dialect", "syntax dialect (common, emuera1824, or emee)")
	stdinKind := &optionFlag{value: "erb"}
	flags.Var(stdinKind, "stdin-kind", "stdin source kind (erb or erh)")
	continuationSeparator := &optionFlag{}
	flags.Var(continuationSeparator, "continuation-separator", "opt into continuation analysis with this literal separator")
	diagnosticStyle := &optionFlag{value: "text"}
	flags.Var(diagnosticStyle, "diagnostic-format", "diagnostic output format (text or json)")
	pathStyle := &optionFlag{value: "text"}
	flags.Var(pathStyle, "path-format", "listed path format (text, nul, or json)")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			if _, writeErr := io.Copy(errOut, &flagOutput); writeErr != nil {
				return 2
			}
			return 0
		}
		style, styleErr := parseDiagnosticFormat(diagnosticStyle.value)
		if styleErr != nil {
			style = diagnosticText
		}
		reporter := diagnosticReporter{out: errOut, format: style}
		if reportErr := reporter.rawFlagError(flagOutput.String()); reportErr != nil {
			return 2
		}
		return 2
	}
	diagnosticStyleValue, err := parseDiagnosticFormat(diagnosticStyle.value)
	if err != nil {
		reporter := diagnosticReporter{out: errOut, format: diagnosticText}
		return usageFailure(&reporter, err.Error())
	}
	reporter := diagnosticReporter{out: errOut, format: diagnosticStyleValue}
	pathOutputStyle, err := parsePathOutputFormat(pathStyle.value)
	if err != nil {
		return usageFailure(&reporter, err.Error())
	}
	inputEncodingValue, err := parseInputEncoding(inputEncodingFlag.value)
	if err != nil {
		return usageFailure(&reporter, err.Error())
	}
	if (inputEncodingValue == inputEncodingCP932 || inputEncodingValue == inputEncodingCP932Fallback) && !*utf8Output {
		return usageFailure(&reporter, "--input-encoding=cp932 or cp932-fallback requires --utf8")
	}
	if continuationSeparator.set {
		if err := validateContinuationSeparator(continuationSeparator.value); err != nil {
			return usageFailure(&reporter, err.Error())
		}
	}
	paths := flags.Args()
	if *rules {
		incompatible := len(paths) != 0
		flags.Visit(func(f *flag.Flag) {
			if f.Name != "rules" && f.Name != "diagnostic-format" {
				incompatible = true
			}
		})
		if incompatible {
			return usageFailure(&reporter, "--rules cannot be combined with input or formatting options")
		}
		if err := writeRuleCatalog(out, format.Rules()); err != nil {
			_ = reporter.error("<stdout>", err)
			return 2
		}
		return 0
	}
	dialectValue, err := parseDialect(dialect.value)
	if err != nil {
		return usageFailure(&reporter, err.Error())
	}
	ruleOptions, err := parseRuleOptions(selectRules, ignoreRules)
	if err != nil {
		return usageFailure(&reporter, err.Error())
	}
	if eolStyle.set {
		eol, err := parseEOL(eolStyle.value)
		if err != nil {
			return usageFailure(&reporter, err.Error())
		}
		ruleOptions.EOL = eol
	}
	ruleOptions.Dialect = dialectValue
	ruleOptions.AllowFullwidthSpace = *allowFullwidthSpace
	if continuationSeparator.set {
		separator := continuationSeparator.value
		ruleOptions.ContinuationSeparator = &separator
	}
	if *check && (*list || *write) {
		return usageFailure(&reporter, "--check cannot be combined with -l or -w")
	}
	if pathStyle.set && !*list && !*check {
		return usageFailure(&reporter, "--path-format requires -l or --check")
	}
	if !strings.EqualFold(stdinKind.value, "erb") && !strings.EqualFold(stdinKind.value, "erh") {
		return usageFailure(&reporter, "--stdin-kind must be erb or erh")
	}
	if len(paths) == 0 {
		if pathStyle.set {
			return usageFailure(&reporter, "--path-format requires file paths")
		}
		if *list || *write {
			return usageFailure(&reporter, "stdin cannot be used with -l or -w")
		}
		return runStdin(in, out, reporter, parseKind(stdinKind.value), *check, *explain, *utf8Output, inputEncodingValue, ruleOptions)
	}
	if stdinKind.set {
		return usageFailure(&reporter, "--stdin-kind is only valid when reading stdin")
	}
	discovery := discoverTargetsWithMetadata(paths)
	targets, discoveryErrors := discovery.targets, discovery.errors
	r := runner{out: out, reporter: reporter, utf8: *utf8Output, pathForm: pathOutputStyle, inputEncoding: inputEncodingValue}
	for _, err := range discoveryErrors {
		if discovery, ok := err.(discoveryError); ok {
			r.reportError(discovery.path, discovery.err)
		} else {
			r.reportError("", err)
		}
	}
	if len(paths) > 1 || discovery.containsDir {
		if !*list && !*write && !*check {
			return usageFailure(&reporter, "multiple files and directories require -l, -w, or --check")
		}
	}
	if len(targets) == 0 {
		if len(discoveryErrors) != 0 {
			return 2
		}
		return 0
	}
	if pathOutputStyle == pathOutputJSON {
		invalidPath := false
		for _, target := range targets {
			if err := validateJSONPath(target.path); err != nil {
				r.reportError(target.path, err)
				invalidPath = true
			}
		}
		if invalidPath {
			return 2
		}
	}
	changed := false
	for _, target := range targets {
		path := target.path
		result, ok := r.process(target, ruleOptions)
		if !ok {
			continue
		}
		if result.unsupported {
			r.code = 2
		}
		if bytes.Equal(result.original, result.output) {
			if *explain && !r.explain(path, result.changes) {
				continue
			}
			if !*list && !*write && !*check {
				if _, err := io.Copy(out, bytes.NewReader(result.original)); err != nil {
					r.reportError("<stdout>", err)
				}
			}
			continue
		}
		changed = true
		if *write {
			if err := replaceFile(path, result.state, result.original, result.output); err != nil {
				r.reportError(path, err)
				continue
			}
		}
		if *explain && !r.explain(path, result.changes) {
			continue
		}
		if !*list && !*write && !*check {
			if _, err := io.Copy(out, bytes.NewReader(result.output)); err != nil {
				r.reportError("<stdout>", err)
			}
			continue
		}
		if *list || *check {
			if err := r.writePath(path); err != nil {
				r.reportError("<stdout>", err)
			}
		}
	}
	if *check && changed && r.code == 0 {
		r.code = 1
	}
	return r.code
}

func runStdin(in io.Reader, out io.Writer, reporter diagnosticReporter, kind format.Kind, check, explain, utf8Output bool, inputEncoding inputEncodingKind, options format.Options) int {
	data, err := io.ReadAll(in)
	if err != nil {
		_ = reporter.error("<stdin>", err)
		return 2
	}
	doc, err := decodeSourceWithEncoding(data, inputEncoding)
	if err != nil {
		_ = reporter.error("<stdin>", err)
		return 2
	}
	formatted := format.FormatWithOptions(doc.text, kind, options)
	diagnostics := formatted.Diagnostics
	unsupported := false
	for _, diagnostic := range diagnostics {
		if err := reporter.diagnostic("<stdin>", diagnostic); err != nil {
			return 2
		}
		if diagnostic.Severity == format.Error {
			return 2
		}
		if diagnostic.Severity == format.Unsupported {
			unsupported = true
		}
	}
	encoded, err := encodeOutput(doc, formatted.Output, utf8Output)
	if err != nil {
		_ = reporter.error("<stdin>", err)
		return 2
	}
	if check {
		if explain {
			if err := reporter.changes("<stdin>", formatted.Changes); err != nil {
				return 2
			}
		}
		if bytes.Equal(data, encoded) {
			if unsupported {
				return 2
			}
			return 0
		}
		if _, err := fmt.Fprintln(out, "<stdin>"); err != nil {
			_ = reporter.error("<stdout>", err)
			return 2
		}
		if unsupported {
			return 2
		}
		return 1
	}
	if explain {
		if err := reporter.changes("<stdin>", formatted.Changes); err != nil {
			return 2
		}
	}
	if _, err := io.Copy(out, bytes.NewReader(encoded)); err != nil {
		_ = reporter.error("<stdout>", err)
		return 2
	}
	if unsupported {
		return 2
	}
	return 0
}

func (r *runner) process(target discoveredTarget, options format.Options) (fileResult, bool) {
	path := target.path
	result := fileResult{}
	fatal := false
	original, state, err := readSource(path, &target.state)
	if err != nil {
		r.reportError(path, err)
		return result, false
	}
	result.state = state
	result.original = original
	doc, err := decodeSourceWithEncoding(original, r.inputEncoding)
	if err != nil {
		r.reportError(path, err)
		return result, false
	}
	kind := kindForPath(path)
	formatted := format.FormatWithOptions(doc.text, kind, options)
	diagnostics := formatted.Diagnostics
	for _, diagnostic := range diagnostics {
		if err := r.reporter.diagnostic(path, diagnostic); err != nil {
			r.code = 2
			fatal = true
		}
		if diagnostic.Severity == format.Error {
			fatal = true
		}
		if diagnostic.Severity == format.Unsupported {
			result.unsupported = true
		}
	}
	if fatal {
		r.code = 2
		return result, false
	}
	encoded, err := encodeOutput(doc, formatted.Output, r.utf8)
	if err != nil {
		r.reportError(path, err)
		return result, false
	}
	result.output = encoded
	result.changes = formatted.Changes
	return result, true
}

func (r *runner) writePath(path string) error {
	switch r.pathForm {
	case pathOutputNUL:
		if _, err := io.WriteString(r.out, path); err != nil {
			return err
		}
		_, err := io.WriteString(r.out, "\x00")
		return err
	case pathOutputJSON:
		if err := validateJSONPath(path); err != nil {
			return err
		}
		encoded, err := json.Marshal(path)
		if err != nil {
			return err
		}
		encoded = append(encoded, '\n')
		_, err = r.out.Write(encoded)
		return err
	default:
		_, err := fmt.Fprintln(r.out, path)
		return err
	}
}

func (r *runner) explain(path string, changes []format.Change) bool {
	if err := r.reporter.changes(path, changes); err != nil {
		r.code = 2
		return false
	}
	return true
}

func parseRuleOptions(selectFlag, ignoreFlag *optionFlag) (format.Options, error) {
	rules := format.Rules()
	known := make(map[format.RuleID]struct{}, len(rules))
	for _, rule := range rules {
		known[rule.ID] = struct{}{}
	}
	options := format.Options{}
	if selectFlag.set {
		value := strings.TrimSpace(selectFlag.value)
		switch strings.ToLower(value) {
		case "all":
		case "none":
			options.Select = make([]format.RuleID, 0)
		default:
			ids, err := parseRuleList(value, "select", known)
			if err != nil {
				return format.Options{}, err
			}
			options.Select = ids
		}
	}
	if ignoreFlag.set {
		ids, err := parseRuleList(strings.TrimSpace(ignoreFlag.value), "ignore", known)
		if err != nil {
			return format.Options{}, err
		}
		options.Ignore = ids
	}
	return options, nil
}

func parseRuleList(value, flagName string, known map[format.RuleID]struct{}) ([]format.RuleID, error) {
	if value == "" {
		return nil, fmt.Errorf("--%s contains an empty rule list", flagName)
	}
	seen := make(map[format.RuleID]struct{})
	ids := make([]format.RuleID, 0)
	for _, part := range strings.Split(value, ",") {
		name := strings.TrimSpace(part)
		if name == "" {
			return nil, fmt.Errorf("--%s contains an empty rule ID", flagName)
		}
		id := format.RuleID(name)
		if _, ok := known[id]; !ok {
			return nil, fmt.Errorf("unknown rule ID %q", name)
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids, nil
}

func parseEOL(value string) (format.EOL, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "lf":
		return format.LF, nil
	case "crlf":
		return format.CRLF, nil
	default:
		return format.DefaultEOL, fmt.Errorf("--eol must be lf or crlf")
	}
}

func parsePathOutputFormat(value string) (pathOutputFormat, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "text":
		return pathOutputText, nil
	case "nul":
		return pathOutputNUL, nil
	case "json":
		return pathOutputJSON, nil
	default:
		return pathOutputText, fmt.Errorf("--path-format must be text, nul, or json")
	}
}

func parseInputEncoding(value string) (inputEncodingKind, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "auto":
		return inputEncodingAuto, nil
	case "cp932":
		return inputEncodingCP932, nil
	case "cp932-fallback":
		return inputEncodingCP932Fallback, nil
	default:
		return inputEncodingAuto, fmt.Errorf("--input-encoding must be auto, cp932, or cp932-fallback")
	}
}

func validateContinuationSeparator(value string) error {
	if !utf8.ValidString(value) {
		return errors.New("--continuation-separator must be valid UTF-8")
	}
	if strings.ContainsAny(value, "\r\n\x00") {
		return errors.New("--continuation-separator cannot contain CR, LF, or NUL")
	}
	return nil
}

func parseDialect(value string) (format.Dialect, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "common":
		return format.CommonDialect, nil
	case "emuera1824":
		return format.Emuera1824, nil
	case "emee":
		return format.EMEE, nil
	default:
		return format.CommonDialect, fmt.Errorf("--dialect must be common, emuera1824, or emee")
	}
}

func writeRuleCatalog(out io.Writer, rules []format.Rule) error {
	for _, rule := range rules {
		if _, err := fmt.Fprintf(out, "%s\t%s\t%s\n", rule.ID, rule.Name, rule.Description); err != nil {
			return err
		}
	}
	return nil
}

func validateJSONPath(path string) error {
	if !utf8.ValidString(path) {
		return errors.New("cannot encode path as JSON: path is not valid UTF-8; use --path-format=nul")
	}
	return nil
}

func (r *diagnosticReporter) changes(path string, changes []format.Change) error {
	for _, change := range changes {
		name := diagnosticRuleNames[change.Rule]
		if name == "" {
			name = string(change.Rule)
		}
		if err := r.change(path, change, name); err != nil {
			return err
		}
	}
	return nil
}

func (r *runner) reportError(path string, err error) {
	_ = r.reporter.error(path, err)
	r.code = 2
}

func (r *diagnosticReporter) error(path string, err error) error {
	return r.diagnostic(path, format.Diagnostic{Line: 0, Column: 0, Severity: format.Error, Code: "io", Message: err.Error()})
}

func kindForPath(path string) format.Kind {
	if strings.EqualFold(filepath.Ext(path), ".erh") {
		return format.ERH
	}
	return format.ERB
}

func parseKind(value string) format.Kind {
	if strings.EqualFold(value, "erh") {
		return format.ERH
	}
	return format.ERB
}
