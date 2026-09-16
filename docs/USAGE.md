# Usage Reference

Verified against CLI help and config/runtime code on 2026-09-16.

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
| `--series` | exact, case-sensitive entry name from `monitoredManga` in config |
| `-d`, `--downloadDirectory` | directory where `.cbz` files are written |
| `-s`, `--source` | source identifier such as `tcbscans` or `mangadex` |
| `-m`, `--manga` | source-specific manga identifier |
| `-g`, `--group` | group identifier for sources that support it |
| `-l`, `--language` | language code for sources that support it; default `en` |
| `-n`, `--naming` | filename template |
| `-o`, `--overwrite` | replace the parsed manga title in the output filename |
| `-f`, `--force` | re-download selected chapters even if their archives already exist |

Chapter selection flags are mutually exclusive:

- `-L`, `--latest`: latest chapter; default when no selector is set
- `-1`, `--first`: first chapter
- `-C`, `--chapters`: specific chapters or ranges such as `1,3,5` or `1-10`
- `-A`, `--all`: all available chapters

Existing archives are skipped unless `-f` / `--force` is supplied. Force works with
any chapter selector above. It replaces only the output paths calculated for
those chapters; it does not search the library for old files. To repair an
archive, use the same download directory, manga title, and naming template that
produced its path. A changed title or template can produce a new file instead.
The `--overwrite` flag changes the manga title used for the output directory and
filename; it does not force a download. Monitor mode has no force option and
continues to skip existing archives.

Each old archive remains in place until the replacement is downloaded and
assembled successfully. If replacement fails, or cancellation is detected before
publication, the old archive is preserved. Cancellation after the final check
can still allow publication. See the [runtime model](./design-docs/runtime-model.md#state)
for the publication sequence.

The command returns status 0 when all requested chapters are downloaded successfully or skipped and enabled logging succeeds. It returns a nonzero status when setup, discovery, selection, any requested chapter download, or file logging fails.

#### Download a configured series

Reuse an existing `monitoredManga` entry without repeating its source, URL/ID, or group:

```bash
mangarr download -c ~/.config/mangarr --series "One Piece" -C "1-3"
```

Quote names containing spaces. `--series` matches the config key, not the provider's
manga title. It uses the same config discovery and environment overrides as
`monitor`. It reads one snapshot without starting monitoring, watching, or
rewriting the config. The config must pass the same validation as `monitor`.
The config file must already exist. A missing selected config causes an error
without creating a sample config, even when `MANGARR__DOWNLOAD_LOCATION` is set.

The entry supplies `source`, `manga`, `group`, `language`, and `overwrite`; global
`downloadLocation` and `namingTemplate` supply the output settings. Omitted entry
language defaults to `en`. Explicit download flags override these values, including
explicit empty `--group` or `--overwrite` to clear a configured value. Precedence is
explicit flags, then environment overrides, then YAML, then built-in defaults.
Config validation runs before flag overrides. Download inputs are then validated,
so flags can supply missing entry values. Global output settings must be valid
in the config even when flags override them.
Chapter selection always comes from the CLI and still defaults to latest.

Unknown entries, missing effective source/manga, and invalid source inputs fail
before provider discovery. A required entry field can be supplied by its CLI flag.
Without `--series`, `-d`, `-s`, and `-m` remain required. Config supplies logging
settings only; it does not replace explicit download inputs. Profiling and
scheduling settings do not change download behavior.

Examples:

```bash
# Configured series with a one-off output directory
mangarr download -c ~/.config/mangarr --series "One Piece" -C "1-3" -d ./downloads

# Latest chapter from TCB Scans
mangarr download -d ./downloads -s tcbscans -m "One Piece"

# First English chapter from MangaDex for one group
mangarr download -d ./downloads -s mangadex -m "801513ba-a712-498c-8f57-cae55b38cc92" -g "277df5c9-a486-40f6-8dfa-c086c6b60935" -l en -1

# Specific chapters from MANGA Plus
mangarr download -d ./downloads -s mangaplus -m "100037" -C "6,17"

# Repair selected chapters already on disk
mangarr download -d ./downloads -s tcbscans -m "One Piece" -C "6,17" --force

# Chapter range from Cubari
mangarr download -d ./downloads -s cubari -m "https://git.io/OPM" -g "/r/OnePunchMan" -C "1-3"

# Latest chapter from Asura Scans
mangarr download -d ./downloads -s asurascans -m "https://asurascans.com/comics/solo-max-level-newbie-7f873ca6" -L

# Latest chapter from Atsumaru
mangarr download -d ./downloads -s atsumaru -m "https://atsu.moe/manga/Q5Mqy" -g "cmgzlsevifjhtm191rqugvee3" -L
```

#### Download logs

The existing `logPath` setting enables file logging for both commands. Monitor
writes that file; each manual download writes a separate JSONL file under
`<logPath>.downloads/`. Downloads never append to the monitor's active file or
backups. The terminal prints the actual run-log path and keeps its current
human-readable diagnostics. Stdout is not captured.

Download reads an available config using the [lookup order below](#monitor),
with the same `MANGARR__` environment overrides. Use `-c` to select another config,
not to enable logging. Ordinary downloads still work without a config; logging
environment settings can also supply a destination when no config is found.
A missing explicitly selected config, or a missing config for `--series`, fails
without creating a sample. An empty log path keeps downloads console-only.

The published Docker image exposes `/config/config.yaml` through the existing
binary-adjacent lookup location. Thus an ordinary exec command uses its logging
settings without an extra flag:

```bash
docker exec mangarr mangarr download -d /downloads -s tcbscans -m "One Piece"
```

Relative log paths are relative to the process working directory, not the config
directory. `logLevel` filters the download file only; it does not hide terminal
summaries. Run files use owner-only permissions on Unix. Check directory access
permissions on other platforms. Persisted diagnostics remove URL credentials,
query strings, and fragments. In each text field, a URL query or fragment also
removes all subsequent text because it can contain query secrets. The file marks
this text as `[redacted URL suffix]`. Terminal output stays unchanged. Logs can
still contain series names, URL paths, and local paths. Review logs before sharing
them.

Retention uses the existing settings, without another enable option:

- `logMaxSize` bounds each run file in MiB. At the limit, the file records a size
  warning and stops accepting diagnostics. Downloads continue with terminal
  output, but the command returns a logging error when it finishes.
- `logMaxBackups` bounds completed run files, keeping the most recently started
  runs. Active runs are additional and are never deleted by cleanup. Each active
  run has the same per-file size bound.
- Cleanup runs at startup and completion. An interrupted process releases its
  run lock, so its file becomes eligible for the next cleanup. Only generated
  `download-<timestamp>-<random>.jsonl` files in this dedicated directory are
  managed. Other files, monitor logs, and monitor backups are left alone.
- Use a local filesystem that supports OS file locks for the run-log directory.

If logging cannot initialize, the command fails before provider work. Later
write, sync, or cleanup failures are reported and return a nonzero status;
existing download errors are preserved. A logging failure does not undo completed
archives. Config parsing errors and command-line parsing errors can occur before
a destination is available and therefore remain terminal-only.

#### Bulk downloads and rate limits

`--all` keeps the same chapter selection and skip-on-existing behavior. If a run
fails, rerun it to download missing chapters without replacing existing archives.

Image downloads and adapters using shared direct HTTP retry transport failures,
HTTP 429, and HTTP 500/502/503/504, with **up to three attempts total per request**.
Between retries, the fallback wait is one second, then two seconds, each with up
to 250 ms of jitter. A valid `Retry-After` header (seconds or HTTP date) extends
that wait when needed; zero, expired, missing, or malformed guidance never
shortens the fallback wait.

A server-directed wait above five minutes fails with an explanation instead of
retrying before the server permits it or waiting indefinitely. Cancellation
interrupts retry waits. Exhausted retries still report the failing chapter and
make `download` return a nonzero status; partial chapters are not published.
HTTP 401/403/404/405 and other non-retryable statuses still fail immediately.

This policy also applies to shared HTTP requests in monitor mode. It adds no
config or CLI settings and does not change concurrency or monitor polling.
It does not coordinate a provider-wide cooldown across requests or processes;
persistent limits can still fail after the retry budget. Colly-based HTML
scraping has separate request handling; its discovery/page requests do not gain
these retries, although its image downloads do.

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
2. `<user-config-dir>/mangarr/config.yaml` (for example,
   `~/.config/mangarr/config.yaml` on Linux).
3. `~/.mangarr/config.yaml`
4. `config.yaml` next to the binary

The current working directory is not an implicit config location. Use `-c .` to
load `./config.yaml` explicitly.

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
each reload. Atomic replacement and delete-then-recreate saves remain watched.
An invalid, incomplete, or temporarily missing config keeps the last valid
settings active. Changes to pprof and log file output settings require a restart.

### `version`

Show local build/version info and attempt an advisory update check. Local version
output succeeds when GitHub is unavailable.

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

- Go 1.27.0 or later

```bash
git clone https://github.com/nuxencs/mangarr.git
cd mangarr
go build -o mangarr
```
