package utils

import (
	"net/url"
	"strconv"
)

const (
	DefaultPage     = 1
	DefaultPageSize = 20
	MaxPageSize     = 100
)

// ParsePage extracts and clamps the "page" query parameter (1-based).
func ParsePage(q url.Values) int {
	page, err := strconv.Atoi(q.Get("page"))
	if err != nil || page < 1 {
		return DefaultPage
	}
	return page
}

// ParsePageSize extracts and clamps the "page_size" query parameter.
func ParsePageSize(q url.Values) int {
	size, err := strconv.Atoi(q.Get("page_size"))
	if err != nil || size < 1 {
		return DefaultPageSize
	}
	if size > MaxPageSize {
		return MaxPageSize
	}
	return size
}

// Offset computes the SQL OFFSET for a given 1-based page and page size.
func Offset(page, pageSize int) int {
	return (page - 1) * pageSize
}

// TotalPages computes the number of pages needed for totalItems at pageSize,
// with a floor of 1 page even when there are zero items.
func TotalPages(totalItems int64, pageSize int) int {
	if pageSize <= 0 {
		return 1
	}
	pages := int((totalItems + int64(pageSize) - 1) / int64(pageSize))
	if pages < 1 {
		return 1
	}
	return pages
}

// ParseSort validates the requested sort field against an allow-list and
// returns (field, direction), falling back to defaultField/"desc" for any
// invalid or missing input. This allow-list check is what prevents SQL
// injection via the sort parameter — the value is never interpolated
// unless it first matches an entry in allowed.
func ParseSort(q url.Values, allowed []string, defaultField string) (string, string) {
	field := q.Get("sort_by")
	valid := false
	for _, a := range allowed {
		if a == field {
			valid = true
			break
		}
	}
	if !valid {
		field = defaultField
	}

	dir := q.Get("sort_dir")
	if dir != "asc" && dir != "desc" {
		dir = "desc"
	}

	return field, dir
}
