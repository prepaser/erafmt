package format

func expressionExcluded(src []byte) bool {
	return expressionExcludedWithSession(newExpressionSession(src), src)
}

func formEdits(src []byte, rules ruleSet, mode formMode) ([]edit, error) {
	session := newExpressionSession(src)
	return formEditsWithSession(session, expressionSourceOriginal, src, 0, rules, mode, true)
}

func formFingerprint(src []byte) ([]byte, error) {
	return formFingerprintMode(src, formQuoted)
}

// formatExpression formats the argument portion of one known expression
// command. Delimiters belonging to the command are deliberately outside this
// function; assignment payloads remain outside this expression path.
func formatExpression(src []byte, caseMode bool) ([]byte, error) {
	rules, _ := resolveRules(Options{})
	edits, err := expressionEdits(src, caseMode, rules)
	if err != nil {
		return cloneBytes(src), err
	}
	return applyEdits(src, edits), nil
}

func headerArgumentIssueWithSession(src []byte, start, end int, session *expressionSession, dialect Dialect) (headerArgumentIssue, bool) {
	for _, diagnostic := range headerArgumentDiagnosticsWithSession(src, start, end, session, dialect) {
		if diagnostic.severity == Warning {
			continue
		}
		return headerArgumentIssue{offset: diagnostic.offset, message: diagnostic.message}, true
	}
	return headerArgumentIssue{}, false
}

func linesOf(src []byte) []line {
	return linesOfWithSpace(src, false)
}

func validateHeader(head []byte) headerValidation {
	return validateHeaderWithSession(head, newExpressionSession(head))
}

func expressionEdits(src []byte, caseMode bool, rules ruleSet) ([]edit, error) {
	session := newExpressionSession(src)
	return expressionEditsWithSession(session, expressionSourceOriginal, src, 0, caseMode, rules, true)
}

func formFingerprintMode(src []byte, mode formMode) ([]byte, error) {
	session := newExpressionSession(src)
	return session.formFingerprintAt(expressionSourceOriginal, 0, len(src), mode)
}

func linesOfWithSpace(src []byte, allowFullwidth bool) []line {
	return makeLinesWithSpace(src, allowFullwidth, 0)
}

func cloneBytes(src []byte) []byte {
	return append([]byte(nil), src...)
}

func lexExpression(src []byte, base int) ([]exprToken, error) {
	session := newExpressionSession(src)
	return session.lexRange(expressionSourceOriginal, 0, len(src), base)
}
