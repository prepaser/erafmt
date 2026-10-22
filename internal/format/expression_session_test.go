package format

import (
	"strconv"
	"strings"
	"testing"
)

func BenchmarkFormIslands(b *testing.B) {
	for _, fields := range []int{8000, 32000} {
		b.Run(strconv.Itoa(fields), func(b *testing.B) {
			source := []byte("@F\nPRINTFORM " + strings.Repeat("{1+2}", fields) + "\n")
			options := Options{Select: []RuleID{BinarySpacing}}
			for b.Loop() {
				result := FormatWithOptions(source, ERB, options)
				if len(result.Diagnostics) != 0 || len(result.Changes) == 0 {
					b.Fatalf("diagnostics=%+v changes=%d", result.Diagnostics, len(result.Changes))
				}
			}
		})
	}
}
