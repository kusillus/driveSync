# driveSync - Google Drive GoPro Synchronizer ⚡

`driveSync` es una herramienta interactiva de terminal (TUI) escrita en Go diseñada para simplificar y automatizar la sincronización de archivos pesados (como videos de cámaras GoPro) desde tu laptop hacia Google Drive. 

Para optimizar el ancho de banda y la velocidad en tu sistema (como CachyOS/Arch Linux), utiliza una base de datos SQLite local para rastrear el estado de cada archivo, evitando tener que calcular pesados hashes (MD5) en cada ejecución.

---

## Características principales

- **Interfaz interactiva (TUI)**: Diseñada al estilo `lazygit` usando el framework Bubbletea de Charm.
- **Sincronización Selectiva**: Mapeo manual de carpetas locales a carpetas específicas en Google Drive mediante un archivo de configuración simple.
- **Resumable Uploads**: Carga fraccionada (chunks de 5MB) para soportar subidas de videos pesados sin sufrir por micro-cortes de red.
- **Caché Inteligente de Metadatos**: Almacenamiento local SQLite para saber al instante si un archivo está sincronizado, pendiente de subida, o en progreso.
- **Autenticación Simple**: Flujo de autorización OAuth 2.0 por servidor local que abre tu navegador la primera vez y almacena un token refrescable.

---

## Requisitos previos

1. Tener **Go (1.20+)** instalado en tu sistema.
2. Un proyecto en **Google Cloud Console** con la API de Google Drive habilitada.

---

## Configuración paso a paso

### 1. Configurar credenciales en Google Cloud Console
Para evitar límites de cuota y bloqueos por aplicación "no verificada", es necesario usar tus propias credenciales de desarrollador:
1. Ve a [Google Cloud Console](https://console.cloud.google.com/).
2. Crea un nuevo proyecto (ej. `driveSync`).
3. Busca **Google Drive API** y haz clic en **Habilitar**.
4. Ve a la pestaña **Pantalla de consentimiento de OAuth**:
   - Selecciona tipo de usuario **Externo**.
   - Completa los datos obligatorios (nombre de la app, correo).
   - En la sección **Ámbitos (Scopes)**, agrega el ámbito de `../auth/drive` (o acceso completo a archivos).
   - Agrega tu propio correo en la lista de **Usuarios de prueba** (esto es crítico para poder iniciar sesión en desarrollo).
5. Ve a **Credenciales**:
   - Haz clic en **Crear credenciales** -> **ID de cliente de OAuth**.
   - En tipo de aplicación selecciona **Aplicación de escritorio** (Desktop App).
   - Asigna un nombre y haz clic en **Crear**.
   - Copia tu **ID de cliente** (`client_id`) y **Secreto de cliente** (`client_secret`).

### 2. Configurar la Aplicación
Al correr la herramienta por primera vez, se creará el directorio de configuración automáticamente en tu directorio home:
`~/.config/drivesync/`

Crea o edita el archivo `~/.config/drivesync/config.yaml` con el siguiente formato:

```yaml
# ~/.config/drivesync/config.yaml
client_id: "TU_GOOGLE_CLIENT_ID"
client_secret: "TU_GOOGLE_CLIENT_SECRET"

sync_folders:
  - local_path: "/home/usuario/Videos/GoPro"
    drive_folder_id: "ID_DE_LA_CARPETA_REMOTA_EN_GOOGLE_DRIVE"
```

> **Tip**: El `drive_folder_id` es el código alfanumérico que aparece al final de la URL cuando abres tu carpeta en Google Drive desde el navegador (ej: `https://drive.google.com/drive/folders/1abc123XYZ...`).

---

## Compilación y Ejecución

Compila el binario ejecutable desde la raíz del proyecto:
```bash
go build -o drivesync ./cmd/drivesync/...
```

Ejecuta la herramienta:
```bash
./drivesync
```

> **Primer inicio**: La primera vez que lo corras, la aplicación abrirá una pestaña en tu navegador para loguearte con tu cuenta de Google. Una vez concedido el permiso, verás un mensaje de éxito en pantalla, el token se guardará en `~/.config/drivesync/token.json` y se iniciará la interfaz interactiva.

---

## Controles en la Interfaz (TUI)

- **`↑ / ↓` o `j / k`**: Navegar por la lista de archivos detectados.
- **`s`**: Iniciar la sincronización bidireccional (escanea, sube archivos pendientes y descarga archivos nuevos del Drive).
- **`h / ?`**: Alternar el panel de Ayuda y README embebido.
- **`q` o `Ctrl + C`**: Salir de la aplicación.
