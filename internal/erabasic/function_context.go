package erabasic

type FunctionContext struct {
	FunctionStart       int
	FunctionName        string
	EventFunctionKnown  bool
	EventFunctionExact  bool
	SystemFunctionKnown bool
	PrivateDeclarations map[string]bool
	SharpAttributes     sharpAttributeFlags
	SharpFunctionKind   string
	SharpStateUnknown   bool
	LocalSizeSeen       [2]bool
	DollarLabels        map[string]bool
	LabelsUnknown       bool
}

type ContinuationContextCandidate struct {
	Head               []byte
	Function           FunctionContext
	PendingDeclaration string
	PendingLabel       string
}

type sharpAttributeFlags uint8

const (
	SharpAttributeSingle sharpAttributeFlags = 1 << iota
	sharpAttributeLater
	sharpAttributePri
	sharpAttributeOnly
)

func sharpAttributeFlag(name string) sharpAttributeFlags {
	switch name {
	case "SINGLE":
		return SharpAttributeSingle
	case "LATER":
		return sharpAttributeLater
	case "PRI":
		return sharpAttributePri
	case "ONLY":
		return sharpAttributeOnly
	default:
		return 0
	}
}

func (c *FunctionContext) CommitContinuationCandidate(candidate ContinuationContextCandidate, labelAccepted, declarationAccepted, proven bool) {
	head := candidate.Head
	dollar := len(head) > 0 && head[0] == '$'
	sharp := len(head) > 0 && head[0] == '#'
	if dollar {
		if proven && labelAccepted {
			if candidate.PendingLabel != "" {
				if c.DollarLabels == nil {
					c.DollarLabels = make(map[string]bool)
				}
				c.DollarLabels[candidate.PendingLabel] = true
			}
			c.LabelsUnknown = candidate.Function.LabelsUnknown
		} else {
			c.LabelsUnknown = true
		}
	}
	if sharp {
		if proven && declarationAccepted {
			if candidate.PendingDeclaration != "" {
				if c.PrivateDeclarations == nil {
					c.PrivateDeclarations = make(map[string]bool)
				}
				c.PrivateDeclarations[candidate.PendingDeclaration] = true
			}
			c.SharpAttributes = candidate.Function.SharpAttributes
			c.SharpFunctionKind = candidate.Function.SharpFunctionKind
			c.LocalSizeSeen = candidate.Function.LocalSizeSeen
		} else {
			c.SharpStateUnknown = true
		}
	}
}
