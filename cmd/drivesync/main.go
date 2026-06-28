package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"driveSync/internal/adapters/gdrive"
	"driveSync/internal/adapters/sqlite"
	"driveSync/internal/domain"
	"driveSync/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
	"gopkg.in/yaml.v3"
)

const (
	configDirName = "drivesync"
	configFileName = "config.yaml"
	dbFileName     = "metadata.db"
	tokenFileName  = "token.json"
)

func main() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("Error obteniendo el directorio home: %v\n", err)
		os.Exit(1)
	}

	appConfigDir := filepath.Join(homeDir, ".config", configDirName)
	if err := os.MkdirAll(appConfigDir, 0755); err != nil {
		fmt.Printf("Error creando directorio de configuración: %v\n", err)
		os.Exit(1)
	}

	configPath := filepath.Join(appConfigDir, configFileName)
	dbPath := filepath.Join(appConfigDir, dbFileName)
	tokenPath := filepath.Join(appConfigDir, tokenFileName)

	// Load or create configuration
	config, err := loadConfig(configPath)
	if err != nil {
		fmt.Printf("Error cargando configuración: %v\n", err)
		os.Exit(1)
	}

	// Validate credentials
	if config.ClientID == "" || config.ClientID == "TU_GOOGLE_CLIENT_ID" ||
		config.ClientSecret == "" || config.ClientSecret == "TU_GOOGLE_CLIENT_SECRET" {
		fmt.Printf("Configuración inicial creada en: %s\n", configPath)
		fmt.Println("Por favor, edita el archivo y configura tus credenciales de Google API antes de continuar.")
		os.Exit(1)
	}

	// Initialize SQLite Database
	repo, err := sqlite.NewSqliteRepository(dbPath)
	if err != nil {
		fmt.Printf("Error inicializando base de datos SQLite: %v\n", err)
		os.Exit(1)
	}
	defer repo.Close()

	// Initialize Google Drive Adapter
	oauthMgr := gdrive.NewOAuthManager(config.ClientID, config.ClientSecret, tokenPath)
	driveAdapter := gdrive.NewGDriveAdapter(oauthMgr)

	// Run auth check in background / flow when requested by user or TUI
	// For convenience, we authenticate at startup so the loopback server triggers immediately if token is missing.
	ctx := context.Background()
	fmt.Println("Verificando autenticación con Google Drive...")
	if err := driveAdapter.Authenticate(ctx); err != nil {
		fmt.Printf("Error de autenticación con Google Drive: %v\n", err)
		os.Exit(1)
	}

	// Start Bubbletea TUI Application
	m := tui.NewModel(repo, driveAdapter, config.SyncFolders)
	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error ejecutando la interfaz de terminal: %v\n", err)
		os.Exit(1)
	}
}

func loadConfig(path string) (*domain.Config, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Create default config file if it does not exist
		defaultCfg := &domain.Config{
			ClientID:     "TU_GOOGLE_CLIENT_ID",
			ClientSecret: "TU_GOOGLE_CLIENT_SECRET",
			SyncFolders: []domain.SyncFolderMapping{
				{
					LocalPath:     "/absolute/path/to/local/videos",
					DriveFolderID: "ID_DE_LA_CARPETA_EN_GOOGLE_DRIVE",
				},
			},
		}

		data, err := yaml.Marshal(defaultCfg)
		if err != nil {
			return nil, err
		}

		if err := os.WriteFile(path, data, 0600); err != nil {
			return nil, err
		}

		return defaultCfg, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var cfg domain.Config
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
