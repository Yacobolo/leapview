package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	manageddataapi "github.com/flidai/leapview/internal/manageddata/api"
	manageddatagen "github.com/flidai/leapview/internal/manageddata/api/gen"
)

type managedDataCLIClient struct {
	http   *http.Client
	target string
	token  string
}

func newManagedDataCLIClient(client *http.Client, target, token string) *managedDataCLIClient {
	if client == nil {
		client = http.DefaultClient
	}
	return &managedDataCLIClient{http: client, target: strings.TrimRight(target, "/"), token: token}
}

func (client *managedDataCLIClient) createUploadSession(ctx context.Context, project, connection, key string, body manageddataapi.ManagedDataUploadSessionCreateRequest) (manageddataapi.ManagedDataUploadSessionResponse, error) {
	var response manageddataapi.ManagedDataUploadSessionResponse
	err := client.json(ctx, http.MethodPost, "createManagedDataUploadSession", map[string]string{"project": project, "connection": connection}, nil, key, body, &response)
	return response, err
}

func (client *managedDataCLIClient) finalizeUploadSession(ctx context.Context, project, connection, uploadID, key string) (manageddataapi.ManagedDataUploadSessionResponse, error) {
	var response manageddataapi.ManagedDataUploadSessionResponse
	err := client.json(ctx, http.MethodPost, "finalizeManagedDataUploadSession", managedDataUploadPath(project, connection, uploadID), nil, key, nil, &response)
	return response, err
}

func (client *managedDataCLIClient) getUploadSession(ctx context.Context, project, connection, uploadID string) (manageddataapi.ManagedDataUploadSessionResponse, error) {
	var response manageddataapi.ManagedDataUploadSessionResponse
	err := client.json(ctx, http.MethodGet, "getManagedDataUploadSession", managedDataUploadPath(project, connection, uploadID), nil, "", nil, &response)
	return response, err
}

func (client *managedDataCLIClient) abortUploadSession(ctx context.Context, project, connection, uploadID, key string) {
	var response manageddataapi.ManagedDataUploadSessionResponse
	_ = client.json(ctx, http.MethodPost, "cancelManagedDataUploadSession", managedDataUploadPath(project, connection, uploadID), nil, key, nil, &response)
}

func (client *managedDataCLIClient) createMultipart(ctx context.Context, project, connection, uploadID, key, logicalPath string) (manageddataapi.ManagedDataS3MultipartUploadResponse, error) {
	var response manageddataapi.ManagedDataS3MultipartUploadResponse
	err := client.json(ctx, http.MethodPost, "createManagedDataS3MultipartUpload", managedDataUploadPath(project, connection, uploadID), nil, key, manageddataapi.ManagedDataS3MultipartCreateRequest{Path: logicalPath}, &response)
	return response, err
}

func (client *managedDataCLIClient) signMultipartPart(ctx context.Context, project, connection, uploadID, multipartID string, partNumber int32, body manageddataapi.ManagedDataS3MultipartSignPartRequest) (manageddataapi.ManagedDataS3MultipartSignedPartResponse, error) {
	params := managedDataMultipartPath(project, connection, uploadID, multipartID)
	params["partNumber"] = strconv.FormatInt(int64(partNumber), 10)
	var response manageddataapi.ManagedDataS3MultipartSignedPartResponse
	err := client.json(ctx, http.MethodPost, "signManagedDataS3MultipartPart", params, nil, "", body, &response)
	return response, err
}

func (client *managedDataCLIClient) completeMultipart(ctx context.Context, project, connection, uploadID, multipartID, key string, body manageddataapi.ManagedDataS3MultipartCompleteRequest) (manageddataapi.ManagedDataS3MultipartUploadResponse, error) {
	var response manageddataapi.ManagedDataS3MultipartUploadResponse
	err := client.json(ctx, http.MethodPost, "completeManagedDataS3MultipartUpload", managedDataMultipartPath(project, connection, uploadID, multipartID), nil, key, body, &response)
	return response, err
}

func (client *managedDataCLIClient) abortMultipart(ctx context.Context, project, connection, uploadID, multipartID, key string) {
	var response manageddataapi.ManagedDataS3MultipartUploadResponse
	_ = client.json(ctx, http.MethodPost, "abortManagedDataS3MultipartUpload", managedDataMultipartPath(project, connection, uploadID, multipartID), nil, key, nil, &response)
}

func (client *managedDataCLIClient) listRevisions(ctx context.Context, project, connection string, query url.Values) (manageddataapi.ManagedDataRevisionListResponse, error) {
	var response manageddataapi.ManagedDataRevisionListResponse
	err := client.json(ctx, http.MethodGet, "listManagedDataRevisions", map[string]string{"project": project, "connection": connection}, query, "", nil, &response)
	return response, err
}

func (client *managedDataCLIClient) currentRevision(ctx context.Context, project, connection, _ string) (manageddataapi.ManagedDataActiveRevisionResponse, error) {
	var response manageddataapi.ManagedDataActiveRevisionResponse
	err := client.json(ctx, http.MethodGet, "getActiveManagedDataRevision", map[string]string{"project": project, "connection": connection}, nil, "", nil, &response)
	return response, err
}

func (client *managedDataCLIClient) json(ctx context.Context, method, operation string, pathParams map[string]string, query url.Values, idempotencyKey string, body, out any) error {
	endpoint, err := managedDataOperationURL(client.target, operation, pathParams, query)
	if err != nil {
		return fmt.Errorf("build managed-data request: %w", err)
	}
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode managed-data request: %w", err)
		}
		reader = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return fmt.Errorf("build managed-data request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if client.token != "" {
		request.Header.Set("Authorization", "Bearer "+client.token)
	}
	if idempotencyKey != "" {
		request.Header.Set("Idempotency-Key", idempotencyKey)
	}
	response, err := client.http.Do(request)
	if err != nil {
		return fmt.Errorf("operation %s could not reach the server", operation)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		contents, _ := io.ReadAll(io.LimitReader(response.Body, 64<<10))
		var problem struct {
			Code   string `json:"code"`
			Detail string `json:"detail"`
		}
		if json.Unmarshal(contents, &problem) == nil && strings.TrimSpace(problem.Detail) != "" {
			if strings.TrimSpace(problem.Code) != "" {
				return fmt.Errorf("operation %s failed with HTTP %d (%s): %s", operation, response.StatusCode, problem.Code, problem.Detail)
			}
			return fmt.Errorf("operation %s failed with HTTP %d: %s", operation, response.StatusCode, problem.Detail)
		}
		return fmt.Errorf("operation %s failed with HTTP %d", operation, response.StatusCode)
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, response.Body)
		return nil
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 16<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return fmt.Errorf("decode operation %s response: %w", operation, err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("decode operation %s response: trailing data", operation)
	}
	return nil
}

func managedDataOperationURL(target, operation string, pathParams map[string]string, query url.Values) (string, error) {
	contract, ok := manageddatagen.GetAPIGenOperationContract(operation)
	if !ok {
		return "", fmt.Errorf("unknown Managed Data API operation %q", operation)
	}
	path := contract.Path
	for name, value := range pathParams {
		path = strings.ReplaceAll(path, "{"+name+"}", url.PathEscape(value))
	}
	if strings.Contains(path, "{") {
		return "", fmt.Errorf("unresolved API path parameter in %q", path)
	}
	endpoint, err := url.Parse(strings.TrimRight(target, "/") + path)
	if err != nil {
		return "", err
	}
	if len(query) > 0 {
		endpoint.RawQuery = query.Encode()
	}
	return endpoint.String(), nil
}

func managedDataUploadPath(project, connection, uploadID string) map[string]string {
	return map[string]string{"project": project, "connection": connection, "uploadSession": uploadID}
}

func managedDataMultipartPath(project, connection, uploadID, multipartID string) map[string]string {
	params := managedDataUploadPath(project, connection, uploadID)
	params["multipartUpload"] = multipartID
	return params
}
