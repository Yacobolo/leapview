# Tree

Use a tree to show hierarchical paths when parent-child structure should remain explicit.

Every preview on this page is generated from the YAML shown below it using a fixed documentation dataset.

## Category branches

Use six product categories as compact roots and fulfillment statuses as children so the first example visibly branches instead of collapsing into dozens of one-order state roots.

{{< visual id="category_status_tree" >}}

```yaml visual-example=category_status_tree
visuals:
  category_status_tree:
    title: Category and status tree
    description: Shows product categories and fulfillment statuses as a compact hierarchy.
    type: tree
    presentation:
      show_labels: true
    query:
      dimensions:
        category: orders.category
        status: orders.status
      measures:
        order_count: null
      sort:
        - field: value
          direction: desc
      limit: 80
```

## Delivery and status hierarchy

Use five delivery-speed bands as roots to compare how fulfillment outcomes branch within operational SLA groupings.

{{< visual id="delivery_status_tree" >}}

```yaml visual-example=delivery_status_tree
visuals:
  delivery_status_tree:
    title: Delivery speed and status tree
    type: tree
    presentation:
      show_labels: true
    query:
      dimensions:
        delivery_bucket: orders.delivery_bucket
        status: orders.status
      measures:
        order_count: null
      sort:
        - field: value
          direction: desc
      limit: 80
```

## Three-level hierarchy

Add delivery speed as an intermediate level, use `initial_depth` to limit the initial expansion, and apply a dense label policy so deeper nodes remain legible as the card resizes.

{{< visual id="category_delivery_status_tree" >}}

```yaml visual-example=category_delivery_status_tree
visuals:
  category_delivery_status_tree:
    title: Category, delivery speed, and status tree
    type: tree
    presentation:
      show_labels: true
      orientation: vertical
      initial_depth: 2
      labels: {density: dense, priority: [selected, anomaly, threshold], max_characters: 16, minimum_spacing: 2, tooltip_fallback: true}
    query:
      dimensions:
        category: orders.category
        delivery_bucket: orders.delivery_bucket
        status: orders.status
      measures:
        order_count: null
      sort:
        - field: value
          direction: desc
      limit: 120
```
