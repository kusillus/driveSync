package tui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"driveSync/internal/domain"
	"driveSync/internal/sync"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Styles for the TUI layout
var (
	docStyle = lipgloss.NewStyle().Margin(1, 2)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#874BFD")).
			Padding(1, 2)

	selectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#AD7CFF"))

	normalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#DDDDDD"))

	statusSyncedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#04B575")).
				Bold(true)

	statusPendingStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FF9E3B")).
				Bold(true)

	statusUploadingStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#3C6FF4")).
				Bold(true)

	statusFailedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FF2A54")).
				Bold(true)

	helpTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#00FFF0"))

	helpKeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#874BFD")).
			Bold(true)
)

type Model struct {
	repo             domain.MetadataRepository
	cloud            domain.CloudStoragePort
	coordinator      *sync.SyncCoordinator
	mappings         []domain.SyncFolderMapping
	files            []*domain.FileMetadata
	selectedIndex    int
	showHelp         bool
	syncing          bool
	syncChan         chan sync.SyncProgress
	currentSyncFile  string
	currentSyncDir   string
	currentSyncBytes int64
	currentSyncTotal int64
	err              error
	width            int
	height           int
	progressBar      progress.Model
}

func NewModel(repo domain.MetadataRepository, cloud domain.CloudStoragePort, mappings []domain.SyncFolderMapping) *Model {
	pg := progress.New(
		progress.WithDefaultGradient(),
		progress.WithoutPercentage(),
	)

	return &Model{
		repo:        repo,
		cloud:       cloud,
		mappings:    mappings,
		coordinator: sync.NewSyncCoordinator(repo, cloud),
		progressBar: pg,
	}
}

func (m *Model) Init() tea.Cmd {
	m.loadFiles()
	return nil
}

func (m *Model) loadFiles() {
	ctx := context.Background()
	// Scan local directory to catch new/modified files
	_, _ = m.coordinator.ScanLocal(ctx, m.mappings)
	// Fetch all tracked files from SQLite
	files, err := m.repo.ListAll(ctx)
	if err == nil {
		m.files = files
	}
}

// listenForProgress is a tea.Cmd that waits for progress events from the sync channel
func listenForProgress(ch <-chan sync.SyncProgress) tea.Cmd {
	return func() tea.Msg {
		p, ok := <-ch
		if !ok {
			return "sync-finished"
		}
		return p
	}
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "up", "k":
			if !m.showHelp && m.selectedIndex > 0 {
				m.selectedIndex--
			}

		case "down", "j":
			if !m.showHelp && m.selectedIndex < len(m.files)-1 {
				m.selectedIndex++
			}

		case "h", "?":
			m.showHelp = !m.showHelp

		case "s":
			if !m.syncing && !m.showHelp {
				m.syncing = true
				m.err = nil
				m.syncChan = make(chan sync.SyncProgress, 100)

				// Start synchronization in background goroutine
				go m.coordinator.Sync(context.Background(), m.mappings, m.syncChan)

				return m, listenForProgress(m.syncChan)
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.progressBar.Width = msg.Width - 10
		if m.progressBar.Width > 80 {
			m.progressBar.Width = 80
		}

	case sync.SyncProgress:
		switch msg.Type {
		case sync.ProgressStart:
			m.currentSyncFile = msg.Path
			m.currentSyncDir = msg.Direction
			m.currentSyncBytes = 0
			m.currentSyncTotal = msg.TotalBytes
		case sync.ProgressUpdate:
			m.currentSyncBytes = msg.BytesSynced
		case sync.ProgressComplete:
			m.currentSyncFile = ""
			m.currentSyncDir = ""
			m.currentSyncBytes = 0
			m.currentSyncTotal = 0
			m.loadFiles()
		case sync.ProgressError:
			m.err = msg.Error
		}
		// Listen for the next progress message
		return m, listenForProgress(m.syncChan)

	case string:
		if msg == "sync-finished" {
			m.syncing = false
			m.loadFiles()
		}
	}

	return m, nil
}

func (m *Model) View() string {
	var s strings.Builder

	// Title Banner
	s.WriteString(titleStyle.Render("⚡ driveSync - Google Drive GoPro Synchronizer ⚡") + "\n\n")

	if m.showHelp {
		s.WriteString(m.helpView())
	} else {
		s.WriteString(m.mainDashboardView())
	}

	// Footer / Hotkeys
	s.WriteString("\n" + m.footerView())

	return docStyle.Render(s.String())
}

func (m *Model) mainDashboardView() string {
	var left, right strings.Builder

	// Calculate widths
	contentWidth := m.width - 8
	if contentWidth < 40 {
		contentWidth = 40
	}
	leftWidth := int(float64(contentWidth) * 0.6)
	rightWidth := contentWidth - leftWidth

	// Left Panel: Files List
	if len(m.files) == 0 {
		left.WriteString("No se encontraron archivos locales ni sincronizados.\nPresiona [S] para escanear y sincronizar.")
	} else {
		for i, f := range m.files {
			statusIcon := "[ ]"
			switch f.Status {
			case domain.StatusSynced:
				statusIcon = statusSyncedStyle.Render("[✔]")
			case domain.StatusPending:
				statusIcon = statusPendingStyle.Render("[⟳]")
			case domain.StatusUploading:
				statusIcon = statusUploadingStyle.Render("[⬆]")
			case domain.StatusFailed:
				statusIcon = statusFailedStyle.Render("[❌]")
			}

			filename := filepath.Base(f.Path)
			line := fmt.Sprintf("%s %s", statusIcon, filename)

			// Truncate line if it exceeds list width
			if len(line) > leftWidth-5 {
				line = line[:leftWidth-8] + "..."
			}

			if i == m.selectedIndex {
				left.WriteString(selectedStyle.Render("> "+line) + "\n")
			} else {
				left.WriteString(normalStyle.Render("  "+line) + "\n")
			}
		}
	}

	// Right Panel: Selected File Details
	if len(m.files) > 0 && m.selectedIndex < len(m.files) {
		selected := m.files[m.selectedIndex]
		right.WriteString(lipgloss.NewStyle().Bold(true).Render("Detalle del archivo:") + "\n\n")
		right.WriteString(fmt.Sprintf("Ruta: %s\n", selected.Path))
		right.WriteString(fmt.Sprintf("Tamaño: %.2f MB\n", float64(selected.Size)/(1024*1024)))
		right.WriteString(fmt.Sprintf("Modificado: %s\n", selected.MTime.Format("2006-01-02 15:04:05")))
		right.WriteString(fmt.Sprintf("Estado: %s\n", string(selected.Status)))

		driveID := selected.DriveID
		if driveID == "" {
			driveID = "No subido"
		}
		right.WriteString(fmt.Sprintf("Google Drive ID: %s\n", driveID))

		uploadedAt := "Nunca"
		if !selected.LastUploadedAt.IsZero() {
			uploadedAt = selected.LastUploadedAt.Format("2006-01-02 15:04:05")
		}
		right.WriteString(fmt.Sprintf("Fecha de subida: %s\n", uploadedAt))

		syncedAt := "Nunca"
		if !selected.LastSyncedAt.IsZero() {
			syncedAt = selected.LastSyncedAt.Format("2006-01-02 15:04:05")
		}
		right.WriteString(fmt.Sprintf("Última sincronización: %s\n", syncedAt))
	} else {
		right.WriteString("Selecciona un archivo para ver sus detalles.")
	}

	// Layout packaging
	leftBox := boxStyle.Width(leftWidth).Height(12).Render(left.String())
	rightBox := boxStyle.Width(rightWidth).Height(12).Render(right.String())

	return lipgloss.JoinHorizontal(lipgloss.Top, leftBox, rightBox)
}

func (m *Model) helpView() string {
	var h strings.Builder

	h.WriteString(helpTitleStyle.Render("Guía de uso & Ayuda (README)") + "\n\n")
	h.WriteString("Esta herramienta te permite sincronizar carpetas locales con Google Drive.\n")
	h.WriteString("Los archivos grandes se cargan asincrónicamente con barras de progreso.\n\n")

	h.WriteString(helpTitleStyle.Render("Configuración:") + "\n")
	h.WriteString("El archivo de configuración debe estar ubicado en:\n")
	h.WriteString("  ~/.config/drivesync/config.yaml\n\n")
	h.WriteString("Formato de ejemplo:\n")
	h.WriteString("  client_id: \"TU_GOOGLE_CLIENT_ID\"\n")
	h.WriteString("  client_secret: \"TU_GOOGLE_CLIENT_SECRET\"\n")
	h.WriteString("  sync_folders:\n")
	h.WriteString("    - local_path: \"/home/usuario/Videos/GoPro\"\n")
	h.WriteString("      drive_folder_id: \"ID_CARPETA_DRIVE\"\n\n")

	h.WriteString(helpTitleStyle.Render("Atajos de teclado:") + "\n")
	h.WriteString(fmt.Sprintf("  %s / %s : Navegar por la lista de archivos locales\n", helpKeyStyle.Render("↑/↓"), helpKeyStyle.Render("j/k")))
	h.WriteString(fmt.Sprintf("  %s     : Iniciar sincronización de archivos (subir pendientes / descargar nuevos)\n", helpKeyStyle.Render("s")))
	h.WriteString(fmt.Sprintf("  %s     : Alternar pantalla de ayuda (README)\n", helpKeyStyle.Render("h")))
	h.WriteString(fmt.Sprintf("  %s     : Salir de la aplicación\n", helpKeyStyle.Render("q")))

	// Wrap in a stylized box
	width := m.width - 8
	if width < 40 {
		width = 40
	}
	return boxStyle.Width(width).Render(h.String())
}

func (m *Model) footerView() string {
	var f strings.Builder

	if m.syncing {
		// Syncing: Show Progress bar and active file details
		f.WriteString("Sincronizando: ")
		filename := filepath.Base(m.currentSyncFile)
		if m.currentSyncDir == "upload" {
			f.WriteString(fmt.Sprintf("[Subiendo] %s\n", filename))
		} else {
			f.WriteString(fmt.Sprintf("[Descargando] %s\n", filename))
		}

		// Calculate progress ratio
		var ratio float64
		if m.currentSyncTotal > 0 {
			ratio = float64(m.currentSyncBytes) / float64(m.currentSyncTotal)
		}
		f.WriteString(m.progressBar.ViewAs(ratio) + " ")
		f.WriteString(fmt.Sprintf("%.1f / %.1f MB", float64(m.currentSyncBytes)/(1024*1024), float64(m.currentSyncTotal)/(1024*1024)))
	} else {
		// Idle / Complete: Show hotkeys
		f.WriteString("[S] Sincronizar  •  [H] Ayuda  •  [Q] Salir")
		if m.err != nil {
			f.WriteString("\n" + statusFailedStyle.Render(fmt.Sprintf("Error: %v", m.err)))
		}
	}

	return f.String()
}
