package composectl

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const qualificationBrowserImage = "mcr.microsoft.com/playwright:v1.61.1-noble"

type qualificationAuthoringOptions struct {
	BundleRoot      string
	Image           string
	ClientBaseImage string
	CredentialsFile string
	ComposeProject  string
	EvidenceDir     string
	SourceRevision  string
	Target          string
	Project         string
	ProjectID       string
	Environment     string
	WorkspaceID     string
}

type qualificationCredentials struct {
	Email                 string `json:"email"`
	TemporaryPassword     string `json:"temporaryPassword"`
	PublisherToken        string `json:"publisherToken"`
	PublisherTokenExpires string `json:"publisherTokenExpiresAt"`
	WorkloadToken         string `json:"workloadToken,omitempty"`
	ProjectDataToken      string `json:"projectDataToken,omitempty"`
	RecoveryControlToken  string `json:"recoveryControlToken,omitempty"`
	QualificationPassword string `json:"qualificationPassword"`
}

func (credentials qualificationCredentials) workloadToken() (string, error) {
	token := strings.TrimSpace(credentials.WorkloadToken)
	if token == "" {
		return "", fmt.Errorf("dedicated qualification workload token is required")
	}
	return token, nil
}

func (credentials qualificationCredentials) projectDataToken() (string, error) {
	token := strings.TrimSpace(credentials.ProjectDataToken)
	if token == "" {
		return "", fmt.Errorf("dedicated qualification project-data token is required")
	}
	return token, nil
}

func (credentials qualificationCredentials) recoveryControlToken() (string, error) {
	token := strings.TrimSpace(credentials.RecoveryControlToken)
	if token == "" {
		return "", fmt.Errorf("dedicated qualification recovery-control token is required")
	}
	return token, nil
}

func qualificationWorkloadPrivileges() []string {
	return []string{
		"USE_WORKSPACE",
		"VIEW_ITEM",
		"QUERY_DATA",
		"REFRESH_DATA",
	}
}

func qualificationProjectDataPrivileges() []string {
	return []string{"VIEW_DATA"}
}

type qualificationAuthoringReport struct {
	SchemaVersion  int                          `json:"schemaVersion"`
	Result         string                       `json:"result"`
	Target         string                       `json:"target"`
	Candidate      string                       `json:"candidate"`
	Revision       int64                        `json:"revision"`
	SourceArtifact string                       `json:"sourceArtifact"`
	Artifact       string                       `json:"artifact"`
	ReleaseDigest  string                       `json:"releaseDigest"`
	Principal      string                       `json:"principal"`
	SourceRevision string                       `json:"sourceRevision"`
	Phases         []qualificationPhaseEvidence `json:"phases"`
	Assertions     struct {
		BrowserApprovedLogin    bool `json:"browserApprovedLogin"`
		NativeKeyring           bool `json:"nativeKeyring"`
		PrivatePreview          bool `json:"privatePreview"`
		ExactCandidateActivated bool `json:"exactCandidateActivated"`
	} `json:"assertions"`
}

type qualificationBrowserToken struct {
	AccessToken string `json:"accessToken"`
}

type qualificationReviewerResponse struct {
	Principal struct {
		ID string `json:"id"`
	} `json:"principal"`
	TemporaryPassword string `json:"temporaryPassword"`
}

type qualificationDeploymentResponse struct {
	Status    string `json:"status"`
	Error     string `json:"error"`
	CreatedBy string `json:"createdBy"`
	Approval  *struct {
		ID       string `json:"id"`
		Status   string `json:"status"`
		Revision int64  `json:"revision"`
	} `json:"approval"`
	Evidence struct {
		CandidateID       string `json:"candidateId"`
		CandidateRevision int64  `json:"candidateRevision"`
		TargetID          string `json:"targetId"`
		ArtifactDigest    string `json:"artifactDigest"`
		ReleaseDigest     string `json:"releaseDigest"`
	} `json:"evidence"`
}

func (c *Controller) runQualificationAuthoring(
	ctx context.Context,
	options qualificationAuthoringOptions,
) (report qualificationAuthoringReport, runErr error) {
	rootContext := ctx
	options = normalizeQualificationAuthoringOptions(options)
	if err := validateQualificationAuthoringOptions(options); err != nil {
		return report, err
	}
	var credentials qualificationCredentials
	if err := readQualificationJSON(options.CredentialsFile, &credentials); err != nil {
		return report, err
	}
	if credentials.Email == "" || credentials.TemporaryPassword == "" ||
		credentials.QualificationPassword == "" {
		return report, fmt.Errorf("authoring qualification credentials are incomplete")
	}
	if err := os.MkdirAll(options.EvidenceDir, 0o700); err != nil {
		return report, err
	}
	workDir := filepath.Join(options.EvidenceDir, ".authoring-work")
	if err := os.RemoveAll(workDir); err != nil {
		return report, err
	}
	if err := os.MkdirAll(workDir, 0o700); err != nil {
		return report, err
	}
	report.SchemaVersion = qualificationEvidenceSchema
	report.Result = "failure"
	phases := newQualificationPhaseTracker(c.now)
	ctx = phases.Begin(rootContext, "browser and client setup", 15*time.Minute)

	runSuffix := normalizedQualificationName(
		options.ComposeProject + "-" + strconv.Itoa(os.Getpid()),
	)
	clientImage := "leapview-authoring-client:" + runSuffix
	clientContainer := "leapview-authoring-client-" + runSuffix
	browserContainer := "leapview-authoring-browser-" + runSuffix
	certificateFile := filepath.Join(workDir, "caddy-root.crt")

	cleanup := qualificationCleanup{}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), qualificationCleanupTimeout)
		defer cancel()
		runErr = joinQualificationError(runErr, cleanup.Run(cleanupCtx))
		runErr = phases.Finish(runErr)
		if runErr != nil {
			report.Result = "failure"
		}
		report.Phases = phases.Evidence()
		if runErr != nil {
			_ = writeQualificationJSON(
				filepath.Join(options.EvidenceDir, "authoring-report.json"),
				report,
			)
		}
	}()
	cleanup.Add(func(context.Context) error { return os.RemoveAll(workDir) })

	caddyOutput, err := c.qualificationCompose(ctx, options.BundleRoot, "ps", "--quiet", "caddy")
	if err != nil {
		return report, err
	}
	caddyContainer := strings.TrimSpace(string(caddyOutput))
	if caddyContainer == "" {
		return report, fmt.Errorf("qualification Caddy container is not running")
	}
	certCtx, cancelCert := qualificationContext(ctx, time.Minute)
	err = qualificationWait(certCtx, time.Second, func(waitCtx context.Context) (bool, error) {
		_, copyErr := c.qualificationDocker(
			waitCtx,
			nil,
			"cp",
			caddyContainer+":/data/caddy/pki/authorities/local/root.crt",
			certificateFile,
		)
		if copyErr != nil {
			return false, nil
		}
		info, statErr := os.Stat(certificateFile)
		return statErr == nil && info.Size() > 0, nil
	})
	cancelCert()
	if err != nil {
		return report, fmt.Errorf("copy qualification CA certificate: %w", err)
	}
	if err := os.Chmod(certificateFile, 0o644); err != nil {
		return report, err
	}

	qualificationRoot := filepath.Join(options.BundleRoot, "qualification")
	if _, err := c.qualificationDocker(
		ctx,
		nil,
		"build",
		"--file", filepath.Join(qualificationRoot, "Dockerfile.authoring-client"),
		"--build-arg", "LEAPVIEW_IMAGE="+options.ClientBaseImage,
		"--tag", clientImage,
		qualificationRoot,
	); err != nil {
		return report, fmt.Errorf("build qualification client: %w", err)
	}
	cleanup.Add(func(cleanupCtx context.Context) error {
		_, err := c.qualificationDocker(cleanupCtx, nil, "image", "rm", "--force", clientImage)
		return err
	})
	if _, err := c.qualificationDocker(ctx, nil, "pull", qualificationBrowserImage); err != nil {
		return report, fmt.Errorf("pull qualification browser: %w", err)
	}

	browser, err := c.qualificationContainers.Start(ctx, qualificationContainerRequest{
		Name: browserContainer, Image: qualificationBrowserImage, NetworkMode: "host",
		Volumes: []qualificationContainerVolume{
			{Source: qualificationRoot, Target: "/qualification", ReadOnly: true},
			{Source: options.EvidenceDir, Target: "/evidence"},
		},
		Environment: map[string]string{
			"QUALIFICATION_URL":           options.Target,
			"QUALIFICATION_PROJECT_ID":    options.ProjectID,
			"QUALIFICATION_EVIDENCE_ROOT": "/evidence",
		},
		Command: []string{"sleep", "infinity"},
	})
	if err != nil {
		return report, fmt.Errorf("start qualification browser: %w", err)
	}
	cleanup.Add(func(cleanupCtx context.Context) error {
		_, err := browser.Remove(cleanupCtx)
		return err
	})
	if _, err := browser.Exec(ctx, nil, "mkdir", "-p", "/work"); err != nil {
		return report, qualificationContainerOperationError(
			ctx, browser, "prepare authoring browser work directory", err,
		)
	}
	for _, name := range []string{"package.json", "authoring-worker.mjs"} {
		if _, err := browser.CopyTo(
			ctx,
			filepath.Join(qualificationRoot, name),
			"/work/"+name,
		); err != nil {
			return report, qualificationContainerOperationError(
				ctx, browser, "copy authoring browser asset "+name, err,
			)
		}
	}
	if _, err := browser.Exec(
		ctx, nil,
		"npm", "install", "--prefix", "/work", "--no-audit", "--no-fund", "--silent",
	); err != nil {
		return report, qualificationContainerOperationError(
			ctx, browser, "install authoring browser dependencies", err,
		)
	}
	browserWorker, err := startQualificationJSONWorker(
		rootContext,
		c.root,
		os.Environ(),
		c.dockerBin,
		"exec", "-i", browserContainer,
		"node", "/work/authoring-worker.mjs",
	)
	if err != nil {
		return report, fmt.Errorf("start qualification browser worker: %w", err)
	}
	cleanup.Add(func(context.Context) error { return browserWorker.Kill() })
	if err := phases.Finish(nil); err != nil {
		return report, err
	}
	ctx = phases.Begin(rootContext, "reviewer provisioning", 10*time.Minute)

	var authenticated struct {
		Authenticated bool `json:"authenticated"`
	}
	if err := browserWorker.CallContext(ctx, "signInAdministrator", map[string]string{
		"email":             credentials.Email,
		"temporaryPassword": credentials.TemporaryPassword,
		"password":          credentials.QualificationPassword,
	}, &authenticated, nil); err != nil {
		return report, err
	}
	var administratorToken qualificationBrowserToken
	if err := browserWorker.CallContext(ctx, "issueAdministratorToken", map[string]any{
		"privileges": []string{"MANAGE_GRANTS"},
	}, &administratorToken, nil); err != nil {
		return report, err
	}
	if administratorToken.AccessToken == "" {
		return report, fmt.Errorf("browser worker returned an empty administrator token")
	}

	apiClient := qualificationHTTPSClient()
	reviewerEmail := fmt.Sprintf("authoring-reviewer-%d@qualification.invalid", time.Now().UnixNano())
	var reviewer qualificationReviewerResponse
	if err := qualificationAPI(
		ctx,
		apiClient,
		http.MethodPost,
		options.Target+"/api/v1/principals",
		administratorToken.AccessToken,
		map[string]string{
			"email":       reviewerEmail,
			"displayName": "Authoring Qualification Reviewer",
		},
		"authoring-reviewer-"+runSuffix,
		&reviewer,
	); err != nil {
		return report, fmt.Errorf("create qualification reviewer: %w", err)
	}
	if reviewer.Principal.ID == "" || reviewer.TemporaryPassword == "" {
		return report, fmt.Errorf("reviewer creation returned incomplete credentials")
	}
	for _, privilege := range []string{
		"VIEW_ITEM",
		"APPROVE_DEPLOYMENT",
		"ACTIVATE_DEPLOYMENT",
	} {
		if err := qualificationAPI(
			ctx,
			apiClient,
			http.MethodPost,
			fmt.Sprintf(
				"%s/api/v1/workspaces/%s/grants",
				options.Target,
				url.PathEscape(options.ProjectID),
			),
			administratorToken.AccessToken,
			map[string]string{
				"objectType":  "project_environment",
				"objectId":    options.Environment,
				"privilege":   privilege,
				"subjectId":   reviewer.Principal.ID,
				"subjectType": "principal",
			},
			"authoring-reviewer-"+strings.ToLower(privilege)+"-"+runSuffix,
			nil,
		); err != nil {
			return report, fmt.Errorf(
				"grant qualification reviewer %s: %w",
				privilege,
				err,
			)
		}
	}
	if err := browserWorker.CallContext(ctx, "signInReviewer", map[string]string{
		"email":             reviewerEmail,
		"temporaryPassword": reviewer.TemporaryPassword,
		"password":          credentials.QualificationPassword + "-reviewer",
	}, &authenticated, nil); err != nil {
		return report, err
	}
	var reviewerToken qualificationBrowserToken
	if err := browserWorker.CallContext(ctx, "issueReviewerToken", map[string]any{
		"privileges": []string{
			"VIEW_ITEM",
			"APPROVE_DEPLOYMENT",
			"ACTIVATE_DEPLOYMENT",
		},
	}, &reviewerToken, nil); err != nil {
		return report, err
	}
	if reviewerToken.AccessToken == "" {
		return report, fmt.Errorf("browser worker returned an empty reviewer token")
	}
	if err := phases.Finish(nil); err != nil {
		return report, err
	}
	ctx = phases.Begin(rootContext, "native keyring login", 10*time.Minute)

	keyringPassword, err := randomHex(24)
	if err != nil {
		return report, err
	}
	clientEnvironment := append(
		os.Environ(),
		"QUALIFICATION_KEYRING_PASSWORD="+keyringPassword,
	)
	clientWorker, err := startQualificationJSONWorker(
		rootContext,
		c.root,
		clientEnvironment,
		c.dockerBin,
		"run", "--rm", "-i",
		"--name", clientContainer,
		"--network", "host",
		"--volume", certificateFile+":/run/certs/caddy-root.crt:ro",
		"--env", "QUALIFICATION_KEYRING_PASSWORD",
		"--env", "SSL_CERT_FILE=/run/certs/caddy-root.crt",
		clientImage,
		"dbus-run-session", "--",
		"/usr/local/libexec/leapviewctl",
		"qualify", "client-worker",
		"--target", options.Target,
		"--project", options.Project,
		"--source-revision", options.SourceRevision,
	)
	if err != nil {
		return report, fmt.Errorf("start qualification CLI worker: %w", err)
	}
	keyringPassword = ""
	cleanup.Add(func(context.Context) error { return clientWorker.Kill() })
	cleanup.Add(func(cleanupCtx context.Context) error {
		_, err := c.qualificationContainers.Existing(clientContainer).Remove(cleanupCtx)
		return ignoreQualificationNotFound(err)
	})

	if err := clientWorker.CallContext(
		ctx,
		"login",
		nil,
		&authenticated,
		func(event string, raw json.RawMessage) error {
			if event != "device_challenge" {
				return fmt.Errorf("unexpected CLI worker event %q", event)
			}
			var challenge qualificationLoginChallenge
			if err := json.Unmarshal(raw, &challenge); err != nil {
				return err
			}
			var authorized struct {
				Authorized bool `json:"authorized"`
			}
			return browserWorker.CallContext(ctx, "authorizeCLI", challenge, &authorized, nil)
		},
	); err != nil {
		return report, err
	}
	if err := phases.Finish(nil); err != nil {
		return report, err
	}
	ctx = phases.Begin(rootContext, "private candidate preview", 10*time.Minute)
	var candidate QualificationCandidate
	if err := clientWorker.CallContext(ctx, "dev", nil, &candidate, nil); err != nil {
		return report, err
	}
	var preview struct {
		CandidateID       string `json:"candidateId"`
		GovernedOrderRows int    `json:"governedOrderRows"`
	}
	if err := browserWorker.CallContext(ctx, "verifyPreview", map[string]string{
		"candidateId": candidate.ID,
		"previewUrl":  candidate.PreviewURL,
	}, &preview, nil); err != nil {
		return report, err
	}
	if preview.CandidateID != candidate.ID || preview.GovernedOrderRows != 24 {
		return report, fmt.Errorf("browser verified the wrong candidate or governed row count")
	}
	if err := phases.Finish(nil); err != nil {
		return report, err
	}
	ctx = phases.Begin(rootContext, "protected publish", 15*time.Minute)
	var publication QualificationPublication
	if err := clientWorker.CallContext(ctx, "publish", nil, &publication, nil); err != nil {
		return report, err
	}
	deployment, err := approveAndActivateQualificationPublication(
		ctx,
		apiClient,
		options,
		reviewerToken.AccessToken,
		publication,
		runSuffix,
	)
	if err != nil {
		return report, err
	}
	if err := verifyExactAuthoringCandidate(candidate, publication, deployment); err != nil {
		return report, err
	}
	workloadToken, err := c.createQualificationAPIToken(
		ctx,
		apiClient,
		options.Target,
		administratorToken.AccessToken,
		"qualification-workload",
		options.WorkspaceID,
		qualificationWorkloadPrivileges(),
		"qualification-workload-"+runSuffix,
	)
	if err != nil {
		return report, err
	}
	projectDataToken, err := c.createQualificationAPIToken(
		ctx,
		apiClient,
		options.Target,
		administratorToken.AccessToken,
		"qualification-project-data",
		"",
		qualificationProjectDataPrivileges(),
		"qualification-project-data-"+runSuffix,
	)
	if err != nil {
		return report, err
	}
	credentials.WorkloadToken = workloadToken
	credentials.ProjectDataToken = projectDataToken
	credentials.RecoveryControlToken = reviewerToken.AccessToken
	if err := writeQualificationJSON(options.CredentialsFile, credentials); err != nil {
		return report, fmt.Errorf("persist qualification scoped credentials: %w", err)
	}
	if err := phases.Finish(nil); err != nil {
		return report, err
	}

	report.Result = "success"
	report.Target = candidate.TargetID
	report.Candidate = candidate.ID
	report.Revision = candidate.Revision
	report.SourceArtifact = candidate.ArtifactDigest
	report.Artifact = publication.ArtifactDigest
	report.ReleaseDigest = candidate.ProvenanceDigest
	report.Principal = candidate.PrincipalID
	report.SourceRevision = publication.SourceRevision
	report.Assertions.BrowserApprovedLogin = true
	report.Assertions.NativeKeyring = true
	report.Assertions.PrivatePreview = true
	report.Assertions.ExactCandidateActivated = true
	report.Phases = phases.Evidence()
	if err := writeQualificationJSON(
		filepath.Join(options.EvidenceDir, "authoring-report.json"),
		report,
	); err != nil {
		return report, err
	}
	if _, err := fmt.Fprintf(
		c.stdout,
		"enterprise authoring qualification passed for candidate %s revision %d\n",
		candidate.ID,
		candidate.Revision,
	); err != nil {
		return report, err
	}
	return report, nil
}

func (c *Controller) createQualificationAPIToken(
	ctx context.Context,
	client *http.Client,
	target string,
	authorizationToken string,
	name string,
	workspaceID string,
	privileges []string,
	idempotencyKey string,
) (string, error) {
	body := map[string]any{
		"name":       name,
		"privileges": privileges,
		"expiresAt":  c.now().UTC().Add(2 * time.Hour).Format(time.RFC3339),
	}
	if workspaceID != "" {
		body["workspaceId"] = workspaceID
	}
	var created struct {
		Token string `json:"token"`
	}
	if err := qualificationAPI(
		ctx,
		client,
		http.MethodPost,
		target+"/api/v1/me/api-tokens",
		authorizationToken,
		body,
		idempotencyKey,
		&created,
	); err != nil {
		return "", err
	}
	if created.Token == "" {
		return "", fmt.Errorf("%s token creation returned an empty credential", name)
	}
	return created.Token, nil
}

func normalizeQualificationAuthoringOptions(options qualificationAuthoringOptions) qualificationAuthoringOptions {
	if options.ClientBaseImage == "" {
		options.ClientBaseImage = options.Image
	}
	if options.Target == "" {
		options.Target = "https://localhost"
	}
	if options.Project == "" {
		options.Project = "/workspace/evaluation/project/leapview.yaml"
	}
	if options.ProjectID == "" {
		options.ProjectID = "leapview-evaluation"
	}
	if options.Environment == "" {
		options.Environment = "evaluation"
	}
	if options.WorkspaceID == "" {
		options.WorkspaceID = "evaluation"
	}
	return options
}

func validateQualificationAuthoringOptions(options qualificationAuthoringOptions) error {
	for label, value := range map[string]string{
		"bundle root":        options.BundleRoot,
		"image":              options.Image,
		"client base image":  options.ClientBaseImage,
		"credentials file":   options.CredentialsFile,
		"Compose project":    options.ComposeProject,
		"evidence directory": options.EvidenceDir,
		"target":             options.Target,
		"project":            options.Project,
		"project ID":         options.ProjectID,
		"environment":        options.Environment,
		"workspace ID":       options.WorkspaceID,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("qualification authoring %s is required", label)
		}
	}
	return nil
}

func qualificationHTTPSClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			// The isolated production Compose target uses its generated local
			// Caddy CA. This client is restricted to that disposable target.
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
		},
	}
}

func qualificationAPI(
	ctx context.Context,
	client *http.Client,
	method string,
	endpoint string,
	token string,
	body any,
	idempotencyKey string,
	result any,
) error {
	var reader io.Reader
	if body != nil {
		contents, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(contents)
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return err
	}
	applyQualificationLoopbackHost(request)
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if idempotencyKey != "" {
		request.Header.Set("Idempotency-Key", idempotencyKey)
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	contents, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("%s %s returned %d: %s", method, endpoint, response.StatusCode, contents)
	}
	if result != nil && len(bytes.TrimSpace(contents)) > 0 {
		if err := json.Unmarshal(contents, result); err != nil {
			return fmt.Errorf("decode %s %s: %w", method, endpoint, err)
		}
	}
	return nil
}

func approveAndActivateQualificationPublication(
	ctx context.Context,
	client *http.Client,
	options qualificationAuthoringOptions,
	token string,
	publication QualificationPublication,
	runSuffix string,
) (QualificationDeployment, error) {
	endpoint := fmt.Sprintf(
		"%s/api/v1/projects/%s/deployments/%s",
		options.Target,
		url.PathEscape(options.ProjectID),
		url.PathEscape(publication.DeploymentID),
	)
	var response qualificationDeploymentResponse
	if err := qualificationAPI(ctx, client, http.MethodGet, endpoint, token, nil, "", &response); err != nil {
		return QualificationDeployment{}, err
	}
	if response.Approval == nil || response.Approval.Status != "pending" {
		return QualificationDeployment{}, fmt.Errorf("publication approval is not pending")
	}
	approvalEndpoint := fmt.Sprintf(
		"%s/approval-requests/%s/approve",
		endpoint,
		url.PathEscape(response.Approval.ID),
	)
	var approval struct {
		Status string `json:"status"`
	}
	if err := qualificationAPI(
		ctx,
		client,
		http.MethodPost,
		approvalEndpoint,
		token,
		map[string]int64{"expectedRevision": response.Approval.Revision},
		"authoring-approve-"+runSuffix,
		&approval,
	); err != nil {
		return QualificationDeployment{}, err
	}
	if approval.Status != "approved" {
		return QualificationDeployment{}, fmt.Errorf("publication approval transitioned to %q", approval.Status)
	}
	if err := qualificationAPI(
		ctx,
		client,
		http.MethodPost,
		endpoint+"/activate",
		token,
		nil,
		"authoring-activate-"+runSuffix,
		nil,
	); err != nil {
		return QualificationDeployment{}, err
	}
	activationCtx, cancel := qualificationContext(ctx, 5*time.Minute)
	defer cancel()
	err := qualificationWait(activationCtx, 250*time.Millisecond, func(waitCtx context.Context) (bool, error) {
		if err := qualificationAPI(
			waitCtx,
			client,
			http.MethodGet,
			endpoint,
			token,
			nil,
			"",
			&response,
		); err != nil {
			return false, err
		}
		switch response.Status {
		case "active":
			return true, nil
		case "cancelled", "failed", "superseded":
			return false, fmt.Errorf(
				"publication activation ended in %s: %s",
				response.Status,
				response.Error,
			)
		default:
			return false, nil
		}
	})
	if err != nil {
		return QualificationDeployment{}, err
	}
	return QualificationDeployment{
		CandidateID:       response.Evidence.CandidateID,
		CandidateRevision: response.Evidence.CandidateRevision,
		TargetID:          response.Evidence.TargetID,
		PrincipalID:       response.CreatedBy,
		ArtifactDigest:    response.Evidence.ArtifactDigest,
		ReleaseDigest:     response.Evidence.ReleaseDigest,
		Status:            response.Status,
	}, nil
}

func qualificationArchitecture() string {
	return runtime.GOARCH
}
