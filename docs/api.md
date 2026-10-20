# Core API

The core in [`internal/format`](../internal/format) is independent of filesystems and codecs. It is an internal Go package for use within this module:

```go
func Format(src []byte, kind Kind) ([]byte, []Diagnostic)

func FormatWithOptions(src []byte, kind Kind, options Options) Result
func Rules() []Rule
```

`Format` remains the compatibility entry point and enables all rules. `Options` contains `Select`, `Ignore`, `EOL`, `Dialect`, `AllowFullwidthSpace`, and the optional `ContinuationSeparator`. `Dialect` is `CommonDialect` (zero value), `Emuera1824`, or `EMEE`. `Options.Select` chooses all rules when nil, none when empty but non-nil, or the listed IDs. `Options.Ignore` is applied afterward. `Options.Dialect` changes syntax recognition without changing rule selection. `AllowFullwidthSpace` opts U+3000 into grammar whitespace handling. A non-nil `ContinuationSeparator` supplies the literal virtual separator used while analyzing continuation rows; nil leaves continuation bodies opaque. `Result.Changes` contains only committed corrections; `Result.Diagnostics` reports parsing, preservation, and input errors separately. `Rules()` returns the implementation catalog. `src` must be valid UTF-8 without a BOM. The CLI owns decoding, encoding restoration, path discovery, diagnostics, and guarded file replacement. `Kind` is `ERB` or `ERH`.

The core leaves `src` unchanged. `Result.Output` can share its backing array when no edit is applied, including preserved input. Callers that need an independently mutable result should clone the output. Input and option data must remain unchanged during a call.

See the [rule catalog](rules.md) for the stable 37-rule IDs and ownership order.
