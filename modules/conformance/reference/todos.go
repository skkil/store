package reference

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"

	"github.com/skkil/store/modules/conformance/writepath"
)

// The rejection reasons of the todo mutations.
const (
	ReasonEmptyTitle    = "title must not be empty"
	ReasonEmptyItem     = "item text must not be empty"
	ReasonTodoNotFound  = "todo not found"
	ReasonTodoExists    = "todo already exists"
	ReasonBadArguments  = "invalid arguments"
	reasonUnknownPrefix = "unknown mutation: "
)

type createTodoArgs struct {
	Title string   `json:"title"`
	Items []string `json:"items"`
}

type renameTodoArgs struct {
	ID    uuid.UUID `json:"id"`
	Title string    `json:"title"`
}

type todoRefArgs struct {
	ID uuid.UUID `json:"id"`
}

type addItemArgs struct {
	TodoID uuid.UUID `json:"todo_id"`
	Text   string    `json:"text"`
}

// Todos returns the reference product's mutations:
//
//   - todo.create {title, items?}: a todo with the mutation's first derived
//     id and one item per text with the following ids. An empty title is
//     rejected before any write; an empty item text after the todo and the
//     items before it were written, so only the savepoint keeps the
//     rejection whole.
//   - todo.rename {id, title}, todo.complete {id}, todo.delete {id} (its
//     items go by ON DELETE CASCADE).
//   - item.create {todo_id, text}.
func Todos() *Registry {
	r := NewRegistry()
	r.Register("todo.create", createTodo)
	r.Register("todo.rename", renameTodo)
	r.Register("todo.complete", completeTodo)
	r.Register("todo.delete", deleteTodo)
	r.Register("item.create", addItem)
	return r
}

func createTodo(ctx context.Context, m *writepath.Mutation, raw json.RawMessage) error {
	var args createTodoArgs
	if err := decode(raw, &args); err != nil {
		return err
	}
	if strings.TrimSpace(args.Title) == "" {
		return writepath.Reject(ReasonEmptyTitle)
	}
	id := m.NextID()
	var exists bool
	if err := m.Tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM todo WHERE id = $1)", id).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return writepath.Reject(ReasonTodoExists)
	}
	if _, err := m.Tx.Exec(ctx, "INSERT INTO todo (id, scope_id, title) VALUES ($1, $2, $3)", id, m.Scope, args.Title); err != nil {
		return err
	}
	for _, text := range args.Items {
		if strings.TrimSpace(text) == "" {
			return writepath.Reject(ReasonEmptyItem)
		}
		if _, err := m.Tx.Exec(ctx, "INSERT INTO todo_item (id, todo_id, text) VALUES ($1, $2, $3)", m.NextID(), id, text); err != nil {
			return err
		}
	}
	return nil
}

func renameTodo(ctx context.Context, m *writepath.Mutation, raw json.RawMessage) error {
	var args renameTodoArgs
	if err := decode(raw, &args); err != nil {
		return err
	}
	if strings.TrimSpace(args.Title) == "" {
		return writepath.Reject(ReasonEmptyTitle)
	}
	return updateOne(ctx, m, "UPDATE todo SET title = $3 WHERE id = $1 AND scope_id = $2", args.ID, args.Title)
}

func completeTodo(ctx context.Context, m *writepath.Mutation, raw json.RawMessage) error {
	var args todoRefArgs
	if err := decode(raw, &args); err != nil {
		return err
	}
	return updateOne(ctx, m, "UPDATE todo SET done = true WHERE id = $1 AND scope_id = $2", args.ID)
}

func deleteTodo(ctx context.Context, m *writepath.Mutation, raw json.RawMessage) error {
	var args todoRefArgs
	if err := decode(raw, &args); err != nil {
		return err
	}
	return updateOne(ctx, m, "DELETE FROM todo WHERE id = $1 AND scope_id = $2", args.ID)
}

func addItem(ctx context.Context, m *writepath.Mutation, raw json.RawMessage) error {
	var args addItemArgs
	if err := decode(raw, &args); err != nil {
		return err
	}
	if strings.TrimSpace(args.Text) == "" {
		return writepath.Reject(ReasonEmptyItem)
	}
	var exists bool
	if err := m.Tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM todo WHERE id = $1 AND scope_id = $2)", args.TodoID, m.Scope).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return writepath.Reject(ReasonTodoNotFound)
	}
	_, err := m.Tx.Exec(ctx, "INSERT INTO todo_item (id, todo_id, text) VALUES ($1, $2, $3)", m.NextID(), args.TodoID, args.Text)
	return err
}

func updateOne(ctx context.Context, m *writepath.Mutation, sql string, id uuid.UUID, extra ...any) error {
	tag, err := m.Tx.Exec(ctx, sql, append([]any{id, m.Scope}, extra...)...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return writepath.Reject(ReasonTodoNotFound)
	}
	return nil
}

func decode(raw json.RawMessage, into any) error {
	if err := json.Unmarshal(raw, into); err != nil {
		return writepath.Reject(ReasonBadArguments + ": " + err.Error())
	}
	return nil
}
