# Map

Use a map to compare measures across a supported geographic boundary.

Every preview on this page is generated from the YAML shown below it using a fixed documentation dataset.

## Basic

Use `type: map`, select a typed geometry asset, and return region identifiers that match its boundaries. Here the state values use Brazilian two-letter codes.

{{< visual id="state_order_map" >}}

```yaml visual-example=state_order_map
visuals:
  state_order_map:
    title: Orders by state
    description: Maps order count by customer state.
    type: map
    presentation: {}
    query:
      dimensions:
        state: orders.state
      measures:
        order_count: null
      sort:
        - field: value
          direction: desc
      limit: 27
    geo:
      geometry_asset: brazil_states
```

## Alternate measure

Keep the same geographic dimension and replace the measure with revenue to recolor each state by monetary value.

{{< visual id="state_revenue_map" >}}

```yaml visual-example=state_revenue_map
visuals:
  state_revenue_map:
    title: Revenue by state
    type: map
    presentation: {}
    query:
      dimensions:
        state: orders.state
      measures:
        revenue: null
      sort:
        - field: value
          direction: desc
      limit: 27
    geo:
      geometry_asset: brazil_states
```

## Labels and roaming

Enable `show_labels` for visible region codes and `roam` when readers need to pan or zoom into small boundaries.

{{< visual id="state_revenue_map_labeled" >}}

```yaml visual-example=state_revenue_map_labeled
visuals:
  state_revenue_map_labeled:
    title: Labeled revenue map
    type: map
    presentation:
      show_labels: true
      roam: true
    query:
      dimensions:
        state: orders.state
      measures:
        revenue: null
      sort:
        - field: value
          direction: desc
      limit: 27
    geo:
      geometry_asset: brazil_states
```
