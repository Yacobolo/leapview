package model

import (
	"reflect"
	"testing"
)

func TestExpressionParsesReferencesAndSafeDivide(t *testing.T) {
	expression, err := ParseExpression("safe_divide(${refunds}, ${revenue}) * 100")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := expression.References(), []string{"refunds", "revenue"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("references = %#v, want %#v", got, want)
	}
	sql, err := expression.SQL(func(ref string) (string, error) { return "m_" + ref, nil })
	if err != nil {
		t.Fatal(err)
	}
	if sql != "((m_refunds / NULLIF(m_revenue, 0)) * 100)" {
		t.Fatalf("SQL = %q", sql)
	}
}

func TestExpressionRejectsAggregateSQLAndBareIdentifiers(t *testing.T) {
	for _, input := range []string{"SUM(${orders.revenue})", "orders.revenue", "${}"} {
		if _, err := ParseExpression(input); err == nil {
			t.Fatalf("ParseExpression(%q) succeeded", input)
		}
	}
}

func TestExpressionReportsAllowlistedFunctions(t *testing.T) {
	expression, err := ParseExpression("round(abs(${ratings.score}), 2) + safe_divide(1, 2)")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := expression.Functions(), []string{"round", "abs", "safe_divide"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %#v, want %#v", got, want)
	}
}
