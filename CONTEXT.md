
### Metadata Provider Chain
- **Metadata Provider**: A uniform interface (e.g., `Provider`) encapsulating an external data source (MusicBrainz, Spotify, Netease, iTunes). It attempts to fetch and normalize album/track metadata based on a given search context.
- **Provider Chain**: A Chain of Responsibility pattern. The orchestrator calls `chain.Fetch()`, and the chain delegates the request down a prioritized sequence of providers until reliable metadata is found, completely shielding the main conversion loop from API-specific degradation logic.
