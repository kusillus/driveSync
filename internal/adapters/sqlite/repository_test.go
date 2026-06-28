package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"driveSync/internal/domain"
)

func TestSqliteMetadataRepository(t *testing.T) {
	// Use t.TempDir() to create a real SQLite file in a temporary folder
	dbPath := filepath.Join(t.TempDir(), "test_metadata.db")

	repo, err := NewSqliteRepository(dbPath)
	if err != nil {
		t.Fatalf("failed to initialize repository: %v", err)
	}
	defer repo.Close()

	ctx := context.Background()

	t.Run("Save and FindByPath", func(t *testing.T) {
		meta := &domain.FileMetadata{
			Path:    "/videos/gopro1.mp4",
			Size:    1024,
			MTime:   time.Now().Truncate(time.Second),
			DriveID: "",
			Status:  domain.StatusPending,
		}

		err := repo.Save(ctx, meta)
		if err != nil {
			t.Fatalf("failed to save metadata: %v", err)
		}

		retrieved, err := repo.FindByPath(ctx, "/videos/gopro1.mp4")
		if err != nil {
			t.Fatalf("failed to find metadata: %v", err)
		}

		if retrieved == nil {
			t.Fatal("expected retrieved metadata to be non-nil")
		}

		if retrieved.Path != meta.Path {
			t.Errorf("expected path %s, got %s", meta.Path, retrieved.Path)
		}
		if retrieved.Size != meta.Size {
			t.Errorf("expected size %d, got %d", meta.Size, retrieved.Size)
		}
		// Compare times by converting to UTC and ignoring sub-second differences
		if !retrieved.MTime.Equal(meta.MTime) {
			t.Errorf("expected mtime %v, got %v", meta.MTime, retrieved.MTime)
		}
		if retrieved.Status != meta.Status {
			t.Errorf("expected status %s, got %s", meta.Status, retrieved.Status)
		}
	})

	t.Run("UpdateStatus", func(t *testing.T) {
		path := "/videos/gopro1.mp4"
		driveID := "drive-12345"

		err := repo.UpdateStatus(ctx, path, domain.StatusSynced, driveID)
		if err != nil {
			t.Fatalf("failed to update status: %v", err)
		}

		retrieved, err := repo.FindByPath(ctx, path)
		if err != nil {
			t.Fatalf("failed to find metadata: %v", err)
		}

		if retrieved.Status != domain.StatusSynced {
			t.Errorf("expected status %s, got %s", domain.StatusSynced, retrieved.Status)
		}
		if retrieved.DriveID != driveID {
			t.Errorf("expected driveID %s, got %s", driveID, retrieved.DriveID)
		}
		if retrieved.LastUploadedAt.IsZero() {
			t.Error("expected LastUploadedAt to be set")
		}
		if retrieved.LastSyncedAt.IsZero() {
			t.Error("expected LastSyncedAt to be set")
		}
	})

	t.Run("ListPending and ListAll", func(t *testing.T) {
		meta2 := &domain.FileMetadata{
			Path:    "/videos/gopro2.mp4",
			Size:    2048,
			MTime:   time.Now().Truncate(time.Second),
			DriveID: "",
			Status:  domain.StatusPending,
		}
		if err := repo.Save(ctx, meta2); err != nil {
			t.Fatalf("failed to save meta2: %v", err)
		}

		// List all (should return /videos/gopro1.mp4 and /videos/gopro2.mp4)
		all, err := repo.ListAll(ctx)
		if err != nil {
			t.Fatalf("failed to list all: %v", err)
		}
		if len(all) != 2 {
			t.Errorf("expected 2 records, got %d", len(all))
		}

		// List pending (should return only /videos/gopro2.mp4 since gopro1 is synced)
		pending, err := repo.ListPending(ctx)
		if err != nil {
			t.Fatalf("failed to list pending: %v", err)
		}
		if len(pending) != 1 {
			t.Errorf("expected 1 pending record, got %d", len(pending))
		}
		if pending[0].Path != "/videos/gopro2.mp4" {
			t.Errorf("expected pending path '/videos/gopro2.mp4', got %s", pending[0].Path)
		}
	})
}
