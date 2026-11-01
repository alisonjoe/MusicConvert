pub mod chain;
pub mod error;
pub mod itunes;
pub mod localization;
pub mod musicbrainz;
pub mod netease;
pub mod query;
pub mod trait_provider;

pub use chain::ProviderChain;
pub use error::ProviderError;
pub use itunes::ITunesProvider;
pub use localization::localize_metadata;
pub use musicbrainz::MusicBrainzProvider;
pub use netease::NeteaseProvider;
pub use query::MetadataQuery;
pub use trait_provider::MetadataProvider;
