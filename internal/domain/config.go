package domain

// SyncFolderMapping defines the relationship between a local folder and a Google Drive folder ID
type SyncFolderMapping struct {
	LocalPath     string `yaml:"local_path"`
	DriveFolderID string `yaml:"drive_folder_id"`
}

// Config holds all the configuration parameters for driveSync
type Config struct {
	ClientID     string              `yaml:"client_id"`
	ClientSecret string              `yaml:"client_secret"`
	SyncFolders  []SyncFolderMapping `yaml:"sync_folders"`
}
