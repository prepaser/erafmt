package erabasic

type Kind uint8

const (
	ERB Kind = iota
	ERH
)

type Severity string

const (
	Error       Severity = "error"
	Unsupported Severity = "unsupported"
	Warning     Severity = "warning"
)
