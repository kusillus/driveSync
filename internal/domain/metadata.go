package domain

import (
	"context"
	"time"
)

// FileStatus represents the sync state of a file
type FileStatus string

const (
	StatusPending   FileStatus = "pending"
	StatusSynced    FileStatus = "synced"
	StatusUploading FileStatus = "uploading"
	StatusFailed    FileStatus = "failed"
)

// FileMetadata stores local and remote synchronization state for a single file
type FileMetadata struct {
	Path           string     `json:"path"`
	Size           int64      `json:"size"`
	MTime          time.Time  `json:"mtime"`
	DriveID        string     `json:"drive_id"`
	Status         FileStatus `json:"status"`
	LastUploadedAt time.Time  `json:"last_uploaded_at"`
	LastSyncedAt   time.Time  `json:"last_synced_at"`
}

// MetadataRepository defines the storage interface for local file sync metadata
type MetadataRepository interface {
	Save(ctx context.Context, meta *FileMetadata) error
	FindByPath(ctx context.Context, path string) (*FileMetadata, error)
	UpdateStatus(ctx context.Context, path string, status FileStatus, driveID string) error
	ListPending(ctx context.Context) ([]*FileMetadata, error)
	ListAll(ctx context.Context) ([]*FileMetadata, error)
}
