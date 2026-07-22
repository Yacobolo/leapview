package arrowresult

import (
	"errors"
	"testing"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"
)

func TestBuilderRetainsBorrowedRecordsUntilFinalLeaseRelease(t *testing.T) {
	allocator := memory.NewCheckedAllocator(memory.DefaultAllocator)
	defer allocator.AssertSize(t, 0)

	builder := array.NewInt64Builder(allocator)
	builder.AppendValues([]int64{1, 2, 3}, nil)
	values := builder.NewArray()
	builder.Release()
	record := array.NewRecordBatch(
		arrow.NewSchema([]arrow.Field{{Name: "id", Type: arrow.PrimitiveTypes.Int64}}, nil),
		[]arrow.Array{values},
		3,
	)
	values.Release()

	collector := NewBuilderWithAllocator(allocator)
	if err := collector.WriteSchema(record.Schema()); err != nil {
		t.Fatal(err)
	}
	if err := collector.WriteRecord(record); err != nil {
		t.Fatal(err)
	}
	result, err := collector.Finish()
	if err != nil {
		t.Fatal(err)
	}
	record.Release() // simulate duckdb-go advancing the reader

	first, err := result.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	second, err := result.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := first.Rows(), int64(3); got != want {
		t.Fatalf("rows = %d, want %d", got, want)
	}
	rows, err := DecodeRows(first)
	if err != nil {
		t.Fatal(err)
	}
	if got := rows[2]["id"]; got != int64(3) {
		t.Fatalf("retained value = %#v", got)
	}
	if first.Bytes() <= 0 {
		t.Fatalf("bytes = %d, want positive", first.Bytes())
	}

	result.Release()
	first.Release()
	first.Release()
	if allocator.CurrentAlloc() == 0 {
		t.Fatal("second lease did not pin Arrow buffers")
	}
	second.Release()
}

func TestBuilderRejectsInvalidLifecycle(t *testing.T) {
	collector := NewBuilder()
	if err := collector.WriteRecord(nil); !errors.Is(err, ErrSchemaRequired) {
		t.Fatalf("write before schema error = %v", err)
	}
	schema := arrow.NewSchema([]arrow.Field{{Name: "id", Type: arrow.PrimitiveTypes.Int64}}, nil)
	if err := collector.WriteSchema(schema); err != nil {
		t.Fatal(err)
	}
	if err := collector.WriteSchema(schema); !errors.Is(err, ErrSchemaAlreadySet) {
		t.Fatalf("second schema error = %v", err)
	}
	result, err := collector.Finish()
	if err != nil {
		t.Fatal(err)
	}
	defer result.Release()
	if _, err := collector.Finish(); !errors.Is(err, ErrBuilderFinished) {
		t.Fatalf("second finish error = %v", err)
	}
	if result.Rows() != 0 {
		t.Fatalf("empty rows = %d", result.Rows())
	}
}

func TestAbortReleasesRetainedRecords(t *testing.T) {
	allocator := memory.NewCheckedAllocator(memory.DefaultAllocator)
	defer allocator.AssertSize(t, 0)
	builder := array.NewStringBuilder(allocator)
	builder.Append("value")
	values := builder.NewArray()
	builder.Release()
	record := array.NewRecordBatch(arrow.NewSchema([]arrow.Field{{Name: "value", Type: arrow.BinaryTypes.String}}, nil), []arrow.Array{values}, 1)
	values.Release()

	collector := NewBuilderWithAllocator(allocator)
	if err := collector.WriteSchema(record.Schema()); err != nil {
		t.Fatal(err)
	}
	if err := collector.WriteRecord(record); err != nil {
		t.Fatal(err)
	}
	record.Release()
	collector.Abort()
	collector.Abort()
}
