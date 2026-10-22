package erabasic

import "slices"

// frame is the language-level state of one open block.
type frame struct {
	Kind          string
	ListCommand   string
	Line          int
	Branch        bool
	LastBranch    bool
	RepeatTrusted bool
	SelectorType  KnownValueType
	HasData       bool
	DataKnown     bool
}

func (f frame) depth() int {
	if f.Kind == "SELECTCASE" && f.Branch {
		return 2
	}
	return 1
}

type State struct {
	Frames         []frame
	Sif            int
	SifOrigin      int
	SifOriginKnown bool
	Declarations   bool
}

func (s State) Clone() State {
	s.Frames = slices.Clone(s.Frames)
	return s
}

func (s State) equal(other State) bool {
	if (s.Sif != 0) != (other.Sif != 0) || len(s.Frames) != len(other.Frames) {
		return false
	}
	for i, f := range s.Frames {
		g := other.Frames[i]
		if f.Kind != g.Kind || f.Branch != g.Branch || f.LastBranch != g.LastBranch {
			return false
		}
	}
	return true
}

func (s State) Depth() int {
	n := 0
	for _, f := range s.Frames {
		n += f.depth()
	}
	return n
}

func (s State) hasLoop() bool {
	for _, f := range s.Frames {
		switch f.Kind {
		case "REPEAT", "FOR", "WHILE", "DO":
			return true
		}
	}
	return false
}

var openers = map[string]string{
	"IF":           "IF",
	"SELECTCASE":   "SELECTCASE",
	"REPEAT":       "REPEAT",
	"FOR":          "FOR",
	"WHILE":        "WHILE",
	"DO":           "DO",
	"NOSKIP":       "NOSKIP",
	"TRYCCALL":     "TRYC",
	"TRYCJUMP":     "TRYC",
	"TRYCGOTO":     "TRYC",
	"TRYCCALLFORM": "TRYC",
	"TRYCJUMPFORM": "TRYC",
	"TRYCGOTOFORM": "TRYC",
	"PRINTDATA":    "DATA",
	"PRINTDATAL":   "DATA",
	"PRINTDATAW":   "DATA",
	"PRINTDATAK":   "DATA",
	"PRINTDATAKL":  "DATA",
	"PRINTDATAKW":  "DATA",
	"PRINTDATAD":   "DATA",
	"PRINTDATADL":  "DATA",
	"PRINTDATADW":  "DATA",
	"STRDATA":      "DATA",
	"DATALIST":     "DATALIST",
	"TRYCALLLIST":  "FUNCLIST",
	"TRYJUMPLIST":  "FUNCLIST",
	"TRYGOTOLIST":  "FUNCLIST",
}

var closers = map[string]string{
	"ENDIF":     "IF",
	"ENDSELECT": "SELECTCASE",
	"REND":      "REPEAT",
	"NEXT":      "FOR",
	"WEND":      "WHILE",
	"LOOP":      "DO",
	"ENDNOSKIP": "NOSKIP",
	"ENDCATCH":  "TRYC",
	"ENDDATA":   "DATA",
	"ENDLIST":   "DATALIST",
	"ENDFUNC":   "FUNCLIST",
}

func Partial(command string) bool {
	return openers[command] != "" || closers[command] != "" || slices.Contains([]string{"SIF", "ELSE", "ELSEIF", "CASE", "CASEELSE", "CATCH", "DATA", "DATAFORM", "FUNC"}, command)
}

func OpenerKind(command string) string {
	return openers[command]
}

func ExpressionCommand(command string) bool {
	return slices.Contains([]string{"IF", "ELSEIF", "SIF", "WHILE", "REPEAT", "LOOP", "SELECTCASE", "CASE"}, command)
}

func (s *State) Instruction(command string, number int) (int, string) {
	depth := s.Depth()
	if len(s.Frames) > 0 {
		top := s.Frames[len(s.Frames)-1]
		var allowed []string
		switch top.Kind {
		case "SELECTCASE":
			if !top.Branch {
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
			return depth, "instruction is not allowed in " + top.Kind
		}
	}
	if s.Sif != 0 {
		if Partial(command) {
			return depth, "SIF cannot target " + command
		}
		s.Sif = 0
		s.SifOrigin = 0
		s.SifOriginKnown = true
		depth++
	}
	if command == "SIF" {
		s.Sif = number
		s.SifOrigin = number
		s.SifOriginKnown = true
		return depth, ""
	}
	if command == "ELSE" || command == "ELSEIF" || command == "CASE" || command == "CASEELSE" || command == "CATCH" {
		expected := "IF"
		if command == "CASE" || command == "CASEELSE" {
			expected = "SELECTCASE"
		} else if command == "CATCH" {
			expected = "TRYC"
		}
		if len(s.Frames) == 0 || s.Frames[len(s.Frames)-1].Kind != expected {
			return depth, command + " has no matching opener"
		}
		f := &s.Frames[len(s.Frames)-1]
		if command == "CATCH" && f.Branch {
			return depth, command + " follows a final branch"
		}
		depth -= f.depth()
		if expected == "SELECTCASE" {
			depth++
		}
		if expected != "IF" {
			f.Branch = true
		}
		f.LastBranch = command == "ELSE" || command == "CASEELSE"
		return depth, ""
	}
	if expected := closers[command]; expected != "" {
		if len(s.Frames) == 0 || s.Frames[len(s.Frames)-1].Kind != expected {
			return depth, command + " has no matching opener"
		}
		f := s.Frames[len(s.Frames)-1]
		if command == "ENDCATCH" && !f.Branch {
			return depth, "ENDCATCH has no matching CATCH"
		}
		s.Frames = s.Frames[:len(s.Frames)-1]
		return depth - f.depth(), ""
	}
	if command == "DATA" || command == "DATAFORM" || command == "FUNC" {
		if len(s.Frames) == 0 {
			return depth, command + " is outside its block"
		}
		top := s.Frames[len(s.Frames)-1].Kind
		if (command == "FUNC" && top != "FUNCLIST") || (command != "FUNC" && top != "DATA" && top != "DATALIST") {
			return depth, command + " is outside its block"
		}
	}
	if kind := openers[command]; kind != "" {
		if kind == "DATALIST" && (len(s.Frames) == 0 || s.Frames[len(s.Frames)-1].Kind != "DATA") {
			return depth, "DATALIST is outside a data block"
		}
		if kind == "NOSKIP" || kind == "FUNCLIST" {
			for _, f := range s.Frames {
				if f.Kind == kind {
					return depth, "nested " + kind + " is not supported by the runtime"
				}
			}
		}
		opened := frame{Kind: kind, Line: number, DataKnown: true}
		if kind == "FUNCLIST" {
			opened.ListCommand = command
		}
		s.Frames = append(s.Frames, opened)
	}
	return depth, ""
}

func (s State) branchAfterFinal(command string) bool {
	if len(s.Frames) == 0 {
		return false
	}
	f := s.Frames[len(s.Frames)-1]
	if !f.LastBranch {
		return false
	}
	switch f.Kind {
	case "IF":
		return command == "ELSE" || command == "ELSEIF"
	case "SELECTCASE":
		return command == "CASE" || command == "CASEELSE"
	default:
		return false
	}
}

func (s State) hasTrustedRepeat() bool {
	for _, f := range s.Frames {
		if f.Kind == "REPEAT" && f.RepeatTrusted {
			return true
		}
	}
	return false
}

func mergeBranchState(states []State) State {
	merged := states[0].Clone()
	if len(states) > 1 && merged.Sif != 0 {
		for _, branch := range states[1:] {
			if branch.Sif != merged.Sif || branch.SifOrigin != merged.SifOrigin || branch.SifOriginKnown != merged.SifOriginKnown {
				merged.SifOrigin = 0
				merged.SifOriginKnown = false
				break
			}
		}
	}
	for i, frame := range merged.Frames {
		if frame.Kind == "REPEAT" && frame.RepeatTrusted {
			for _, branch := range states[1:] {
				if !branch.Frames[i].RepeatTrusted {
					merged.Frames[i].RepeatTrusted = false
					break
				}
			}
		}
		selector := frame.SelectorType
		for _, branch := range states[1:] {
			if branch.Frames[i].SelectorType != selector {
				selector = KnownValueUnknown
				break
			}
		}
		merged.Frames[i].SelectorType = selector
		if frame.ListCommand != "" {
			for _, branch := range states[1:] {
				if branch.Frames[i].ListCommand != frame.ListCommand {
					merged.Frames[i].ListCommand = ""
					break
				}
			}
		} else {
			merged.Frames[i].ListCommand = ""
		}
		known, present := frame.DataKnown, frame.HasData
		for _, branch := range states[1:] {
			other := branch.Frames[i]
			if !known || !other.DataKnown || present != other.HasData {
				known = false
				present = false
				break
			}
		}
		merged.Frames[i].DataKnown = known
		merged.Frames[i].HasData = present
	}
	return merged
}

func (s *State) SetSelectorType(value KnownValueType) {
	if len(s.Frames) == 0 || s.Frames[len(s.Frames)-1].Kind != "SELECTCASE" {
		return
	}
	s.Frames[len(s.Frames)-1].SelectorType = value
}

func (s *State) markDataOpener(command string, trusted bool) {
	kind := openers[command]
	if kind != "DATA" && kind != "DATALIST" || len(s.Frames) == 0 {
		return
	}
	current := &s.Frames[len(s.Frames)-1]
	if !trusted {
		current.DataKnown = false
	}
	if kind == "DATALIST" && len(s.Frames) > 1 && !trusted {
		parent := &s.Frames[len(s.Frames)-2]
		if parent.Kind == "DATA" {
			parent.DataKnown = false
		}
	}
}

func (s *State) markData(command string, trusted bool) {
	if command != "DATA" && command != "DATAFORM" || len(s.Frames) == 0 {
		return
	}
	current := &s.Frames[len(s.Frames)-1]
	if current.Kind != "DATA" && current.Kind != "DATALIST" {
		return
	}
	if !trusted {
		current.DataKnown = false
		return
	}
	if current.DataKnown {
		current.HasData = true
	}
}

func (s *State) MarkCurrentDataUncertain() {
	if len(s.Frames) == 0 {
		return
	}
	current := &s.Frames[len(s.Frames)-1]
	if current.Kind == "DATA" || current.Kind == "DATALIST" {
		current.DataKnown = false
	}
}

func (s State) emptyData(command string) bool {
	if len(s.Frames) == 0 {
		return false
	}
	frame := s.Frames[len(s.Frames)-1]
	if command == "ENDDATA" && frame.Kind == "DATA" || command == "ENDLIST" && frame.Kind == "DATALIST" {
		return frame.DataKnown && !frame.HasData
	}
	return false
}

func (s *State) finishData(command string, trusted bool) {
	if command != "ENDLIST" || len(s.Frames) < 2 {
		return
	}
	child := s.Frames[len(s.Frames)-1]
	parent := &s.Frames[len(s.Frames)-2]
	if child.Kind != "DATALIST" || parent.Kind != "DATA" {
		return
	}
	if !trusted || !child.DataKnown || !parent.DataKnown {
		parent.DataKnown = false
		return
	}
	parent.HasData = true
}
