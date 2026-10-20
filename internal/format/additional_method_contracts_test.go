package format

import "testing"

func TestAdditionalXMLMethodContracts(t *testing.T) {
	tests := []struct {
		name string
		src  string
		bad  bool
	}{
		{"regexp-two", "REGEXPMATCH \"x\",\"x\"", false},
		{"regexp-three", "REGEXPMATCH \"x\",\"x\",1", false},
		{"regexp-four", "REGEXPMATCH \"x\",\"x\",RESULT,RESULTS", false},
		{"regexp-indexed-output", "REGEXPMATCH \"x\",\"x\",RESULT,RESULTS:0", false},
		{"regexp-bad-output", "REGEXPMATCH \"x\",\"x\",RESULTS,RESULT", true},
		{"regexp-extra", "REGEXPMATCH \"x\",\"x\",1,RESULT,RESULTS", true},
		{"regexp-bad-third", "REGEXPMATCH \"x\",\"x\",1,RESULT", true},
		{"select-one", "DT_SELECT \"x\"", false},
		{"select-three", "DT_SELECT \"x\",\"f\",\"s\"", false},
		{"select-output", "DT_SELECT \"x\",\"f\",\"s\",RESULT", false},
		{"select-parenthesized-output", "DT_SELECT \"x\",\"f\",\"s\",(RESULT)", false},
		{"select-bad-output-literal", "DT_SELECT \"x\",\"f\",\"s\",\"bad\"", true},
		{"select-bad-output-string-array", "DT_SELECT \"x\",\"f\",\"s\",RESULTS", true},
		{"select-indexed-output", "DT_SELECT \"x\",\"f\",\"s\",RESULT:0", false},
		{"select-private-output-defer", "DT_SELECT \"x\",\"f\",\"s\",PRIVATE:0", false},
		{"select-scoped-output-defer", "DT_SELECT \"x\",\"f\",\"s\",LOCAL@F:0", false},
		{"select-terminal-comma", "DT_SELECT \"x\",\"f\",\"s\",", false},
		{"select-omitted-output", "DT_SELECT \"x\",\"f\",\"s\",,", true},
		{"xml-get-two", "XML_GET 1,\"/x\"", false},
		{"xml-get-terminal-comma", "XML_GET 1,\"/x\",", false},
		{"xml-get-int-output", "XML_GET \"x\",\"/x\",1", false},
		{"xml-get-string-output", "XML_GET \"x\",\"/x\",RESULTS", false},
		{"xml-get-indexed-string-output", "XML_GET \"x\",\"/x\",RESULTS:0", false},
		{"xml-get-omitted-output", "XML_GET \"x\",\"/x\",,1", true},
		{"xml-get-omitted-target", "XML_GET ,\"/x\"", true},
		{"xml-get-byname", "XML_GET_BYNAME \"id\",\"/x\",RESULTS,1", false},
		{"xml-get-byname-omitted-target", "XML_GET_BYNAME ,\"/x\"", true},
		{"xml-get-bad-path", "XML_GET 1,1", true},
		{"xml-get-bad-output", "XML_GET 1,\"/x\",\"bad\"", true},
		{"xml-get-bad-style", "XML_GET 1,\"/x\",RESULTS,\"x\"", true},
		{"xml-get-extra", "XML_GET 1,\"/x\",1,2,3", true},
		{"xml-add-node", "XML_ADDNODE 1,\"/x\",\"<y/>\",0,1", false},
		{"xml-add-node-terminal-comma", "XML_ADDNODE 1,\"/x\",\"<y/>\",", false},
		{"xml-add-node-omitted-flag", "XML_ADDNODE 1,\"/x\",\"<y/>\",,1", true},
		{"xml-add-node-byname", "XML_ADDNODE_BYNAME \"id\",\"/x\",\"<y/>\",0,1", false},
		{"xml-add-node-bad-target", "XML_ADDNODE \"xml\",\"/x\",\"<y/>\"", true},
		{"xml-add-node-bad-path", "XML_ADDNODE 1,1,\"<y/>\"", true},
		{"xml-remove-node", "XML_REMOVENODE 1,\"/x\",1", false},
		{"xml-remove-node-terminal-comma", "XML_REMOVENODE 1,\"/x\",", false},
		{"xml-remove-node-omitted-flag", "XML_REMOVENODE 1,\"/x\",,1", true},
		{"xml-remove-node-byname", "XML_REMOVENODE_BYNAME \"id\",\"/x\",1", false},
		{"xml-remove-node-bad-target", "XML_REMOVENODE \"xml\",\"/x\"", true},
		{"xml-remove-node-bad-arity", "XML_REMOVENODE 1", true},
		{"xml-unknown-target-defer", "XML_ADDNODE UNKNOWN,\"/x\",\"<y/>\"", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := FormatWithOptions([]byte("@F\n"+tt.src+"\n"), ERB, Options{Dialect: EMEE})
			if (len(r.Diagnostics) != 0) != tt.bad {
				t.Fatalf("diagnostics=%+v bad=%v", r.Diagnostics, tt.bad)
			}
			if tt.bad {
				return
			}
			again := FormatWithOptions(r.Output, ERB, Options{Dialect: EMEE})
			if string(again.Output) != string(r.Output) || len(again.Changes) != 0 {
				t.Fatalf("not idempotent: %+v", again)
			}
		})
	}
}
