package query

import "testing"

func TestFilterSQLValidatesUnaryOperatorValues(t *testing.T) {
	for _, operator := range []string{"equals", "contains", "not_contains", "starts_with", "greater_than_or_equal", "less_than"} {
		if _, _, err := filterSQL("value", Filter{Operator: operator}); err == nil {
			t.Fatalf("filterSQL(%q) accepted an empty value list", operator)
		}
	}
}

func TestFilterSQLSupportsNotContains(t *testing.T) {
	sql, args, err := filterSQL("value", Filter{Operator: "not_contains", Values: []any{"internal"}})
	if err != nil {
		t.Fatal(err)
	}
	if sql != "lower(value) NOT LIKE lower(?)" || len(args) != 1 || args[0] != "%internal%" {
		t.Fatalf("filter = %q %#v", sql, args)
	}
}
