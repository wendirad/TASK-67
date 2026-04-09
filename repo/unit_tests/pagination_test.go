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

func TestPaginationParamsOffset(t *testing.T) {
	tests := []struct {
		page     int
		pageSize int
		offset   int
	}{
		{1, 20, 0},
		{2, 20, 20},
		{3, 10, 20},
		{5, 50, 200},
	}
	for _, tt := range tests {
		p := handlers.PaginationParams{
			Page:     tt.page,
			PageSize: tt.pageSize,
			Offset:   (tt.page - 1) * tt.pageSize,
		}
		if p.Offset != tt.offset {
			t.Errorf("Page=%d, PageSize=%d: Offset = %d, want %d", tt.page, tt.pageSize, p.Offset, tt.offset)
		}
	}
}
