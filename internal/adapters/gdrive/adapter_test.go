package gdrive

import (
	"testing"

	"driveSync/internal/domain"
)

// Ensure GDriveAdapter implements domain.CloudStoragePort at compile time
func TestInterfaceImplementation(t *testing.T) {
	var _ domain.CloudStoragePort = (*GDriveAdapter)(nil)
}
