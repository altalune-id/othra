package db

import (
	"strings"

	"github.com/go-jet/jet/v2/postgres"
)

// TextArray is elems as a Postgres text[] value, cast explicitly as a text parameter has no assignment cast to an array.
func TextArray(elems []string) postgres.StringExpression {
	return postgres.StringExp(postgres.CAST(postgres.String(TextArrayLiteral(elems))).AS("text[]"))
}

// TextArrayLiteral is elems as a Postgres array literal, every element quoted.
func TextArrayLiteral(elems []string) string {
	esc := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	parts := make([]string, len(elems))
	for i, e := range elems {
		parts[i] = `"` + esc.Replace(e) + `"`
	}
	return "{" + strings.Join(parts, ",") + "}"
}
