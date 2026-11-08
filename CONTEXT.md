
### Metadata Provider Chain
- **Metadata Provider**: A uniform interface (e.g., `Provider`) encapsulating an external data source (MusicBrainz, Spotify, Netease, iTunes). It attempts to fetch and normalize album/track metadata based on a given search context.
- **Provider Chain**: A Chain of Responsibility pattern. The orchestrator calls `chain.Fetch()`, and the chain delegates the request down a prioritized sequence of providers until reliable metadata is found, completely shielding the main conversion loop from API-specific degradation logic.

### Album Batch Processing
- **ReleaseBatch**: A grouping of one or more physical directories (e.g., a single album directory, or a parent directory containing `CD1` and `CD2` subdirectories) that logically represent a single music release.
- **Directory Batch Processing**: The system processes a `ReleaseBatch` as a single unit, fetching metadata once for the entire release to guarantee consistency across all tracks and discs, preventing fragmentation (e.g., Compilation Disaster, Split Editions).

### Task Observability & Diagnostic Logs
- **TaskLogItem**: A structured event entry associated with a specific `BatchTask`, containing `timestamp`, `level` (INFO, WARN, ERROR, DEBUG), `stage` (e.g. `LocalProbe`, `MusicBrainz`, `NetEase`, `ITunes`, `Resolve`, `Transcode`, `Tagging`), and an informative `message`.
- **TaskLogAppended**: A reactive event emitted through the `EventBus` and streamed to the UI via SSE `/api/events`, enabling real-time terminal-like observability for scraping and transcoding lifecycles.
- **Scraping Observability**: Visualizing external metadata chain decisions, confidence score evaluations, matched candidates, and fallback rationales on the Web Console.
