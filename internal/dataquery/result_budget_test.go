package dataquery

import (
	"context"
	"testing"
)

func TestResultBudgetEnforcesRowsAndBytesWithoutCommittingRejectedRow(t *testing.T) {
	rows := &ResultBudget{limits: ResultLimits{MaxRows: 1, MaxBytes: 1024}}
	if err := rows.ConsumeRow(Row{"v": "one"}); err != nil {
		t.Fatal(err)
	}
	if err := rows.ConsumeRow(Row{"v": "two"}); reason(err) != ResultRows {
		t.Fatalf("error=%v", err)
	}
	if got, _ := rows.Usage(); got != 1 {
		t.Fatalf("rows=%d", got)
	}
	bytes := &ResultBudget{limits: ResultLimits{MaxRows: 10, MaxBytes: 64}}
	if err := bytes.ConsumeRow(Row{"v": string(make([]byte, 128))}); reason(err) != ResultBytes {
		t.Fatalf("error=%v", err)
	}
}

func TestWithResultBudgetReusesLogicalBudget(t *testing.T) {
	ctx := WithResultBudget(context.Background(), ResultLimits{MaxRows: 2, MaxBytes: 1024})
	first, _ := ResultBudgetFromContext(ctx)
	secondCtx := WithResultBudget(ctx, ResultLimits{MaxRows: 100, MaxBytes: 1 << 20})
	second, _ := ResultBudgetFromContext(secondCtx)
	if first != second {
		t.Fatal("nested result budget was replaced")
	}
}
func reason(err error) ResultLimitReason { value, _ := ResultLimitReasonOf(err); return value }
