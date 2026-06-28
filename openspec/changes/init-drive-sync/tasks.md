## 1. Project Scaffolding

- [x] 1.1 Inicializar el módulo Go en el directorio raíz (`go mod init driveSync`)
- [x] 1.2 Instalar dependencias requeridas en el archivo `go.mod` (`bubbletea`, `bubbles`, `lipgloss`, `modernc.org/sqlite`, `gopkg.in/yaml.v3`, `google.golang.org/api/drive/v3`, `golang.org/x/oauth2`)
- [x] 1.3 Crear estructura de directorios: `internal/domain`, `internal/adapters/sqlite`, `internal/adapters/gdrive`, `internal/tui`, `cmd/drivesync`

## 2. Domain Layer

- [x] 2.1 Definir estructura `FileMetadata` (`Path`, `Size`, `MTime`, `DriveID`, `Status`, `LastUploadedAt`, `LastSyncedAt`)
- [x] 2.2 Definir interfaz `MetadataRepository` (métodos: `Save`, `FindByPath`, `UpdateStatus`, `ListPending`, `ListAll`)
- [x] 2.3 Definir interfaz `CloudStoragePort` (métodos: `Authenticate`, `UploadFile`, `DownloadFile`, `ListRemoteFolder`)
- [x] 2.4 Definir estructuras de configuración: `Config` y `SyncFolderMapping` (`LocalPath`, `DriveFolderID`)

## 3. SQLite Metadata Adapter

- [ ] 3.1 Implementar inicialización de base de datos SQLite y creación de tabla `file_metadata`
- [ ] 3.2 Implementar adaptador `SqliteMetadataRepository` que implemente la interfaz `MetadataRepository` utilizando el driver en Go puro `modernc.org/sqlite`
- [ ] 3.3 Escribir tests unitarios para verificar inserciones, actualizaciones y listado de archivos en SQLite

## 4. Google Drive Cloud Adapter

- [ ] 4.1 Implementar servidor HTTP loopback temporal en Go para capturar el token de redirección de OAuth 2.0 y guardarlo en `token.json`
- [ ] 4.2 Implementar adaptador `GDriveAdapter` que implemente `CloudStoragePort` utilizando la SDK de Google Drive v3
- [ ] 4.3 Implementar carga resumible (Resumable Upload) de Google Drive para soportar archivos de gran tamaño sin interrupción
- [ ] 4.4 Implementar descarga de archivos desde Drive

## 5. Sync Engine Lógica

- [ ] 5.1 Implementar escáner del sistema de archivos local que compare archivos contra la DB para marcar archivos nuevos o modificados como `pending`
- [ ] 5.2 Implementar motor de sincronización (`SyncCoordinator`) que escanee local y remoto, encole subidas y descargas, y ejecute en goroutines enviando progreso mediante canales de Go

## 6. Interfaz de Terminal (TUI) con Bubbletea

- [ ] 6.1 Configurar modelo base `tea.Model` y estructura del estado de la aplicación
- [ ] 6.2 Implementar el panel lateral izquierdo con listado de archivos locales y estados (`✔`, `⟳`, `⬆`)
- [ ] 6.3 Implementar el panel derecho con el detalle del archivo seleccionado
- [ ] 6.4 Implementar la barra inferior de atajos de teclado y la barra de progreso de subida activa (usando `bubbles/progress`)
- [ ] 6.5 Implementar la vista del Help/README embebido (reemplaza la pantalla principal al presionar `h`)
- [ ] 6.6 Conectar el motor de sincronización con la UI enviando mensajes de progreso en goroutines usando comandos `tea.Cmd`

## 7. Main Entrypoint & Config

- [ ] 7.1 Crear el punto de entrada principal en `cmd/drivesync/main.go`
- [ ] 7.2 Implementar parseo del archivo de configuración `config.yaml` y resolución de rutas de sistema (`~/.config/drivesync/`)
- [ ] 7.3 Crear plantilla base de ejemplo `config.yaml.example`

## 8. Documentación

- [ ] 8.1 Crear `README.md` detallado explicando cómo crear el proyecto en Google Cloud Console, obtener las credenciales de OAuth, configurar `config.yaml` y atajos de teclado
