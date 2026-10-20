# Development

## Source layout

- `main.go` starts the CLI.
- `internal/cli` handles arguments, paths, encoding, diagnostics, and file writes.
- `internal/format` implements parsing, formatting rules, and source preservation.
- `docs` contains usage and development references.

## Build

`VERSION` at the repository root contains the project version. Update it when preparing a release.

```sh
go build -buildvcs=false .
```

## Standard checks

```sh
go test ./...
go vet ./...
go test -race ./...
```

`go test ./...` and `go vet ./...` run without external runtime sources or game data.

Committed fixtures use synthetic identifiers and neutral text. Non-ASCII samples exercise Unicode handling and encoding boundaries; Japanese strings are retained where CP932 byte mappings or localized runtime diagnostics require them.

## Corpus audits

```sh
ERAFMT_CORPUS=/path/to/copied/scripts \
ERAFMT_CORPUS_DIALECT=emee \
ERAFMT_CORPUS_INPUT_ENCODING=cp932-fallback \
ERAFMT_CORPUS_REPORT=/path/to/report.json \
go test ./internal/cli -run '^TestCorpus$' -count=1
```

Optional corpus audits use `ERAFMT_CORPUS=/path/to/copied/scripts` and `ERAFMT_CORPUS_DIALECT=common|emuera1824|emee`. `TestCorpus` fails when no ERB or ERH targets are discovered or a discovered file cannot be read; read failures retain their paths and errors in the report. `ERAFMT_CORPUS_INPUT_ENCODING=auto|cp932|cp932-fallback` selects the audit input profile. Default auto verifies source encoding/BOM round trips; legacy profiles verify UTF-8 BOM conversion for nonfatal results. Fatal inputs retain their original encoded bytes and count as unchanged. Reports record the effective input/output profiles and the exact continuation separator, including an empty value or surrounding spaces. `ERAFMT_CORPUS_ALLOW_FULLWIDTH_SPACE=true|false` passes the same grammar option to formatting, preservation checks, and representative CLI writes. `ERAFMT_CORPUS_CONTINUATION_SEPARATOR` opts corpus verification into the exact continuation join, including an explicitly empty value; unset keeps continuation interiors byte-sensitive.

`TestCorpus` verifies that a second formatting pass preserves output bytes and severity/code counts and produces no further changes. Source positions may shift after formatting. A failed reentry check excludes the file from `Verified`.

The independent source-preservation oracle separates executable syntax from ignored NUL suffixes before checking argument shapes or normalizing whitespace. It allows proven formatting before termination and compares the NUL and remaining bytes exactly, including across explicit continuation rows. Raw CALL targets and FORM text terminate at NUL; expression identifiers retain embedded NUL bytes. Ordinary CALL comments are checked as comments, with configured grammar padding kept separate from literal comment text.

Continuation argument checks retain both logical token identity and physical payload boundaries. CALL targets, quoted input fields, FORM literals, and opaque arguments retain their exact bytes in each physical row; the configured virtual separator participates in lexical analysis without becoming editable source.

## Audit reports

Set `ERAFMT_CORPUS_REPORT=/path/to/report.json` to save the corpus audit report. `Diagnostics` retains aggregate counts by severity/code. `DiagnosticCategories` adds occurrence counts and relative path/line/column representatives, with counts and representatives for each normalized reason and unsupported command. Unclosed-block messages group across different source line references, and command names retain their original Unicode characters. It retains up to 64 categories and 32 reasons per category; excess occurrences are counted in `DiagnosticCategoryOccurrencesOmitted` and each category's `ReasonOccurrencesOmitted`. Reason limits do not discard category totals or representatives. Long reasons and command names retain a digest to distinguish truncated text. `Samples` retains the first 20 diagnostics for compatibility. These counts describe diagnostic occurrences, not distinct files or confirmed runtime errors; inspect representatives before treating partial formatting as a support gap.

`Games` is a sorted array of records containing `Name`, optional `NameBytes`, and `Counts`. `NameBytes`, `RootBytes`, and path records' `PathBytes` preserve invalid UTF-8 filesystem bytes as Base64; the readable name or path remains available alongside them. Receipt records use `RelativePathBytes` for the same purpose. Receipt paths are relative to the configured corpus root, including when the root has a trailing separator. This avoids colliding JSON object keys and retains exact file identities. `Samples` remains a human-readable diagnostic list.

Report and receipt output paths must identify a different file from every source configured through `ERAFMT_CORPUS` or `ERAFMT_CORPUS_UTF8`. This applies to `ERAFMT_CORPUS_REPORT`, `ERAFMT_PROFILE_RECEIPT`, and `ERAFMT_PROFILE_ENCODING_ERROR_RECEIPT`. Audits check both discovery and current file identities before processing and before writing, rejecting direct paths, symbolic links, and hardlinks that refer to a source. Missing or nonregular audited files also prevent artifact writes. Ordinary distinct artifact files may still be replaced.

## Representative write audits

`ERAFMT_CORPUS_UTF8` selects a copied corpus for representative UTF-8 write tests; those tests write only temporary copies and select changed and unchanged encoded outputs separately for each rule selection, encoding, BOM, status, and diagnostic class. Representative write audits also exercise the portable rename path on macOS and other Unix platforms. Set `ERAFMT_PROFILE_RECEIPT=/path/to/receipt.json` with `ERAFMT_CORPUS` to record representatives by encoding, BOM, actual complete/partial/fatal status, first-pass diagnostic class, and whether the requested encoded output changes. Profile audits fail on empty target sets or empty receipts. Receipts include both CLI passes' diagnostic classes and output disposition. Representative UTF-8 and fallback write audits also compare the emitted CLI status and diagnostic class on both passes, including encoding failures. Decode failures receive separate encoding-error representatives by input profile and failure category; both CLI passes must retain the original bytes.

`ERAFMT_PROFILE_ENCODING_ERROR_RECEIPT=/path/to/receipt.json` optionally records the synthetic decoding-error representatives from `TestInputProfileEncodingErrorRepresentatives` using the same source protections.

## Runtime conformance

Runtime comparison is optional test infrastructure. Normal builds, formatting, and standard checks do not require an external runtime.

### Emuera 1.824

Set `ERAFMT_EMUERA1824_ROOT` to a base runtime source tree to enable `TestRuntimeConformanceBase`. The test copies and builds the runtime, then compares its parser results before and after formatting without starting a game or GUI. Argument evidence fingerprints instance fields, expression children, operator and method identity, variable identity and indexes, and literal values; variable storage is excluded. String fingerprints retain every UTF-16 code unit, including unpaired surrogates. Negative controls verify that deliberate changes to these arguments produce different evidence. Traversal is limited to depth 24, 64 items per collection, 4,096 nodes, and 65,536 characters of fingerprint input. Unsupported evidence types, cycles, and exceeded limits fail the probe rather than reporting equivalence. This compares the inspected parser structures, not complete program behavior. Linux requires `xbuild`, `mcs`, and `mono`; Windows requires `msbuild` and `csc`.

### EMEE

`TestRuntimeConformanceModern` requires Windows, a .NET 10 SDK, and `ERAFMT_EMEE_ASSEMBLY` pointing to the runtime assembly with its dependencies. Missing tools or probe infrastructure failures fail enabled probes rather than counting as syntax rejection.
