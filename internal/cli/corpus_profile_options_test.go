package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/prepaser/erafmt/internal/format"
)

func corpusDialect() (format.Dialect, error) {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("ERAFMT_CORPUS_DIALECT"))) {
	case "", "common":
		return format.CommonDialect, nil
	case "emuera1824":
		return format.Emuera1824, nil
	case "emee":
		return format.EMEE, nil
	default:
		return format.CommonDialect, fmt.Errorf("ERAFMT_CORPUS_DIALECT must be common, emuera1824, or emee")
	}
}

func reportDialect(dialect format.Dialect) string {
	switch dialect {
	case format.Emuera1824:
		return "emuera1824"
	case format.EMEE:
		return "emee"
	default:
		return "common"
	}
}

func corpusInputEncoding() (inputEncodingKind, string, error) {
	value := strings.TrimSpace(os.Getenv("ERAFMT_CORPUS_INPUT_ENCODING"))
	if value == "" {
		value = "auto"
	}
	kind, err := parseInputEncoding(value)
	if err != nil {
		return kind, value, err
	}
	switch kind {
	case inputEncodingCP932:
		return kind, "cp932", nil
	case inputEncodingCP932Fallback:
		return kind, "cp932-fallback", nil
	default:
		return kind, "auto", nil
	}
}

func corpusAllowFullwidthSpace() (bool, error) {
	value := strings.TrimSpace(os.Getenv("ERAFMT_CORPUS_ALLOW_FULLWIDTH_SPACE"))
	if value == "" {
		return false, nil
	}
	allow, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("ERAFMT_CORPUS_ALLOW_FULLWIDTH_SPACE must be a boolean")
	}
	return allow, nil
}
