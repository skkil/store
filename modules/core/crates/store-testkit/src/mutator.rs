use store_core::{
    MutationContext, MutationError, Mutator,
    rusqlite::{OptionalExtension, Transaction},
    serde_json::{Value, json},
};

/// The planner app's mutations, run by clients and by the [`FakeServer`](crate::FakeServer).
///
/// | Name | Arguments | Rule |
/// | --- | --- | --- |
/// | `project.create` | `name` | name required |
/// | `project.archive` | `id` | project exists |
/// | `project.delete` | `id` | project exists; tasks cascade |
/// | `task.create` | `project_id`, `title`, `labels?`, `attachment_hex?` | title required, project exists; `position` = tasks in the project |
/// | `task.rename` | `id`, `title` | task exists, title required |
/// | `task.complete` | `id` | task exists |
/// | `task.increment` | `id` | task exists; `counter += 1` |
/// | `task.delete` | `id` | task exists |
#[derive(Debug, Clone, Copy, Default)]
pub struct PlannerMutator;

impl Mutator for PlannerMutator {
    fn apply(
        &self,
        tx: &Transaction<'_>,
        ctx: &mut MutationContext,
        name: &str,
        args: &Value,
    ) -> Result<Value, MutationError> {
        match name {
            "project.create" => {
                let name = required_text(args, "name")?;
                let id = ctx.next_id().to_string();
                tx.execute(
                    "INSERT INTO project (id, name, created_at) VALUES (?1, ?2, ?3)",
                    (&id, name, ctx.now()),
                )?;
                Ok(json!({ "id": id }))
            }
            "project.archive" => {
                let id = existing(tx, "project", args)?;
                tx.execute("UPDATE project SET archived = 1 WHERE id = ?1", [&id])?;
                Ok(Value::Null)
            }
            "project.delete" => {
                let id = existing(tx, "project", args)?;
                tx.execute("DELETE FROM project WHERE id = ?1", [&id])?;
                Ok(Value::Null)
            }
            "task.create" => {
                let title = required_text(args, "title")?;
                let project_id = text(args, "project_id")?;
                if !exists(tx, "project", &project_id)? {
                    return Err(MutationError::rejected("project not found"));
                }
                let position: i64 = tx.query_row(
                    "SELECT count(*) FROM task WHERE project_id = ?1",
                    [&project_id],
                    |row| row.get(0),
                )?;
                let labels = args.get("labels").cloned().unwrap_or_else(|| json!([]));
                let attachment = args
                    .get("attachment_hex")
                    .and_then(Value::as_str)
                    .map(decode_hex)
                    .transpose()?;
                let id = ctx.next_id().to_string();
                tx.execute(
                    "INSERT INTO task (id, project_id, title, position, labels, attachment, created_at)
                     VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7)",
                    (
                        &id,
                        &project_id,
                        title,
                        position,
                        labels.to_string(),
                        attachment,
                        ctx.now(),
                    ),
                )?;
                Ok(json!({ "id": id, "position": position }))
            }
            "task.rename" => {
                let title = required_text(args, "title")?;
                let id = existing(tx, "task", args)?;
                tx.execute("UPDATE task SET title = ?2 WHERE id = ?1", (&id, title))?;
                Ok(Value::Null)
            }
            "task.complete" => {
                let id = existing(tx, "task", args)?;
                tx.execute("UPDATE task SET done = 1 WHERE id = ?1", [&id])?;
                Ok(Value::Null)
            }
            "task.increment" => {
                let id = existing(tx, "task", args)?;
                tx.execute("UPDATE task SET counter = counter + 1 WHERE id = ?1", [&id])?;
                Ok(Value::Null)
            }
            "task.delete" => {
                let id = existing(tx, "task", args)?;
                tx.execute("DELETE FROM task WHERE id = ?1", [&id])?;
                Ok(Value::Null)
            }
            other => Err(MutationError::Unknown(other.to_owned())),
        }
    }
}

fn text(args: &Value, key: &str) -> Result<String, MutationError> {
    args.get(key)
        .and_then(Value::as_str)
        .map(str::to_owned)
        .ok_or_else(|| MutationError::InvalidArgs(format!("{key} must be a string")))
}

fn required_text(args: &Value, key: &str) -> Result<String, MutationError> {
    let value = text(args, key)?;
    if value.trim().is_empty() {
        return Err(MutationError::rejected(format!("{key} is required")));
    }
    Ok(value)
}

fn existing(tx: &Transaction<'_>, table: &str, args: &Value) -> Result<String, MutationError> {
    let id = text(args, "id")?;
    if exists(tx, table, &id)? {
        Ok(id)
    } else {
        Err(MutationError::rejected(format!("{table} not found")))
    }
}

fn exists(tx: &Transaction<'_>, table: &str, id: &str) -> Result<bool, MutationError> {
    Ok(tx
        .query_row(
            &format!("SELECT 1 FROM {table} WHERE id = ?1"),
            [id],
            |_| Ok(()),
        )
        .optional()?
        .is_some())
}

fn decode_hex(hex: &str) -> Result<Vec<u8>, MutationError> {
    (0..hex.len())
        .step_by(2)
        .map(|index| {
            hex.get(index..index + 2)
                .and_then(|pair| u8::from_str_radix(pair, 16).ok())
                .ok_or_else(|| MutationError::InvalidArgs("attachment_hex is not hex".into()))
        })
        .collect()
}
