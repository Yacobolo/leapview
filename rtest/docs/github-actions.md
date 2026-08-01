# GitHub Actions

GitHub Actions authenticates directly to the rtest HTTPS control plane with OIDC. There
is no SSH key, shared rtest token, Tailscale identity, Docker credential, or persistent CI
secret.

## One-time service policy

Create a project trust with immutable GitHub owner and repository IDs and the narrowest
workflow policy that fits the repository:

```console
rtest trust github create \
  --project poc \
  --owner-id <repository_owner_id> \
  --repository-id <repository_id> \
  --workflow-ref 'flidai/leapview/.github/workflows/rtest-poc.yml@refs/heads/*' \
  --ref 'refs/heads/*' \
  --environment rtest-poc \
  --event workflow_dispatch
```

The server validates GitHub's issuer, signature/JWKS, audience, expiry, not-before time,
immutable IDs, workflow, ref, environment, and event before issuing a project-scoped
session lasting at most 15 minutes.

## Repository configuration

Create a protected GitHub environment named `rtest-poc`, then configure variables:

- `RTEST_SERVICE_URL`: the control-plane URL, such as `https://rtest.example.com`;
- `RTEST_PROJECT_IMAGE`: an optional digest-pinned per-run override; normally omit it and
  use the digest-pinned image activated on the rtest project;
- `RTEST_CA_CERTIFICATE`: the service CA PEM when HTTPS uses the rtest private CA. Omit
  this variable after placing the control plane behind a publicly trusted certificate.

The workflow needs `id-token: write` and `contents: read`. The composite action writes a
mode-0600 configuration under `RUNNER_TEMP`; the CLI detects the standard Actions OIDC
environment, requests an ID token for the rtest audience, and exchanges it on demand.
The action exposes its required `project` input as `RTEST_PROJECT`, so the OIDC exchange
and every subsequent operation are bound to that selected project. It does not create or
rely on a user-wide default.

The POC stays `workflow_dispatch`-only until its manual hosted proof passes. For a
`pull_request` trust, rtest requires an `--environment`; configure that GitHub environment
with required reviewers or an equally strong deployment protection rule. GitHub's signed
OIDC claims include `head_ref` but do not include the head repository's immutable ID, so
the rtest server cannot honestly distinguish a same-repository PR from a fork by JWT
claims alone. Environment approval is therefore the explicit transition from untrusted to
trusted. Do not approve a fork, Dependabot run, or workflow change until its code is safe
to execute on the shared Docker worker.

Repository names remain audit metadata rather than authorization keys. A rename continues
to work when the immutable `repository_id` and `repository_owner_id` still match; reusing
an old name with different IDs does not grant access.
