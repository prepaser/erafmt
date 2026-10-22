package erabasic

import "bytes"

type expressionAnalysis struct {
	Start       int
	End         int
	Tokens      []ExprToken
	Unsupported bool
}

func SameExpressionTokens(a, b []ExprToken) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Kind != b[i].Kind {
			return false
		}
		if a[i].Kind == ExprForm {
			if a[i].FormMode != b[i].FormMode || !bytes.Equal(a[i].Fingerprint, b[i].Fingerprint) {
				return false
			}
			continue
		}
		if !bytes.Equal(a[i].Text, b[i].Text) {
			return false
		}
	}
	return true
}

func AnalyzeExpressionWithSession(session *ExpressionSession, source ExpressionSourceID, src []byte, cacheStart int, caseMode bool) (expressionAnalysis, error) {
	if len(src) == 0 {
		return expressionAnalysis{}, nil
	}
	if !session.ValidUTF8(source) {
		return expressionAnalysis{}, &ExprError{Msg: "invalid UTF-8"}
	}
	bodyEnd, unsupported, err := expressionBodyEndWithSession(session, source, src, cacheStart)
	if err != nil {
		return expressionAnalysis{}, err
	}
	result := expressionAnalysis{End: bodyEnd, Unsupported: unsupported}
	if unsupported {
		return result, nil
	}
	result.Start = LeadingTriviaEndWithSession(session, src[:bodyEnd])
	result.End = TrailingTriviaStartWithSession(session, src[:bodyEnd])
	if result.Start >= result.End {
		return result, nil
	}
	tokens, err := session.LexRange(source, cacheStart+result.Start, cacheStart+result.End, result.Start)
	if err != nil {
		return expressionAnalysis{}, err
	}
	if !session.SkipFormTypes {
		if err := provenExpressionsValid(tokens, session.BuiltinTypes, session.Dialect); err != nil {
			return expressionAnalysis{}, err
		}
	}
	classifyOperators(tokens, caseMode)
	result.Tokens = tokens
	return result, nil
}
