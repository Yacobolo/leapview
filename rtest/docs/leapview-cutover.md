# LeapView rtest cutover

LeapView uses the generic rtest service for CPU-heavy repository gates and trusted image
builds. The repository supplies a normal OCI image in `Dockerfile.rtest`, selects the
`leapview` project through `rtest.json`, and keeps orchestration in the existing Taskfile.
rtest has no LeapView-specific runner, preparation hook, or task language.

## Developer workflow

Run focused tests locally during the edit loop. An authenticated developer can send the
complete generated test suite or canonical CI contract to the shared worker:

```console
task rtest:test
task rtest:ci
```

Both commands execute the existing repository tasks inside the active project image. The
declared project caches persist Go build output, Go modules, Bun packages, and Terraform
providers across jobs. `task ci` remains the local fallback when remote execution is
unavailable; local Docker is only required by tasks that use Testcontainers or build
images.

## GitHub Actions

Trusted same-repository image jobs exchange GitHub OIDC identity for a short-lived,
project-scoped rtest operation token. They use the same `rtest build` command and project
as developers. No long-lived rtest credential is stored in GitHub.

Fork and Dependabot pull requests cannot receive the OIDC permission or shared worker.
The `site-image-fork` and `production-image-fork` jobs retain equivalent GitHub-hosted
Buildx builds and smoke tests for those untrusted inputs.

## Rollback

If a newly activated runner image is faulty, restore the immediately preceding immutable
digest and verify the worker before retrying CI:

```console
rtest image history --project leapview
rtest image rollback --project leapview
rtest doctor
task rtest:ci
```

If the shared service itself is unavailable, run `task ci` on a capable machine. For a
GitHub-side service outage, temporarily route the trusted image jobs through the same
GitHub-hosted Buildx and smoke-test sequence already encoded by `site-image-fork` and
`production-image-fork`. This preserves the build contract without restoring Depot.

Revoke a compromised or obsolete GitHub trust with `rtest trust github revoke <trust-id>`.
Changing or revoking a trust does not require changing repository credentials because the
workflow stores none.

The measured cutover result and immutable image references are recorded in
`rtest/evidence/service/leapview-cutover.json`.
