/// Where the two halves of the protocol live.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Endpoints {
    /// Base URL of the product server that implements `PushService`, such
    /// as `https://api.example.com`.
    pub push_base_url: String,
    /// Base URL of the store service that implements `StoreService`.
    pub store_base_url: String,
}

impl Endpoints {
    /// Endpoints from the two base URLs.
    pub fn new(push_base_url: impl Into<String>, store_base_url: impl Into<String>) -> Self {
        Self {
            push_base_url: push_base_url.into(),
            store_base_url: store_base_url.into(),
        }
    }

    pub(crate) fn push(&self) -> String {
        join(&self.push_base_url, "skkil.store.v1.PushService/Push")
    }

    pub(crate) fn pull(&self) -> String {
        join(&self.store_base_url, "skkil.store.v1.StoreService/Pull")
    }

    pub(crate) fn pokes(&self) -> String {
        join(&self.store_base_url, "skkil.store.v1.StoreService/Pokes")
    }
}

fn join(base: &str, path: &str) -> String {
    format!("{}/{path}", base.trim_end_matches('/'))
}

#[cfg(test)]
mod tests {
    use super::Endpoints;

    #[test]
    fn joins_procedure_paths() {
        let endpoints = Endpoints::new("https://api.example/", "https://store.example/sync");
        assert_eq!(
            endpoints.push(),
            "https://api.example/skkil.store.v1.PushService/Push"
        );
        assert_eq!(
            endpoints.pull(),
            "https://store.example/sync/skkil.store.v1.StoreService/Pull"
        );
        assert_eq!(
            endpoints.pokes(),
            "https://store.example/sync/skkil.store.v1.StoreService/Pokes"
        );
    }
}
