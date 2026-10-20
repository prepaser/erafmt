package format

import "slices"

type frame struct {
	kind          string
	listCommand   string
	line          int
	branch        bool
	lastBranch    bool
	repeatTrusted bool
	selectorType  knownValueType
	hasData       bool
	dataKnown     bool
}

func (f frame) depth() int {
	if f.kind == "SELECTCASE" && f.branch {
		return 2
	}
	return 1
}

type state struct {
	frames         []frame
	sif            int
	sifOrigin      int
	sifOriginKnown bool
	declarations   bool
}

func (s state) clone() state {
	s.frames = slices.Clone(s.frames)
	return s
}

func (s state) equal(other state) bool {
	if (s.sif != 0) != (other.sif != 0) || len(s.frames) != len(other.frames) {
		return false
	}
	for i, f := range s.frames {
		g := other.frames[i]
		if f.kind != g.kind || f.branch != g.branch || f.lastBranch != g.lastBranch {
			return false
		}
	}
	return true
}

func (s state) depth() int {
	n := 0
	for _, f := range s.frames {
		n += f.depth()
	}
	return n
}

func (s state) hasLoop() bool {
	for _, f := range s.frames {
		switch f.kind {
		case "REPEAT", "FOR", "WHILE", "DO":
			return true
		}
	}
	return false
}

var openers = map[string]string{
	"IF": "IF", "SELECTCASE": "SELECTCASE", "REPEAT": "REPEAT", "FOR": "FOR",
	"WHILE": "WHILE", "DO": "DO", "NOSKIP": "NOSKIP",
	"TRYCCALL": "TRYC", "TRYCJUMP": "TRYC", "TRYCGOTO": "TRYC",
	"TRYCCALLFORM": "TRYC", "TRYCJUMPFORM": "TRYC", "TRYCGOTOFORM": "TRYC",
	"PRINTDATA": "DATA", "PRINTDATAL": "DATA", "PRINTDATAW": "DATA",
	"PRINTDATAK": "DATA", "PRINTDATAKL": "DATA", "PRINTDATAKW": "DATA",
	"PRINTDATAD": "DATA", "PRINTDATADL": "DATA", "PRINTDATADW": "DATA",
	"STRDATA": "DATA", "DATALIST": "DATALIST",
	"TRYCALLLIST": "FUNCLIST", "TRYJUMPLIST": "FUNCLIST", "TRYGOTOLIST": "FUNCLIST",
}

var closers = map[string]string{
	"ENDIF": "IF", "ENDSELECT": "SELECTCASE", "REND": "REPEAT", "NEXT": "FOR",
	"WEND": "WHILE", "LOOP": "DO", "ENDNOSKIP": "NOSKIP", "ENDCATCH": "TRYC",
	"ENDDATA": "DATA", "ENDLIST": "DATALIST", "ENDFUNC": "FUNCLIST",
}

func partial(command string) bool {
	return openers[command] != "" || closers[command] != "" || slices.Contains(
		[]string{"SIF", "ELSE", "ELSEIF", "CASE", "CASEELSE", "CATCH", "DATA", "DATAFORM", "FUNC"}, command)
}

func expressionCommand(command string) bool {
	return slices.Contains([]string{"IF", "ELSEIF", "SIF", "WHILE", "REPEAT", "LOOP", "SELECTCASE", "CASE"}, command)
}

func (s *state) instruction(command string, number int) (int, string) {
	depth := s.depth()
	if len(s.frames) > 0 {
		top := s.frames[len(s.frames)-1]
		var allowed []string
		switch top.kind {
		case "SELECTCASE":
			if !top.branch {
				allowed = []string{"CASE", "CASEELSE", "ENDSELECT"}
			}
		case "DATA":
			allowed = []string{"DATA", "DATAFORM", "DATALIST", "ENDDATA"}
		case "DATALIST":
			allowed = []string{"DATA", "DATAFORM", "ENDLIST"}
		case "FUNCLIST":
			allowed = []string{"FUNC", "ENDFUNC"}
		}
		if allowed != nil && !slices.Contains(allowed, command) {
			return depth, "instruction is not allowed in " + top.kind
		}
	}
	if s.sif != 0 {
		if partial(command) {
			return depth, "SIF cannot target " + command
		}
		s.sif = 0
		s.sifOrigin = 0
		s.sifOriginKnown = true
		depth++
	}
	if command == "SIF" {
		s.sif = number
		s.sifOrigin = number
		s.sifOriginKnown = true
		return depth, ""
	}
	if command == "ELSE" || command == "ELSEIF" || command == "CASE" || command == "CASEELSE" || command == "CATCH" {
		expected := "IF"
		if command == "CASE" || command == "CASEELSE" {
			expected = "SELECTCASE"
		} else if command == "CATCH" {
			expected = "TRYC"
		}
		if len(s.frames) == 0 || s.frames[len(s.frames)-1].kind != expected {
			return depth, command + " has no matching opener"
		}
		f := &s.frames[len(s.frames)-1]
		if command == "CATCH" && f.branch {
			return depth, command + " follows a final branch"
		}
		depth -= f.depth()
		if expected == "SELECTCASE" {
			depth++
		}
		if expected != "IF" {
			f.branch = true
		}
		f.lastBranch = command == "ELSE" || command == "CASEELSE"
		return depth, ""
	}
	if expected := closers[command]; expected != "" {
		if len(s.frames) == 0 || s.frames[len(s.frames)-1].kind != expected {
			return depth, command + " has no matching opener"
		}
		f := s.frames[len(s.frames)-1]
		if command == "ENDCATCH" && !f.branch {
			return depth, "ENDCATCH has no matching CATCH"
		}
		s.frames = s.frames[:len(s.frames)-1]
		return depth - f.depth(), ""
	}
	if command == "DATA" || command == "DATAFORM" || command == "FUNC" {
		if len(s.frames) == 0 {
			return depth, command + " is outside its block"
		}
		top := s.frames[len(s.frames)-1].kind
		if (command == "FUNC" && top != "FUNCLIST") || (command != "FUNC" && top != "DATA" && top != "DATALIST") {
			return depth, command + " is outside its block"
		}
	}
	if kind := openers[command]; kind != "" {
		if kind == "DATALIST" && (len(s.frames) == 0 || s.frames[len(s.frames)-1].kind != "DATA") {
			return depth, "DATALIST is outside a data block"
		}
		if kind == "NOSKIP" || kind == "FUNCLIST" {
			for _, f := range s.frames {
				if f.kind == kind {
					return depth, "nested " + kind + " is not supported by the runtime"
				}
			}
		}
		opened := frame{kind: kind, line: number, dataKnown: true}
		if kind == "FUNCLIST" {
			opened.listCommand = command
		}
		s.frames = append(s.frames, opened)
	}
	return depth, ""
}

func (s state) branchAfterFinal(command string) bool {
	if len(s.frames) == 0 {
		return false
	}
	f := s.frames[len(s.frames)-1]
	if !f.lastBranch {
		return false
	}
	switch f.kind {
	case "IF":
		return command == "ELSE" || command == "ELSEIF"
	case "SELECTCASE":
		return command == "CASE" || command == "CASEELSE"
	default:
		return false
	}
}

func (s state) hasTrustedRepeat() bool {
	for _, f := range s.frames {
		if f.kind == "REPEAT" && f.repeatTrusted {
			return true
		}
	}
	return false
}

func mergeBranchState(states []state) state {
	merged := states[0].clone()
	if len(states) > 1 && merged.sif != 0 {
		for _, branch := range states[1:] {
			if branch.sif != merged.sif || branch.sifOrigin != merged.sifOrigin || branch.sifOriginKnown != merged.sifOriginKnown {
				merged.sifOrigin = 0
				merged.sifOriginKnown = false
				break
			}
		}
	}
	for i, frame := range merged.frames {
		if frame.kind == "REPEAT" && frame.repeatTrusted {
			for _, branch := range states[1:] {
				if !branch.frames[i].repeatTrusted {
					merged.frames[i].repeatTrusted = false
					break
				}
			}
		}
		selector := frame.selectorType
		for _, branch := range states[1:] {
			if branch.frames[i].selectorType != selector {
				selector = knownValueUnknown
				break
			}
		}
		merged.frames[i].selectorType = selector
		if frame.listCommand != "" {
			for _, branch := range states[1:] {
				if branch.frames[i].listCommand != frame.listCommand {
					merged.frames[i].listCommand = ""
					break
				}
			}
		} else {
			merged.frames[i].listCommand = ""
		}
		known, present := frame.dataKnown, frame.hasData
		for _, branch := range states[1:] {
			other := branch.frames[i]
			if !known || !other.dataKnown || present != other.hasData {
				known = false
				present = false
				break
			}
		}
		merged.frames[i].dataKnown = known
		merged.frames[i].hasData = present
	}
	return merged
}

func (s *state) setSelectorType(value knownValueType) {
	if len(s.frames) == 0 || s.frames[len(s.frames)-1].kind != "SELECTCASE" {
		return
	}
	s.frames[len(s.frames)-1].selectorType = value
}

func (s *state) markDataOpener(command string, trusted bool) {
	kind := openers[command]
	if kind != "DATA" && kind != "DATALIST" || len(s.frames) == 0 {
		return
	}
	current := &s.frames[len(s.frames)-1]
	if !trusted {
		current.dataKnown = false
	}
	if kind == "DATALIST" && len(s.frames) > 1 && !trusted {
		parent := &s.frames[len(s.frames)-2]
		if parent.kind == "DATA" {
			parent.dataKnown = false
		}
	}
}

func (s *state) markData(command string, trusted bool) {
	if command != "DATA" && command != "DATAFORM" || len(s.frames) == 0 {
		return
	}
	current := &s.frames[len(s.frames)-1]
	if current.kind != "DATA" && current.kind != "DATALIST" {
		return
	}
	if !trusted {
		current.dataKnown = false
		return
	}
	if current.dataKnown {
		current.hasData = true
	}
}

func (s *state) markCurrentDataUncertain() {
	if len(s.frames) == 0 {
		return
	}
	current := &s.frames[len(s.frames)-1]
	if current.kind == "DATA" || current.kind == "DATALIST" {
		current.dataKnown = false
	}
}

func (s state) emptyData(command string) bool {
	if len(s.frames) == 0 {
		return false
	}
	frame := s.frames[len(s.frames)-1]
	if command == "ENDDATA" && frame.kind == "DATA" || command == "ENDLIST" && frame.kind == "DATALIST" {
		return frame.dataKnown && !frame.hasData
	}
	return false
}

func (s *state) finishData(command string, trusted bool) {
	if command != "ENDLIST" || len(s.frames) < 2 {
		return
	}
	child := s.frames[len(s.frames)-1]
	parent := &s.frames[len(s.frames)-2]
	if child.kind != "DATALIST" || parent.kind != "DATA" {
		return
	}
	if !trusted || !child.dataKnown || !parent.dataKnown {
		parent.dataKnown = false
		return
	}
	parent.hasData = true
}
