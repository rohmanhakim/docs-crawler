# End-to-End and CLI Finalization

## Goal
Make the crawler usable via the command line and verify it works end-to-end against a real (or mock) documentation site.

## Current State
- `cmd/crawler/main.go` exists but might not be fully wired up to the new configuration or scheduler logic.
- `internal/cli` contains some CLI logic (e.g., `root.go`, `eventprinter.go`).

## Tasks

1. **CLI Wiring**:
   Ensure `cmd/crawler/main.go` correctly parses flags/config files using `internal/config`, initializes the `Scheduler`, and starts the crawl.

2. **Event Printing**:
   Verify that `internal/cli/eventprinter.go` correctly consumes events from the `MetadataSink` and prints them to the console in a readable format (especially important for dry runs).

3. **End-to-End Test**:
   Create a simple local HTTP server serving static HTML files (or use an existing test fixture). Run the compiled CLI against this server and verify:
   - It respects `robots.txt`.
   - It follows links up to the configured depth.
   - It extracts the main content correctly.
   - It outputs valid Markdown files in the specified output directory.

4. **Documentation**:
   Update the main `README.md` with instructions on how to build, configure, and run the crawler.