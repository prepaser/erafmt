package erabasic

func expressionExcluded(src []byte) bool {
	return ExpressionExcludedWithSession(NewExpressionSession(src), src)
}

func formFingerprint(src []byte) ([]byte, error) {
	return formFingerprintMode(src, FormQuoted)
}

func headerArgumentIssueWithSession(src []byte, start, end int, session *ExpressionSession, dialect Dialect) (headerArgumentIssue, bool) {
	for _, diagnostic := range HeaderArgumentDiagnosticsWithSession(src, start, end, session, dialect) {
		if diagnostic.Severity == Warning {
			continue
		}
		return headerArgumentIssue{Offset: diagnostic.Offset, Message: diagnostic.Message}, true
	}
	return headerArgumentIssue{}, false
}

func linesOf(src []byte) []Line {
	return linesOfWithSpace(src, false)
}

func validateHeader(head []byte) headerValidation {
	return validateHeaderWithSession(head, NewExpressionSession(head))
}

func formFingerprintMode(src []byte, mode FormMode) ([]byte, error) {
	session := NewExpressionSession(src)
	return session.FormFingerprintAt(ExpressionSourceOriginal, 0, len(src), mode)
}

func linesOfWithSpace(src []byte, allowFullwidth bool) []Line {
	return MakeLinesWithSpace(src, allowFullwidth, 0)
}

func lexExpression(src []byte, base int) ([]ExprToken, error) {
	session := NewExpressionSession(src)
	return session.LexRange(ExpressionSourceOriginal, 0, len(src), base)
}
