package metadata

/*
Metadata Collected
- Fetch timestamps
- HTTP status codes
- Content hashes
- Crawl depth

Logging Goals
- Debuggable crawl behavior
- Post-run auditability
- Failure diagnostics

Structured logging is preferred.

Allowed:
- Primitive values
- Timestamps
- URLs (as values, not objects with behavior)
- Hashes
- Status codes
- Durations
- Identifiers (page ID, crawl ID)
*/

type Recorder struct {
}
