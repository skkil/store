#![allow(dead_code)]

use std::{
    io::Cursor,
    sync::{Arc, Mutex},
    thread,
};

use tiny_http::{Header, Response, Server, StatusCode};

/// A request as the test server saw it.
#[derive(Debug, Clone)]
pub struct Recorded {
    pub path: String,
    pub headers: Vec<(String, String)>,
    pub body: Vec<u8>,
}

impl Recorded {
    pub fn header(&self, name: &str) -> Option<&str> {
        self.headers
            .iter()
            .find(|(key, _)| key.eq_ignore_ascii_case(name))
            .map(|(_, value)| value.as_str())
    }
}

/// What the test server answers.
pub struct Reply {
    pub status: u16,
    pub content_type: &'static str,
    pub body: Vec<u8>,
}

impl Reply {
    pub fn proto(body: Vec<u8>) -> Self {
        Self {
            status: 200,
            content_type: "application/proto",
            body,
        }
    }

    pub fn json(status: u16, body: &str) -> Self {
        Self {
            status,
            content_type: "application/json",
            body: body.as_bytes().to_vec(),
        }
    }

    pub fn stream(body: Vec<u8>) -> Self {
        Self {
            status: 200,
            content_type: "application/connect+proto",
            body,
        }
    }
}

/// A local HTTP server answering every request with `handler`.
pub struct TestServer {
    pub url: String,
    pub requests: Arc<Mutex<Vec<Recorded>>>,
}

impl TestServer {
    pub fn start(handler: impl Fn(&Recorded) -> Reply + Send + 'static) -> Self {
        let server = Server::http("127.0.0.1:0").expect("bind test server");
        let port = server.server_addr().to_ip().expect("ip address").port();
        let requests = Arc::new(Mutex::new(Vec::new()));
        let log = Arc::clone(&requests);
        thread::spawn(move || {
            for mut request in server.incoming_requests() {
                let mut body = Vec::new();
                request
                    .as_reader()
                    .read_to_end(&mut body)
                    .expect("read body");
                let recorded = Recorded {
                    path: request.url().to_owned(),
                    headers: request
                        .headers()
                        .iter()
                        .map(|header| (header.field.to_string(), header.value.to_string()))
                        .collect(),
                    body,
                };
                let reply = handler(&recorded);
                log.lock().unwrap().push(recorded);
                let content_type = Header::from_bytes("Content-Type", reply.content_type).unwrap();
                let response = Response::new(
                    StatusCode(reply.status),
                    vec![content_type],
                    Cursor::new(reply.body),
                    None,
                    None,
                );
                let _ = request.respond(response);
            }
        });
        Self {
            url: format!("http://127.0.0.1:{port}"),
            requests,
        }
    }

    pub fn recorded(&self) -> Vec<Recorded> {
        self.requests.lock().unwrap().clone()
    }
}
