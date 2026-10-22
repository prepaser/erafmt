package erabasic

import "testing"

func TestExpressionExcluded(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{`A+@"form"`, false},
		{`A+"@form"`, false},
		{`A+[[alias]]`, true},
		{`A+"[[alias]]"`, true},
		{`A+@"form";#; literal`, true},
		{`A+@"form";^; literal`, false},
		{"A+\u3000B", true},
		{`A+"\u3000B"`, false},
		{`A+B;!;`, false},
		{`A+";!;"`, false},
		{`A+B; comment`, false},
		{`A+\@ X?yes#no\@+B`, false},
		{`A+\@ X?yes\@+B`, false},
	}
	for _, tt := range tests {
		if got := expressionExcluded([]byte(tt.input)); got != tt.want {
			t.Errorf("expressionExcluded(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
