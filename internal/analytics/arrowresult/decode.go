package arrowresult

import (
	"fmt"
	"math/big"
	"time"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
)

// DecodeRows is the single boundary conversion from the analytical Arrow data
// plane into Go domain values. It never mutates or retains the leased records.
func DecodeRows(lease *Lease) ([]map[string]any, error) {
	if lease == nil || lease.Schema() == nil {
		return nil, ErrResultReleased
	}
	rows := make([]map[string]any, 0, lease.Rows())
	err := lease.VisitRecords(func(record arrow.RecordBatch) error {
		for rowIndex := 0; rowIndex < int(record.NumRows()); rowIndex++ {
			row := make(map[string]any, int(record.NumCols()))
			for columnIndex := 0; columnIndex < int(record.NumCols()); columnIndex++ {
				value, err := decodeValue(record.Column(columnIndex), rowIndex)
				if err != nil {
					return fmt.Errorf("decode Arrow column %q: %w", record.ColumnName(columnIndex), err)
				}
				row[record.ColumnName(columnIndex)] = value
			}
			rows = append(rows, row)
		}
		return nil
	})
	return rows, err
}

type marshalValue interface{ GetOneForMarshal(int) interface{} }

func decodeValue(values arrow.Array, index int) (any, error) {
	if values == nil || values.IsNull(index) {
		return nil, nil
	}
	switch values := values.(type) {
	case *array.Boolean:
		return values.Value(index), nil
	case *array.Int8:
		return values.Value(index), nil
	case *array.Int16:
		return values.Value(index), nil
	case *array.Int32:
		return values.Value(index), nil
	case *array.Int64:
		return values.Value(index), nil
	case *array.Uint8:
		return values.Value(index), nil
	case *array.Uint16:
		return values.Value(index), nil
	case *array.Uint32:
		return values.Value(index), nil
	case *array.Uint64:
		return values.Value(index), nil
	case *array.Float32:
		return values.Value(index), nil
	case *array.Float64:
		return values.Value(index), nil
	case *array.String:
		return values.Value(index), nil
	case *array.LargeString:
		return values.Value(index), nil
	case *array.Binary:
		return append([]byte{}, values.Value(index)...), nil
	case *array.LargeBinary:
		return append([]byte{}, values.Value(index)...), nil
	case *array.Date32:
		return values.Value(index).ToTime(), nil
	case *array.Date64:
		return values.Value(index).ToTime(), nil
	case *array.Timestamp:
		typeInfo, ok := values.DataType().(*arrow.TimestampType)
		if !ok {
			return nil, fmt.Errorf("timestamp has invalid data type %T", values.DataType())
		}
		return values.Value(index).ToTime(typeInfo.Unit).In(time.UTC), nil
	case *array.Decimal32:
		typeInfo := values.DataType().(*arrow.Decimal32Type)
		return decodeDecimal(values.Value(index).ToString(typeInfo.Scale), typeInfo.Scale), nil
	case *array.Decimal64:
		typeInfo := values.DataType().(*arrow.Decimal64Type)
		return decodeDecimal(values.Value(index).ToString(typeInfo.Scale), typeInfo.Scale), nil
	case *array.Decimal128:
		typeInfo := values.DataType().(*arrow.Decimal128Type)
		return decodeDecimal(values.Value(index).ToString(typeInfo.Scale), typeInfo.Scale), nil
	case *array.Decimal256:
		typeInfo := values.DataType().(*arrow.Decimal256Type)
		return decodeDecimal(values.Value(index).ToString(typeInfo.Scale), typeInfo.Scale), nil
	case *array.Dictionary:
		return decodeValue(values.Dictionary(), values.GetValueIndex(index))
	default:
		if marshal, ok := values.(marshalValue); ok {
			return marshal.GetOneForMarshal(index), nil
		}
		return nil, fmt.Errorf("unsupported Arrow type %s", values.DataType())
	}
}

func decodeDecimal(value string, scale int32) any {
	if scale != 0 {
		return value
	}
	integer := new(big.Int)
	if _, ok := integer.SetString(value, 10); ok {
		return integer
	}
	return value
}
