## Why

Actualmente el proceso de transferir videos de alta definición (como los grabados con GoPro) desde la laptop a Google Drive requiere de una carga manual a través del navegador, lo cual es ineficiente y propenso a interrupciones. Se necesita una herramienta de terminal (TUI) en CachyOS que permita visualizar el estado de sincronización de los archivos, ver el progreso de carga y disparar la sincronización de carpetas seleccionadas manualmente, optimizando el ancho de banda y el almacenamiento local sin la complejidad de conflictos de edición multi-dispositivo.

## What Changes

- **New**: Aplicación de terminal (TUI) escrita en Go utilizando el framework Bubbletea (`charmbracelet/bubbletea`).
- **New**: Base de datos local SQLite para almacenar el estado de sincronización de cada archivo (ruta, tamaño, fecha de modificación, ID de Google Drive, estado de sincronización y fecha de subida).
- **New**: Integración con la API de Google Drive v3 con flujo de autenticación OAuth 2.0 (que abre el navegador para loguearse y almacena un token local).
- **New**: Configuración manual de carpetas mediante un archivo `config.yaml` local, permitiendo sincronización selectiva de IDs de carpetas específicas de Drive.
- **New**: Lógica de sincronización bidireccional simple: subida de archivos locales nuevos y descarga de archivos remotos nuevos dentro de las carpetas configuradas.
- **New**: Panel de ayuda y guía de uso (tipo README) embebido directamente dentro de la interfaz de la TUI.

## Capabilities

### New Capabilities

- `sync-engine`: Motor en Go que maneja el escaneo del sistema de archivos local, comparación contra SQLite para detección de cambios (por tamaño/mtime), y llamadas a la API de Google Drive para subir/descargar archivos de forma asincrónica.
- `tui-dashboard`: Interfaz de usuario basada en Bubbletea con una lista interactiva de archivos y sus estados (`✔` Sincronizado, `⟳` Pendiente, `⬆` Subiendo), barra de progreso para cargas pesadas, atajos de teclado y una sección de README/Ayuda integrada.

### Modified Capabilities

<!-- Ninguna - Proyecto Inicial -->

## Impact

- **Dependencias**:
  - `google.golang.org/api/drive/v3` (API de Google Drive)
  - `github.com/charmbracelet/bubbletea` (TUI framework)
  - `github.com/charmbracelet/bubbles` (Componentes de UI como barras de progreso y listas)
  - `github.com/charmbracelet/lipgloss` (Estilos para la terminal)
  - `modernc.org/sqlite` (Driver de SQLite en Go puro, evitando dependencias de CGO)
  - `gopkg.in/yaml.v3` (Parser de configuración)
- **Configuración y Almacenamiento**:
  - DB de metadata: `~/.config/drivesync/metadata.db`
  - Configuración de carpetas: `~/.config/drivesync/config.yaml`
  - Token de Drive: `~/.config/drivesync/token.json`
