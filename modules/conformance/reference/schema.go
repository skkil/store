// Package reference is a minimal product server for store: a todo list
// whose Push runs every mutation through the Go write path (package
// writepath) and whose token endpoint issues ES256 sync tokens. It is the
// product the conformance suite is proven against, and a worked example of
// what a product server does.
package reference

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Schema is the reference product's tables, registered for sync. It runs
// after the store migrations in modules/sql. The scope column stays on the
// server: clients never see it.
const Schema = `
CREATE TABLE todo (
  id uuid PRIMARY KEY,
  scope_id uuid NOT NULL,
  title text NOT NULL,
  done boolean NOT NULL DEFAULT false
);
CREATE INDEX todo_scope ON todo (scope_id);
CREATE TABLE todo_item (
  id uuid PRIMARY KEY,
  todo_id uuid NOT NULL REFERENCES todo ON DELETE CASCADE,
  text text NOT NULL
);
CREATE INDEX todo_item_todo ON todo_item (todo_id);
SELECT store_register('todo', 'id', ARRAY['scope_id']);
SELECT store_register('todo_item');
`

// Migrate applies Schema to a database that already has the store migrations.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, Schema); err != nil {
		return fmt.Errorf("reference schema: %w", err)
	}
	return nil
}
