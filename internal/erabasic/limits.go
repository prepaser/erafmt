package erabasic

import "unicode/utf8"

const (
	MaxStructureDepth       = 512
	MaxPreprocessorDepth    = 512
	MaxSyntaxBytes          = 1 << 20
	MaxSourceBufferBytes    = 64 << 20
	maxContinuationSegments = MaxSourceBufferBytes / 64
)

type ppFrame struct {
	StateUnits int
	Kind       string
	Line       int
	Entry      State
	Branches   []State
	HasElse    bool
	Malformed  bool
}

func erhUnsupportedDeclaration(name string) bool {
	switch name {
	case "DEFINE", "DIM", "DIMS":
		return false
	default:
		return true
	}
}

func erhUnsupportedDeclarationLine(head []byte, allowFullwidthSpace bool) bool {
	name := declarationName(head)
	if erhUnsupportedDeclaration(name) {
		return true
	}
	if name != "DIM" && name != "DIMS" {
		return false
	}
	return declarationHasRefModifier(head, DeclarationWordEnd(head), len(head), allowFullwidthSpace)
}

func appendWithinLimit(current, added, limit int) bool {
	return current >= 0 && added >= 0 && current <= limit && added <= limit-current
}

func continuationContentLimitExceeded(rows int) bool {
	return rows >= maxContinuationSegments
}

func PreprocessorFields(head []byte, end int, allowFullwidthSpace bool) (first, second []byte, count int) {
	start := TrimInputLeft(head, 1, end, allowFullwidthSpace)
	if start >= end {
		return nil, nil, 0
	}
	pos := start
	for pos < end && InputSpaceSize(head, pos, allowFullwidthSpace) == 0 {
		pos++
	}
	first = head[start:pos]
	count = 1
	start = TrimInputLeft(head, pos, end, allowFullwidthSpace)
	if start >= end {
		return first, nil, count
	}
	pos = start
	for pos < end && InputSpaceSize(head, pos, allowFullwidthSpace) == 0 {
		pos++
	}
	second = head[start:pos]
	count = 2
	if TrimInputLeft(head, pos, end, allowFullwidthSpace) < end {
		count = 3
	}
	return first, second, count
}

func preprocessorConditionToken(field []byte) bool {
	if len(field) == 0 {
		return false
	}
	for len(field) > 0 {
		r, size := utf8.DecodeRune(field)
		if r == '\u3000' || IsDelimiter(r) {
			return false
		}
		field = field[size:]
	}
	return true
}
