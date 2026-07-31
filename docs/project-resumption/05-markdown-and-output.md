# Markdown and Output Finalization

## Goal
Ensure the final output (Markdown files and assets) is deterministic, correctly formatted, and ready for RAG ingestion.

## Current State
- `internal/mdconvert` handles the HTML to Markdown conversion.
- `internal/assets` handles downloading and rewriting asset links.
- `internal/normalize` handles frontmatter injection.
- `internal/storage` handles writing to disk.

## Tasks

1. **Verify Asset Deduplication**:
   Ensure that if multiple pages reference the same image, it is only downloaded once, and all pages reference the same local hashed filename. This is critical for storage efficiency.

2. **Verify Frontmatter**:
   Check that the frontmatter injected by `internal/normalize` contains all necessary metadata (source URL, fetch time, title, etc.) required for RAG chunking and attribution.

3. **Verify Determinism**:
   Run the crawler twice on the same static site. The output directory must be byte-for-byte identical (excluding timestamps in frontmatter, if any, though ideally even those should be controllable or ignored in diffs).

4. **Handle Missing Assets**:
   Ensure the pipeline gracefully handles 404s for images. The Markdown should probably retain the original remote link or indicate a broken local link rather than failing the entire page conversion.