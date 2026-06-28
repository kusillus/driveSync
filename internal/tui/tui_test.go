package tui

import (
	"context"
	"testing"

	"driveSync/internal/domain"

	tea "github.com/charmbracelet/bubbletea"
)

// MockRepository implements domain.MetadataRepository
type MockRepository struct{}

func (m *MockRepository) Save(ctx context.Context, meta *domain.FileMetadata) error { return nil }
func (m *MockRepository) FindByPath(ctx context.Context, path string) (*domain.FileMetadata, error) {
	return nil, nil
}
func (m *MockRepository) UpdateStatus(ctx context.Context, path string, status domain.FileStatus, driveID string) error {
	return nil
}
func (m *MockRepository) ListPending(ctx context.Context) ([]*domain.FileMetadata, error) {
	return nil, nil
}
func (m *MockRepository) ListAll(ctx context.Context) ([]*domain.FileMetadata, error) {
	return []*domain.FileMetadata{
		{Path: "/videos/gopro1.mp4", Size: 100, Status: domain.StatusPending},
		{Path: "/videos/gopro2.mp4", Size: 200, Status: domain.StatusSynced},
	}, nil
}

// MockCloud implements domain.CloudStoragePort
type MockCloud struct{}

func (m *MockCloud) Authenticate(ctx context.Context) error { return nil }
func (m *MockCloud) UploadFile(ctx context.Context, localPath string, relativePath string, driveFolderID string, progressChan chan<- int64) (string, error) {
	return "", nil
}
func (m *MockCloud) DownloadFile(ctx context.Context, driveID string, destPath string, progressChan chan<- int64) error {
	return nil
}
func (m *MockCloud) ListRemoteFolder(ctx context.Context, driveFolderID string) ([]domain.RemoteFile, error) {
	return nil, nil
}

func TestTUI_StateTransitions(t *testing.T) {
	repo := &MockRepository{}
	cloud := &MockCloud{}
	mappings := []domain.SyncFolderMapping{
		{LocalPath: "/tmp", DriveFolderID: "folder-123"},
	}

	model := NewModel(repo, cloud, mappings)
	model.Init()

	t.Run("Toggle Help Panel", func(t *testing.T) {
		if model.showHelp {
			t.Error("expected showHelp to be false initially")
		}

		// Send "h" key msg
		updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
		m := updatedModel.(*Model)

		if !m.showHelp {
			t.Error("expected showHelp to be true after pressing 'h'")
		}

		// Send "h" key msg again to toggle back
		updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
		m = updatedModel.(*Model)

		if m.showHelp {
			t.Error("expected showHelp to be false after pressing 'h' again")
		}
	})

	t.Run("List Navigation", func(t *testing.T) {
		if model.selectedIndex != 0 {
			t.Errorf("expected selectedIndex to be 0, got %d", model.selectedIndex)
		}

		// Press "down" (j)
		updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
		m := updatedModel.(*Model)

		if m.selectedIndex != 1 {
			t.Errorf("expected selectedIndex to be 1, got %d", m.selectedIndex)
		}

		// Press "down" (j) again (should cap at max index: 1)
		updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
		m = updatedModel.(*Model)

		if m.selectedIndex != 1 {
			t.Errorf("expected selectedIndex to still be 1, got %d", m.selectedIndex)
		}

		// Press "up" (k)
		updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
		m = updatedModel.(*Model)

		if m.selectedIndex != 0 {
			t.Errorf("expected selectedIndex to return to 0, got %d", m.selectedIndex)
		}
	})

	t.Run("Start Syncing", func(t *testing.T) {
		if model.syncing {
			t.Error("expected syncing to be false initially")
		}

		// Press "s"
		updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
		m := updatedModel.(*Model)

		if !m.syncing {
			t.Error("expected syncing to be true after pressing 's'")
		}
	})
}
