package capabilities

import (
	"net/http"

	apigenapi "github.com/Yacobolo/leapview/internal/api/gen"
	apitransport "github.com/Yacobolo/leapview/internal/api/transport"
	"github.com/Yacobolo/leapview/internal/staticasset"
	visualizationir "github.com/Yacobolo/leapview/internal/visualization/ir"
)

type Config struct {
	Environment string
	TUS         bool
	S3Multipart bool
}

func Write(w http.ResponseWriter, config Config) {
	uploadProtocols := []apigenapi.UploadProtocol{}
	if config.TUS {
		uploadProtocols = append(uploadProtocols, apigenapi.UploadProtocolTus)
	}
	if config.S3Multipart {
		uploadProtocols = append(uploadProtocols, apigenapi.UploadProtocolS3Multipart)
	}
	apitransport.WriteJSON(w, http.StatusOK, apigenapi.CapabilitiesResponse{
		ApiVersion: "v1", BuildVersion: staticasset.Version(),
		Authentication: []apigenapi.AuthenticationMode{apigenapi.AuthenticationModeBearer},
		Environment:    config.Environment,
		QueryFormats: []apigenapi.QueryFormat{
			apigenapi.QueryFormatApplicationJson,
			apigenapi.QueryFormatApplicationVndApacheArrowStream,
		},
		UploadProtocols: uploadProtocols,
		Visualization: apigenapi.VisualizationCapabilities{
			SchemaVersion: visualizationir.CurrentSchemaVersion,
			Renderers: []apigenapi.VisualizationRendererCapability{
				{Id: apigenapi.VisualizationRendererIDEcharts, Version: "6.1.0", SchemaVersion: visualizationir.CurrentSchemaVersion, Kinds: []apigenapi.VisualizationSpecKind{apigenapi.VisualizationSpecKindCartesian, apigenapi.VisualizationSpecKindProportional, apigenapi.VisualizationSpecKindHierarchy, apigenapi.VisualizationSpecKindPolar}},
				{Id: apigenapi.VisualizationRendererIDTanstack, Version: "9.0.0-beta.12", SchemaVersion: visualizationir.CurrentSchemaVersion, Kinds: []apigenapi.VisualizationSpecKind{apigenapi.VisualizationSpecKindTable, apigenapi.VisualizationSpecKindMatrix, apigenapi.VisualizationSpecKindPivot}},
				{Id: apigenapi.VisualizationRendererIDHtml, Version: "1", SchemaVersion: visualizationir.CurrentSchemaVersion, Kinds: []apigenapi.VisualizationSpecKind{apigenapi.VisualizationSpecKindKpi}},
				{Id: apigenapi.VisualizationRendererIDMaplibre, Version: "5.19.0", SchemaVersion: visualizationir.CurrentSchemaVersion, Kinds: []apigenapi.VisualizationSpecKind{apigenapi.VisualizationSpecKindGeographic}},
				{Id: apigenapi.VisualizationRendererIDVegaLiteSandbox, Version: "6.4.3", SchemaVersion: visualizationir.CurrentSchemaVersion, Kinds: []apigenapi.VisualizationSpecKind{apigenapi.VisualizationSpecKindCustom}},
			},
		},
	})
}
