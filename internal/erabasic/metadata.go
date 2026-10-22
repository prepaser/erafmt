package erabasic

const (
	MaxMetadataBytes  = 32 << 20
	LineMetadataBytes = 128
)

type MetadataBudget struct{ used int }

func NewMetadataBudget(used int) *MetadataBudget {
	return &MetadataBudget{used: used}
}

func (b *MetadataBudget) Used() int {
	return b.used
}

func (b *MetadataBudget) Reserve(size int) bool {
	if !appendWithinLimit(b.used, size, MaxMetadataBytes) {
		return false
	}
	b.used += size
	return true
}

func SourceLineCount(src []byte, limit int) (int, bool) {
	count := 0
	for pos := 0; pos < len(src); {
		if count == limit {
			return count, false
		}
		count++
		for pos < len(src) && src[pos] != '\r' && src[pos] != '\n' {
			pos++
		}
		if pos < len(src) {
			cr := src[pos] == '\r'
			pos++
			if cr && pos < len(src) && src[pos] == '\n' {
				pos++
			}
		}
	}
	return count, true
}
