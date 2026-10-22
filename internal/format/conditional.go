package format

import "github.com/prepaser/erafmt/internal/erabasic"

func (p *formatter) optionalSIFPair(first int, head []byte, prefix int) bool {
	return p.preprocessorContext().OptionalSIFPair(first, head, prefix)
}

func (p *formatter) branchesConverge(i int, branches []erabasic.State) bool {
	return p.preprocessorContext().BranchesConverge(i, branches)
}

func (p *formatter) consumeConvergenceWork(states []erabasic.State, line int) bool {
	return p.preprocessorContext().ConsumeConvergenceWork(states, line)
}

func (p *formatter) headRole(head []byte) string {
	return p.preprocessorContext().HeadRole(head)
}

func (p *formatter) printVHeadCommand(head []byte) string {
	return p.preprocessorContext().PrintVHeadCommand(head)
}

func (p *formatter) headRoleAt(line, base int, head []byte) string {
	return p.preprocessorContext().HeadRoleAt(line, base, head)
}

func (p *formatter) assignmentBoundaryAt(line, base int, head []byte) (int, erabasic.AssignmentOp, bool) {
	return p.preprocessorContext().AssignmentBoundaryAt(line, base, head)
}

func (p *formatter) rawHeadRole(head []byte) string {
	return p.preprocessorContext().RawHeadRole(head)
}

func (p *formatter) rawPayloadCommand(command string) bool {
	return p.preprocessorContext().RawPayloadCommand(command)
}

func (p *formatter) unavailableAssignment(head []byte, name string) bool {
	return p.preprocessorContext().UnavailableAssignment(head, name)
}

func (p *formatter) unavailableMethodAssignment(head []byte, command string) (erabasic.AssignmentOp, bool) {
	return p.preprocessorContext().UnavailableMethodAssignment(head, command)
}
