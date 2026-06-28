package gdrive

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"driveSync/internal/domain"

	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

type GDriveAdapter struct {
	oauthManager *OAuthManager
	srv          *drive.Service
	folderCache  map[string]string
}

func NewGDriveAdapter(oauthManager *OAuthManager) *GDriveAdapter {
	return &GDriveAdapter{
		oauthManager: oauthManager,
		folderCache:  make(map[string]string),
	}
}

// Authenticate initializes the Google Drive API client using OAuth credentials
func (a *GDriveAdapter) Authenticate(ctx context.Context) error {
	client, err := a.oauthManager.GetClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to get authenticated client: %w", err)
	}

	srv, err := drive.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return fmt.Errorf("failed to initialize drive service: %w", err)
	}

	a.srv = srv
	return nil
}

// ProgressReader wraps an io.Reader to report read progress through a channel
type ProgressReader struct {
	r        io.Reader
	progress chan<- int64
	read     int64
}

func (pr *ProgressReader) Read(p []byte) (int, error) {
	n, err := pr.r.Read(p)
	if n > 0 {
		pr.read += int64(n)
		select {
		case pr.progress <- pr.read:
		default:
		}
	}
	return n, err
}

func (a *GDriveAdapter) findOrCreateFolder(ctx context.Context, parentID string, name string) (string, error) {
	query := fmt.Sprintf("'%s' in parents and name = '%s' and mimeType = 'application/vnd.google-apps.folder' and trashed = false", parentID, strings.ReplaceAll(name, "'", "\\'"))
	call := a.srv.Files.List().Q(query).Fields("files(id)")
	call.Context(ctx)
	res, err := call.Do()
	if err != nil {
		return "", fmt.Errorf("failed to search for folder %s: %w", name, err)
	}

	if len(res.Files) > 0 {
		return res.Files[0].Id, nil
	}

	driveFile := &drive.File{
		Name:     name,
		MimeType: "application/vnd.google-apps.folder",
		Parents:  []string{parentID},
	}
	createCall := a.srv.Files.Create(driveFile).Fields("id")
	createCall.Context(ctx)
	newFolder, err := createCall.Do()
	if err != nil {
		return "", fmt.Errorf("failed to create folder %s: %w", name, err)
	}

	return newFolder.Id, nil
}

func (a *GDriveAdapter) resolveParentFolderID(ctx context.Context, rootFolderID string, relPath string) (string, error) {
	dir := filepath.Dir(relPath)
	if dir == "." || dir == "" || dir == "/" {
		return rootFolderID, nil
	}

	dir = filepath.ToSlash(dir)
	parts := strings.Split(dir, "/")

	currentParentID := rootFolderID
	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}

		cacheKey := currentParentID + "/" + part
		if cachedID, ok := a.folderCache[cacheKey]; ok {
			currentParentID = cachedID
			continue
		}

		folderID, err := a.findOrCreateFolder(ctx, currentParentID, part)
		if err != nil {
			return "", err
		}

		a.folderCache[cacheKey] = folderID
		currentParentID = folderID
	}

	return currentParentID, nil
}

// UploadFile uploads a local file to a Google Drive folder using resumable chunks
func (a *GDriveAdapter) UploadFile(ctx context.Context, localPath string, relativePath string, driveFolderID string, progressChan chan<- int64) (string, error) {
	if a.srv == nil {
		return "", fmt.Errorf("google drive service not initialized")
	}

	file, err := os.Open(localPath)
	if err != nil {
		return "", fmt.Errorf("failed to open file %s: %w", localPath, err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("failed to stat file: %w", err)
	}

	parentID, err := a.resolveParentFolderID(ctx, driveFolderID, relativePath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve parent folder ID: %w", err)
	}

	fileName := filepath.Base(localPath)
	driveFile := &drive.File{
		Name:    fileName,
		Parents: []string{parentID},
	}

	progressReader := &ProgressReader{
		r:        file,
		progress: progressChan,
	}

	// Use Google Drive SDK resumable upload with a 5MB chunk size (suitable for large videos)
	call := a.srv.Files.Create(driveFile).Media(progressReader, googleapi.ChunkSize(5*1024*1024))
	call.Context(ctx)

	res, err := call.Do()
	if err != nil {
		return "", fmt.Errorf("failed to upload file: %w", err)
	}

	// Final progress signal
	if progressChan != nil {
		select {
		case progressChan <- fileInfo.Size():
		default:
		}
	}

	return res.Id, nil
}

// DownloadFile downloads a file from Google Drive to a local destination
func (a *GDriveAdapter) DownloadFile(ctx context.Context, driveID string, destPath string, progressChan chan<- int64) error {
	if a.srv == nil {
		return fmt.Errorf("google drive service not initialized")
	}

	res, err := a.srv.Files.Get(driveID).Download()
	if err != nil {
		return fmt.Errorf("failed to fetch download reader: %w", err)
	}
	defer res.Body.Close()

	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("failed to create destination dir: %w", err)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create local file: %w", err)
	}
	defer out.Close()

	progressReader := &ProgressReader{
		r:        res.Body,
		progress: progressChan,
	}

	_, err = io.Copy(out, progressReader)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// ListRemoteFolder lists files located in a specific Google Drive folder and its subfolders recursively
func (a *GDriveAdapter) ListRemoteFolder(ctx context.Context, driveFolderID string) ([]domain.RemoteFile, error) {
	if a.srv == nil {
		return nil, fmt.Errorf("google drive service not initialized")
	}

	type queueItem struct {
		id      string
		relPath string
	}

	queue := []queueItem{{id: driveFolderID, relPath: ""}}
	var files []domain.RemoteFile

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		query := fmt.Sprintf("'%s' in parents and trashed = false", curr.id)
		call := a.srv.Files.List().Q(query).Fields("files(id, name, size, modifiedTime, mimeType)")
		call.Context(ctx)

		res, err := call.Do()
		if err != nil {
			return nil, fmt.Errorf("failed to list drive files in %s (rel: %s): %w", curr.id, curr.relPath, err)
		}

		for _, f := range res.Files {
			var itemRelPath string
			if curr.relPath == "" {
				itemRelPath = f.Name
			} else {
				itemRelPath = filepath.Join(curr.relPath, f.Name)
			}

			if f.MimeType == "application/vnd.google-apps.folder" {
				queue = append(queue, queueItem{id: f.Id, relPath: itemRelPath})
			} else {
				mtime, err := time.Parse(time.RFC3339, f.ModifiedTime)
				if err != nil {
					mtime = time.Now()
				}

				files = append(files, domain.RemoteFile{
					ID:    f.Id,
					Name:  itemRelPath,
					Size:  f.Size,
					MTime: mtime,
				})
			}
		}
	}

	return files, nil
}
