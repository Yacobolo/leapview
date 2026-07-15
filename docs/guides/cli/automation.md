# Automation and CI

Treat validation and publishing as separate CI steps. Validate every change, generate a plan for review, then publish only from an approved branch.

```sh
libredash validate dashboards/libredash.yaml --json
libredash plan dashboards/libredash.yaml --target "$LIBREDASH_TARGET" --json
libredash publish dashboards/libredash.yaml --target "$LIBREDASH_TARGET" --auto-approve
```

Provide tokens only through the deployment environment or your CI secret manager.
