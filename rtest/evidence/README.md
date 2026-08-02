# Evidence policy

This directory commits only compact, reviewed, machine-readable evidence:

- `service-local/proof.json` records the final local shared-service E2E;
- `service/proof.json` records the final CPX32 shared-service E2E;
- `github-actions/proof.json` records the hosted GitHub OIDC and Testcontainers E2E;
- `service/leapview-cutover.json` records the project image, canonical remote CI run,
  and Depot cutover proof for LeapView;
- benchmark `summary.json` files record the five-run warm-cache samples used
  by `docs/benchmarks.md`.

The E2E and benchmark scripts also produce raw logs, job responses, Docker
status, and intermediate measurements. Those files are intentionally ignored:
they are noisy, environment-specific operational output and can contain host
details that do not belong in source control. Re-run the documented scripts
when raw evidence is needed for diagnosis.

No credential, certificate, private key, bootstrap token, Terraform state, or
provider artifact may be committed as evidence.
