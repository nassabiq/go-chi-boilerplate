package helper

import (
	"fmt"
	"strings"

	"github.com/nassabiq/go-chi-boilerplate/internal/core/domain/payloads"
	"gorm.io/gorm"
)

type ListParams struct {
	Skip          int
	Limit         int
	Filters       map[string]string
	Search        string
	SortBy        string
	SortDirection string
}

type ListConfig struct {
	SearchColumns        []string
	SortColumns          map[string]string
	FilterColumns        map[string]string
	DefaultSortBy        string
	DefaultSortDirection string
}

func ApplySearch(query *gorm.DB, search string, columns []string) *gorm.DB {
	if search == "" || len(columns) == 0 {
		return query
	}

	value := "%" + search + "%"
	conditions := make([]string, 0, len(columns))
	args := make([]any, 0, len(columns))

	for _, column := range columns {
		conditions = append(conditions, fmt.Sprintf("%s ILIKE ?", column))
		args = append(args, value)
	}

	return query.Where(strings.Join(conditions, " OR "), args...)
}

func ApplySort(query *gorm.DB, params ListParams, config ListConfig) *gorm.DB {
	sortBy := config.DefaultSortBy
	if column, ok := config.SortColumns[params.SortBy]; ok {
		sortBy = column
	}

	sortDirection := strings.ToUpper(params.SortDirection)
	if sortDirection != "ASC" && sortDirection != "DESC" {
		sortDirection = config.DefaultSortDirection
	}

	return query.Order(fmt.Sprintf("%s %s", sortBy, sortDirection))
}

func ApplyFilter(query *gorm.DB, filters map[string]string, columns map[string]string) *gorm.DB {
	for key, value := range filters {
		column, ok := columns[key]
		if !ok {
			continue
		}

		query.Where(column+" = ?", value)
	}

	return query
}

func ApplyPagination(query *gorm.DB, params ListParams) *gorm.DB {
	return query.Offset(params.Skip).Limit(params.Limit)
}

type FilterParser func(value string) (string, error)

const (
	DefaultListLimit = 10
	MaxListLimit     = 100
)

func ValidatePayload(payload payloads.ListPayload, filters map[string]FilterParser) (ListParams, error) {
	if payload.Skip < 0 {
		payload.Skip = 0
	}

	if payload.Limit <= 0 {
		payload.Limit = DefaultListLimit
	}

	if payload.Limit > MaxListLimit {
		payload.Limit = MaxListLimit
	}

	params := ListParams{
		Skip:          payload.Skip,
		Limit:         payload.Limit,
		Search:        payload.Search,
		SortBy:        payload.SortBy,
		SortDirection: payload.SortDirection,
		Filters:       make(map[string]string),
	}

	for key, value := range payload.Filters {
		parser, ok := filters[key]
		if !ok {
			continue
		}

		parsed, err := parser(value)
		if err != nil {
			return ListParams{}, fmt.Errorf("invalid filter %s: %w", key, err)
		}

		params.Filters[key] = parsed
	}

	return params, nil
}
