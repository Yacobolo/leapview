package platform

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

func TestGlobalAgentMigrationPreservesConversationMessagesRunsAndEvents(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", sqliteDSN(filepath.Join(t.TempDir(), "migration.db")))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	goose.SetBaseFS(migrationsFS)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("set dialect: %v", err)
	}
	if err := goose.UpToContext(ctx, db, "migrations", 35); err != nil {
		t.Fatalf("migrate to 35: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO principals (id, email, display_name) VALUES ('principal-1', 'person@example.com', 'Person')`); err != nil {
		t.Fatalf("seed principal: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO agent_conversations (id, workspace_id, principal_id, title, status) VALUES ('conversation-1', 'sales', 'principal-1', 'Preserved', 'active')`); err != nil {
		t.Fatalf("seed conversation: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO agent_runs (id, conversation_id, status) VALUES ('run-1', 'conversation-1', 'completed')`); err != nil {
		t.Fatalf("seed run: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO agent_messages (id, conversation_id, run_id, seq, role, content_text) VALUES ('message-1', 'conversation-1', 'run-1', 1, 'assistant', 'Preserved answer')`); err != nil {
		t.Fatalf("seed message: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO api_async_events (resource_kind, resource_id, event_id, event_type, data_json, created_at) VALUES ('agent_run', 'run-1', 1, 'agent_run.completed', '{}', CURRENT_TIMESTAMP)`); err != nil {
		t.Fatalf("seed event: %v", err)
	}

	if err := goose.UpToContext(ctx, db, "migrations", 36); err != nil {
		t.Fatalf("migrate to 36: %v", err)
	}
	var workspaceColumns int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_table_info('agent_conversations') WHERE name = 'workspace_id'`).Scan(&workspaceColumns); err != nil {
		t.Fatalf("inspect conversation schema: %v", err)
	}
	if workspaceColumns != 0 {
		t.Fatalf("workspace_id columns = %d, want 0", workspaceColumns)
	}
	for table, id := range map[string]string{
		"agent_conversations": "conversation-1",
		"agent_runs":          "run-1",
		"agent_messages":      "message-1",
	} {
		var count int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table+` WHERE id = ?`, id).Scan(&count); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if count != 1 {
			t.Fatalf("%s row %s was not preserved", table, id)
		}
	}
	var eventCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM api_async_events WHERE resource_kind = 'agent_run' AND resource_id = 'run-1' AND event_id = 1`).Scan(&eventCount); err != nil {
		t.Fatalf("count api_async_events: %v", err)
	}
	if eventCount != 1 {
		t.Fatal("agent run event was not preserved")
	}
}
