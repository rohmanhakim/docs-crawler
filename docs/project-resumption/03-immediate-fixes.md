# Immediate Fixes

## Goal
Get the project building and the test suite completely green.

## The Problem
Running `go test ./...` or `go build ./cmd/crawler` fails with:
```
# github.com/rohmanhakim/docs-crawler/internal/pipeline
internal/pipeline/pipeline.go:37:10: undefined: fetchUser
internal/pipeline/pipeline.go:45:10: undefined: process
```

## Analysis
The `internal/pipeline/pipeline.go` file contains placeholder code (`MyWorkflow`, `fetchUser`, `process`) that was likely sketched out during a design phase but never completed or removed. It does not integrate with the actual crawler components.

## Action Plan
1. **Remove Placeholder Code**: Delete the `MyData`, `ProcessedData`, `Event`, `MyParams`, and `MyWorkflow` structs and methods from `internal/pipeline/pipeline.go`.
2. **Keep Interfaces**: Retain the `Pipeline` interface as it defines the intended contract for the pipeline.
3. **Verify Build**: Run `go build ./cmd/crawler` to ensure the project compiles.
4. **Verify Tests**: Run `go test ./...` to ensure all existing tests pass.

Once the build is green, we can proceed to design the actual pipeline implementation.