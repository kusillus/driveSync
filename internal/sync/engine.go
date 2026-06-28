package sync

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"driveSync/internal/domain"
)

type ProgressType string

const (
	ProgressStart    ProgressType = "start"
	ProgressUpdate   ProgressType = "update"
	ProgressComplete ProgressType = "complete"
	ProgressError    ProgressType = "error"
)

type SyncProgress struct {
	Type        ProgressType
	Path        string
	Direction   string // "upload" or "download"
	BytesSynced int64
	TotalBytes  int64
	Error       error
}

type SyncCoordinator struct {
	repo  domain.MetadataRepository
	cloud domain.CloudStoragePort
}

func NewSyncCoordinator(repo domain.MetadataRepository, cloud domain.CloudStoragePort) *SyncCoordinator {
	return &SyncCoordinator{
		repo:  repo,
		cloud: cloud,
	}
}

// ScanLocal scans the configured local directories, detects new or modified files, and updates SQLite
func (sc *SyncCoordinator) ScanLocal(ctx context.Context, mappings []domain.SyncFolderMapping) ([]*domain.FileMetadata, error) {
	var allPending []*domain.FileMetadata

	for _, mapping := range mappings {
		err := filepath.WalkDir(mapping.LocalPath, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				// Skip hidden directories (like .git, .agent)
				if strings.HasPrefix(d.Name(), ".") && d.Name() != "." {
					return filepath.SkipDir
				}
				return nil
			}

			// Skip hidden files or configuration files we ignore
			if strings.HasPrefix(d.Name(), ".") || strings.HasSuffix(d.Name(), ".db") {
				return nil
			}

			info, err := d.Info()
			if err != nil {
				return err
			}

			dbMeta, err := sc.repo.FindByPath(ctx, path)
			if err != nil {
				return err
			}

			mtimeTrunc := info.ModTime().Truncate(time.Second)

			if dbMeta == nil {
				// New file discovered
				dbMeta = &domain.FileMetadata{
					Path:    path,
					Size:    info.Size(),
					MTime:   mtimeTrunc,
					Status:  domain.StatusPending,
					DriveID: "",
				}
				if err := sc.repo.Save(ctx, dbMeta); err != nil {
					return err
				}
				allPending = append(allPending, dbMeta)
			} else {
				// Existing file: check if size or modtime changed
				dbMTimeTrunc := dbMeta.MTime.Truncate(time.Second)
				if dbMeta.Size != info.Size() || !dbMTimeTrunc.Equal(mtimeTrunc) {
					dbMeta.Size = info.Size()
					dbMeta.MTime = mtimeTrunc
					dbMeta.Status = domain.StatusPending
					if err := sc.repo.Save(ctx, dbMeta); err != nil {
						return err
					}
					allPending = append(allPending, dbMeta)
				} else if dbMeta.Status == domain.StatusPending {
					allPending = append(allPending, dbMeta)
				}
			}

			return nil
		})

		if err != nil {
			return nil, fmt.Errorf("failed to scan local folder %s: %w", mapping.LocalPath, err)
		}
	}

	return allPending, nil
}

// Sync performs bi-directional sync: uploads pending local files and downloads new remote files
func (sc *SyncCoordinator) Sync(ctx context.Context, mappings []domain.SyncFolderMapping, progressChan chan<- SyncProgress) {
	defer close(progressChan)

	// Step 1: Scan local folders to populate/update DB
	_, err := sc.ScanLocal(ctx, mappings)
	if err != nil {
		progressChan <- SyncProgress{Type: ProgressError, Error: fmt.Errorf("scan failed: %w", err)}
		return
	}

	for _, mapping := range mappings {
		// Verify local folder exists
		if _, err := os.Stat(mapping.LocalPath); os.IsNotExist(err) {
			_ = os.MkdirAll(mapping.LocalPath, 0755)
		}

		// Step 2: Fetch remote files from Google Drive
		remoteFiles, err := sc.cloud.ListRemoteFolder(ctx, mapping.DriveFolderID)
		if err != nil {
			progressChan <- SyncProgress{Type: ProgressError, Error: fmt.Errorf("failed listing remote folder: %w", err)}
			return
		}

		// Step 3: Compare and identify downloads
		remoteFileMap := make(map[string]domain.RemoteFile)
		for _, rf := range remoteFiles {
			remoteFileMap[rf.Name] = rf
		}

		// List of files that need downloading
		var pendingDownloads []domain.RemoteFile
		for _, rf := range remoteFiles {
			localDestPath := filepath.Join(mapping.LocalPath, rf.Name)
			if _, err := os.Stat(localDestPath); os.IsNotExist(err) {
				pendingDownloads = append(pendingDownloads, rf)
			}
		}

		// Step 4: Execute uploads for files in this mapping
		pendingUploads, err := sc.repo.ListPending(ctx)
		if err != nil {
			progressChan <- SyncProgress{Type: ProgressError, Error: fmt.Errorf("failed listing pending uploads: %w", err)}
			return
		}

		for _, up := range pendingUploads {
			// Check if file belongs to this mapping's folder
			if !strings.HasPrefix(up.Path, mapping.LocalPath) {
				continue
			}

			fileChan := make(chan int64)
			doneChan := make(chan bool)

			// Progress monitor goroutine
			go func(path string, totalSize int64) {
				for {
					select {
					case bytes, ok := <-fileChan:
						if !ok {
							return
						}
						progressChan <- SyncProgress{
							Type:        ProgressUpdate,
							Path:        path,
							Direction:   "upload",
							BytesSynced: bytes,
							TotalBytes:  totalSize,
						}
					case <-doneChan:
						return
					}
				}
			}(up.Path, up.Size)

			progressChan <- SyncProgress{
				Type:        ProgressStart,
				Path:        up.Path,
				Direction:   "upload",
				TotalBytes:  up.Size,
				BytesSynced: 0,
			}

			_ = sc.repo.UpdateStatus(ctx, up.Path, domain.StatusUploading, "")

			relPath, err := filepath.Rel(mapping.LocalPath, up.Path)
			if err != nil {
				relPath = filepath.Base(up.Path)
			}

			driveID, uploadErr := sc.cloud.UploadFile(ctx, up.Path, relPath, mapping.DriveFolderID, fileChan)
			close(fileChan)
			close(doneChan)

			if uploadErr != nil {
				_ = sc.repo.UpdateStatus(ctx, up.Path, domain.StatusFailed, "")
				progressChan <- SyncProgress{
					Type:      ProgressError,
					Path:      up.Path,
					Direction: "upload",
					Error:     uploadErr,
				}
				continue
			}

			_ = sc.repo.UpdateStatus(ctx, up.Path, domain.StatusSynced, driveID)
			progressChan <- SyncProgress{
				Type:        ProgressComplete,
				Path:        up.Path,
				Direction:   "upload",
				TotalBytes:  up.Size,
				BytesSynced: up.Size,
			}
		}

		// Step 5: Execute downloads for files in this mapping
		for _, dl := range pendingDownloads {
			localDestPath := filepath.Join(mapping.LocalPath, dl.Name)

			fileChan := make(chan int64)
			doneChan := make(chan bool)

			// Progress monitor goroutine
			go func(path string, totalSize int64) {
				for {
					select {
					case bytes, ok := <-fileChan:
						if !ok {
							return
						}
						progressChan <- SyncProgress{
							Type:        ProgressUpdate,
							Path:        path,
							Direction:   "download",
							BytesSynced: bytes,
							TotalBytes:  totalSize,
						}
					case <-doneChan:
						return
					}
				}
			}(localDestPath, dl.Size)

			progressChan <- SyncProgress{
				Type:        ProgressStart,
				Path:        localDestPath,
				Direction:   "download",
				TotalBytes:  dl.Size,
				BytesSynced: 0,
			}

			downloadErr := sc.cloud.DownloadFile(ctx, dl.ID, localDestPath, fileChan)
			close(fileChan)
			close(doneChan)

			if downloadErr != nil {
				progressChan <- SyncProgress{
					Type:      ProgressError,
					Path:      localDestPath,
					Direction: "download",
					Error:     downloadErr,
				}
				continue
			}

			// Add download file to SQLite
			meta := &domain.FileMetadata{
				Path:           localDestPath,
				Size:           dl.Size,
				MTime:          dl.MTime.Truncate(time.Second),
				DriveID:        dl.ID,
				Status:         domain.StatusSynced,
				LastUploadedAt: time.Now().Truncate(time.Second),
				LastSyncedAt:   time.Now().Truncate(time.Second),
			}
			_ = sc.repo.Save(ctx, meta)

			progressChan <- SyncProgress{
				Type:        ProgressComplete,
				Path:        localDestPath,
				Direction:   "download",
				TotalBytes:  dl.Size,
				BytesSynced: dl.Size,
			}
		}
	}
}
