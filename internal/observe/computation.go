package observe

import "github.com/vinnyp/press-quality-lab/internal/strictjson"

// ComputedValueDepth is QA6's depth inside a judge input evidence item.
const ComputedValueDepth = 61

// ComputedValueBytes is QA6's inclusive output bound.
const ComputedValueBytes = 1 << 20

// ParseComputed applies QA6 to drained bytes, keeping valid JSON null distinct
// from an invalid value through its boolean result. The darwin source must pass
// cut || Run.OutputCutAfterExit as cut.
func ParseComputed(data []byte, cut bool, count int64) (any, bool) {
	if cut || count < 0 || count > ComputedValueBytes || int64(len(data)) != count {
		return nil, false
	}
	return strictjson.Read(data, strictjson.Bounds{Bytes: ComputedValueBytes, Depth: ComputedValueDepth, NumberChars: 32, ExponentDigits: 4}, nil)
}
