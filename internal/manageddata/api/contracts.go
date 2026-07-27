package api

type PageInfo struct {
	NextCursor *string `json:"nextCursor,omitempty"`
}

type ProblemFieldError struct {
	Code   string `json:"code"`
	Detail string `json:"detail"`
	Field  string `json:"field"`
}

type ProblemDetails struct {
	Code      string              `json:"code"`
	Detail    string              `json:"detail"`
	Errors    []ProblemFieldError `json:"errors"`
	Instance  string              `json:"instance"`
	RequestId string              `json:"requestId"`
	Status    int32               `json:"status"`
	Title     string              `json:"title"`
	Type      string              `json:"type"`
}

type PageParams struct {
	Limit     *int32
	PageToken *string
}

type IdempotencyHeaders struct {
	IdempotencyKey string
}

type GenListManagedDataRevisionsParams = PageParams
type GenListManagedDataUploadSessionsParams = PageParams
type GenListManagedDataUploadSessionEventsParams = PageParams
type GenCreateManagedDataUploadSessionHeaders = IdempotencyHeaders
type GenCancelManagedDataUploadSessionHeaders = IdempotencyHeaders
type GenFinalizeManagedDataUploadSessionHeaders = IdempotencyHeaders
type GenCreateManagedDataS3MultipartUploadHeaders = IdempotencyHeaders
type GenCompleteManagedDataS3MultipartUploadHeaders = IdempotencyHeaders
type GenAbortManagedDataS3MultipartUploadHeaders = IdempotencyHeaders

type GenListManagedDataUploadSessionEventsHeaders struct {
	Accept      *string
	LastEventID *string
}

type ManagedDataActiveRevisionResponse struct {
	ActivatedAt  *string                             `json:"activatedAt,omitempty"`
	DeploymentId *string                             `json:"deploymentId,omitempty"`
	Revision     *ManagedDataRevisionSummaryResponse `json:"revision,omitempty"`
}

type ManagedDataFileMetadata struct {
	Path   string `json:"path"`
	Sha256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

type ManagedDataFileUploadResponse struct {
	Error       *string                       `json:"error,omitempty"`
	File        ManagedDataFileMetadata       `json:"file"`
	Negotiation *ManagedDataUploadNegotiation `json:"negotiation,omitempty"`
	Status      ManagedDataFileUploadStatus   `json:"status"`
}

type ManagedDataFileUploadStatus string

const (
	ManagedDataFileUploadStatusPending   ManagedDataFileUploadStatus = "pending"
	ManagedDataFileUploadStatusUploading ManagedDataFileUploadStatus = "uploading"
	ManagedDataFileUploadStatusUploaded  ManagedDataFileUploadStatus = "uploaded"
	ManagedDataFileUploadStatusVerified  ManagedDataFileUploadStatus = "verified"
	ManagedDataFileUploadStatusSkipped   ManagedDataFileUploadStatus = "skipped"
	ManagedDataFileUploadStatusFailed    ManagedDataFileUploadStatus = "failed"
)

type ManagedDataHTTPHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type ManagedDataManifest struct {
	Files []ManagedDataFileMetadata `json:"files"`
}

type ManagedDataRevisionListResponse struct {
	Items []ManagedDataRevisionSummaryResponse `json:"items"`
	Page  PageInfo                             `json:"page"`
}

type ManagedDataRevisionResponse struct {
	CreatedAt       string                    `json:"createdAt"`
	FileCount       int32                     `json:"fileCount"`
	Id              string                    `json:"id"`
	Manifest        ManagedDataManifest       `json:"manifest"`
	Size            int64                     `json:"size"`
	Status          ManagedDataRevisionStatus `json:"status"`
	UploadSessionId string                    `json:"uploadSessionId"`
}

type ManagedDataRevisionStatus string

const ManagedDataRevisionStatusAvailable ManagedDataRevisionStatus = "available"

type ManagedDataRevisionSummaryResponse struct {
	CreatedAt       string                    `json:"createdAt"`
	FileCount       int32                     `json:"fileCount"`
	Id              string                    `json:"id"`
	Size            int64                     `json:"size"`
	Status          ManagedDataRevisionStatus `json:"status"`
	UploadSessionId string                    `json:"uploadSessionId"`
}

type ManagedDataS3CompletedPart struct {
	Etag       string  `json:"etag"`
	PartNumber int32   `json:"partNumber"`
	Sha256     *string `json:"sha256,omitempty"`
}

type ManagedDataS3MultipartCompleteRequest struct {
	Parts []ManagedDataS3CompletedPart `json:"parts"`
}

type ManagedDataS3MultipartCreateRequest struct {
	Path string `json:"path"`
}

type ManagedDataS3MultipartNegotiation struct {
	CreateEndpoint  string `json:"createEndpoint"`
	MaximumPartSize int64  `json:"maximumPartSize"`
	MaximumParts    int32  `json:"maximumParts"`
	MinimumPartSize int64  `json:"minimumPartSize"`
}

type ManagedDataS3MultipartSignPartRequest struct {
	Sha256 *string `json:"sha256,omitempty"`
	Size   int64   `json:"size"`
}

type ManagedDataS3MultipartSignedPartResponse struct {
	ExpiresAt  string                  `json:"expiresAt"`
	Headers    []ManagedDataHTTPHeader `json:"headers"`
	PartNumber int32                   `json:"partNumber"`
	Url        string                  `json:"url"`
}

type ManagedDataS3MultipartStatus string

const (
	ManagedDataS3MultipartStatusOpen      ManagedDataS3MultipartStatus = "open"
	ManagedDataS3MultipartStatusCompleted ManagedDataS3MultipartStatus = "completed"
	ManagedDataS3MultipartStatusAborted   ManagedDataS3MultipartStatus = "aborted"
)

type ManagedDataS3MultipartUploadResponse struct {
	CreatedAt       string                       `json:"createdAt"`
	Existing        bool                         `json:"existing"`
	ExpiresAt       *string                      `json:"expiresAt,omitempty"`
	File            ManagedDataFileMetadata      `json:"file"`
	Id              string                       `json:"id"`
	Status          ManagedDataS3MultipartStatus `json:"status"`
	UploadSessionId string                       `json:"uploadSessionId"`
}

type ManagedDataTusUploadNegotiation struct {
	Endpoint  string `json:"endpoint"`
	ExpiresAt string `json:"expiresAt"`
	Offset    int64  `json:"offset"`
	UploadId  string `json:"uploadId"`
}

type ManagedDataUploadNegotiation struct {
	Protocol    ManagedDataUploadProtocol          `json:"protocol"`
	S3Multipart *ManagedDataS3MultipartNegotiation `json:"s3Multipart,omitempty"`
	Tus         *ManagedDataTusUploadNegotiation   `json:"tus,omitempty"`
}

type ManagedDataUploadProtocol string

const (
	ManagedDataUploadProtocolTus            ManagedDataUploadProtocol = "tus"
	ManagedDataUploadProtocolS3Multipart    ManagedDataUploadProtocol = "s3_multipart"
	ManagedDataUploadProtocolAlreadyPresent ManagedDataUploadProtocol = "already_present"
)

type ManagedDataUploadSessionCreateRequest struct {
	Manifest ManagedDataManifest `json:"manifest"`
}

type ManagedDataUploadSessionListResponse struct {
	Items []ManagedDataUploadSessionResponse `json:"items"`
	Page  PageInfo                           `json:"page"`
}

type ManagedDataUploadSessionResponse struct {
	CompletedAt *string                         `json:"completedAt,omitempty"`
	Connection  string                          `json:"connection"`
	CreatedAt   string                          `json:"createdAt"`
	Error       *string                         `json:"error,omitempty"`
	ExpiresAt   string                          `json:"expiresAt"`
	Files       []ManagedDataFileUploadResponse `json:"files"`
	Id          string                          `json:"id"`
	Manifest    ManagedDataManifest             `json:"manifest"`
	Project     string                          `json:"project"`
	RevisionId  string                          `json:"revisionId"`
	Status      ManagedDataUploadSessionStatus  `json:"status"`
}

type ManagedDataUploadSessionStatus string

const (
	ManagedDataUploadSessionStatusOpen       ManagedDataUploadSessionStatus = "open"
	ManagedDataUploadSessionStatusFinalizing ManagedDataUploadSessionStatus = "finalizing"
	ManagedDataUploadSessionStatusCompleted  ManagedDataUploadSessionStatus = "completed"
	ManagedDataUploadSessionStatusCancelled  ManagedDataUploadSessionStatus = "cancelled"
	ManagedDataUploadSessionStatusFailed     ManagedDataUploadSessionStatus = "failed"
	ManagedDataUploadSessionStatusExpired    ManagedDataUploadSessionStatus = "expired"
)
