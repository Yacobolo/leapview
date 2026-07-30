# KPI

Use a KPI for a governed current value, an optional comparison and delta, an explicit goal, and a compact historical trend. Comparison, goal, and trend datasets run through the same semantic model and active filters as the primary value.

Every preview on this page is generated from the YAML shown below it using a fixed documentation dataset.

## Current value

Use compact mode when the current value is meaningful without a comparison. A note may add context, but it should not duplicate the title.

{{< visual id="total_orders" >}}

```yaml visual-example=total_orders
visuals:
  total_orders:
    type: kpi
    description: Shows the filtered count of distinct orders.
    query:
      measures:
        order_count: null
    kpi:
      mode: compact
    presentation:
      note: Filtered order count
      tone: ink
```

## Favorable comparison and trend

Bind comparison and trend to named datasets. `favorable_direction` is required whenever a comparison exists; LeapView never assumes that an increase is good.

{{< visual id="revenue_kpi_favorable" >}}

```yaml visual-example=revenue_kpi_favorable
visuals:
  revenue_kpi_favorable:
    title: Revenue versus baseline
    type: kpi
    description: Shows revenue, its filter-aligned baseline, and monthly trend.
    query:
      measures:
        revenue: null
    datasets:
      comparison:
        measures:
          value:
            measure: revenue_baseline
        limit: 1
      trend:
        dimensions:
          period: orders.purchase_month
        measures:
          value:
            measure: revenue
        sort:
          - field: period
            direction: asc
        limit: 12
    kpi:
      mode: compact
      comparison:
        dataset: comparison
        field: value
        reducer: first
        label: Baseline
      trend:
        dataset: trend
        category: period
        value: value
      delta: relative
      favorable_direction: increase
      missing_comparison: show_unavailable
```

## Unfavorable direction

The same positive delta is unfavorable when the authored decision context says decreases are better. The arrow, value, and status word keep the meaning available without color.

{{< visual id="revenue_kpi_unfavorable" >}}

```yaml visual-example=revenue_kpi_unfavorable
visuals:
  revenue_kpi_unfavorable:
    title: Cost proxy versus baseline
    type: kpi
    description: Demonstrates an increase that is explicitly unfavorable.
    query:
      measures:
        revenue: null
    datasets:
      comparison:
        measures:
          value:
            measure: revenue_baseline
        limit: 1
    kpi:
      comparison:
        dataset: comparison
        field: value
        label: Baseline
      delta: relative
      favorable_direction: decrease
```

## Bullet with an explicit goal

Bullet and progress modes require a goal binding. Qualitative ranges are ordered, non-overlapping, and labeled so status never depends on color alone.

{{< visual id="revenue_kpi_bullet" >}}

```yaml visual-example=revenue_kpi_bullet
visuals:
  revenue_kpi_bullet:
    title: Revenue goal
    type: kpi
    description: Shows revenue against a filter-aligned target.
    query:
      measures:
        revenue: null
    datasets:
      goal:
        measures:
          value:
            measure: revenue_target
        limit: 1
    kpi:
      mode: bullet
      goal:
        dataset: goal
        field: value
        label: Target
      ranges:
        - maximum: 4000
          label: Behind
          tone: danger
        - minimum: 4000
          maximum: 5000
          label: On track
          tone: success
        - minimum: 5000
          label: Ahead
          tone: ink
```

## Progress with an out-of-range value

The progress fill is visually clamped to its track, while the actual value and the explicit “Out of range” status remain truthful.

{{< visual id="revenue_kpi_out_of_range" >}}

```yaml visual-example=revenue_kpi_out_of_range
visuals:
  revenue_kpi_out_of_range:
    title: Revenue outside the operating band
    type: kpi
    description: Demonstrates explicit out-of-range status.
    query:
      measures:
        revenue: null
    datasets:
      goal:
        measures:
          value:
            measure: revenue_target
        limit: 1
    kpi:
      mode: progress
      goal:
        dataset: goal
        field: value
        label: Target
      ranges:
        - maximum: 4000
          label: Operating band
          tone: neutral
```

## Missing comparison

Choose whether a missing comparison is displayed as unavailable or hidden. Showing it is the safer default because it distinguishes missing context from a zero delta.

{{< visual id="revenue_kpi_missing_comparison" >}}

```yaml visual-example=revenue_kpi_missing_comparison
visuals:
  revenue_kpi_missing_comparison:
    title: Revenue with unavailable comparison
    type: kpi
    description: Demonstrates an explicitly unavailable comparison.
    query:
      measures:
        revenue: null
    datasets:
      comparison:
        measures:
          value:
            measure: missing_revenue
        limit: 1
    kpi:
      comparison:
        dataset: comparison
        field: value
        label: Prior period
      delta: absolute
      favorable_direction: neutral
      missing_comparison: show_unavailable
```
