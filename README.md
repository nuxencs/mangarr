# Mangarr

**Mangarr** downloads manga chapters from supported sources and saves them as `.cbz` archives.
Use it for quick one-off downloads or let it run in monitor mode and fetch new chapters for you.

[![Release](https://img.shields.io/github/v/release/nuxencs/mangarr?display_name=tag&style=flat-square)](https://github.com/nuxencs/mangarr/releases/latest)
[![License](https://img.shields.io/github/license/nuxencs/mangarr?style=flat-square)](https://github.com/nuxencs/mangarr/blob/main/LICENSE)

## Why Use It

- Download the latest, first, specific, or all available chapters.
- Monitor series and auto-download new releases.
- Save chapters as `.cbz` files that work with comic and manga readers.
- Run as a native binary or with Docker.

## Install

### Binary

Download the latest release for your platform from the [releases page](https://github.com/nuxencs/mangarr/releases/latest).

Available for:

- Linux (`amd64`, `arm`, `arm64`)
- Windows (`amd64`)
- macOS (`amd64`, `arm64`)
- FreeBSD (`amd64`)

### Docker

```bash
docker pull ghcr.io/nuxencs/mangarr:latest
```

Check the installed version:

```bash
mangarr version
```

## Quick Start

Download the latest chapter from a source:

```bash
mangarr download --help
mangarr download -d ./downloads -s tcbscans -m "One Piece"
```

If you want a different source, the value you pass to `-m` changes by provider. Use the table below.

## Monitor Mode

Create a `config.yaml`:

```yaml
downloadLocation: "/path/to/downloads"
checkInterval: 15

monitoredManga:
  One Piece:
    source: "tcbscans"
    manga: "One Piece"
```

Run the monitor:

```bash
mangarr monitor -c ~/.config/mangarr
```

`-c` should point to the directory that contains `config.yaml`.
If you omit it, mangarr also checks common default locations. Full config and override details live in [docs/USAGE.md](./docs/USAGE.md).

## Supported Sources

| Source | `-s` value | Pass to `-m` | Extra input |
| --- | --- | --- | --- |
| [TCB Scans](https://tcbonepiecechapters.com/) | `tcbscans` | exact manga title | none |
| [MangaDex](https://mangadex.org/) | `mangadex` | manga UUID | optional `-g` group UUID, optional `-l` language |
| [MANGA Plus](https://mangaplus.shueisha.co.jp/) | `mangaplus` | numeric title ID | none |
| [Flame Comics](https://flamecomics.xyz/) | `flamecomics` | full series URL | none |
| [Asura Scans](https://asurascans.com/) | `asurascans` | full `https://asurascans.com/comics/...` URL | locked early-access chapters are skipped until public |
| [Cubari](https://cubari.moe/) | `cubari` | gist URL | required `-g` group |
| [Weeb Central](https://weebcentral.com/) | `weebcentral` | full `https://weebcentral.com/...` URL | none |
| [Comix](https://comix.to/) | `comix` | full `https://comix.to/title/...` URL | optional `-g` numeric group ID |
| [Atsumaru](https://atsu.moe/) | `atsumaru` | full `https://atsu.moe/manga/...` URL | required `-g` scan ID |

Comix uses a private frontend protocol. A Comix frontend update can require a Mangarr update.

## More Docs

User docs:

- [Usage and config reference](./docs/USAGE.md)
- [Security notes](./docs/SECURITY.md)

Maintainer docs:

- [Architecture](./ARCHITECTURE.md)
- [Design guide](./docs/DESIGN.md)
- [Plans guide](./docs/PLANS.md)

## License

This project is licensed under the MIT License. See [LICENSE](./LICENSE).

## Disclaimer

Personal use only. Respect the rights of creators and publishers, and support official releases when possible.
