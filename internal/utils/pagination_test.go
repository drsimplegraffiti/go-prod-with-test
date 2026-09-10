package utils

import (
	"net/url"
	"testing"
)

func TestParsePage(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want int
	}{
		{"missing defaults to 1", "", 1},
		{"valid value", "page=3", 3},
		{"zero falls back to default", "page=0", 1},
		{"negative falls back to default", "page=-5", 1},
		{"non-numeric falls back to default", "page=abc", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q, _ := url.ParseQuery(tc.raw)
			if got := ParsePage(q); got != tc.want {
				t.Errorf("ParsePage(%q) = %d, want %d", tc.raw, got, tc.want)
			}
		})
	}
}

func TestParsePageSize(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want int
	}{
		{"missing defaults to 20", "", DefaultPageSize},
		{"valid value", "page_size=50", 50},
		{"zero falls back to default", "page_size=0", DefaultPageSize},
		{"exceeds max is clamped", "page_size=1000", MaxPageSize},
		{"non-numeric falls back to default", "page_size=xx", DefaultPageSize},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q, _ := url.ParseQuery(tc.raw)
			if got := ParsePageSize(q); got != tc.want {
				t.Errorf("ParsePageSize(%q) = %d, want %d", tc.raw, got, tc.want)
			}
		})
	}
}

func TestOffset(t *testing.T) {
	cases := []struct {
		page, pageSize, want int
	}{
		{1, 20, 0},
		{2, 20, 20},
		{3, 10, 20},
	}
	for _, tc := range cases {
		if got := Offset(tc.page, tc.pageSize); got != tc.want {
			t.Errorf("Offset(%d, %d) = %d, want %d", tc.page, tc.pageSize, got, tc.want)
		}
	}
}

func TestTotalPages(t *testing.T) {
	cases := []struct {
		total    int64
		pageSize int
		want     int
	}{
		{0, 20, 1},
		{1, 20, 1},
		{20, 20, 1},
		{21, 20, 2},
		{100, 20, 5},
		{101, 20, 6},
	}
	for _, tc := range cases {
		if got := TotalPages(tc.total, tc.pageSize); got != tc.want {
			t.Errorf("TotalPages(%d, %d) = %d, want %d", tc.total, tc.pageSize, got, tc.want)
		}
	}
}

func TestParseSort(t *testing.T) {
	allowed := []string{"created_at", "title"}

	t.Run("valid field and direction", func(t *testing.T) {
		q, _ := url.ParseQuery("sort_by=title&sort_dir=asc")
		field, dir := ParseSort(q, allowed, "created_at")
		if field != "title" || dir != "asc" {
			t.Errorf("got (%s, %s), want (title, asc)", field, dir)
		}
	})

	t.Run("invalid field falls back to default", func(t *testing.T) {
		q, _ := url.ParseQuery("sort_by=password_hash; DROP TABLE users;--")
		field, _ := ParseSort(q, allowed, "created_at")
		if field != "created_at" {
			t.Errorf("expected fallback to created_at, got %s", field)
		}
	})

	t.Run("invalid direction falls back to desc", func(t *testing.T) {
		q, _ := url.ParseQuery("sort_by=title&sort_dir=sideways")
		_, dir := ParseSort(q, allowed, "created_at")
		if dir != "desc" {
			t.Errorf("expected fallback to desc, got %s", dir)
		}
	})

	t.Run("missing params use defaults", func(t *testing.T) {
		q, _ := url.ParseQuery("")
		field, dir := ParseSort(q, allowed, "created_at")
		if field != "created_at" || dir != "desc" {
			t.Errorf("got (%s, %s), want (created_at, desc)", field, dir)
		}
	})
}
