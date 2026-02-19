package scheduler_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/stretchr/testify/require"
)

// validHTMLForEventStream is a minimal valid HTML document that passes
// the extractor's Layer 1/Layer 2 heuristics.
const validHTMLForEventStream = `<!DOCTYPE html>
<html>
<head><title>Event Stream Integration Test</title></head>
<body>
<main>
<h1>Integration Test Page</h1>
<p>This page verifies that all pipeline stages emit metadata events correctly.</p>
<p>Additional content to ensure the extraction heuristics pass.</p>
</main>
</body>
</html>`

// setupEventStreamServer creates a test HTTP server that serves:
// - GET /robots.txt → allows all crawling
// - GET /docs/guide → returns a valid HTML page with meaningful path for section derivation
// - GET / → redirects to /docs/guide
func setupEventStreamServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("User-agent: *\nAllow: /\n"))
		case "/docs/guide":
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(validHTMLForEventStream))
		case "/":
			// Redirect root to /docs/guide for proper section derivation
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(validHTMLForEventStream))
		default:
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(validHTMLForEventStream))
		}
	}))
}

// writeEventStreamConfig writes a config file for the event stream test.
// The config uses maxDepth: 0 to crawl only the seed URL.
func writeEventStreamConfig(t *testing.T, serverURL string, outputDir string) string {
	t.Helper()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	parsedURL, err := url.Parse(serverURL)
	require.NoError(t, err, "failed to parse server URL")

	configData := fmt.Sprintf(`{
		"seedUrls": [{"Scheme": "%s", "Host": "%s", "Path": "/docs/guide"}],
		"outputDir": "%s",
		"maxDepth": 0
	}`, parsedURL.Scheme, parsedURL.Host, outputDir)

	err = os.WriteFile(configPath, []byte(configData), 0644)
	require.NoError(t, err, "failed to write config file")

	return configPath
}

// collectEventKinds groups events by their EventKind for easier assertions.
func collectEventKinds(events []metadata.Event) map[metadata.EventKind][]metadata.Event {
	result := make(map[metadata.EventKind][]metadata.Event)
	for _, e := range events {
		result[e.Kind()] = append(result[e.Kind()], e)
	}
	return result
}

// collectPipelineStages filters EventKindPipeline events and groups by PipelineStage.
func collectPipelineStages(events []metadata.Event) map[metadata.PipelineStage][]metadata.PipelineEvent {
	result := make(map[metadata.PipelineStage][]metadata.PipelineEvent)
	for _, e := range events {
		if e.Kind() == metadata.EventKindPipeline && e.Pipeline() != nil {
			result[e.Pipeline().Stage()] = append(result[e.Pipeline().Stage()], *e.Pipeline())
		}
	}
	return result
}

// collectFetchKinds filters EventKindFetch events and groups by FetchKind.
func collectFetchKinds(events []metadata.Event) map[metadata.FetchKind][]metadata.FetchEvent {
	result := make(map[metadata.FetchKind][]metadata.FetchEvent)
	for _, e := range events {
		if e.Kind() == metadata.EventKindFetch && e.Fetch() != nil {
			result[e.Fetch().Kind()] = append(result[e.Fetch().Kind()], *e.Fetch())
		}
	}
	return result
}
