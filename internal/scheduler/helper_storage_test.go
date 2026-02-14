package scheduler_test

import (
	"testing"

	"github.com/rohmanhakim/docs-crawler/internal/normalize"
	"github.com/rohmanhakim/docs-crawler/internal/storage"
	"github.com/rohmanhakim/docs-crawler/pkg/failure"
	"github.com/rohmanhakim/docs-crawler/pkg/hashutil"
	"github.com/stretchr/testify/mock"
)

type storageMock struct {
	mock.Mock
}

func (s *storageMock) Write(
	outputDir string,
	normalizedDoc normalize.NormalizedMarkdownDoc,
	hashAlgo hashutil.HashAlgo,
) (storage.WriteResult, failure.ClassifiedError) {
	args := s.Called(outputDir, normalizedDoc, hashAlgo)
	res := args.Get(0).(storage.WriteResult)
	var err failure.ClassifiedError
	if args.Get(1) != nil {
		err = args.Get(1).(failure.ClassifiedError)
	}
	return res, err
}

func newStorageMockForTest(t *testing.T) *storageMock {
	t.Helper()
	m := new(storageMock)
	return m
}
