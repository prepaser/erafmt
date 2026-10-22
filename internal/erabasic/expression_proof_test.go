package erabasic

import (
	"strings"
	"testing"
)

func TestBuiltinParenthesisDepthGuard(t *testing.T) {
	for _, depth := range []int{256, 257, 300} {
		expression := strings.Repeat("(", depth) + "NAME:0:0" + strings.Repeat(")", depth) + "+1"
		tokens, err := lexExpression([]byte(expression), 0)
		if err != nil {
			t.Fatalf("depth=%d lex: %v", depth, err)
		}
		_, valid, proven := BuiltinExpressionType(tokens, CommonDialect)
		if depth <= 257 && (!proven || valid) {
			t.Fatalf("depth=%d invalid index was not proven invalid: valid=%v proven=%v", depth, valid, proven)
		}
		if depth == 300 && proven {
			t.Fatalf("depth=%d exceeded proof guard: valid=%v proven=%v", depth, valid, proven)
		}
	}
}
