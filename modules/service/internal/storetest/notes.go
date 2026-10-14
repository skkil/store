package storetest

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// InsertNote returns a mutation body that creates a note.
func InsertNote(id, scope, title string) func(pgx.Tx) error {
	return func(tx pgx.Tx) error {
		_, err := tx.Exec(context.Background(),
			"INSERT INTO note (id, scope_id, title, secret) VALUES ($1, $2, $3, 'hidden')", id, scope, title)
		return err
	}
}

// RenameNote returns a mutation body that renames a note.
func RenameNote(id, title string) func(pgx.Tx) error {
	return func(tx pgx.Tx) error {
		_, err := tx.Exec(context.Background(), "UPDATE note SET title = $2 WHERE id = $1", id, title)
		return err
	}
}

// DeleteNote returns a mutation body that deletes a note and, by cascade, its tags.
func DeleteNote(id string) func(pgx.Tx) error {
	return func(tx pgx.Tx) error {
		_, err := tx.Exec(context.Background(), "DELETE FROM note WHERE id = $1", id)
		return err
	}
}

// TagNote returns a mutation body that adds a tag to a note.
func TagNote(id, noteID, tag string) func(pgx.Tx) error {
	return func(tx pgx.Tx) error {
		_, err := tx.Exec(context.Background(), "INSERT INTO note_tag (id, note_id, tag) VALUES ($1, $2, $3)", id, noteID, tag)
		return err
	}
}
