package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/flidai/leapview/rtest/internal/protocol"
)

type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

func New(baseURL, token string) (*Client, error) {
	parsed, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, errors.New("RTEST_URL must be an absolute HTTP URL")
	}
	if token == "" {
		return nil, errors.New("RTEST_TOKEN is required")
	}
	return &Client{baseURL: parsed.String(), token: token, http: &http.Client{Timeout: 30 * time.Second}}, nil
}

func (c *Client) Submit(ctx context.Context, manifest protocol.SubmitManifest, source io.Reader) (protocol.Job, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	manifestPart, _ := writer.CreateFormField("manifest")
	if err := json.NewEncoder(manifestPart).Encode(manifest); err != nil {
		return protocol.Job{}, err
	}
	sourcePart, _ := writer.CreateFormFile("source", "source.tar.zst")
	if _, err := io.Copy(sourcePart, source); err != nil {
		return protocol.Job{}, err
	}
	if err := writer.Close(); err != nil {
		return protocol.Job{}, err
	}
	req, err := c.request(ctx, http.MethodPost, "/v1/jobs", &body)
	if err != nil {
		return protocol.Job{}, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	var job protocol.Job
	if err := c.doJSON(req, http.StatusCreated, &job); err != nil {
		return protocol.Job{}, err
	}
	return job, nil
}

func (c *Client) Job(ctx context.Context, id string) (protocol.Job, error) {
	req, err := c.request(ctx, http.MethodGet, "/v1/jobs/"+url.PathEscape(id), nil)
	if err != nil {
		return protocol.Job{}, err
	}
	var job protocol.Job
	err = c.doJSON(req, http.StatusOK, &job)
	return job, err
}

func (c *Client) List(ctx context.Context, repository string, limit int) ([]protocol.Job, error) {
	values := url.Values{}
	if repository != "" {
		values.Set("repository", repository)
	}
	if limit > 0 {
		values.Set("limit", strconv.Itoa(limit))
	}
	path := "/v1/jobs"
	if query := values.Encode(); query != "" {
		path += "?" + query
	}
	req, err := c.request(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var jobs []protocol.Job
	if err := c.doJSON(req, http.StatusOK, &jobs); err != nil {
		return nil, err
	}
	return jobs, nil
}

func (c *Client) Cancel(ctx context.Context, id string) error {
	req, err := c.request(ctx, http.MethodDelete, "/v1/jobs/"+url.PathEscape(id), nil)
	if err != nil {
		return err
	}
	return c.do(req, http.StatusNoContent)
}

func (c *Client) Claim(ctx context.Context, workerID string) (protocol.Job, bool, error) {
	body, _ := json.Marshal(protocol.ClaimRequest{WorkerID: workerID})
	req, err := c.request(ctx, http.MethodPost, "/v1/worker/claim", bytes.NewReader(body))
	if err != nil {
		return protocol.Job{}, false, err
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(req)
	if err != nil {
		return protocol.Job{}, false, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNoContent {
		return protocol.Job{}, false, nil
	}
	if response.StatusCode != http.StatusOK {
		return protocol.Job{}, false, responseError(response)
	}
	var job protocol.Job
	if err := json.NewDecoder(response.Body).Decode(&job); err != nil {
		return protocol.Job{}, false, err
	}
	return job, true, nil
}

func (c *Client) DownloadSource(ctx context.Context, id string) (io.ReadCloser, error) {
	req, err := c.request(ctx, http.MethodGet, "/v1/worker/jobs/"+url.PathEscape(id)+"/source", nil)
	if err != nil {
		return nil, err
	}
	response, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		defer response.Body.Close()
		return nil, responseError(response)
	}
	return response.Body, nil
}

func (c *Client) AppendLog(ctx context.Context, id string, data []byte) error {
	req, err := c.request(ctx, http.MethodPost, "/v1/worker/jobs/"+url.PathEscape(id)+"/logs", bytes.NewReader(data))
	if err != nil {
		return err
	}
	return c.do(req, http.StatusNoContent)
}

func (c *Client) Heartbeat(ctx context.Context, id, workerID string) (protocol.Job, error) {
	body, _ := json.Marshal(protocol.ClaimRequest{WorkerID: workerID})
	req, err := c.request(ctx, http.MethodPost, "/v1/worker/jobs/"+url.PathEscape(id)+"/heartbeat", bytes.NewReader(body))
	if err != nil {
		return protocol.Job{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if err := c.do(req, http.StatusNoContent); err != nil {
		return protocol.Job{}, err
	}
	return c.Job(ctx, id)
}

func (c *Client) Finish(ctx context.Context, id string, finish protocol.FinishRequest) error {
	body, _ := json.Marshal(finish)
	req, err := c.request(ctx, http.MethodPost, "/v1/worker/jobs/"+url.PathEscape(id)+"/finish", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.do(req, http.StatusNoContent)
}

func (c *Client) Stream(ctx context.Context, id string, output io.Writer) (protocol.Job, error) {
	offset := int64(0)
	for attempt := 0; ; attempt++ {
		job, done, next, err := c.streamOnce(ctx, id, output, offset)
		offset = next
		if done {
			return job, nil
		}
		if ctx.Err() != nil {
			return protocol.Job{}, ctx.Err()
		}
		if attempt >= 9 {
			if err == nil {
				err = io.ErrUnexpectedEOF
			}
			return protocol.Job{}, fmt.Errorf("log stream disconnected after retries: %w", err)
		}
		delay := min(100*time.Millisecond*time.Duration(1<<min(attempt, 4)), 2*time.Second)
		select {
		case <-ctx.Done():
			return protocol.Job{}, ctx.Err()
		case <-time.After(delay):
		}
	}
}

func (c *Client) streamOnce(ctx context.Context, id string, output io.Writer, offset int64) (protocol.Job, bool, int64, error) {
	req, err := c.request(ctx, http.MethodGet, "/v1/jobs/"+url.PathEscape(id)+"/logs", nil)
	if err != nil {
		return protocol.Job{}, false, offset, err
	}
	req.Header.Set("Accept", "text/event-stream")
	if offset > 0 {
		req.Header.Set("Last-Event-ID", strconv.FormatInt(offset, 10))
	}
	response, err := (&http.Client{}).Do(req)
	if err != nil {
		return protocol.Job{}, false, offset, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return protocol.Job{}, false, offset, responseError(response)
	}
	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 64<<10), 2<<20)
	var event, data string
	eventOffset := offset
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			switch event {
			case "log":
				decoded, err := base64.StdEncoding.DecodeString(data)
				if err != nil {
					return protocol.Job{}, false, offset, err
				}
				if _, err := output.Write(decoded); err != nil {
					return protocol.Job{}, false, offset, err
				}
				offset = eventOffset
			case "done":
				var job protocol.Job
				if err := json.Unmarshal([]byte(data), &job); err != nil {
					return protocol.Job{}, false, offset, err
				}
				return job, true, eventOffset, nil
			}
			event, data = "", ""
			continue
		}
		if value, ok := strings.CutPrefix(line, "id: "); ok {
			if parsed, err := strconv.ParseInt(value, 10, 64); err == nil {
				eventOffset = parsed
			}
		}
		if value, ok := strings.CutPrefix(line, "event: "); ok {
			event = value
		}
		if value, ok := strings.CutPrefix(line, "data: "); ok {
			data = value
		}
	}
	return protocol.Job{}, false, offset, scanner.Err()
}

func (c *Client) request(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err == nil {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	return req, err
}

func (c *Client) do(req *http.Request, expected int) error {
	response, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != expected {
		return responseError(response)
	}
	return nil
}

func (c *Client) doJSON(req *http.Request, expected int, target any) error {
	response, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != expected {
		return responseError(response)
	}
	return json.NewDecoder(response.Body).Decode(target)
}

func responseError(response *http.Response) error {
	data, _ := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	var body struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(data, &body) == nil && body.Error != "" {
		return fmt.Errorf("remote returned %s: %s", response.Status, body.Error)
	}
	return fmt.Errorf("remote returned %s", response.Status)
}

func ExitCode(job protocol.Job) int {
	if job.ExitCode != nil {
		if *job.ExitCode == 0 && job.Status != protocol.StatusSucceeded {
			return 1
		}
		return *job.ExitCode
	}
	if job.Status == protocol.StatusSucceeded {
		return 0
	}
	return 1
}
