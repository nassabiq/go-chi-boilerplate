package request

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/domain/payloads"
)

func DecodeJSON[T any](r *http.Request) (T, error) {
	var payload T

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&payload); err != nil {
		return payload, err
	}

	return payload, nil
}

func PathParam(req *http.Request, key string) (string, error) {
	value := chi.URLParam(req, key)
	if value == "" {
		return "", errors.New("missing path parameter")
	}

	return value, nil
}

func PathInt64(req *http.Request, key string) (int64, error) {
	value, err := PathParam(req, key)
	if err != nil {
		return 0, err
	}

	result, err := strconv.ParseInt(value, 10, 64)
	if err != nil || result <= 0 {
		return 0, errors.New("invalid path parameter")
	}

	return result, nil
}

func List(req *http.Request) (payloads.ListPayload, error) {
	query := req.URL.Query()

	payload := payloads.ListPayload{
		Search:        query.Get("search"),
		SortBy:        query.Get("sort_by"),
		SortDirection: query.Get("sort"),
		Filters:       map[string]string{},
	}

	if value := query.Get("skip"); value != "" {
		skip, err := strconv.Atoi(value)
		if err != nil {
			return payload, errors.New("invalid skip")
		}

		payload.Skip = skip
	}

	if value := query.Get("limit"); value != "" {
		limit, err := strconv.Atoi(value)
		if err != nil {
			return payload, errors.New("invalid limit")
		}

		payload.Limit = limit
	}

	for key, values := range query {
		if !strings.HasPrefix(key, "filters[") || !strings.HasSuffix(key, "]") {
			continue
		}

		filterKey := strings.TrimSuffix(strings.TrimPrefix(key, "filters["), "]")
		if filterKey == "" || len(values) == 0 {
			continue
		}

		payload.Filters[filterKey] = values[0]
	}

	return payload, nil
}

func ClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}

	return r.RemoteAddr
}
