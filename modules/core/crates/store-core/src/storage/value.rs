use rusqlite::types::Value as SqlValue;
use serde_json::{Map, Value};

/// How a JSON row image was produced, which decides how values are read back.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub(crate) enum ImageEncoding {
    /// A server row (`to_jsonb`): booleans become 0/1, objects and arrays
    /// become JSON text.
    Server,
    /// A local pre-image written by the capture triggers, where a BLOB is
    /// stored as `{"$blob": "<hex>"}`.
    PreImage,
}

pub(crate) const BLOB_KEY: &str = "$blob";

/// Converts one JSON value of a row image to the SQLite value to store.
pub(crate) fn json_to_sql(value: &Value, encoding: ImageEncoding) -> SqlValue {
    match value {
        Value::Null => SqlValue::Null,
        Value::Bool(flag) => SqlValue::Integer(i64::from(*flag)),
        Value::Number(number) => match number.as_i64() {
            Some(integer) => SqlValue::Integer(integer),
            None => SqlValue::Real(number.as_f64().unwrap_or(f64::NAN)),
        },
        Value::String(text) => SqlValue::Text(text.clone()),
        Value::Object(map) if encoding == ImageEncoding::PreImage => {
            blob(map).unwrap_or_else(|| SqlValue::Text(value.to_string()))
        }
        Value::Array(_) | Value::Object(_) => SqlValue::Text(value.to_string()),
    }
}

fn blob(map: &Map<String, Value>) -> Option<SqlValue> {
    if map.len() != 1 {
        return None;
    }
    let hex = map.get(BLOB_KEY)?.as_str()?;
    decode_hex(hex).map(SqlValue::Blob)
}

fn decode_hex(hex: &str) -> Option<Vec<u8>> {
    if !hex.len().is_multiple_of(2) {
        return None;
    }
    (0..hex.len())
        .step_by(2)
        .map(|index| u8::from_str_radix(hex.get(index..index + 2)?, 16).ok())
        .collect()
}

#[cfg(test)]
mod tests {
    use rusqlite::types::Value as SqlValue;
    use serde_json::json;

    use super::{ImageEncoding, json_to_sql};

    #[test]
    fn coerces_server_values() {
        let server = ImageEncoding::Server;
        assert_eq!(json_to_sql(&json!(true), server), SqlValue::Integer(1));
        assert_eq!(json_to_sql(&json!(false), server), SqlValue::Integer(0));
        assert_eq!(json_to_sql(&json!(null), server), SqlValue::Null);
        assert_eq!(json_to_sql(&json!(42), server), SqlValue::Integer(42));
        assert_eq!(json_to_sql(&json!(1.5), server), SqlValue::Real(1.5));
        assert_eq!(json_to_sql(&json!("a"), server), SqlValue::Text("a".into()));
        assert_eq!(
            json_to_sql(&json!({"a": [1, true]}), server),
            SqlValue::Text(r#"{"a":[1,true]}"#.into())
        );
        assert_eq!(
            json_to_sql(&json!([1, 2]), server),
            SqlValue::Text("[1,2]".into())
        );
    }

    #[test]
    fn large_unsigned_numbers_become_reals() {
        assert_eq!(
            json_to_sql(&json!(u64::MAX), ImageEncoding::Server),
            SqlValue::Real(u64::MAX as f64)
        );
    }

    #[test]
    fn pre_images_decode_blobs_and_servers_do_not() {
        let image = json!({"$blob": "00ff10"});
        assert_eq!(
            json_to_sql(&image, ImageEncoding::PreImage),
            SqlValue::Blob(vec![0, 255, 16])
        );
        assert_eq!(
            json_to_sql(&image, ImageEncoding::Server),
            SqlValue::Text(r#"{"$blob":"00ff10"}"#.into())
        );
        assert_eq!(
            json_to_sql(&json!({"$blob": "xyz"}), ImageEncoding::PreImage),
            SqlValue::Text(r#"{"$blob":"xyz"}"#.into())
        );
    }
}
