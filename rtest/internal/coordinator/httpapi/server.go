package httpapi

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/flidai/leapview/rtest/internal/coordinator/sqlite"
	"github.com/flidai/leapview/rtest/internal/protocol"
)

const (
	maxSourceSize = 128 << 20
	maxLogChunk   = 1 << 20
)

type Config struct {
	Token      string
	PayloadDir string
}

type Server struct {
	config Config
	store  *sqlite.Store
	mux    *http.ServeMux
}

func New(config Config, store *sqlite.Store) (http.Handler, error) {
	if config.Token == "" || store == nil || config.PayloadDir == "" {
		return nil, errors.New("token, payload directory, and store are required")
	}
	if err := os.MkdirAll(config.PayloadDir, 0o700); err != nil {
		return nil, fmt.Errorf("create payload directory: %w", err)
	}
	server := &Server{config: config, store: store, mux: http.NewServeMux()}
	server.routes()
	return server.authenticate(server.mux), nil
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	s.mux.HandleFunc("POST /v1/jobs", s.submitJob)
	s.mux.HandleFunc("GET /v1/jobs", s.listJobs)
	s.mux.HandleFunc("GET /v1/jobs/{id}", s.getJob)
	s.mux.HandleFunc("DELETE /v1/jobs/{id}", s.cancelJob)
	s.mux.HandleFunc("GET /v1/jobs/{id}/logs", s.streamLogs)
	s.mux.HandleFunc("POST /v1/worker/claim", s.claimJob)
	s.mux.HandleFunc("GET /v1/worker/jobs/{id}/source", s.getSource)
	s.mux.HandleFunc("POST /v1/worker/jobs/{id}/logs", s.appendLog)
	s.mux.HandleFunc("POST /v1/worker/jobs/{id}/heartbeat", s.heartbeat)
	s.mux.HandleFunc("POST /v1/worker/jobs/{id}/finish", s.finishJob)
}

func (s *Server) listJobs(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if value := r.URL.Query().Get("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 100 {
			writeError(w, http.StatusBadRequest, "limit must be between 1 and 100")
			return
		}
		limit = parsed
	}
	jobs, err := s.store.ListJobs(r.Context(), r.URL.Query().Get("repository"), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list jobs")
		return
	}
	writeJSON(w, http.StatusOK, jobs)
}

func (s *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}
		provided := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if len(provided) != len(s.config.Token) || subtle.ConstantTimeCompare([]byte(provided), []byte(s.config.Token)) != 1 {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) submitJob(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxSourceSize+(1<<20))
	reader, err := r.MultipartReader()
	if err != nil {
		writeError(w, http.StatusBadRequest, "multipart request required")
		return
	}
	var manifest protocol.SubmitManifest
	var uploadPath, uploadDigest string
	var uploadSize int64
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, "read multipart body")
			return
		}
		switch part.FormName() {
		case "manifest":
			if err := json.NewDecoder(io.LimitReader(part, 1<<20)).Decode(&manifest); err != nil {
				writeError(w, http.StatusBadRequest, "invalid manifest")
				return
			}
		case "source":
			temporary, err := os.CreateTemp(s.config.PayloadDir, ".upload-*")
			if err != nil {
				writeError(w, http.StatusInternalServerError, "create upload")
				return
			}
			defer func() {
				_ = temporary.Close()
				_ = os.Remove(temporary.Name())
			}()
			hash := sha256.New()
			size, err := io.Copy(io.MultiWriter(temporary, hash), io.LimitReader(part, maxSourceSize+1))
			if err != nil || size > maxSourceSize {
				writeError(w, http.StatusRequestEntityTooLarge, "source payload too large")
				return
			}
			if err := temporary.Sync(); err != nil {
				writeError(w, http.StatusInternalServerError, "store upload")
				return
			}
			if err := temporary.Close(); err != nil {
				writeError(w, http.StatusInternalServerError, "close upload")
				return
			}
			uploadPath = temporary.Name()
			uploadSize = size
			uploadDigest = "sha256:" + hex.EncodeToString(hash.Sum(nil))
		}
	}
	if uploadPath == "" {
		writeError(w, http.StatusBadRequest, "source part is required")
		return
	}
	if manifest.Suite == "" {
		manifest.Suite = "command"
	}
	if err := validateManifest(manifest); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if manifest.SourceSize != uploadSize || manifest.SourceDigest != uploadDigest {
		writeError(w, http.StatusUnprocessableEntity, "source checksum or size mismatch")
		return
	}
	destination, err := s.payloadPath(manifest.SourceDigest)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := installPayload(uploadPath, destination); err != nil {
		writeError(w, http.StatusInternalServerError, "install upload")
		return
	}
	job, err := s.store.CreateJob(r.Context(), protocol.CreateJob{
		ID: newJobID(), Repository: manifest.Repository, Suite: manifest.Suite, Runner: manifest.Runner, Command: manifest.Command,
		SourceDigest: manifest.SourceDigest, Timeout: time.Duration(manifest.TimeoutSeconds) * time.Second,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create job")
		return
	}
	writeJSON(w, http.StatusCreated, job)
}

func validateManifest(manifest protocol.SubmitManifest) error {
	if manifest.Repository == "" {
		return errors.New("repository is required")
	}
	if manifest.Suite == "" || len(manifest.Suite) > 64 {
		return errors.New("suite is required and must not exceed 64 characters")
	}
	if manifest.Runner != "standard" {
		return errors.New("unsupported runner pack")
	}
	if len(manifest.Command) == 0 || len(manifest.Command) > 128 {
		return errors.New("command argument vector is required")
	}
	for _, argument := range manifest.Command {
		if strings.ContainsRune(argument, '\x00') {
			return errors.New("command contains a NUL byte")
		}
	}
	if manifest.TimeoutSeconds < 1 || manifest.TimeoutSeconds > 3600 {
		return errors.New("timeout must be between 1 and 3600 seconds")
	}
	if manifest.SourceSize < 1 || manifest.SourceSize > maxSourceSize {
		return errors.New("invalid source size")
	}
	return nil
}

func (s *Server) getJob(w http.ResponseWriter, r *http.Request) {
	job, err := s.store.Job(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) cancelJob(w http.ResponseWriter, r *http.Request) {
	if err := s.store.RequestCancel(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) claimJob(w http.ResponseWriter, r *http.Request) {
	var request protocol.ClaimRequest
	if err := decodeJSON(r, &request); err != nil || request.WorkerID == "" {
		writeError(w, http.StatusBadRequest, "worker_id is required")
		return
	}
	job, ok, err := s.store.Claim(r.Context(), request.WorkerID, time.Now().Add(30*time.Second))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "claim job")
		return
	}
	if !ok {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) getSource(w http.ResponseWriter, r *http.Request) {
	job, err := s.store.Job(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	path, err := s.payloadPath(job.SourceDigest)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid stored digest")
		return
	}
	w.Header().Set("Content-Type", "application/zstd")
	http.ServeFile(w, r, path)
}

func (s *Server) appendLog(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(io.LimitReader(r.Body, maxLogChunk+1))
	if err != nil || len(data) > maxLogChunk {
		writeError(w, http.StatusRequestEntityTooLarge, "log chunk too large")
		return
	}
	if err := s.store.AppendLog(r.Context(), r.PathValue("id"), data); err != nil {
		writeError(w, http.StatusNotFound, "job log not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) heartbeat(w http.ResponseWriter, r *http.Request) {
	var request protocol.ClaimRequest
	if err := decodeJSON(r, &request); err != nil || request.WorkerID == "" {
		writeError(w, http.StatusBadRequest, "worker_id is required")
		return
	}
	if err := s.store.RenewLease(r.Context(), r.PathValue("id"), request.WorkerID, time.Now().Add(30*time.Second)); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) finishJob(w http.ResponseWriter, r *http.Request) {
	var request protocol.FinishRequest
	if err := decodeJSON(r, &request); err != nil || !request.Status.Terminal() {
		writeError(w, http.StatusBadRequest, "terminal status is required")
		return
	}
	if err := s.store.Finish(r.Context(), r.PathValue("id"), request.Status, request.ExitCode, request.ErrorMessage); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) streamLogs(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	offset, _ := strconv.ParseInt(r.Header.Get("Last-Event-ID"), 10, 64)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		data, next, err := s.store.ReadLog(r.Context(), r.PathValue("id"), offset, 64<<10)
		if err != nil {
			return
		}
		if len(data) > 0 {
			encoded := base64.StdEncoding.EncodeToString(data)
			_, _ = fmt.Fprintf(w, "id: %d\nevent: log\ndata: %s\n\n", next, encoded)
			offset = next
			flusher.Flush()
			continue
		}
		job, err := s.store.Job(r.Context(), r.PathValue("id"))
		if err != nil {
			return
		}
		if job.Status.Terminal() {
			encoded, _ := json.Marshal(job)
			_, _ = fmt.Fprintf(w, "id: %d\nevent: done\ndata: %s\n\n", offset, encoded)
			flusher.Flush()
			return
		}
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Server) payloadPath(digest string) (string, error) {
	hexDigest := strings.TrimPrefix(digest, "sha256:")
	if len(hexDigest) != 64 {
		return "", errors.New("source digest must be sha256")
	}
	if _, err := hex.DecodeString(hexDigest); err != nil {
		return "", errors.New("source digest must be lowercase hexadecimal")
	}
	if hexDigest != strings.ToLower(hexDigest) {
		return "", errors.New("source digest must be lowercase hexadecimal")
	}
	return filepath.Join(s.config.PayloadDir, hexDigest+".tar.zst"), nil
}

func installPayload(temporary, destination string) error {
	if _, err := os.Stat(destination); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Rename(temporary, destination); err != nil {
		return err
	}
	return os.Chmod(destination, 0o600)
}

func newJobID() string {
	random := make([]byte, 8)
	_, _ = rand.Read(random)
	return time.Now().UTC().Format("20060102T150405.000000000Z") + "-" + hex.EncodeToString(random)
}

func decodeJSON(r *http.Request, target any) error {
	defer r.Body.Close()
	return json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(target)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
