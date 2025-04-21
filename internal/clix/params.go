package clix

import (
	"strings"

	"github.com/spf13/pflag"
)

type PaginationParams struct {
	Limit  int
	Offset int
}

func ParsePagination(flags *pflag.FlagSet) (PaginationParams, error) {
	limit, _ := flags.GetInt("limit")
	offset, _ := flags.GetInt("offset")
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	return PaginationParams{Limit: limit, Offset: offset}, nil
}

func ParseTags(flags *pflag.FlagSet) ([]string, error) {
	tagsStr, _ := flags.GetString("tags")
	var tags []string
	if tagsStr != "" {
		rawTags := strings.Split(tagsStr, ",")
		// Trim space and filter out empty strings in one pass
		for _, t := range rawTags {
			trimmed := strings.TrimSpace(t)
			if trimmed != "" {
				tags = append(tags, trimmed)
			}
		}
	}
	return tags, nil
}
