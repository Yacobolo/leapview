# Map

Use a map for regional comparisons or observations with geographic coordinates.

Every preview on this page is generated from the YAML shown below it using a fixed documentation dataset.

Every map uses LeapView's built-in worldwide vector basemap. The PMTiles archive, style, glyphs, and sprites are embedded in the released Go binary and served through content-addressed same-origin URLs. A deployment needs no map directory, CDN, object store, runtime download, or internet access, and map rendering never sends governed coordinates or browsing activity to a third party.

`geo.basemap` has been removed from the pre-1.0 authoring contract. Existing declarations, including `basemap: blank`, are rejected so a compiled geographic specification can never silently lose geographic context. Remove the field during migration; there is no replacement setting.

The current package is derived from the pinned Protomaps snapshot `https://build.protomaps.com/20260720.pmtiles` with `go-pmtiles v1.31.1`. It contains worldwide zooms 0–6 only. The archive is exactly 44,725,293 bytes with SHA-256 `2d97ee8907670936ab722da7ca06eafec0734392f73fa1cd337d4debd85d676f`; the style SHA-256 is `eeb32e219ad7dd4178377e21a2f11477b44408ab44a4878579692315add1e7f7`; and glyphs and sprites are pinned to Protomaps basemap-assets revision `028c18f713baecad011301ff7a69acc39bcc2ae7`. Data is attributed to OpenStreetMap contributors under ODbL 1.0, and the derived style is BSD-3-Clause.

Maintainers rebuild the checked-in package with `task map-assets:generate`. The generator verifies every digest and the exact archive size before the package can be embedded. Application readiness verifies the embedded inventory, HTTP qualification exercises `200`/`206` range delivery, `Content-Range`, `Accept-Ranges`, immutable caching, and ETags, and production-image CI measures the exact packaged binary against its size budget.

The basemap is intentionally a global decision-context layer, not a street-navigation product. World, country, first-order region, major-city, water, boundary, and primary-road context is qualified through zoom 6. Brazil uses the same worldwide archive and style as every other region; no regional extraction or privileged rendering path exists. Street-level detail above zoom 6 and author-supplied tile sources are explicit non-goals for this package.

The previews on this page exercise rendering only. Semantic and spatial selections require the dashboard command and SSE runtime, so crossfilter behavior belongs in a runtime-capable dashboard rather than this static visual reference. Camera, zoom, reset, compass, deterministic label density, and light/dark basemap themes are typed under `geo`. Cross-visual behavior is documented in the [Filters and interactions guide](/docs/guides/build/filters-interactions).

## Choropleth

A choropleth joins a query dimension to a content-addressed geometry asset. The `join` and `value` properties reference query aliases, not model field names.

{{< visual id="state_order_map" >}}

```yaml visual-example=state_order_map
visuals:
  state_order_map:
    title: Orders by state
    description: Maps order count by Brazilian state.
    type: map
    query:
      dimensions:
        state: orders.state
      measures:
        order_count: null
      sort:
        - field: order_count
          direction: desc
      limit: 27
    geo:
      theme: light
      label_density: normal
      controls: {zoom: true, reset: true, compass: true}
      layers:
        - id: states
          kind: choropleth
          geometry_asset: brazil_states
          join: state
          value: order_count
          tooltip: [state, order_count]
          color:
            kind: sequential
            palette: teal
            null_color: "#d8dee4"
```

## Points

Point layers bind numeric latitude and longitude query aliases. An optional value controls marker size without exposing MapLibre configuration.

The Visual Showcase includes a dedicated `chart-map-scale` page backed by exactly one million deterministic locations. It demonstrates the production spatial-window path: LeapView aggregates the governed viewport at low zoom, returns raw governed points only when the visible cardinality fits, and never sends more than 5,000 rendered features to the browser.

{{< visual id="order_point_map" >}}

```yaml visual-example=order_point_map
visuals:
  order_point_map:
    title: Order locations
    type: map
    query:
      dimensions:
        order_id: orders.order_id
        latitude: orders.latitude
        longitude: orders.longitude
      measures:
        revenue: null
      limit: 100
    geo:
      camera: {mode: fit_data, padding: 32, max_zoom: 9}
      controls: {zoom: true, reset: true, compass: true}
      layers:
        - id: orders
          kind: point
          latitude: latitude
          longitude: longitude
          value: revenue
          label: order_id
          tooltip: [order_id, revenue]
          size: {minimum_radius: 5, maximum_radius: 28}
          stroke: {color: "#ffffff", width: 1.5, opacity: 1}
```

## Heat

Heat layers aggregate a numeric value around each coordinate. Keep the query bounded so the browser receives a predictable frame.

{{< visual id="revenue_heat_map" >}}

```yaml visual-example=revenue_heat_map
visuals:
  revenue_heat_map:
    title: Revenue concentration
    type: map
    query:
      dimensions:
        latitude: orders.latitude
        longitude: orders.longitude
      measures:
        revenue: null
      limit: 100
    geo:
      theme: dark
      layers:
        - id: revenue
          kind: heat
          latitude: latitude
          longitude: longitude
          value: revenue
          heat: {radius: 28, intensity: 1.15}
```

## Density

Density layers emphasize the concentration of observations. The layer needs coordinates but does not require a value binding.

{{< visual id="order_density_map" >}}

```yaml visual-example=order_density_map
visuals:
  order_density_map:
    title: Order density
    type: map
    query:
      dimensions:
        latitude: orders.latitude
        longitude: orders.longitude
      measures:
        order_count: null
      limit: 100
    geo:
      layers:
        - id: orders
          kind: density
          latitude: latitude
          longitude: longitude
          heat: {radius: 22, intensity: 1.35}
```

## Reference boundary

Reference layers add immutable, content-addressed point, line, or polygon context without joining query values into the geometry. They are display-only.

{{< visual id="state_reference_map" >}}

```yaml visual-example=state_reference_map
visuals:
  state_reference_map:
    title: Brazil state reference boundaries
    type: map
    query:
      dimensions:
        state: orders.state
      measures:
        order_count: null
      limit: 27
    geo:
      layers:
        - id: state_boundaries
          kind: reference
          geometry_asset: brazil_states
          color: {kind: sequential, palette: blue, null_color: "#d8dee4"}
          stroke: {color: "#57606a", width: 1.5, opacity: 1}
          opacity: 0.12
```

## Paths

Path layers group coordinate rows by a stable path alias and order vertices deterministically. Use them for governed routes, flows, and trajectories rather than routing-service output.

{{< visual id="state_order_paths" >}}

```yaml visual-example=state_order_paths
visuals:
  state_order_paths:
    title: State order paths
    type: map
    query:
      dimensions:
        state: orders.state
        order_id: orders.order_id
        latitude: orders.latitude
        longitude: orders.longitude
      measures:
        revenue: null
      limit: 100
    geo:
      controls: {zoom: true, reset: true, compass: true}
      layers:
        - id: state_paths
          kind: path
          latitude: latitude
          longitude: longitude
          path: state
          order: order_id
          value: revenue
          tooltip: [state, revenue]
          stroke: {color: "#0969da", width: 3, opacity: 0.9}
          line: {width: 3, curvature: 0}
          opacity: 0.9
```
