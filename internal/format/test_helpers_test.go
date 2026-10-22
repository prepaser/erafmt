package format

import (
	"strconv"
	"strings"

	"github.com/prepaser/erafmt/internal/erabasic"
)

func formEdits(src []byte, rules ruleSet, mode erabasic.FormMode) ([]edit, error) {
	session := erabasic.NewExpressionSession(src)
	return formEditsWithSession(session, erabasic.ExpressionSourceOriginal, src, 0, rules, mode, true)
}

func formatExpression(src []byte, caseMode bool) ([]byte, error) {
	rules, _ := resolveRules(Options{})
	edits, err := expressionEdits(src, caseMode, rules)
	if err != nil {
		return cloneBytes(src), err
	}
	return applyEdits(src, edits), nil
}

func expressionEdits(src []byte, caseMode bool, rules ruleSet) ([]edit, error) {
	session := erabasic.NewExpressionSession(src)
	return expressionEditsWithSession(session, erabasic.ExpressionSourceOriginal, src, 0, caseMode, rules, true)
}

func linesOf(src []byte) []erabasic.Line {
	return linesOfWithSpace(src, false)
}

func linesOfWithSpace(src []byte, allowFullwidth bool) []erabasic.Line {
	return erabasic.MakeLinesWithSpace(src, allowFullwidth, 0)
}

func lexExpression(src []byte, base int) ([]erabasic.ExprToken, error) {
	session := erabasic.NewExpressionSession(src)
	return session.LexRange(erabasic.ExpressionSourceOriginal, 0, len(src), base)
}

func cloneBytes(src []byte) []byte {
	return append([]byte(nil), src...)
}

func nestedFormSource(depth int) []byte {
	source := `@"x { A+1 }"`
	for i := 1; i < depth; i++ {
		source = `@"x { ` + source + ` }"`
	}
	return []byte(source)
}

func commandNumericFields(count int) string {
	var b strings.Builder
	for i := 1; i <= count; i++ {
		if i > 1 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.Itoa(i))
	}
	return b.String()
}

func saveCharaFields(count int, invalidLast bool) string {
	var b strings.Builder
	b.WriteString(`"file","message"`)
	for i := 3; i <= count; i++ {
		b.WriteByte(',')
		if invalidLast && i == count {
			b.WriteString("1+(")
			continue
		}
		b.WriteString(strconv.Itoa(i - 2))
	}
	return b.String()
}

func extraRoleSource(line string) []byte {
	if len(line) >= 9 && line[:9] == "PRINTDATA" || len(line) >= 7 && line[:7] == "STRDATA" {
		return []byte("@F\n" + line + "\nDATA raw\nENDDATA\n")
	}
	return []byte("@F\n" + line + "\n")
}

func formFingerprint(src []byte) ([]byte, error) {
	return formFingerprintMode(src, erabasic.FormQuoted)
}

func formFingerprintMode(src []byte, mode erabasic.FormMode) ([]byte, error) {
	session := erabasic.NewExpressionSession(src)
	return session.FormFingerprintAt(erabasic.ExpressionSourceOriginal, 0, len(src), mode)
}
