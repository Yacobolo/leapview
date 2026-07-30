# Automation and CI

Treat validation, planning, publication, approval, activation, and verification as separate gates. Build one candidate from an immutable Git revision and publish that unchanged candidate only from an approved branch or environment.

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

## Generate a reviewable plan

Plan against the exact target identity and workspace that will receive the deployment:

```sh
leapview plan \
  --project dashboards/leapview.yaml \
  --target "$LEAPVIEW_TARGET" \
  --environment "$TARGET_ENVIRONMENT" \
  --workspace "$TARGET_WORKSPACE" \
  --json > leapview-plan.json
```

Publish `leapview-plan.json` as a protected artifact. Review removals, access-policy changes, resource identity changes, and managed-data revision pins. Regenerate the plan after any source change.

## Publish an immutable deployment request

Run publication from a protected job using the same project, target, environment, and revision pins that produced the reviewed plan:

```sh
leapview deploy \
  --project dashboards/leapview.yaml \
  --target "$LEAPVIEW_TARGET" \
  --environment "$TARGET_ENVIRONMENT" \
  --auto-approve
```

`--auto-approve` accepts the CLI's local confirmation prompt only. It never bypasses target policy. A production target returns the immutable deployment and approval IDs without activating it.

Approve the exact persisted plan with a different principal holding `APPROVE_DEPLOYMENT`, then request cutover with a principal holding `ACTIVATE_DEPLOYMENT`. Immediately before cutover, LeapView rechecks the release, plan digest, approval revision, expiry, reviewer credential and grant, and activator credential and grant. Revocation or expiry closes the workflow safely.

## Preserve evidence and verify

Record the Git revision, target environment, publisher, approval, activator, plan artifact, deployment result, and managed-data digests together. After activation, verify readiness and exercise a representative workspace query or dashboard with a separate verifier identity. A transport retry must reuse the same immutable candidate; never rebuild from a moving branch between attempts.

See [Validate, plan, and deploy](/docs/cli/validate-deploy) for the operational sequence, [Targets and environments](/docs/cli/targets) for environment safeguards, and the generated [`validate`](/docs/cli/validate), [`plan`](/docs/cli/plan), and [`deploy`](/docs/cli/deploy) references for all flags.
