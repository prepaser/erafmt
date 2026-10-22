package erabasic

type ExtraFieldValidation struct {
	Valid bool
	Quiet bool
}

func ExtraFieldBoundsWithSpace(src []byte, field SyntaxSpan, allowFullwidthSpace bool) (int, int) {
	left := field.Start
	left = TrimInputLeft(src, left, field.End, allowFullwidthSpace)
	right := field.End
	right = TrimInputRight(src, left, right, allowFullwidthSpace)
	return left, right
}

func ValidTimesCoefficient(src []byte) bool {
	if len(src) == 0 {
		return false
	}
	i := 0
	if src[i] == '+' || src[i] == '-' {
		i++
		if i == len(src) {
			return false
		}
	}
	digits := 0
	for i < len(src) && src[i] >= '0' && src[i] <= '9' {
		i++
		digits++
	}
	if i < len(src) && src[i] == '.' {
		i++
		for i < len(src) && src[i] >= '0' && src[i] <= '9' {
			i++
			digits++
		}
	}
	if digits == 0 {
		return false
	}
	if i < len(src) && (src[i] == 'e' || src[i] == 'E') {
		i++
		if i < len(src) && src[i] == '-' {
			i++
		}
		exponent := i
		for i < len(src) && src[i] >= '0' && src[i] <= '9' {
			i++
		}
		if exponent == i {
			return false
		}
	}
	return i == len(src)
}

func TimesCoefficientRuntimeWarning(src []byte) bool {
	if ValidTimesCoefficient(src) || len(src) == 0 {
		return false
	}
	i := 0
	if src[i] == '+' || src[i] == '-' {
		i++
		if i == len(src) {
			return false
		}
	}
	if i+1 < len(src) && src[i] == '0' && (src[i+1] == 'x' || src[i+1] == 'X' || src[i+1] == 'b' || src[i+1] == 'B') {
		return false
	}
	digits := 0
	for i < len(src) && src[i] >= '0' && src[i] <= '9' {
		i++
		digits++
	}
	if i < len(src) && src[i] == '.' {
		i++
		for i < len(src) && src[i] >= '0' && src[i] <= '9' {
			i++
			digits++
		}
	}
	return digits > 0
}
