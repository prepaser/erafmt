package erabasic

const (
	MaxExpressionTokens     = 1 << 16
	maxSessionTokens        = 1 << 18
	MaxExpressionDepth      = 512
	MaxFormDepth            = 128
	maxFormFingerprintBytes = 4 << 20
	MaxPreprocessorStates   = 1 << 16
)

func (s *ExpressionSession) resource(offset int, message string) *ExprError {
	if s.Exhausted == nil {
		s.Exhausted = &ExprError{Offset: offset, Msg: message, Resource: true}
		if s.OnResource != nil {
			s.OnResource(offset, message)
		}
	}
	return s.Exhausted
}

func (s *ExpressionSession) enterForm(offset int) *ExprError {
	if s.Exhausted != nil {
		return s.Exhausted
	}
	if s.FormDepth >= MaxFormDepth {
		return s.resource(offset, "FORM nesting exceeds the formatter resource limit")
	}
	s.FormDepth++
	return nil
}

func (s *ExpressionSession) leaveForm() {
	s.FormDepth--
}

func (s *ExpressionSession) checkFormFingerprint(offset, current, addition int) *ExprError {
	if s.Exhausted != nil {
		return s.Exhausted
	}
	if !appendWithinLimit(current, addition, maxFormFingerprintBytes) {
		return s.resource(offset, "FORM fingerprint work exceeds the formatter resource limit")
	}
	return nil
}

func (s *ExpressionSession) reserveFormFingerprint(offset, size int) *ExprError {
	if err := s.checkFormFingerprint(offset, s.FormFingerprintBytes, size); err != nil {
		return err
	}
	s.FormFingerprintBytes += size
	return nil
}
