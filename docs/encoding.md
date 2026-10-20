# Encoding and line endings

## Default input

The default `--input-encoding=auto` accepts strict UTF-8 with an optional BOM and BOM-marked UTF-16LE or UTF-16BE. It rejects malformed UTF-8, invalid UTF-16 length or surrogate pairs, UTF-32, and legacy encodings.

## CP932 input

```sh
erafmt --input-encoding=cp932 --utf8 -w legacy/
erafmt --input-encoding=cp932-fallback --utf8 -w scripts/
```

`--input-encoding=cp932 --utf8` strictly decodes BOM-less Windows code page 932 input and writes UTF-8 with a BOM. BOM-marked UTF-8/UTF-16 remains authoritative, including validation errors. CP932 supports user-defined and vendor extension characters without replacing invalid bytes. The exact `cp932` profile overrides UTF-8 interpretation of BOM-less input; apply it to known CP932 paths. `--input-encoding=cp932-fallback --utf8` keeps valid BOM-less UTF-8 and retries strict CP932 only when UTF-8 validation fails, allowing mixed encoding trees. Fallback remains opt-in because damaged BOM-less UTF-8 can also form valid CP932 bytes. Both legacy profiles honor BOM validation and require `--utf8` in output, check, list, and write modes because CP932 re-encoding can change duplicate byte mappings. CP949 is unsupported.

## Output encoding

```sh
erafmt --utf8 -w scripts/
erafmt --utf8 --select=none -w scripts/
```

UTF-8/UTF-16 encoding and BOM are restored after formatting by default. `--utf8` outputs UTF-8 with a BOM for files and stdin, including empty decoded content. The BOM allows legacy Emuera 1.824 to recognize UTF-8 rather than use its Shift-JIS default. Use `--utf8 --select=none` to convert encoding without changing formatting. Encoding differences count as changes for `--check`, `-l`, and `-w`; `--explain` reports only formatting rule edits. The option does not bypass encoding validation or fatal formatting errors.

## Line endings

F002 normalizes line endings to LF by default or CRLF with `--eol=crlf`; `--eol=lf|crlf` selects style but does not enable F002. When F002 is ignored, explicit `--eol`/`Options.EOL` has priority for inserted L004/L007/F001 terminators; `DefaultEOL` uses the adjacent or last surviving EOL and falls back to LF.

## BOM boundaries

UTF-16LE output that would start with `FF FE 00 00` is rejected before writing because that prefix is indistinguishable from a UTF-32 BOM. Use `--utf8` for such content. Leading indentation and blank lines are retained when removing them would expose a content U+FEFF as an encoding BOM.

See [Formatting behavior](formatting.md#partial-formatting-and-errors) for partial formatting and fatal errors.
