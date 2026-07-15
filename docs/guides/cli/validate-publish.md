# Validate, plan, and publish

Validate before publishing. A plan shows the change set without applying it; publish applies the validated project.

```sh
libredash validate dashboards/libredash.yaml
libredash plan dashboards/libredash.yaml --target https://dash.example.com
libredash publish dashboards/libredash.yaml --target https://dash.example.com
```

Use `--json` when a CI system needs structured validation or plan output.
