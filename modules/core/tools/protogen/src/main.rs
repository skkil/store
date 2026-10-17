//! Writes the prost code for `skkil.store.v1` into `crates/store-protocol/src/generated`.

use std::{error::Error, path::PathBuf};

fn main() -> Result<(), Box<dyn Error>> {
    let core = PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("../..");
    let proto_root = core.join("../protocol/proto");
    let out_dir = core.join("crates/store-protocol/src/generated");
    std::fs::create_dir_all(&out_dir)?;
    prost_build::Config::new()
        .out_dir(&out_dir)
        .compile_protos(
            &[proto_root.join("skkil/store/v1/store.proto")],
            &[proto_root],
        )?;
    Ok(())
}
