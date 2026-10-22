package db

import (
	"encoding/json"
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

// ArrayAsJSON selects the array column col as JSON text under alias, for ParseJSONArray.
func ArrayAsJSON(col postgres.Column, alias string) postgres.Projection {
	return postgres.CAST(postgres.Func("to_json", col)).AS_TEXT().AS(alias)
}

// ParseJSONArray decodes a text array selected by ArrayAsJSON; an empty array is an empty, non-nil slice.
func ParseJSONArray(raw string) ([]string, error) {
	out := []string{}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, err
	}
	return out, nil
}
