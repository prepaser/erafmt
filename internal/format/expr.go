package format

import (
	"bytes"

	"github.com/prepaser/erafmt/internal/erabasic"
)

func expressionEditsWithSession(session *erabasic.ExpressionSession, source erabasic.ExpressionSourceID, src []byte, cacheStart int, caseMode bool, rules ruleSet, verify bool) ([]edit, error) {
	analysis, err := erabasic.AnalyzeExpressionWithSession(session, source, src, cacheStart, caseMode)
	if err != nil {
		return nil, err
	}
	if len(src) == 0 || analysis.Unsupported {
		return nil, nil
	}
	start, end := analysis.Start, analysis.End
	if start >= end {
		return nil, nil
	}
	tokens := analysis.Tokens
	var edits []edit
	for _, token := range tokens {
		if token.Kind != erabasic.ExprForm {
			continue
		}
		nested, nestedErr := formEditsWithSession(session, source, token.Text, cacheStart+token.Off, rules, token.FormMode, false)
		if nestedErr != nil {
			return nil, erabasic.ShiftExpressionError(nestedErr, token.Off)
		}
		for _, edit := range nested {
			edit.start += token.Off
			edit.end += token.Off
			edits = append(edits, edit)
		}
	}
	for i := 1; i < len(tokens); i++ {
		if tokens[i-1].Kind == erabasic.ExprMarker || tokens[i].Kind == erabasic.ExprMarker {
			continue
		}
		rule := expressionGapRule(tokens[i-1], tokens[i])
		if !rules[rule] {
			continue
		}
		text := []byte{}
		if expressionNeedsSpace(tokens, i-1, i, caseMode) {
			text = []byte{' '}
		}
		gapStart := tokens[i-1].Off + len(tokens[i-1].Text)
		gapEnd := tokens[i].Off
		if bytes.Contains(src[gapStart:gapEnd], []byte(";!;")) {
			continue
		}
		if !bytes.Equal(src[gapStart:gapEnd], text) {
			edits = append(edits, edit{gapStart, gapEnd, text, Change{Rule: rule}})
		}
	}
	if len(edits) == 0 {
		return nil, nil
	}
	if !verify {
		return edits, nil
	}
	formatted := applyEdits(src, edits)
	session.SetSource(erabasic.ExpressionSourceFormatted, formatted)
	formattedEnd := len(formatted) - (len(src) - end)
	check, err := session.LexRange(erabasic.ExpressionSourceFormatted, start, formattedEnd, start)
	if err != nil || !erabasic.SameExpressionTokens(tokens, check) {
		if err == nil {
			err = &erabasic.ExprError{Offset: start, Msg: "formatting changed expression tokens"}
		}
		return nil, err
	}
	return edits, nil
}

func expressionGapRule(prev, cur erabasic.ExprToken) RuleID {
	if prev.Kind == erabasic.ExprMarker || cur.Kind == erabasic.ExprMarker {
		return ""
	}
	if cur.Text[0] == ',' {
		return CommaSpacing
	}
	if cur.Text[0] == ')' || cur.Text[0] == ']' {
		return DelimiterSpacing
	}
	if prev.Text[0] == ',' {
		return CommaSpacing
	}
	if prev.Text[0] == ':' || cur.Text[0] == ':' {
		return IndexSpacing
	}
	if prev.Text[0] == '(' || prev.Text[0] == '[' {
		return DelimiterSpacing
	}
	if prev.CaseKeyword || cur.CaseKeyword {
		return CaseSpacing
	}
	if prev.Kind == erabasic.ExprOperator || cur.Kind == erabasic.ExprOperator {
		if prev.Binary || cur.Binary {
			if (prev.Binary && (prev.Text[0] == '?' || prev.Text[0] == '#')) || (cur.Binary && (cur.Text[0] == '?' || cur.Text[0] == '#')) {
				return TernarySpacing
			}
			return BinarySpacing
		}
		return UnarySpacing
	}
	if cur.Text[0] == '(' || cur.Text[0] == '[' {
		return DelimiterSpacing
	}
	return TokenSpacing
}

func expressionNeedsSpace(tokens []erabasic.ExprToken, prevIndex, index int, caseMode bool) bool {
	prev, cur := tokens[prevIndex], tokens[index]
	if prev.Kind == erabasic.ExprOperator && cur.Kind == erabasic.ExprOperator {
		return prev.Binary || cur.Binary || (len(prev.Text) == 1 && erabasic.ValidDoubleOperator(prev.Text[0], cur.Text[0]))
	}
	if cur.Text[0] == ')' || cur.Text[0] == ']' || cur.Text[0] == ',' {
		return false
	}
	if prev.Text[0] == ',' {
		return true
	}
	if prev.Text[0] == '(' || prev.Text[0] == '[' || prev.Text[0] == ':' || cur.Text[0] == ':' {
		return false
	}
	if caseMode {
		if prev.CaseKeyword {
			return cur.Text[0] != ')' && cur.Text[0] != ']' && cur.Text[0] != ','
		}
		if cur.CaseKeyword {
			return prev.Text[0] != '(' && prev.Text[0] != '['
		}
	}
	if prev.Kind == erabasic.ExprOperator {
		return prev.Binary
	}
	if cur.Kind == erabasic.ExprOperator {
		return cur.Binary
	}
	if cur.Text[0] == '(' || cur.Text[0] == '[' {
		return false
	}
	return erabasic.ExpressionWordLike(prev) && erabasic.ExpressionWordLike(cur)
}
