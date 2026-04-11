package unit_tests

import (
	"testing"

	"campusrec/internal/handlers"
)

func TestPaginatedResponse(t *testing.T) {
	items := []string{"a", "b", "c"}
	p := handlers.PaginationParams{Page: 1, PageSize: 20, Offset: 0}
	result := handlers.PaginatedResponse(items, 50, p)

	if result["total"] != 50 {
		t.Errorf("total = %v, want 50", result["total"])
	}
	if result["page"] != 1 {
		t.Errorf("page = %v, want 1", result["page"])
	}
	if result["page_size"] != 20 {
		t.Errorf("page_size = %v, want 20", result["page_size"])
	}
	if result["total_pages"] != 3 {
		t.Errorf("total_pages = %v, want 3", result["total_pages"])
	}
}

func TestPaginatedResponseSinglePage(t *testing.T) {
	p := handlers.PaginationParams{Page: 1, PageSize: 20, Offset: 0}
	result := handlers.PaginatedResponse([]string{}, 5, p)

	if result["total_pages"] != 1 {
		t.Errorf("total_pages = %v, want 1", result["total_pages"])
	}
}

func TestPaginatedResponseExactFit(t *testing.T) {
	p := handlers.PaginationParams{Page: 1, PageSize: 10, Offset: 0}
	result := handlers.PaginatedResponse(nil, 30, p)

	if result["total_pages"] != 3 {
		t.Errorf("total_pages = %v, want 3", result["total_pages"])
	}
}

func TestPaginatedResponseEmptyResult(t *testing.T) {
	p := handlers.PaginationParams{Page: 1, PageSize: 20, Offset: 0}
	result := handlers.PaginatedResponse([]string{}, 0, p)

	if result["total_pages"] != 0 {
		t.Errorf("total_pages = %v, want 0", result["total_pages"])
	}
	if result["total"] != 0 {
		t.Errorf("total = %v, want 0", result["total"])
	}
}

// TestPaginatedResponseTotalPagesEdgeCases verifies total_pages calculation
// using the real PaginatedResponse function for boundary conditions.
func TestPaginatedResponseTotalPagesEdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		total      int
		pageSize   int
		wantPages  int
	}{
		{"zero items", 0, 20, 0},
		{"one item", 1, 20, 1},
		{"exactly one page", 20, 20, 1},
		{"one over page boundary", 21, 20, 2},
		{"large dataset", 1000, 50, 20},
		{"page size 1", 5, 1, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := handlers.PaginationParams{Page: 1, PageSize: tt.pageSize, Offset: 0}
			result := handlers.PaginatedResponse(nil, tt.total, p)
			if result["total_pages"] != tt.wantPages {
				t.Errorf("total_pages = %v, want %d", result["total_pages"], tt.wantPages)
			}
		})
	}
}
