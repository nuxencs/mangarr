# Usage Reference

Verified against CLI help and config/runtime code on 2026-08-07.

Use this doc when the root README is not enough and you need command, config, or operator detail.

## Commands

### `download`

Download one chapter, many chapters, or everything available from a supported source.

```bash
mangarr download -d <download-dir> -s <source> -m <identifier> [flags]
```

Common flags:

| Flag | Meaning |
| --- | --- |
| `-d`, `--downloadDirectory` | directory where `.cbz` files are written |
| `-s`, `--source` | source identifier such as `tcbscans` or `mangadex` |
| `-m`, `--manga` | source-specific manga identifier |
| `-g`, `--group` | group identifier for sources that support it |
| `-l`, `--language` | language code for sources that support it; default `en` |
| `-n`, `--naming` | filename template |
| `-o`, `--overwrite` | replace the parsed manga title in the output filename |

Chapter selection flags are mutually exclusive:

- `-L`, `--latest`: latest chapter; default when no selector is set
- `-1`, `--first`: first chapter
- `-C`, `--chapters`: specific chapters or ranges such as `1,3,5` or `1-10`
- `-A`, `--all`: all available chapters

The command returns status 0 when all requested chapters are downloaded or already exist. It returns a nonzero status when setup, discovery, selection, or any requested chapter download fails.

Examples:

```bash
# Latest chapter from TCB Scans
mangarr download -d ./downloads -s tcbscans -m "One Piece"

# First English chapter from MangaDex for one group
mangarr download -d ./downloads -s mangadex -m "801513ba-a712-498c-8f57-cae55b38cc92" -g "277df5c9-a486-40f6-8dfa-c086c6b60935" -l en -1

# Specific chapters from MANGA Plus
mangarr download -d ./downloads -s mangaplus -m "100037" -C "6,17"

# Chapter range from Cubari
mangarr download -d ./downloads -s cubari -m "https://git.io/OPM" -g "/r/OnePunchMan" -C "1-3"

# Latest chapter from Asura Scans
mangarr download -d ./downloads -s asurascans -m "https://asurascans.com/comics/solo-max-level-newbie-7f873ca6" -L

# Latest chapter from Atsumaru
mangarr download -d ./downloads -s atsumaru -m "https://atsu.moe/manga/Q5Mqy" -g "cmgzlsevifjhtm191rqugvee3" -L
```

### `monitor`

Run mangarr as a long-lived watcher that checks configured series on an interval.

```bash
mangarr monitor -c <config-dir>
```

Minimal config:

```yaml
downloadLocation: "/path/to/downloads"
checkInterval: 15

monitoredManga:
  One Piece:
    source: "tcbscans"
    manga: "One Piece"
```

Expanded config:

```yaml
downloadLocation: "/path/to/downloads"
namingTemplate: "{manga:<.>} Ch. {num:3}{title: - <.>}"
checkInterval: 15
pprofEnabled: false
pprofAddress: "127.0.0.1:6060"

monitoredManga:
  One Piece:
    source: "tcbscans"
    manga: "One Piece"

  Isekai Ojisan:
    source: "mangadex"
    manga: "d8f1d7da-8bb1-407b-8be3-10ac2894d3c6"
    group: "310361d7-52dd-4848-9b36-2eb4fcc95e83"
    language: "en"
    overwrite: "Uncle from Another World"

  Kagurabachi:
    source: "mangaplus"
    manga: "100274"

logLevel: "DEBUG"
#logPath: "/path/to/logs/mangarr.log"
#logMaxSize: 50
#logMaxBackups: 3
```

Config lookup order:

1. If `-c` is set, mangarr reads `<config-dir>/config.yaml`.
2. `~/.config/mangarr/config.yaml`
3. `~/.mangarr/config.yaml`
4. `config.yaml` next to the binary

Environment overrides use the `MANGARR__` prefix:

- `MANGARR__DOWNLOAD_LOCATION`
- `MANGARR__NAMING_TEMPLATE`
- `MANGARR__CHECK_INTERVAL`
- `MANGARR__PPROF_ENABLED`
- `MANGARR__PPROF_ADDRESS`
- `MANGARR__LOG_LEVEL`
- `MANGARR__LOG_PATH`
- `MANGARR__LOG_MAX_SIZE`
- `MANGARR__LOG_MAX_BACKUPS`

Monitor checks all configured manga once at startup. It then waits for the check
interval. Changes to monitored manga, naming, download location, check interval,
and log level reload while monitor runs. Environment overrides are reapplied to
each reload. Changes to pprof and log file output settings require a restart.

### `version`

Show local build/version info.

```bash
mangarr version
```

## Source Inputs

| Source | `-s` value | Pass to `-m` | Extra input |
| --- | --- | --- | --- |
| [TCB Scans](https://tcbonepiecechapters.com/) | `tcbscans` | exact manga title | none |
| [MangaDex](https://mangadex.org/) | `mangadex` | manga UUID from the title URL | optional `-g` group UUID, optional `-l` language |
| [MANGA Plus](https://mangaplus.shueisha.co.jp/) | `mangaplus` | numeric title ID | none |
| [Flame Comics](https://flamecomics.xyz/) | `flamecomics` | full series URL | none |
| [Asura Scans](https://asurascans.com/) | `asurascans` | current `https://asurascans.com/comics/...` series URL | locked early-access chapters are skipped until public |
| [Cubari](https://cubari.moe/) | `cubari` | gist URL | required `-g` group such as `/r/OnePunchMan` |
| [Weeb Central](https://weebcentral.com/) | `weebcentral` | full series URL | none |
| [Comix](https://comix.to/) | `comix` | full `https://comix.to/title/...` URL | optional `-g` numeric group ID |
| [Atsumaru](https://atsu.moe/) | `atsumaru` | full `https://atsu.moe/manga/...` URL | required `-g` scan ID |

Comix uses a private frontend protocol. Mangarr generates request tokens, decodes API responses, sends the required image referer, and reconstructs scrambled image tiles. A Comix frontend update can require a Mangarr update. Use `-g` when a title has duplicate chapter numbers from different groups.

For implementation-level source behavior and validation rules, see [design-docs/source-adapters.md](./design-docs/source-adapters.md).

## Naming Templates

Available variables:

| Variable | Meaning | Example |
| --- | --- | --- |
| `{manga:<.>}` | manga title | `One Piece` |
| `{num:3}` | chapter number, padded to 3 digits | `001` |
| `{num}` | chapter number, unpadded | `1` |
| `{title: - <.>}` | chapter title with a prefix only when a title exists | ` - Romance Dawn` |

Examples:

- `{manga:<.>} Ch. {num:3}{title: - <.>}` -> `One Piece Ch. 001 - Romance Dawn`
- `{manga:<.>} - {num:3}` -> `One Piece - 001`
- `{manga:<.>} Chapter {num}` -> `One Piece Chapter 1`

## Docker Compose

Current sources use direct HTTP and HTML extraction. The published image does not require Chromium.

```yaml
services:
  mangarr:
    container_name: mangarr
    image: ghcr.io/nuxencs/mangarr:latest
    restart: unless-stopped
    user: ${PUID}:${PGID}
    environment:
      - MANGARR__DOWNLOAD_LOCATION=/downloads
      - MANGARR__CHECK_INTERVAL=15
      - MANGARR__LOG_LEVEL=INFO
    volumes:
      - ./config:/config
      - ./downloads:/downloads
```

Start it:

```bash
docker compose up -d
```

## Advanced Ops

Enable profiling:

```yaml
pprofEnabled: true
pprofAddress: "127.0.0.1:6060"
```

Then:

```bash
mangarr monitor -c ./config
curl http://127.0.0.1:6060/debug/pprof/
go tool pprof http://127.0.0.1:6060/debug/pprof/profile?seconds=30
```

## Build From Source

Requirements:

- Go 1.24.2 or later

```bash
git clone https://github.com/nuxencs/mangarr.git
cd mangarr
go build -o mangarr
```
