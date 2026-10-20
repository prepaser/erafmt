package format

const (
	maxExpressionTokens     = 1 << 16
	maxSessionTokens        = 1 << 18
	maxExpressionDepth      = 512
	maxFormDepth            = 128
	maxFormFingerprintBytes = 4 << 20
	maxPreprocessorStates   = 1 << 16
)

func (s *expressionSession) resource(offset int, message string) *exprError {
	if s.exhausted == nil {
		s.exhausted = &exprError{offset: offset, msg: message, resource: true}
		if s.onResource != nil {
			s.onResource(offset, message)
		}
	}
	return s.exhausted
}

func (s *expressionSession) enterForm(offset int) *exprError {
	if s.exhausted != nil {
		return s.exhausted
	}
	if s.formDepth >= maxFormDepth {
		return s.resource(offset, "FORM nesting exceeds the formatter resource limit")
	}
	s.formDepth++
	return nil
}

func (s *expressionSession) leaveForm() { s.formDepth-- }

func (s *expressionSession) checkFormFingerprint(offset, current, addition int) *exprError {
	if s.exhausted != nil {
		return s.exhausted
	}
	if !appendWithinLimit(current, addition, maxFormFingerprintBytes) {
		return s.resource(offset, "FORM fingerprint work exceeds the formatter resource limit")
	}
	return nil
}

func (s *expressionSession) reserveFormFingerprint(offset, size int) *exprError {
	if err := s.checkFormFingerprint(offset, s.formFingerprintBytes, size); err != nil {
		return err
	}
	s.formFingerprintBytes += size
	return nil
}

func (p *formatter) syntaxResource(offset int, message string) {
	if len(p.parseSegments) != 0 {
		segment, local := continuationDiagnosticSegment(p.parseBase+offset, p.parseSegments)
		if segment >= 0 {
			s := p.parseSegments[segment]
			p.resourceProblem(s.line, min(s.sourceEnd, max(s.sourceStart, s.sourceStart+local)), message)
			return
		}
	}
	p.resourceProblem(p.parseLine, p.parseBase+offset, message)
}

func (p *formatter) retainPreprocessorState(i int, f *ppFrame, s state) bool {
	units := 1 + len(s.frames)
	if !appendWithinLimit(p.ppStateUnits, units, maxPreprocessorStates) {
		p.statementResourceProblem(i, p.lines[i].indentEnd, "preprocessor state work exceeds the formatter resource limit")
		return false
	}
	p.ppStateUnits += units
	f.stateUnits += units
	return true
}
