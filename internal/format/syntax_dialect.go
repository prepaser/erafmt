package format

import "bytes"

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

func commandAvailability(command string, dialect Dialect) (known, available bool) {
	if command == "OUTPUTLOG" {
		return true, true
	}
	if mask, ok := emeeOnlyInstructions[command]; ok {
		return true, registeredDialect(mask, dialect)
	}
	if spec, ok := registeredCommandSpecs[command]; ok {
		return true, registeredDialect(spec.dialects, dialect)
	}
	if mask, ok := builtinMethodMasks[command]; ok {
		return true, methodDialect(mask, dialect)
	}
	return false, false
}

func commandUnavailable(command string, dialect Dialect) bool {
	known, available := commandAvailability(command, dialect)
	return known && !available
}

func (p *formatter) unavailableCommandAssignment(head []byte, command string) (assignmentOp, bool) {
	if _, ok := emeeOnlyInstructions[command]; !ok {
		return assignmentOp{}, false
	}
	start, op, ok := p.assignmentBoundary(head)
	if !ok {
		return assignmentOp{}, false
	}
	name, variable := p.variableLHS(head[:start])
	if !variable || !bytes.EqualFold([]byte(name), []byte(command)) {
		return assignmentOp{}, false
	}
	return op, true
}
