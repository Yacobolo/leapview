# Funnel chart

Use a funnel chart to show ordered stages whose values decrease through a process.

Every preview on this page is generated from the YAML shown below it using a fixed documentation dataset.

## Basic

Use cumulative fulfillment-stage rows so every order enters at the top and only qualifying orders advance to each narrower stage.

{{< visual id="status_funnel" >}}

```yaml visual-example=status_funnel
visuals:
  status_funnel:
    title: Order fulfillment funnel
    type: funnel
    query:
      table: order_funnel
      dimensions:
        stage: order_funnel.stage
      measures:
        funnel_order_count: null
      sort:
        - field: value
          direction: desc
```

## Revenue retained by stage

Measure revenue retained at the same fulfillment stages to expose the monetary impact of attrition separately from order volume.

{{< visual id="revenue_funnel" >}}

```yaml visual-example=revenue_funnel
visuals:
  revenue_funnel:
    title: Fulfillment revenue funnel
    type: funnel
    query:
      table: order_funnel
      dimensions:
        stage: order_funnel.stage
      measures:
        funnel_revenue: null
      sort:
        - field: value
          direction: desc
```

## Aligned labels

Set `presentation.align: left` to anchor the stages, keep labels visible, and use `presentation.sort` to control the visual stage order independently.

{{< visual id="status_funnel_left" >}}

```yaml visual-example=status_funnel_left
visuals:
  status_funnel_left:
    title: Labeled fulfillment funnel
    type: funnel
    presentation:
      align: left
      sort: ascending
      labels: {density: automatic, priority: [selected, anomaly, threshold], max_characters: 20, minimum_spacing: 6, tooltip_fallback: true}
    query:
      table: order_funnel
      dimensions:
        stage: order_funnel.stage
      measures:
        funnel_order_count: null
      sort:
        - field: value
          direction: desc
```
