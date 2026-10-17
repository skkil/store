use std::io::{ErrorKind, Read};

use serde::Deserialize;

use crate::{Error, Result};

pub(crate) const PROTOCOL_VERSION_HEADER: &str = "Connect-Protocol-Version";
pub(crate) const UNARY_CONTENT_TYPE: &str = "application/proto";
pub(crate) const STREAM_CONTENT_TYPE: &str = "application/connect+proto";

const FLAG_COMPRESSED: u8 = 0b01;
const FLAG_END_STREAM: u8 = 0b10;
const HEADER_LEN: usize = 5;

/// One message of a Connect stream.
#[derive(Debug)]
pub(crate) enum Frame {
    Message(Vec<u8>),
    End(Option<Error>),
}

/// Wraps `message` in a Connect streaming envelope.
pub(crate) fn envelope(message: &[u8]) -> Vec<u8> {
    let mut framed = Vec::with_capacity(HEADER_LEN + message.len());
    framed.push(0);
    framed.extend_from_slice(&(message.len() as u32).to_be_bytes());
    framed.extend_from_slice(message);
    framed
}

/// Reads the next envelope from `reader`; `None` at a clean end of input.
pub(crate) fn read_frame(reader: &mut impl Read) -> Result<Option<Frame>> {
    let mut header = [0u8; HEADER_LEN];
    match reader.read_exact(&mut header[..1]) {
        Ok(()) => {}
        Err(error) if error.kind() == ErrorKind::UnexpectedEof => return Ok(None),
        Err(error) => return Err(error.into()),
    }
    reader.read_exact(&mut header[1..])?;
    let flags = header[0];
    let len = u32::from_be_bytes([header[1], header[2], header[3], header[4]]) as usize;
    let mut data = vec![0u8; len];
    reader.read_exact(&mut data)?;
    if flags & FLAG_COMPRESSED != 0 {
        return Err(Error::Protocol(
            "compressed messages were not requested".into(),
        ));
    }
    if flags & FLAG_END_STREAM != 0 {
        return Ok(Some(Frame::End(end_stream_error(&data)?)));
    }
    Ok(Some(Frame::Message(data)))
}

#[derive(Deserialize)]
struct WireError {
    code: Option<String>,
    message: Option<String>,
}

#[derive(Deserialize)]
struct EndStream {
    error: Option<WireError>,
}

fn end_stream_error(data: &[u8]) -> Result<Option<Error>> {
    if data.is_empty() {
        return Ok(None);
    }
    let end: EndStream = serde_json::from_slice(data)
        .map_err(|error| Error::Protocol(format!("invalid end-of-stream message: {error}")))?;
    Ok(end.error.map(|error| rpc_error(error, "unknown")))
}

/// The error for a unary response with a non-2xx `status` and `body`.
pub(crate) fn error_from_response(status: u16, body: &[u8]) -> Error {
    let fallback = code_for_status(status);
    match serde_json::from_slice::<WireError>(body) {
        Ok(error) => rpc_error(error, fallback),
        Err(_) => Error::Rpc {
            code: fallback.to_owned(),
            message: format!("HTTP {status}: {}", String::from_utf8_lossy(body)),
        },
    }
}

fn rpc_error(error: WireError, fallback: &str) -> Error {
    Error::Rpc {
        code: error.code.unwrap_or_else(|| fallback.to_owned()),
        message: error.message.unwrap_or_default(),
    }
}

fn code_for_status(status: u16) -> &'static str {
    match status {
        400 => "internal",
        401 => "unauthenticated",
        403 => "permission_denied",
        404 => "unimplemented",
        429 | 502 | 503 | 504 => "unavailable",
        _ => "unknown",
    }
}

#[cfg(test)]
mod tests {
    use std::io::Cursor;

    use super::{Frame, envelope, error_from_response, read_frame};
    use crate::Error;

    #[test]
    fn envelopes_round_trip() {
        let mut stream = Cursor::new([envelope(b"one"), envelope(b"")].concat());
        assert!(
            matches!(read_frame(&mut stream).unwrap(), Some(Frame::Message(data)) if data == b"one")
        );
        assert!(
            matches!(read_frame(&mut stream).unwrap(), Some(Frame::Message(data)) if data.is_empty())
        );
        assert!(read_frame(&mut stream).unwrap().is_none());
    }

    #[test]
    fn end_of_stream_carries_an_optional_error() {
        let mut ok = Cursor::new(vec![2, 0, 0, 0, 2, b'{', b'}']);
        assert!(matches!(
            read_frame(&mut ok).unwrap(),
            Some(Frame::End(None))
        ));
        let body = br#"{"error":{"code":"unauthenticated","message":"expired"}}"#;
        let mut failed = Cursor::new([vec![2, 0, 0, 0, body.len() as u8], body.to_vec()].concat());
        match read_frame(&mut failed).unwrap() {
            Some(Frame::End(Some(Error::Rpc { code, message }))) => {
                assert_eq!(
                    (code.as_str(), message.as_str()),
                    ("unauthenticated", "expired")
                );
            }
            other => panic!("unexpected frame {other:?}"),
        }
    }

    #[test]
    fn truncated_and_compressed_frames_fail() {
        assert!(read_frame(&mut Cursor::new(vec![0, 0, 0, 0, 5, 1])).is_err());
        assert!(matches!(
            read_frame(&mut Cursor::new(vec![1, 0, 0, 0, 0])),
            Err(Error::Protocol(_))
        ));
    }

    #[test]
    fn unary_errors_use_the_body_or_the_status() {
        match error_from_response(
            400,
            br#"{"code":"failed_precondition","message":"protocol 2"}"#,
        ) {
            Error::Rpc { code, message } => {
                assert_eq!(
                    (code.as_str(), message.as_str()),
                    ("failed_precondition", "protocol 2")
                );
            }
            other => panic!("unexpected error {other:?}"),
        }
        match error_from_response(503, b"down") {
            Error::Rpc { code, .. } => assert_eq!(code, "unavailable"),
            other => panic!("unexpected error {other:?}"),
        }
    }
}
