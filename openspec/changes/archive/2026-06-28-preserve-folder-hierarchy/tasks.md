## 1. Domain and Interface Update

- [x] 1.1 Update the `UploadFile` signature in the `CloudStoragePort` interface in `internal/domain/cloud.go` to accept the relative path parameter.

## 2. Google Drive Adapter Implementation

- [x] 2.1 Update `UploadFile` in `internal/adapters/gdrive/adapter.go` to split the relative path, resolve/create parent subfolders recursively on Google Drive, and cache the folder IDs in-memory.
- [x] 2.2 Update `ListRemoteFolder` in `internal/adapters/gdrive/adapter.go` to perform a BFS recursive traversal on Google Drive using a queue, populating `RemoteFile.Name` with the relative path.

## 3. Sync Engine Integration

- [x] 3.1 Update `Sync` in `internal/sync/engine.go` to compute the relative path of files using `filepath.Rel` and pass it to `UploadFile`.

## 4. Verification and Testing

- [x] 4.1 Run existing tests (`go test -v ./...`) to verify there are no regressions.
- [x] 4.2 Add new tests or test runs to verify nested directories sync correctly to Google Drive and download back locally.
