package erabasic

type DocumentSession struct {
	FunctionContext
	ParseLine             int
	ParseBase             int
	ParseSegments         []ContinuationSegment
	Metadata              *MetadataBudget
	ConvergenceWork       int
	PpStateUnits          int
	Src                   []byte
	Lines                 []Line
	Kind                  Kind
	State                 State
	Pp                    []ppFrame
	Base                  int
	Preserved             bool
	PreserveFile          bool
	Optional              bool
	Opaque                []bool
	Headers               []int
	CodeEnds              map[int]int
	BodyStarts            []bool
	PpLines               map[int]int
	Dialect               Dialect
	AllowFullwidthSpace   bool
	ContinuationSeparator *string
	PendingDeclaration    string
	PendingLabel          string
	InContinuationProxy   bool
	SyntaxSession         *ExpressionSession
	SyntaxBase            int
	Columns               map[int][]columnPoint
	StatementOrigins      map[int]SourcePosition
	Aborted               bool
}

func (s *DocumentSession) Conditional() bool {
	return preprocessorHas(s.Pp, "IF")
}

func (s *DocumentSession) Skipped() bool {
	return preprocessorHas(s.Pp, "SKIP")
}
