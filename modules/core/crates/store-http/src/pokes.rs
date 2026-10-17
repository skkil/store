use prost::Message;
use store_core::protocol::v1::Poke;
use ureq::BodyReader;

use crate::{
    Result,
    connect::{Frame, read_frame},
};

/// The open `Pokes` stream: yields each poke as it arrives and ends when the
/// server closes the stream. Blocking; run it on its own thread.
pub struct PokeStream {
    body: BodyReader<'static>,
    done: bool,
}

impl PokeStream {
    pub(crate) fn new(body: BodyReader<'static>) -> Self {
        Self { body, done: false }
    }
}

impl Iterator for PokeStream {
    type Item = Result<Poke>;

    fn next(&mut self) -> Option<Self::Item> {
        if self.done {
            return None;
        }
        let frame = read_frame(&mut self.body);
        match frame {
            Ok(Some(Frame::Message(data))) => {
                Some(Poke::decode(data.as_slice()).map_err(Into::into))
            }
            Ok(Some(Frame::End(error))) => {
                self.done = true;
                error.map(Err)
            }
            Ok(None) => {
                self.done = true;
                None
            }
            Err(error) => {
                self.done = true;
                Some(Err(error))
            }
        }
    }
}
