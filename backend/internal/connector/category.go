package connector

// categories is the single list of valid connector categories, in canonical
// order. It must match the connectors.category CHECK constraint (migration
// 000064, both dialects) and the OpenAPI ConnectorCategory enum; the backup
// tests pin both.
var categories = []string{
	"virtualization",
	"containers_paas",
	"networking",
	"dns",
	"storage",
	"monitoring",
	"media",
	"other",
}

var validCategories = func() map[string]bool {
	set := make(map[string]bool, len(categories))
	for _, c := range categories {
		set[c] = true
	}
	return set
}()

// Categories returns a copy of the valid connector categories in canonical order.
func Categories() []string {
	return append([]string(nil), categories...)
}

// ValidCategory reports whether category is one of the valid connector categories.
func ValidCategory(category string) bool {
	return validCategories[category]
}
