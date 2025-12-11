# Mangarr

**Mangarr** is a command-line tool and monitoring service for downloading manga chapters from various sources. It supports both one-time downloads and continuous monitoring for new chapter releases.

[![Release](https://img.shields.io/github/v/release/nuxencs/mangarr?display_name=tag&style=flat-square)](https://github.com/nuxencs/mangarr/releases/latest)
[![License](https://img.shields.io/github/license/nuxencs/mangarr?style=flat-square)](https://github.com/nuxencs/mangarr/blob/main/LICENSE)
[![Docker Pulls](https://img.shields.io/docker/pulls/ghcr.io/nuxencs/mangarr?style=flat-square)](https://ghcr.io/nuxencs/mangarr)

## Features

- 📥 **Download chapters** from multiple manga sources with a single command
- 🔄 **Monitor manga** automatically and download new chapters as they're released
- 📦 **CBZ archive format** - chapters are saved as `.cbz` files for easy reading with comic book readers
- 🎨 **Customizable naming** - define your own chapter naming templates
- 🌍 **Multi-language support** - download manga in different languages (source-dependent)
- 📋 **Flexible chapter selection** - download first, latest, specific chapters, ranges, or all chapters
- ⚙️ **Configuration** - use YAML config files or environment variables
- 🐳 **Docker support** - run as a containerized service

## Supported Sources

Mangarr currently supports downloading from **9 different manga sources**:

| Source | Identifier | Notes |
|--------|-----------|-------|
| [TCB Scans](https://tcbonepiecechapters.com/) | `tcbscans` | Requires manga title |
| [MangaDex](https://mangadex.org/) | `mangadex` | Requires manga ID (UUID), supports group filtering and language selection |
| [MANGA Plus](https://mangaplus.shueisha.co.jp/) | `mangaplus` | Requires manga ID (numeric) |
| [Flame Comics](https://flamecomics.xyz/) | `flamecomics` | Requires full manga URL |
| [Asura Scans](https://asuracomic.net/) | `asurascans` | Requires manga identifier, uses browser automation |
| [Cubari](https://cubari.moe/) | `cubari` | Requires gist URL and group identifier |
| [MangaPark](https://mangapark.net/) | `mangapark` | Requires manga identifier, uses browser automation |
| [Weeb Central](https://weebcentral.com/) | `weebcentral` | Requires manga identifier, uses browser automation |
| [Comix](https://comix-online.org/) | `comix` | Requires manga identifier and group |

## Installation

### Download Binary

Download the latest release for your platform from the [releases page](https://github.com/nuxencs/mangarr/releases/latest).

Available for:
- Linux (amd64, arm, arm64)
- Windows (amd64)
- macOS (amd64, arm64)
- FreeBSD (amd64)

### Docker

Pull the Docker image:

```bash
docker pull ghcr.io/nuxencs/mangarr:latest
```

Or use Docker Compose - see the [Docker Compose Setup](#docker-compose-setup) section.

## Usage

### Quick Start - Download Command

The `download` command allows you to download manga chapters on-demand:

```bash
mangarr download -d <download-directory> -s <source> -m <manga-identifier> [options]
```

**Required Flags:**
- `-d, --downloadDirectory` - Directory where downloads will be saved
- `-s, --source` - Source to download from (see [Supported Sources](#supported-sources))
- `-m, --manga` - Manga identifier (varies by source: title, ID, or URL)

**Optional Flags:**
- `-n, --naming` - Naming template (default: `{manga:<.>} Ch. {num:3}{title: - <.>}`)
- `-o, --overwrite` - Override the manga title
- `-g, --group` - Scanlation group (for sources that support it)
- `-l, --language` - Language code (default: `en`, for sources that support it)

**Chapter Selection (mutually exclusive):**
- `-1, --first` - Download the first chapter
- `-L, --latest` - Download the latest chapter (default if no selection is specified)
- `-C, --chapters` - Download specific chapters (e.g., `1,3,5` or `1-10`)
- `-A, --all` - Download all available chapters

### Examples

**Download the latest chapter from TCB Scans:**
```bash
mangarr download -d ./downloads -s tcbscans -m "One Piece"
```

**Download the first chapter from MangaDex with specific group:**
```bash
mangarr download -d ./downloads -s mangadex -m "801513ba-a712-498c-8f57-cae55b38cc92" -g "277df5c9-a486-40f6-8dfa-c086c6b60935" -l en -1
```

**Download specific chapters from MANGA Plus:**
```bash
mangarr download -d ./downloads -s mangaplus -m "100037" -C "6,17"
```

**Download chapter range from Cubari:**
```bash
mangarr download -d ./downloads -s cubari -m "https://git.io/OPM" -g "/r/OnePunchMan" -C "1-3"
```

**Download all chapters from Flame Comics:**
```bash
mangarr download -d ./downloads -s flamecomics -m "https://flamecomics.xyz/series/solo-leveling-ragnarok/" -A
```

**Start monitoring all the manga in your config:**
```bash
mangarr monitor -c ./config/mangarr
```

### Monitor Command

The `monitor` command runs continuously and checks for new chapters at regular intervals:

```bash
mangarr monitor -c <config-path>
```

**Flags:**
- `-c, --config` - Path to configuration directory (optional)

If no config path is specified, mangarr will search for `config.yaml` in:
1. Current directory
2. `~/.config/mangarr/`
3. `~/.mangarr/`

### Version Command

Check the installed version and check for updates:

```bash
mangarr version
```

## Configuration

### Configuration File

Create a `config.yaml` file to configure the monitor mode:

```yaml
# Download Location (required)
downloadLocation: "/path/to/downloads"

# Naming Template
# Available variables: {manga:<.>}, {num:3}, {title: - <.>}
# Default: "{manga:<.>} Ch. {num:3}{title: - <.>}"
namingTemplate: "{manga:<.>} Ch. {num:3}{title: - <.>}"

# Check interval in minutes
# Default: 15
checkInterval: 15

# Monitored Manga
monitoredManga:
  # Entry for TCB Scans
  One Piece:
    source: "tcbscans"
    manga: "One Piece"

  # Entry for MangaDex with group and language
  Isekai Ojisan:
    source: "mangadex"
    manga: "d8f1d7da-8bb1-407b-8be3-10ac2894d3c6"
    group: "310361d7-52dd-4848-9b36-2eb4fcc95e83"
    language: "en"
    overwrite: "Uncle from Another World"

  # Entry for MANGA Plus
  Kagurabachi:
    source: "mangaplus"
    manga: "100274"

  # Entry for Flame Comics
  Solo Leveling Ragnarok:
    source: "flamecomics"
    manga: "https://flamecomics.xyz/series/solo-leveling-ragnarok/"

  # Entry for Cubari
  One Punch Man:
    source: "cubari"
    manga: "https://git.io/OPM"
    group: "/r/OnePunchMan"

# Logging configuration (optional)
#logPath: "/path/to/logs/mangarr.log"
logLevel: "DEBUG"  # Options: ERROR, DEBUG, INFO, WARN, TRACE
#logMaxSize: 50     # Max log size in megabytes
#logMaxBackups: 3   # Max number of old log files
```

### Configuration Locations

Mangarr will search for the configuration file in the following order:

1. Path specified with `-c` or `--config` flag
2. `config.yaml` in the default user configuration directory (`~/.config/mangarr/`)
3. `config.yaml` in a folder inside your home directory (`~/.mangarr/`)
4. `config.yaml` in the directory of the binary

### Environment Variables

You can override configuration values using environment variables with the `MANGARR__` prefix:

- `MANGARR__DOWNLOAD_LOCATION` - Download directory path
- `MANGARR__NAMING_TEMPLATE` - Naming template string
- `MANGARR__CHECK_INTERVAL` - Check interval in minutes
- `MANGARR__LOG_LEVEL` - Log level (ERROR, DEBUG, INFO, WARN, TRACE)
- `MANGARR__LOG_PATH` - Path to log file
- `MANGARR__LOG_MAX_SIZE` - Maximum log file size in megabytes
- `MANGARR__LOG_MAX_BACKUPS` - Maximum number of old log files to keep

### Naming Templates

Customize how chapters are named using template variables:

| Variable | Description | Example |
|----------|-------------|---------|
| `{manga:<.>}` | Manga title | `One Piece` |
| `{num:3}` | Chapter number (padded to 3 digits) | `001`, `010`, `100` |
| `{num}` | Chapter number (no padding) | `1`, `10`, `100` |
| `{title: - <.>}` | Chapter title with prefix | ` - Chapter Title` |

**Example naming templates:**

- Default: `{manga:<.>} Ch. {num:3}{title: - <.>}` → `One Piece Ch. 001 - Romance Dawn`
- Simple: `{manga:<.>} - {num:3}` → `One Piece - 001`
- With chapter word: `{manga:<.>} Chapter {num}` → `One Piece Chapter 1`

## Docker Compose Setup

Create a `docker-compose.yml` file:

```yaml
services:
  mangarr:
    container_name: mangarr
    image: ghcr.io/nuxencs/mangarr:latest
    restart: unless-stopped
    user: ${PUID}:${PGID}  # Optional: Set user/group ID
    environment:
      - MANGARR__DOWNLOAD_LOCATION=/downloads
      - MANGARR__CHECK_INTERVAL=15
      - MANGARR__LOG_LEVEL=INFO
    volumes:
      - ./config:/config           # Configuration directory
      - ./downloads:/downloads     # Download directory
```

Start the service:

```bash
docker-compose up -d
```

## Source-Specific Notes

### MangaDex

- **Manga ID**: Use the UUID from the manga URL (e.g., `https://mangadex.org/title/UUID`)
- **Group ID**: Optional UUID of the scanlation group
- **Language**: ISO 639-1 language code (default: `en`)

### MANGA Plus

- **Manga ID**: Numeric ID from the manga URL

### TCB Scans

- **Manga Title**: Exact manga title as it appears on the site

### Flame Comics

- **Manga URL**: Full URL to the manga page

### Cubari

- **Manga URL**: Gist URL for the manga
- **Group**: Group identifier (e.g., `/r/OnePunchMan`)

### Browser-Based Sources (Asura Scans, MangaPark, Weeb Central)

Some sources use browser automation to bypass Cloudflare protection. These may take slightly longer to process.

## How It Works

1. **Monitor Mode**: Mangarr checks configured manga sources at regular intervals
2. **Chapter Detection**: Identifies the latest chapter number for each manga
3. **Download Check**: Verifies if the chapter already exists locally
4. **Download**: If new, downloads all chapter images and creates a CBZ archive
5. **File Naming**: Uses the configured naming template to name the file

Downloaded chapters are organized as:
```
downloads/
└── Manga Title/
    ├── Manga Title Ch. 001.cbz
    ├── Manga Title Ch. 002.cbz
    └── Manga Title Ch. 003.cbz
```

## Building from Source

Requirements:
- Go 1.24.2 or later

```bash
git clone https://github.com/nuxencs/mangarr.git
cd mangarr
go build -o mangarr
```

## Contributing

Contributions are welcome! Please feel free to submit issues or pull requests.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Disclaimer

This tool is for personal use only. Please respect the rights of content creators and publishers. Support official releases when possible.