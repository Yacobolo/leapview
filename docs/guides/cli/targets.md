# Targets and environments

Use an explicit target for production-like operations and an explicit environment when creating a publish plan. Keep local development, staging, and production targets separate.

```sh
libredash plan dashboards/libredash.yaml \
  --target https://dash.staging.example.com \
  --environment staging
```

The same project definition can be validated locally before it is published to a remote target.
