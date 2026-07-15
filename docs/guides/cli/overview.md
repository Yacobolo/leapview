# CLI overview

The `libredash` CLI runs the local server, validates a dashboard-as-code project, and publishes it to a LibreDash instance.

## Common workflow

```sh
libredash validate dashboards/libredash.yaml
libredash plan dashboards/libredash.yaml
libredash publish dashboards/libredash.yaml --target https://dash.example.com
```

Use the generated command reference when you need every flag and subcommand.
