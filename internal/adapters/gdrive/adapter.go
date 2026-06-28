package gdrive

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"driveSync/internal/domain"

	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

type GDriveAdapter struct {
	oauthManager *OAuthManager
	srv          *drive.Service
}

func NewGDriveAdapter(oauthManager *OAuthManager) *GDriveAdapter {
	return &GDriveAdapter{
		oauthManager: oauthManager,
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

// UploadFile uploads a local file to a Google Drive folder using resumable chunks
func (a *GDriveAdapter) UploadFile(ctx context.Context, localPath string, driveFolderID string, progressChan chan<- int64) (string, error) {
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

	fileName := filepath.Base(localPath)
	driveFile := &drive.File{
		Name:    fileName,
		Parents: []string{driveFolderID},
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

// ListRemoteFolder lists files located in a specific Google Drive folder
func (a *GDriveAdapter) ListRemoteFolder(ctx context.Context, driveFolderID string) ([]domain.RemoteFile, error) {
	if a.srv == nil {
		return nil, fmt.Errorf("google drive service not initialized")
	}

	query := fmt.Sprintf("'%s' in parents and trashed = false", driveFolderID)
	// Fetch ID, Name, Size, and ModifiedTime
	call := a.srv.Files.List().Q(query).Fields("files(id, name, size, modifiedTime)")
	call.Context(ctx)

	res, err := call.Do()
	if err != nil {
		return nil, fmt.Errorf("failed to list drive files: %w", err)
	}

	var files []domain.RemoteFile
	for _, f := range res.Files {
		mtime, err := time.Parse(time.RFC3339, f.ModifiedTime)
		if err != nil {
			// Fallback if parsing fails
			mtime = time.Now()
		}

		files = append(files, domain.RemoteFile{
			ID:    f.Id,
			Name:  f.Name,
			Size:  f.Size,
			MTime: mtime,
		})
	}

	return files, nil
}
