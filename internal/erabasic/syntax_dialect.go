package erabasic

const emeeOnlyInstructionMask = dialectEMEEMask

var emeeOnlyInstructions = map[string]uint8{
	"CALLSHARP":              emeeOnlyInstructionMask,
	"FORCE_QUIT":             emeeOnlyInstructionMask,
	"FORCE_QUIT_AND_RESTART": emeeOnlyInstructionMask,
	"QUIT_AND_RESTART":       emeeOnlyInstructionMask,
	"SKIPLOG":                emeeOnlyInstructionMask,
	"TRYCALLF":               emeeOnlyInstructionMask,
	"TRYCALLFORMF":           emeeOnlyInstructionMask,
}

func EmeeOnlyInstruction(command string) bool {
	_, ok := emeeOnlyInstructions[command]
	return ok
}

func commandAvailability(command string, dialect Dialect) (known, available bool) {
	if command == "OUTPUTLOG" {
		return true, true
	}
	if mask, ok := emeeOnlyInstructions[command]; ok {
		return true, registeredDialect(mask, dialect)
	}
	if spec, ok := registeredCommandSpecs[command]; ok {
		return true, registeredDialect(spec.Dialects, dialect)
	}
	if mask, ok := builtinMethodMasks[command]; ok {
		return true, methodDialect(mask, dialect)
	}
	return false, false
}

func CommandUnavailable(command string, dialect Dialect) bool {
	known, available := commandAvailability(command, dialect)
	return known && !available
}
