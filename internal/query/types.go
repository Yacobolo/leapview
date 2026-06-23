package query

import "github.com/Yacobolo/libredash/internal/semantic"

type Field struct {
	Field   string
	Alias   string
	Measure semantic.MetricMeasure
}

type Time struct {
	Field string
	Grain string
	Alias string
}

type Filter struct {
	Field    string
	Operator string
	Values   []any
}

type Sort struct {
	Field     string
	Direction string
}

type Request struct {
	Table      string
	Dimensions []Field
	Measures   []Field
	Time       Time
	Filters    []Filter
	Sort       []Sort
	Limit      int
}

type RowRequest struct {
	Table      string
	Dimensions []Field
	Measures   []Field
	Filters    []Filter
	Sort       []Sort
	Limit      int
	Offset     int
}

type RawValueRequest struct {
	Table      string
	Dimensions []Field
	Measure    Field
	Filters    []Filter
	Sort       []Sort
	Limit      int
}

type CountRequest struct {
	Table   string
	Filters []Filter
}

type Plan struct {
	SQL     string
	Args    []any
	Columns []string
}
