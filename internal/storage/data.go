package storage

// Persistence

type WriteResult struct {
	artifact Artifact
}

type Artifact struct {
	path string
}
