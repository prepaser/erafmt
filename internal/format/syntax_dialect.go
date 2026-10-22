package format

import (
	"bytes"

	"github.com/prepaser/erafmt/internal/erabasic"
)

func (p *formatter) unavailableCommandAssignment(head []byte, command string) (erabasic.AssignmentOp, bool) {
	if !erabasic.EmeeOnlyInstruction(command) {
		return erabasic.AssignmentOp{}, false
	}
	start, op, ok := p.assignmentBoundary(head)
	if !ok {
		return erabasic.AssignmentOp{}, false
	}
	name, variable := p.variableLHS(head[:start])
	if !variable || !bytes.EqualFold([]byte(name), []byte(command)) {
		return erabasic.AssignmentOp{}, false
	}
	return op, true
}
