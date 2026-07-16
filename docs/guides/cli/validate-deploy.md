# Validate, plan, and deploy

Validate before deploying. A plan shows the change set without applying it; deploy applies the validated project.

```sh
libredash validate --project dashboards/libredash.yaml
libredash plan --project dashboards/libredash.yaml --target https://dash.example.com
libredash deploy --project dashboards/libredash.yaml --target https://dash.example.com
```

Use `--json` when a CI system needs structured validation or plan output.
