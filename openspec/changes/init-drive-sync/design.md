## Context

Este proyecto es una herramienta de terminal (TUI) en Go que ayuda a sincronizar videos grabados con una cámara GoPro (archivos pesados) desde una laptop (corriendo CachyOS/Arch Linux) hacia Google Drive. La sincronización se realiza de manera selectiva configurando las carpetas en un archivo de configuración YAML local. La UI de terminal emula el comportamiento interactivo de herramientas como `lazygit` usando el ecosistema de Charm (`bubbletea`, `lipgloss`). Para evitar calcular el hash de archivos pesados en cada ejecución, mantenemos un caché de metadatos en una base de datos SQLite local.

## Goals / Non-Goals

**Goals:**
- Interfaz interactiva de terminal (TUI) responsiva que liste archivos y su estado.
- Sincronización a demanda (subir archivos locales nuevos, descargar archivos remotos nuevos).
- Barra de progreso interactiva para visualizar cargas/descargas en tiempo real.
- Base de datos SQLite local para registrar el estado de sincronización (evitando recálculos de hash).
- Autenticación OAuth 2.0 automática abriendo el navegador.
- Selección manual de carpetas a sincronizar configuradas en `config.yaml`.
- Panel de README/Ayuda integrado en la interfaz.

**Non-Goals:**
- Sincronización en segundo plano automática (demonio o inotify).
- Resolución automática de conflictos multi-dispositivo (asumimos que la laptop es la fuente de verdad).
- Creación de carpetas o gestión avanzada de Drive desde la TUI (solo sincronización).

## Decisions

### 1. SQLite para Metadatos
- **Decision**: SQLite usando el driver en Go puro `modernc.org/sqlite`.
- **Rationale**: Evitamos el uso de CGO, haciendo que la compilación del binario en CachyOS sea sumamente simple y portable. SQLite guardará la ruta local, tamaño y fecha de modificación del archivo para detectar cambios rápidamente.
- **Schema**:
```sql
CREATE TABLE IF NOT EXISTS file_metadata (
    local_path TEXT PRIMARY KEY,
    size INTEGER NOT NULL,
    mtime DATETIME NOT NULL,
    drive_id TEXT,
    status TEXT NOT NULL, -- 'pending', 'synced', 'uploading', 'failed'
    last_uploaded_at DATETIME,
    last_synced_at DATETIME
);
```

### 2. Autenticación de Google Drive (OAuth 2.0)
- **Decision**: Servidor loopback HTTP local temporal para la redirección de OAuth.
- **Rationale**: Cuando el usuario inicie la aplicación sin un token válido, se abrirá el navegador apuntando a Google OAuth. Una vez concedido el permiso, Google redireccionará a `http://localhost:<puerto_libre>` administrado temporalmente por la aplicación. Esto intercepta el código de autorización, obtiene el token de acceso/refresco y lo guarda en `~/.config/drivesync/token.json`.
- **Configuración de credenciales**: El usuario deberá crear su propio proyecto en Google Cloud y poner su `client_id` y `client_secret` en el archivo de configuración `config.yaml`. Esto evita problemas de cuotas y bloqueos por aplicación "no verificada" de Google.

### 3. Algoritmo de Sincronización
- **Decision**: Sincronización manual secuencial por carpetas mapeadas.
- **Flujo**:
  1. Leer `config.yaml` para obtener las carpetas locales y los IDs de carpeta correspondientes en Google Drive.
  2. Escanear recursivamente los archivos locales.
  3. Consultar la base de datos SQLite y comparar el tamaño y fecha de modificación (`mtime`) de los archivos físicos con los registrados:
     - Si no existen o difieren, se marcan en la base de datos como `pending`.
  4. Escanear la carpeta remota de Google Drive para identificar archivos que estén en Drive pero no localmente (comparación por nombre/estructura).
  5. Iniciar la cola de operaciones:
     - **Subidas (Uploads)**: Archivos locales `pending` se suben usando *Resumable Uploads* (API de Drive) para tolerar cortes de red en videos pesados. Al finalizar con éxito, se actualiza `drive_id`, `status` a `synced` y se guarda la fecha de subida.
     - **Descargas (Downloads)**: Archivos remotos inexistentes localmente se descargan a la ruta local correspondiente.

### 4. Estructura y Framework de la UI (TUI)
- **Decision**: Utilizar Bubbletea (`github.com/charmbracelet/bubbletea`), Bubbles para la barra de progreso y Lipgloss para los estilos.
- **Pantallas**:
  - **Main Dashboard**: Dividido en un panel izquierdo (lista de archivos locales y su estado: `✔`, `⟳`, `⬆`) y un panel derecho (detalles del archivo seleccionado: tamaño, última fecha de modificación, fecha de subida, ID en Drive).
  - **Help View**: Panel tipo README con instrucciones de configuración, atajos de teclado (`s` para sincronizar, `h` para ayuda, `q` para salir) que reemplaza temporalmente la vista principal.
  - **Progress Bar**: Sección inferior que se activa durante la sincronización activa.

```
┌───────────────────────────────────────┬───────────────────────────────────────┐
│ ARCHIVOS LOCALES                      │ DETALLE DEL ARCHIVO                   │
├───────────────────────────────────────┼───────────────────────────────────────┤
│ [✔] video_gopro_001.mp4               │ Ruta: /videos/video_gopro_001.mp4     │
│ [⟳] video_gopro_002.mp4               │ Tamaño: 2.4 GB                        │
│ [⬆] video_gopro_003.mp4  (45%)        │ Modificado: 2026-06-27 18:30:12       │
│                                       │ Estado: Subiendo                      │
│                                       │ Subido: -                             │
└───────────────────────────────────────┴───────────────────────────────────────┘
  [S] Sincronizar  •  [H] Ayuda  •  [Q] Salir
```

## Risks / Trade-offs

- **Límites de cuota de Google Drive**: Al usar credenciales propias del usuario en `config.yaml`, no afectamos a otros usuarios, pero la cuota de subida puede ser un cuello de botella.
- **Videos muy pesados (GigaBytes)**: Una subida estándar en HTTP puede fallar en conexiones inestables. Mitigation: Forzar el uso del protocolo resumible de Google Drive SDK para archivos mayores a 5MB.
- **Bloqueo del TUI durante subidas**: Si la subida corre en el mismo hilo, el TUI se freezaría. Mitigation: Ejecutar la lógica de sincronización en goroutines separadas y reportar progreso a Bubbletea enviando mensajes `tea.Cmd`.

## Migration Plan

1. Crear estructura básica de Go e inicializar el módulo `go mod init driveSync`.
2. Implementar la interfaz TUI estática con Bubbletea.
3. Desarrollar la capa SQLite y el escáner del sistema de archivos local.
4. Integrar la autenticación de Google Drive v3 y probar la creación de carpetas remota.
5. Implementar el motor de carga asincrónica con soporte para archivos grandes.
6. Armar el panel de README/Ayuda en la UI de terminal.

## Open Questions

- ¿Deberíamos permitir que el usuario defina más de un par de carpetas (Local <-> Remoto) en la configuración? -> Sí, la estructura de `config.yaml` soportará una lista de mapeos:
  ```yaml
  sync_folders:
    - local_path: "/home/kusillus/Videos/GoPro"
      drive_folder_id: "1abc123XYZ..."
  ```
- ¿Qué pasa si el usuario elimina un archivo local? ¿Debería eliminarse en Drive? -> No, para esta primera etapa (alcance inicial), no propagamos eliminaciones para evitar pérdidas accidentales de datos (YAGNI/Seguridad).
