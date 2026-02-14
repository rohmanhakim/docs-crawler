package storage

// Persistence

type WriteResult struct {
	urlHash     string // identity (filename without extension)
	path        string
	contentHash string
}

func NewWriteResult(
	urlHash string,
	path string,
	contentHash string,
) WriteResult {
	return WriteResult{
		urlHash:     urlHash,
		path:        path,
		contentHash: contentHash,
	}
}
func (w *WriteResult) URLHash() string {
	return w.urlHash
}

func (w *WriteResult) Path() string {
	return w.path
}

func (w *WriteResult) ContentHash() string {
	return w.contentHash
}
