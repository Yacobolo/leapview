# Automation and CI

Treat validation, candidate creation, publication, approval, activation, and verification as separate gates. Build one candidate from an immutable Git revision and publish that unchanged candidate only from an approved branch or environment.

## Provide bounded credentials

Use `LEAPVIEW_WORKLOAD_CLIENT_ID`, `LEAPVIEW_WORKLOAD_CLIENT_SECRET`, and `LEAPVIEW_WORKLOAD_PROJECT` for production CI. Inject the service-principal secret from the CI secret manager and prevent pull requests from untrusted forks from reading it. The validation job does not need target credentials. `LEAPVIEW_API_TOKEN` remains a compatibility option for smaller teams.

Keep the expected environment and workspace in reviewable pipeline configuration:

```sh
export TARGET_ENVIRONMENT=production
export TARGET_WORKSPACE=retail
```

## Validate without network access

Compile the complete project first and retain structured diagnostics as a job artifact:

```sh
leapview validate --project dashboards/leapview.yaml --json
```

Stop the pipeline on any non-zero exit status. Do not allow a later deployment job to replace or edit the project after validation.

## Create the immutable target candidate

Synchronize the project to the exact target that will receive the deployment:

```sh
leapview dev --once \
  --project dashboards/leapview.yaml \
  --target "$LEAPVIEW_TARGET"
```

The target compiles the uploaded source snapshot, resolves target-owned connection evidence and managed-data pins, prepares an owner-isolated preview runtime, and retains immutable provenance for that exact candidate. `dev` stores only the non-secret candidate handoff locally; credentials remain in the configured login or workload-identity flow. Run `dev` again after any source change and complete review against the candidate preview before publication.

## Publish an immutable deployment request

Run publication from a protected job using the same project path and target used by `dev`:

```sh
leapview publish \
  --project dashboards/leapview.yaml \
  --target "$LEAPVIEW_TARGET"
```

`publish` does not read, compile, or upload the project again. It submits the exact retained candidate revision and provenance produced by `dev`. An environment configured for immediate publication waits for activation to finish. A protected environment returns the immutable deployment and approval request without activating it.

Approve the exact persisted plan with a different principal holding `APPROVE_DEPLOYMENT`, then request cutover with a principal holding `ACTIVATE_DEPLOYMENT`. Immediately before cutover, LeapView rechecks the release, plan digest, approval revision, expiry, reviewer credential and grant, and activator credential and grant. Revocation or expiry closes the workflow safely.

## Preserve evidence and verify

Record the Git revision, target environment, publisher, approval, activator, plan artifact, deployment result, and managed-data digests together. After activation, verify readiness and exercise a representative workspace query or dashboard with a separate verifier identity. A transport retry must reuse the same immutable candidate; never rebuild from a moving branch between attempts.

See [Targets and environments](/docs/cli/targets) for environment safeguards and the generated [`validate`](/docs/cli/validate), [`dev`](/docs/cli/dev), and [`publish`](/docs/cli/publish) references for all flags.
