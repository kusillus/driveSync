package sync

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"driveSync/internal/domain"
)

// MockMetadataRepository implements domain.MetadataRepository
type MockMetadataRepository struct {
	db map[string]*domain.FileMetadata
}

func NewMockRepo() *MockMetadataRepository {
	return &MockMetadataRepository{db: make(map[string]*domain.FileMetadata)}
}

func (m *MockMetadataRepository) Save(ctx context.Context, meta *domain.FileMetadata) error {
	m.db[meta.Path] = meta
	return nil
}

func (m *MockMetadataRepository) FindByPath(ctx context.Context, path string) (*domain.FileMetadata, error) {
	meta, ok := m.db[path]
	if !ok {
		return nil, nil
	}
	return meta, nil
}

func (m *MockMetadataRepository) UpdateStatus(ctx context.Context, path string, status domain.FileStatus, driveID string) error {
	meta, ok := m.db[path]
	if !ok {
		return errors.New("not found")
	}
	meta.Status = status
	if driveID != "" {
		meta.DriveID = driveID
	}
	if status == domain.StatusSynced {
		meta.LastUploadedAt = time.Now()
		meta.LastSyncedAt = time.Now()
	}
	return nil
}

func (m *MockMetadataRepository) ListPending(ctx context.Context) ([]*domain.FileMetadata, error) {
	var list []*domain.FileMetadata
	for _, meta := range m.db {
		if meta.Status == domain.StatusPending {
			list = append(list, meta)
		}
	}
	return list, nil
}

func (m *MockMetadataRepository) ListAll(ctx context.Context) ([]*domain.FileMetadata, error) {
	var list []*domain.FileMetadata
	for _, meta := range m.db {
		list = append(list, meta)
	}
	return list, nil
}

// MockCloudStoragePort implements domain.CloudStoragePort
type MockCloudStoragePort struct {
	remoteFiles []domain.RemoteFile
}

func (m *MockCloudStoragePort) Authenticate(ctx context.Context) error {
	return nil
}

func (m *MockCloudStoragePort) UploadFile(ctx context.Context, localPath string, driveFolderID string, progressChan chan<- int64) (string, error) {
	if progressChan != nil {
		progressChan <- 100
	}
	return "drive-id-new", nil
}

func (m *MockCloudStoragePort) DownloadFile(ctx context.Context, driveID string, destPath string, progressChan chan<- int64) error {
	if progressChan != nil {
		progressChan <- 100
	}
	// Create dummy destination file
	_ = os.WriteFile(destPath, []byte("data"), 0644)
	return nil
}

func (m *MockCloudStoragePort) ListRemoteFolder(ctx context.Context, driveFolderID string) ([]domain.RemoteFile, error) {
	return m.remoteFiles, nil
}

func TestSyncCoordinator_ScanLocal(t *testing.T) {
	tempDir := t.TempDir()

	// Create test file
	filePath := filepath.Join(tempDir, "video1.mp4")
	if err := os.WriteFile(filePath, []byte("largevideocontent"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	repo := NewMockRepo()
	cloud := &MockCloudStoragePort{}
	coordinator := NewSyncCoordinator(repo, cloud)

	mappings := []domain.SyncFolderMapping{
		{LocalPath: tempDir, DriveFolderID: "folder-123"},
	}

	ctx := context.Background()

	// 1. First scan: should discover the file
	pending, err := coordinator.ScanLocal(ctx, mappings)
	if err != nil {
		t.Fatalf("ScanLocal failed: %v", err)
	}

	if len(pending) != 1 {
		t.Errorf("expected 1 pending file, got %d", len(pending))
	}

	meta, err := repo.FindByPath(ctx, filePath)
	if err != nil {
		t.Fatalf("FindByPath failed: %v", err)
	}
	if meta == nil {
		t.Fatal("expected file metadata to be saved in repo")
	}
	if meta.Status != domain.StatusPending {
		t.Errorf("expected status 'pending', got %s", meta.Status)
	}

	// 2. Second scan without changes: should not return new pending since it's already pending
	pending2, err := coordinator.ScanLocal(ctx, mappings)
	if err != nil {
		t.Fatalf("ScanLocal failed: %v", err)
	}
	if len(pending2) != 1 {
		t.Errorf("expected 1 pending file, got %d", len(pending2))
	}
}

func TestSyncCoordinator_Sync(t *testing.T) {
	tempDir := t.TempDir()

	// Create a local file that needs uploading
	filePath := filepath.Join(tempDir, "video1.mp4")
	if err := os.WriteFile(filePath, []byte("data"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	repo := NewMockRepo()
	// Add remote file that needs downloading
	cloud := &MockCloudStoragePort{
		remoteFiles: []domain.RemoteFile{
			{ID: "remote-id-2", Name: "remote_video.mp4", Size: 4, MTime: time.Now()},
		},
	}
	coordinator := NewSyncCoordinator(repo, cloud)

	mappings := []domain.SyncFolderMapping{
		{LocalPath: tempDir, DriveFolderID: "folder-123"},
	}

	progressChan := make(chan SyncProgress, 10)
	ctx := context.Background()

	// Run Sync in background
	go coordinator.Sync(ctx, mappings, progressChan)

	var startEvents, updateEvents, completeEvents int
	var errors []error

	for p := range progressChan {
		switch p.Type {
		case ProgressStart:
			startEvents++
		case ProgressUpdate:
			updateEvents++
		case ProgressComplete:
			completeEvents++
		case ProgressError:
			errors = append(errors, p.Error)
		}
	}

	if len(errors) > 0 {
		t.Fatalf("sync generated errors: %v", errors)
	}

	// We expect 2 sync actions (1 upload, 1 download)
	if startEvents != 2 {
		t.Errorf("expected 2 start events, got %d", startEvents)
	}
	if completeEvents != 2 {
		t.Errorf("expected 2 complete events, got %d", completeEvents)
	}

	// Verify local downloaded file exists
	downloadedPath := filepath.Join(tempDir, "remote_video.mp4")
	if _, err := os.Stat(downloadedPath); os.IsNotExist(err) {
		t.Error("expected downloaded file to exist locally")
	}

	// Verify DB status for both is synced
	metaUpload, _ := repo.FindByPath(ctx, filePath)
	if metaUpload.Status != domain.StatusSynced {
		t.Errorf("expected upload file to be synced, got %s", metaUpload.Status)
	}

	metaDownload, _ := repo.FindByPath(ctx, downloadedPath)
	if metaDownload.Status != domain.StatusSynced {
		t.Errorf("expected download file to be synced, got %s", metaDownload.Status)
	}
}
