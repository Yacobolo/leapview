# Get started with LibreDash

LibreDash keeps the semantic layer, report definition, and visual runtime together. Start with the included workspace, then make it your own.

## Bootstrap the workspace

Download the sample data and prepare the local workspace.

```sh
task bootstrap
```

## Run LibreDash

Start the local application and open the dashboard workspace.

```sh
task dev
```

## Edit the model and dashboard

Keep your semantic model and report definition together under `dashboards/`.

```text
dashboards/
  catalog.yaml
  olist/
    model.yaml
    executive-sales.yaml
```

## Explore the visual system

See the chart, table, matrix, and pivot components that the dashboard contract can render in the [visual gallery](/charts). The project source and issue tracker are available on [GitHub](https://github.com/Yacobolo/libredash).
