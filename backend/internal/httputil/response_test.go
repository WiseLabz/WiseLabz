package httputil

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestPaginateClampsOverflowingPage guards against issue #150: an
// unbounded ?page= value multiplied by pageSize overflowed int and
// produced a negative offset.
func TestPaginateClampsOverflowingPage(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?page=9223372036854775807&pageSize=100", nil)

	page, pageSize, offset := Paginate(r)

	if offset < 0 {
		t.Fatalf("Paginate() offset = %d, want non-negative", offset)
	}
	if page > MaxPage {
		t.Errorf("Paginate() page = %d, want <= MaxPage (%d)", page, MaxPage)
	}
	if pageSize != 100 {
		t.Errorf("Paginate() pageSize = %d, want 100", pageSize)
	}
}
