
### Metadata Provider Chain
- **Metadata Provider**: A uniform interface (e.g., `Provider`) encapsulating an external data source (MusicBrainz, Spotify, Netease, iTunes). It attempts to fetch and normalize album/track metadata based on a given search context.
- **Provider Chain**: A Chain of Responsibility pattern. The orchestrator calls `chain.Fetch()`, and the chain delegates the request down a prioritized sequence of providers until reliable metadata is found, completely shielding the main conversion loop from API-specific degradation logic.

### Album Batch Processing
- **ReleaseBatch**: A grouping of one or more physical directories (e.g., a single album directory, or a parent directory containing `CD1` and `CD2` subdirectories) that logically represent a single music release.
- **Directory Batch Processing**: The system processes a `ReleaseBatch` as a single unit, fetching metadata once for the entire release to guarantee consistency across all tracks and discs, preventing fragmentation (e.g., Compilation Disaster, Split Editions).
