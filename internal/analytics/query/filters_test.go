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

func TestFilterSQLSupportsNullSelection(t *testing.T) {
	for _, test := range []struct {
		operator string
		wantSQL  string
	}{
		{operator: "is_null", wantSQL: "value IS NULL"},
		{operator: "is_not_null", wantSQL: "value IS NOT NULL"},
	} {
		sql, args, err := filterSQL("value", Filter{Operator: test.operator})
		if err != nil {
			t.Fatalf("filterSQL(%q): %v", test.operator, err)
		}
		if sql != test.wantSQL || len(args) != 0 {
			t.Fatalf("filterSQL(%q) = %q %#v, want %q with no args", test.operator, sql, args, test.wantSQL)
		}
		if _, _, err := filterSQL("value", Filter{Operator: test.operator, Values: []any{"unexpected"}}); err == nil {
			t.Fatalf("filterSQL(%q) accepted a value", test.operator)
		}
	}
}
