## Why

Currently, driveSync uploads all files directly to the root of the specified Google Drive folder, ignoring local directory structures and flattening the hierarchy. Users need subfolders and nested files to sync recursively while maintaining the exact same directory hierarchy on Google Drive to keep files organized.

## What Changes

- Recursive local file scan that correctly tracks subdirectories relative to the mapped sync folder root.
- Parent-folder resolution on Google Drive: before uploading a nested file, any parent folders in the file's relative path will be looked up (and created if missing) on Google Drive under the mapped root folder.
- Hierarchy preservation during download: files listed inside nested remote folders will be downloaded to their corresponding relative path in the local directory structure.

## Capabilities

### New Capabilities
- `hierarchical-sync`: Sync files recursively to Google Drive while maintaining the same directory hierarchy as the local PC.

### Modified Capabilities

## Impact

- `internal/sync/engine.go`: Traverse directories recursively, resolve remote parent folder IDs, and recreate directory structure locally on download.
- `internal/adapters/gdrive/adapter.go`: Support folder creation, subfolder listing, and path-to-ID hierarchy resolution.
- `internal/domain/cloud.go`: Extend interfaces/types if needed to represent remote folders and creation methods.
