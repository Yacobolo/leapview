-- +goose Up
-- Lineage edges are domain relationships, so the same typed edge should only
-- appear once within a deployment graph.

CREATE UNIQUE INDEX IF NOT EXISTS asset_edges_unique_idx
  ON asset_edges(deployment_id, from_asset_id, to_asset_id, edge_type);
