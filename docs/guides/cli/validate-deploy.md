# Develop, review, and publish

Validation proves that the project is structurally coherent. `dev` creates a private candidate on the intended target and keeps it synchronized while you work. `publish` promotes that exact reviewed candidate without rebuilding it.

## Before you begin

Authenticate to the intended target with `leapview login` or a bounded workload identity. The developer needs permission to create and preview private candidates; publishing and approval remain separate target-side privileges. For managed connections, upload data to the target before creating the candidate.

Use the same project path and target throughout the sequence:

1. Validate the complete project locally.
2. Run `dev` against the exact target.
3. Review the private candidate preview.
4. Publish the unchanged candidate.
5. Verify the active workspace and representative queries.

Store target and credential values outside shell history:

```sh
export LEAPVIEW_TARGET=https://dash.example.com
leapview login "$LEAPVIEW_TARGET"
```

## Validate the project

Run validation before contacting a target:

```sh
leapview validate --project dashboards/leapview.yaml
```

Resolve every configuration, discovery, reference, and policy diagnostic. Validation covers the complete resource graph, so a failure in an unchanged file can still block a candidate whose combined state is invalid.

Use `--json` in CI and fail the job on a non-zero exit status. Keep human-readable output for local review.

## Create and review the candidate

Start the authoring loop against the intended target:

```sh
leapview dev \
  --project dashboards/leapview.yaml \
  --target "$LEAPVIEW_TARGET"
```

LeapView uploads content-addressed source files, compiles them on the target, resolves target-owned connection evidence and managed-data pins, and returns an owner-isolated preview URL. Source edits update that candidate while the command is running. The local handoff contains candidate IDs and digests only; it never contains connection secrets.

For a single CI synchronization, use `leapview dev --once`. Review removals, access changes, and managed-data pins from the candidate evidence and preview. Run `dev` again after any source edit.

## Publish the reviewed candidate

Publish with the same project path and target:

```sh
leapview publish \
  --project dashboards/leapview.yaml \
  --target "$LEAPVIEW_TARGET"
```

`publish` loads the exact candidate handoff produced by `dev`; it does not reread or rebuild the project. Immediate-policy targets wait for activation to finish. Protected targets persist a pending approval request bound to the candidate revision, provenance, plan, policy, connection evidence, managed-data pins, and base generation.

Record the source revision, candidate and provenance digests, target environment, deployment result, and operator or automation identity together.

## Verify the deployment

Open the target workspace and confirm the expected project revision is active. Exercise one catalog page, one representative semantic query, one dashboard with a filter, and one model refresh or table window affected by the change. Check application and audit logs for rejected candidates or background failures.

A failed candidate must leave the last valid serving state active. Treat that protection as recovery behavior, not as a reason to skip pre-deployment checks.

## Troubleshooting

If validation succeeds but `dev` fails, verify target authentication, environment identity, server/client contract compatibility, target connection bindings, and managed-data availability. If `publish` reports that no candidate exists, run `dev` with the same project path and target. If publication is pending, use a different authorized principal for approval. For authorization failures, inspect effective grants rather than broadening credentials immediately.

## Next steps

Automate this sequence with protected environments and immutable source revisions. Continue with [Deploy and operate](/docs/guides/operate), [Production configuration](/docs/guides/operate/production-configuration), and the generated [`dev`](/docs/cli/dev), [`publish`](/docs/cli/publish), and [`validate`](/docs/cli/validate) references.
