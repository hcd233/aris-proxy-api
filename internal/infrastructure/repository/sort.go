package repository

import "github.com/samber/lo"

func safeSortField(field string) string {
	if lo.EveryBy([]rune(field), func(c rune) bool {
		return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
	}) {
		return field
	}
	return ""
}
