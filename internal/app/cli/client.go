package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	apiaggregate "github.com/Yacobolo/leapview/internal/app/api/aggregate"
	"github.com/Yacobolo/leapview/internal/app/config"
	"github.com/Yacobolo/leapview/internal/platform/cliapi"
	apigenclient "github.com/Yacobolo/toolbelt/apigen/runtime/client"
)

type capabilityAPIClient struct{}

func (capabilityAPIClient) Resolve(_ context.Context, credentials cliapi.Credentials) (cliapi.Credentials, error) {
	target, token, err := clientTargetAndTokenValues(credentials.Target, credentials.Token)
	if err != nil {
		return cliapi.Credentials{}, err
	}
	return cliapi.Credentials{Target: target, Token: token}, nil
}

func (client capabilityAPIClient) Environment(ctx context.Context, credentials cliapi.Credentials, asserted string) (string, error) {
	resolved, err := client.Resolve(ctx, credentials)
	if err != nil {
		return "", err
	}
	return targetEnvironment(ctx, http.DefaultClient, resolved.Target, resolved.Token, asserted)
}

func (client capabilityAPIClient) Transport(ctx context.Context, credentials cliapi.Credentials) (apigenclient.Transport, error) {
	resolved, err := client.Resolve(ctx, credentials)
	if err != nil {
		return nil, err
	}
	return capabilityAPITransport{
		target: resolved.Target,
		token:  resolved.Token,
		client: http.DefaultClient,
	}, nil
}

type capabilityAPITransport struct {
	target string
	token  string
	client *http.Client
}

func (transport capabilityAPITransport) DoAPIGen(ctx context.Context, request apigenclient.Request, out any) (apigenclient.Response, error) {
	endpoint, err := apiRequestURL(transport.target, request.Path, request.PathParams, request.Query)
	if err != nil {
		return apigenclient.Response{}, err
	}
	var body io.Reader
	if request.Body != nil {
		if strings.Contains(strings.ToLower(request.ContentType), "json") {
			encoded, err := json.Marshal(request.Body)
			if err != nil {
				return apigenclient.Response{}, fmt.Errorf("encode %s request: %w", request.OperationID, err)
			}
			body = bytes.NewReader(encoded)
		} else {
			switch value := request.Body.(type) {
			case []byte:
				body = bytes.NewReader(value)
			case string:
				body = strings.NewReader(value)
			default:
				return apigenclient.Response{}, fmt.Errorf("encode %s request: unsupported %s body type %T", request.OperationID, request.ContentType, request.Body)
			}
		}
	}
	httpRequest, err := http.NewRequestWithContext(ctx, request.Method, endpoint, body)
	if err != nil {
		return apigenclient.Response{}, err
	}
	httpRequest.Header = request.Headers.Clone()
	if httpRequest.Header == nil {
		httpRequest.Header = make(http.Header)
	}
	if request.Accept != "" {
		httpRequest.Header.Set("Accept", request.Accept)
	}
	if request.ContentType != "" {
		httpRequest.Header.Set("Content-Type", request.ContentType)
	}
	if transport.token != "" {
		httpRequest.Header.Set("Authorization", "Bearer "+transport.token)
	}
	httpClient := transport.client
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	httpResponse, err := httpClient.Do(httpRequest)
	if err != nil {
		return apigenclient.Response{}, err
	}
	defer httpResponse.Body.Close()
	payload, readErr := io.ReadAll(httpResponse.Body)
	metadata := apigenclient.Response{
		StatusCode:  httpResponse.StatusCode,
		Headers:     httpResponse.Header.Clone(),
		ContentType: httpResponse.Header.Get("Content-Type"),
	}
	if readErr != nil {
		return metadata, readErr
	}
	if httpResponse.StatusCode >= http.StatusMultipleChoices {
		return metadata, fmt.Errorf("%s %s: %s", request.Method, endpoint, strings.TrimSpace(string(payload)))
	}
	if !apiaggregate.APIGenOperationAllowsStatus(request.OperationID, httpResponse.StatusCode) {
		return metadata, fmt.Errorf("%s %s: unexpected success status %d for operation %s", request.Method, endpoint, httpResponse.StatusCode, request.OperationID)
	}
	if out == nil || len(payload) == 0 {
		return metadata, nil
	}
	switch destination := out.(type) {
	case *[]byte:
		*destination = append((*destination)[:0], payload...)
	case *string:
		*destination = string(payload)
	default:
		if err := json.Unmarshal(payload, out); err != nil {
			return metadata, fmt.Errorf("decode %s response: %w", request.OperationID, err)
		}
	}
	return metadata, nil
}

func doJSON(ctx context.Context, method, endpoint, token string, body io.Reader, out any) error {
	return doJSONWithHeaders(ctx, method, endpoint, token, nil, body, out)
}

func doJSONWithHeaders(ctx context.Context, method, endpoint, token string, headers map[string]string, body io.Reader, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	bytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("%s %s: %s", method, endpoint, strings.TrimSpace(string(bytes)))
	}
	if out == nil || len(bytes) == 0 {
		return nil
	}
	return json.Unmarshal(bytes, out)
}

type clientConfig struct {
	Targets map[string]clientTarget `json:"targets"`
}

type clientTarget struct {
	Token string `json:"token"`
}

func targetEnvironment(ctx context.Context, client *http.Client, target, token, asserted string) (string, error) {
	instance, err := newDeploymentCLIClient(client, target, token).instance(ctx)
	if err != nil {
		return "", fmt.Errorf("read target instance: %w", err)
	}
	environment := strings.TrimSpace(instance.Environment)
	if environment == "" {
		return "", fmt.Errorf("target instance returned an empty environment")
	}
	if asserted = strings.TrimSpace(asserted); asserted != "" && asserted != environment {
		return "", fmt.Errorf("requested environment %q does not match target instance environment %q", asserted, environment)
	}
	return environment, nil
}

func clientTargetAndToken(opts *rootOptions) (string, string, error) {
	return clientTargetAndTokenValues(opts.target, opts.token)
}

func clientTargetAndTokenValues(targetValue, tokenValue string) (string, string, error) {
	cfg := config.MustLoad()
	target := strings.TrimRight(targetValue, "/")
	if target == "" {
		target = strings.TrimRight(cfg.Target, "/")
	}
	token := tokenValue
	if token == "" {
		token = cfg.APIToken
	}
	config, _ := loadClientConfig()
	if target != "" && token == "" {
		token = config.Targets[target].Token
	}
	if target == "" {
		return "", "", fmt.Errorf("target is required")
	}
	if token == "" {
		return "", "", fmt.Errorf("API token is required; use --token, LEAPVIEW_API_TOKEN, or leapview login --target %s --token <token>", target)
	}
	return target, token, nil
}

func loadClientConfig() (clientConfig, error) {
	path := clientConfigPath()
	bytes, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return clientConfig{Targets: map[string]clientTarget{}}, nil
	}
	if err != nil {
		return clientConfig{}, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return clientConfig{}, err
	}
	var config clientConfig
	if err := json.Unmarshal(bytes, &config); err != nil {
		return clientConfig{}, err
	}
	if config.Targets == nil {
		config.Targets = map[string]clientTarget{}
	}
	return config, nil
}

func saveClientConfig(config clientConfig) error {
	path := clientConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	bytes, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, bytes, 0o600)
}

func clientConfigPath() string {
	return config.MustLoad().ClientConfigPath()
}

func shortDigest(value string) string {
	if len(value) <= 12 {
		return value
	}
	return value[:12]
}

func init() {
	http.DefaultClient.Timeout = 5 * time.Minute
}
