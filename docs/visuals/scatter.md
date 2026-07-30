# Scatter chart

Use a scatter chart to compare category positions, expose series, or emphasize individual points.

Every preview on this page is generated from the YAML shown below it using a fixed documentation dataset.

## Basic

Rank delivery-speed bands by average review score so isolated satisfaction values and gaps remain visible without implying a continuous line or a false lexical SLA sequence.

{{< visual id="delivery_scatter" >}}

```yaml visual-example=delivery_scatter
visuals:
  delivery_scatter:
    title: Review score by delivery speed
    type: scatter
    query:
      dimensions:
        delivery_bucket: orders.delivery_bucket
      measures:
        review_score: null
      sort:
        - field: value
          direction: desc
      limit: 30
```

## Multiple series

Map status through `query.series` to compare fulfillment outcomes while ranking the delivery-speed bands by review score.

{{< visual id="delivery_scatter_status" >}}

```yaml visual-example=delivery_scatter_status
visuals:
  delivery_scatter_status:
    title: Delivery satisfaction by status
    type: scatter
    query:
      dimensions:
        delivery_bucket: orders.delivery_bucket
      series:
        field: orders.status
        alias: status
      measures:
        review_score: null
      sort:
        - field: value
          direction: desc
      limit: 60
```

## Labeled points

Enable labels and place them above larger symbols when exact point values matter and the dataset is small enough to avoid overlap.

{{< visual id="delivery_scatter_labeled" >}}

```yaml visual-example=delivery_scatter_labeled
visuals:
  delivery_scatter_labeled:
    title: Revenue by product category
    type: scatter
    presentation:
      show_labels: true
      label_position: top
      symbol_size: 12
    query:
      dimensions:
        category: orders.category
      measures:
        revenue: null
      sort:
        - field: value
          direction: desc
```
