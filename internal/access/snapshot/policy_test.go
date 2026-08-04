package snapshot

import (
	"testing"

	accesspolicy "github.com/flidai/leapview/internal/access/policy"
	"github.com/stretchr/testify/require"
)

func TestDecodeCompilesDataPolicies(t *testing.T) {
	value, err := Decode([]byte(`{
		"dataPolicies": {
			"mask-email": {
				"id": "policy-email",
				"name": "Mask email",
				"object": {"type":"column","id":"sales/orders/email"},
				"policyType": "column_mask",
				"expressionJson": "{\"field\":\"orders.email\",\"mask\":\"redacted\"}"
			}
		}
	}`))
	require.NoError(t, err)
	require.Equal(t, accesspolicy.MaskRedact, value.DataPolicies["mask-email"].Compiled.ColumnMask.Mask)
}

func TestDecodeRejectsInvalidDataPolicy(t *testing.T) {
	_, err := Decode([]byte(`{
		"dataPolicies": {
			"unsafe": {
				"object": {"type":"dataset","id":"sales/orders"},
				"policyType": "row_filter",
				"expressionJson": "{}"
			}
		}
	}`))
	require.Error(t, err)
	require.ErrorContains(t, err, "unsafe")
}
