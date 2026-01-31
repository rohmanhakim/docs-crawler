package storage

import (
	"errors"
	"time"

	"github.com/rohmanhakim/docs-crawler/internal"
	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/internal/normalize"
)

/*
Responsibilities
- Persist Markdown files
- Write assets
- Ensure deterministic filenames

Output Characteristics
- Stable directory layout
- Idempotent writes
- Overwrite-safe reruns
*/

type Sink struct {
	metadataSink metadata.MetadataSink
}

func NewSink(
	metadataSink metadata.MetadataSink,
) Sink {
	return Sink{
		metadataSink: metadataSink,
	}
}

func (s *Sink) Write(
	normalizedDoc normalize.NormalizedMarkdownDoc,
) (WriteResult, internal.ClassifiedError) {
	writeResult, err := write()
	if err != nil {
		var storageError *StorageError
		errors.As(err, &storageError)
		s.metadataSink.RecordError(
			time.Now(),
			"storage",
			"Sink.Write",
			mapStorageErrorToMetadataCause(storageError),
			err.Error(),
			[]metadata.Attribute{
				metadata.NewAttr(metadata.AttrWritePath, "path/to/write"),
			},
		)
		return WriteResult{}, storageError
	}
	s.metadataSink.RecordArtifact(writeResult.artifact.path)
	return writeResult, nil
}

func write() (WriteResult, *StorageError) {
	return WriteResult{}, nil
}
