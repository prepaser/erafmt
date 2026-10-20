package format

type functionContext struct {
	functionStart       int
	functionName        string
	eventFunctionKnown  bool
	eventFunctionExact  bool
	systemFunctionKnown bool
	privateDeclarations map[string]bool
	sharpAttributes     sharpAttributeFlags
	sharpFunctionKind   string
	sharpStateUnknown   bool
	localSizeSeen       [2]bool
	dollarLabels        map[string]bool
	labelsUnknown       bool
}

type sharpAttributeFlags uint8

const (
	sharpAttributeSingle sharpAttributeFlags = 1 << iota
	sharpAttributeLater
	sharpAttributePri
	sharpAttributeOnly
)

func sharpAttributeFlag(name string) sharpAttributeFlags {
	switch name {
	case "SINGLE":
		return sharpAttributeSingle
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

func (p *formatter) continuationContext() functionContext {
	ctx := p.functionContext
	ctx.sharpStateUnknown = ctx.sharpStateUnknown || p.optional || p.conditional()
	return ctx
}

func (p *formatter) commitContinuationContext(proof continuationProof) {
	proxy, logical, mappingOK := proof.proxy, proof.logical, proof.mappingOK
	head := proof.head
	sharp := len(head) > 0 && head[0] == '#'
	dollar := len(head) > 0 && head[0] == '$'
	if dollar {
		if mappingOK && continuationLabelStateCommitted(proxy, logical) {
			if proxy.pendingLabel != "" {
				if p.dollarLabels == nil {
					p.dollarLabels = make(map[string]bool)
				}
				p.dollarLabels[proxy.pendingLabel] = true
			}
			p.labelsUnknown = proxy.labelsUnknown
		} else {
			p.labelsUnknown = true
		}
	}
	if sharp {
		if mappingOK && continuationDeclarationCommitted(proxy, logical) {
			if proxy.pendingDeclaration != "" {
				p.privateDeclarations[proxy.pendingDeclaration] = true
			}
			p.sharpAttributes = proxy.sharpAttributes
			p.sharpFunctionKind = proxy.sharpFunctionKind
			p.localSizeSeen = proxy.localSizeSeen
		} else {
			p.sharpStateUnknown = true
		}
	}
}
