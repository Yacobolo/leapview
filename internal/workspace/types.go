package workspace

type AssetType string

const (
	AssetTypeCatalog       AssetType = "catalog"
	AssetTypeSemanticModel AssetType = "semantic_model"
	AssetTypeConnection    AssetType = "connection"
	AssetTypeSource        AssetType = "source"
	AssetTypeModelTable    AssetType = "model_table"
	AssetTypeField         AssetType = "field"
	AssetTypeMeasure       AssetType = "measure"
	AssetTypeDashboard     AssetType = "dashboard"
	AssetTypePage          AssetType = "page"
	AssetTypeFilter        AssetType = "filter"
	AssetTypeVisual        AssetType = "visual"
	AssetTypeTable         AssetType = "table"
)

type AssetEdgeType string

const (
	AssetEdgeContains          AssetEdgeType = "contains"
	AssetEdgeUsesConnection    AssetEdgeType = "uses_connection"
	AssetEdgeReadsSource       AssetEdgeType = "reads_source"
	AssetEdgeUsesSemanticModel AssetEdgeType = "uses_semantic_model"
	AssetEdgeUsesModelTable    AssetEdgeType = "uses_model_table"
	AssetEdgeUsesMeasure       AssetEdgeType = "uses_measure"
	AssetEdgeUsesField         AssetEdgeType = "uses_field"
	AssetEdgeFiltersField      AssetEdgeType = "filters_field"
)
