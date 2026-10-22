package erabasic

import "iter"

// DeclarationRegionLine is the source fact set needed to decide whether a
// function's declaration prologue has a proven body. Comment and blank-line
// classification is supplied by the caller.
type DeclarationRegionLine struct {
	Line            int
	Head            []byte
	Optional        bool
	Marked          bool
	Opaque          bool
	Blank           bool
	OrdinaryComment bool
	CodeEnd         bool
	BodyStart       bool
}

type DeclarationRegionAnalysis struct {
	Source              []byte
	Dialect             Dialect
	AllowFullwidthSpace bool
	NewSession          func([]byte) *ExpressionSession
}

func (a *DeclarationRegionAnalysis) HeaderValid(head []byte) bool {
	start := TrimInputLeft(head, 1, len(head), a.AllowFullwidthSpace)
	label := AnalyzeLabel(head, '@', start, a.session(head))
	return label.Valid && label.Tail
}

func (a *DeclarationRegionAnalysis) Prologue(line DeclarationRegionLine, sawDim *bool) bool {
	if line.Opaque || line.Optional || line.Marked || len(line.Head) < 2 || line.Head[0] != '#' || !line.CodeEnd {
		return false
	}
	name := declarationName(line.Head)
	switch name {
	case "DIM", "DIMS":
		wordEnd := DeclarationWordEnd(line.Head)
		if wordEnd >= len(line.Head) || len(TrimInput(line.Head[wordEnd:], a.AllowFullwidthSpace)) == 0 {
			return false
		}
		*sawDim = true
		return true
	case "FUNCTION", "FUNCTIONS":
		return BareSharpFunction(line.Head, name, a.AllowFullwidthSpace)
	case "LOCALSIZE", "LOCALSSIZE":
		wordEnd := DeclarationWordEnd(line.Head)
		return wordEnd < len(line.Head) && len(TrimInput(line.Head[wordEnd:], a.AllowFullwidthSpace)) > 0
	default:
		return false
	}
}

func (a *DeclarationRegionAnalysis) ProvenBody(line DeclarationRegionLine) bool {
	if line.BodyStart {
		return true
	}
	if line.Opaque {
		return false
	}
	if line.CodeEnd {
		return true
	}
	start, _, ok := AssignmentBoundary(line.Head, a.session(line.Head))
	if !ok || start < 0 || start > len(line.Head) {
		return false
	}
	src := TrimInput(line.Head[:start], a.AllowFullwidthSpace)
	lhsSession := a.session(src)
	tokens, err := lhsSession.LexRange(ExpressionSourceOriginal, 0, len(src), 0)
	name, variable := VariableLHSFromTokens(tokens, err)
	return variable && !ReservedInstruction(name) && !methodDialect(builtinMethodMasks[name], a.Dialect)
}

func (a *DeclarationRegionAnalysis) BodyStart(lines iter.Seq[DeclarationRegionLine]) (int, bool) {
	sawDim := false
	sawDeclaration := false
	pendingComment := false
	for line := range lines {
		if line.Opaque && !line.BodyStart {
			return 0, false
		}
		if line.Blank {
			return 0, false
		}
		if line.OrdinaryComment {
			if !sawDeclaration {
				continue
			}
			pendingComment = true
			continue
		}
		if a.Prologue(line, &sawDim) {
			sawDeclaration = true
			pendingComment = false
			continue
		}
		if !sawDim || pendingComment || !a.ProvenBody(line) {
			return 0, false
		}
		if line.Optional || line.Marked || len(line.Head) == 0 {
			return 0, false
		}
		return line.Line, true
	}
	return 0, false
}

func (a *DeclarationRegionAnalysis) session(src []byte) *ExpressionSession {
	if a.NewSession != nil {
		return a.NewSession(src)
	}
	session := NewExpressionSessionWithSpace(src, a.AllowFullwidthSpace)
	session.BuiltinTypes = true
	session.Dialect = a.Dialect
	return session
}
