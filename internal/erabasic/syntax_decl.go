package erabasic

func SyntaxHeaderMarkerEnd(head []byte) int {
	for i := 1; i < len(head); {
		if next, atom := SyntaxAtomEnd(head, i, len(head)); atom {
			if next < 0 {
				return len(head)
			}
			i = next
			continue
		}
		if ExecutableMarker(head, i) {
			return i
		}
		i++
	}
	return len(head)
}

var reservedDeclarationNames = map[string]bool{"IS": true, "TO": true, "INT": true, "REFFUNC": true, "PRIVATE": true, "__DEBUG__": true, "__SKIP__": true, "_": true}

func HeaderPrefix(head []byte, start int) (int, int) {
	for i := start; i < len(head); i++ {
		if head[i] == '[' || head[i] == '(' || head[i] == ',' || head[i] == ';' {
			return i, i
		}
	}
	return len(head), -1
}
