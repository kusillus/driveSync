package domain

import (
	"context"
	"time"
)

// RemoteFile represents file metadata fetched from Google Drive
type RemoteFile struct {
	ID    string    `json:"id"`
	Name  string    `json:"name"`
	Size  int64     `json:"size"`
	MTime time.Time `json:"mtime"`
}

// CloudStoragePort defines the interface for interacting with Google Drive
type CloudStoragePort interface {
	Authenticate(ctx context.Context) error
	UploadFile(ctx context.Context, localPath string, driveFolderID string, progressChan chan<- int64) (string, error)
	DownloadFile(ctx context.Context, driveID string, destPath string, progressChan chan<- int64) error
	ListRemoteFolder(ctx context.Context, driveFolderID string) ([]RemoteFile, error)
}
