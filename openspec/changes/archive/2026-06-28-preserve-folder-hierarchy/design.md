## Context

Currently, driveSync ignores the local directory hierarchy and flattens all files under the root Google Drive folder during uploads and downloads. To support folder synchronization, we need to handle subfolder creation and listing recursively on Google Drive.

## Goals / Non-Goals

**Goals:**
- Maintain the local directory structure on Google Drive.
- Recreate the Google Drive directory structure locally on download.
- Avoid redundant Google Drive API calls by caching resolved folder IDs in-memory.

**Non-Goals:**
- Bi-directional directory deletion (deleting empty folders locally or on Drive is out of scope).

## Decisions

### 1. Relative Path Representation in Sync Engine
We will change `RemoteFile.Name` to represent the relative path from the mapped root folder (e.g. `subfolder/file.txt`).
- **Rationale**: This allows the sync engine's comparison logic (`engine.go`) to remain simple and unmodified, as it naturally maps files to their local paths using `filepath.Join(mapping.LocalPath, rf.Name)`.

### 2. CloudStoragePort Interface Update
Update the `UploadFile` signature in `CloudStoragePort` to accept the relative path of the file:
```go
UploadFile(ctx context.Context, localPath string, relativePath string, driveFolderID string, progressChan chan<- int64) (string, error)
```
- **Rationale**: The adapter needs the relative path of the file to determine which remote subfolders need to be resolved or created.

### 3. Folder Resolution Cache in Adapter
We will implement an in-memory cache (`map[string]string`) in `GDriveAdapter` mapping a subfolder's relative path to its Google Drive ID.
- **Rationale**: Avoids making redundant API calls to search for or create the same parent directories for every file in the same subfolder.

### 4. Recursive Remote Folder Listing
Implement a breadth-first search (BFS) queue in `ListRemoteFolder` to traverse folders recursively on Google Drive.
- **Rationale**: Since Google Drive is an ID-based flat object storage, recursion must be handled by listing files/folders for each parent ID and traversing subfolders.

## Risks / Trade-offs

- **[Risk]** Deep directory trees could cause many API requests during folder creation/listing.
- **[Mitigation]** The in-memory folder cache minimizes folder creation requests. The BFS queue processes folders efficiently.
