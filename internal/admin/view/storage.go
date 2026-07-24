package view

type AdminStorageData struct {
	CatalogPath        string
	DataPath           string
	Status             string
	DatabaseCount      int
	CatalogSizeBytes   int64
	CatalogSizeLabel   string
	DataSizeBytes      int64
	DataSizeLabel      string
	TotalSizeBytes     int64
	TotalSizeLabel     string
	TotalDataSizeBytes int64
	TotalDataSizeLabel string
	TableCount         int
	SnapshotCount      int
	DataFileCount      int
	Databases          []AdminStorageDatabase
	Tables             []AdminStorageTable
	Snapshots          []AdminStorageSnapshot
	ServingStates      []AdminStorageServingState
	Warnings           []string
}

type AdminStorageDatabase struct {
	ID        string
	Name      string
	Path      string
	ModelID   string
	ModelName string
	SizeBytes int64
	SizeLabel string
}

type AdminStorageTable struct {
	DatabaseID    string
	DatabaseName  string
	DatabasePath  string
	ModelID       string
	ModelName     string
	Schema        string
	Name          string
	Type          string
	TableID       int64
	TableUUID     string
	DuckLakePath  string
	BeginSnapshot int64
	EndSnapshot   int64
	RowCount      int64
	RowCountLabel string
	ColumnCount   int
	FileCount     int
	SizeBytes     int64
	SizeLabel     string
	Columns       []AdminStorageColumn
	Files         []AdminStorageFile
	History       []AdminStorageTableHistory
	ServingStates []AdminStorageServingState
}

type AdminStorageColumn struct {
	ID                  int64
	Name                string
	Type                string
	Ordinal             int
	Nullable            string
	Default             string
	InitialDefault      string
	DefaultValueType    string
	DefaultValueDialect string
	BeginSnapshot       int64
	ContainsNull        string
	ContainsNaN         string
	MinValue            string
	MaxValue            string
	ExtraStats          string
}

type AdminStorageFile struct {
	ID               int64
	Path             string
	Format           string
	RecordCount      int64
	RecordCountLabel string
	SizeBytes        int64
	SizeLabel        string
	BeginSnapshot    int64
	EndSnapshot      int64
}

type AdminStorageTableHistory struct {
	SnapshotID    int64
	Time          string
	SchemaVersion int64
	Source        string
	Changes       string
	Author        string
	Message       string
	ExtraInfo     string
}

type AdminStorageSnapshot struct {
	ID                int64
	Time              string
	SchemaVersion     int64
	Author            string
	Message           string
	Changes           string
	ExtraInfo         string
	Protected         bool
	ServingStateCount int
}

type AdminStorageServingState struct {
	WorkspaceID    string
	Environment    string
	ServingStateID string
	Status         string
	SnapshotID     int64
	Digest         string
	Active         bool
	ActivatedAt    string
}
