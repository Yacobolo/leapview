-- +goose Up
-- A freshly migrated database must satisfy the access schema invariants even
-- before the access module performs its idempotent startup reconciliation.

INSERT INTO roles (id, name, privileges_json)
VALUES
  ('role_owner', 'owner', '["USE_WORKSPACE","VIEW_ITEM","EDIT_ITEM","MANAGE_ITEM","QUERY_DATA","PREVIEW_DATA","REFRESH_DATA","DEPLOY","ACTIVATE_DEPLOYMENT","MANAGE_PUBLICATIONS","USE_AGENT","VIEW_AGENT","MANAGE_GRANTS","VIEW_AUDIT","MANAGE_WORKSPACE"]'),
  ('role_admin', 'admin', '["USE_WORKSPACE","VIEW_ITEM","EDIT_ITEM","MANAGE_ITEM","QUERY_DATA","PREVIEW_DATA","REFRESH_DATA","DEPLOY","ACTIVATE_DEPLOYMENT","MANAGE_PUBLICATIONS","USE_AGENT","VIEW_AGENT","MANAGE_GRANTS","VIEW_AUDIT","MANAGE_WORKSPACE"]'),
  ('role_deployer', 'deployer', '["USE_WORKSPACE","VIEW_ITEM","QUERY_DATA","REFRESH_DATA","DEPLOY","ACTIVATE_DEPLOYMENT"]'),
  ('role_contributor', 'contributor', '["USE_WORKSPACE","VIEW_ITEM","EDIT_ITEM","QUERY_DATA","REFRESH_DATA","DEPLOY","USE_AGENT","VIEW_AGENT"]'),
  ('role_editor', 'editor', '["USE_WORKSPACE","VIEW_ITEM","EDIT_ITEM","QUERY_DATA","REFRESH_DATA","USE_AGENT","VIEW_AGENT"]'),
  ('role_member', 'member', '["USE_WORKSPACE","VIEW_ITEM","EDIT_ITEM","MANAGE_ITEM","QUERY_DATA","REFRESH_DATA","DEPLOY","USE_AGENT","VIEW_AGENT"]'),
  ('role_viewer', 'viewer', '["USE_WORKSPACE","VIEW_ITEM","QUERY_DATA","USE_AGENT","VIEW_AGENT"]'),
  ('role_data_deployer', 'data_deployer', '["VIEW_DATA","INGEST_DATA"]'),
  ('role_platform_admin', 'platform_admin', '["MANAGE_PLATFORM","VIEW_DATA","INGEST_DATA","USE_WORKSPACE","VIEW_ITEM","EDIT_ITEM","MANAGE_ITEM","QUERY_DATA","PREVIEW_DATA","REFRESH_DATA","DEPLOY","ACTIVATE_DEPLOYMENT","MANAGE_PUBLICATIONS","USE_AGENT","VIEW_AGENT","MANAGE_GRANTS","VIEW_AUDIT","MANAGE_WORKSPACE"]')
ON CONFLICT(name) DO UPDATE SET
  id = excluded.id,
  privileges_json = excluded.privileges_json;

DELETE FROM role_grant_templates;

INSERT INTO role_grant_templates (role_name, privilege)
SELECT roles.name, CAST(json_each.value AS TEXT)
FROM roles, json_each(roles.privileges_json);

INSERT INTO securable_objects (id, object_type, display_name)
VALUES ('platform', 'platform', 'Platform')
ON CONFLICT(id) DO UPDATE SET
  object_type = excluded.object_type,
  workspace_id = '',
  parent_id = '',
  display_name = excluded.display_name;
