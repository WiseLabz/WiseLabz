package connectortest

// JSONType maps a Go attribute value to the AttributeSpec type vocabulary.
func JSONType(v any) string {
	switch v.(type) {
	case bool:
		return "boolean"
	case string:
		return "string"
	case int, int64, float64:
		return "number"
	case []string:
		return "string_array"
	default:
		return "unknown"
	}
}
