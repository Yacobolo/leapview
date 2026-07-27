package app

import (
	"reflect"
	"testing"

	apigenapi "github.com/Yacobolo/leapview/internal/app/api/gen"
	manageddatagen "github.com/Yacobolo/leapview/internal/manageddata/api/gen"
	releasegen "github.com/Yacobolo/leapview/internal/release/api/gen"
)

func TestManagedDataGeneratedByteCountsAreInt64(t *testing.T) {
	for _, value := range []any{
		manageddatagen.ManagedDataFileMetadata{},
		manageddatagen.ManagedDataRevisionSummaryResponse{},
		manageddatagen.ManagedDataS3MultipartNegotiation{},
		manageddatagen.ManagedDataS3MultipartSignPartRequest{},
		manageddatagen.ManagedDataTusUploadNegotiation{},
	} {
		typeOf := reflect.TypeOf(value)
		for _, fieldName := range []string{"Size", "Offset", "MinimumPartSize", "MaximumPartSize"} {
			field, ok := typeOf.FieldByName(fieldName)
			if ok && field.Type.Kind() != reflect.Int64 {
				t.Fatalf("%s.%s type = %s, want int64", typeOf.Name(), fieldName, field.Type)
			}
		}
	}
}

func TestReleaseArtifactGeneratedSizeIsInt64(t *testing.T) {
	typeOf := reflect.TypeOf(releasegen.ReleaseArtifactResponse{})
	field, ok := typeOf.FieldByName("SizeBytes")
	if !ok || field.Type.Kind() != reflect.Int64 {
		t.Fatalf("%s.SizeBytes type = %v, want int64", typeOf.Name(), field.Type)
	}
}

func TestApplicationAPIGenAdapterImplementsRemainingGeneratedOperations(t *testing.T) {
	var _ apigenapi.GenOperationDispatcher = apiGenDispatcher{}
}
