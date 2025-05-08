# gh-install

A GitHub CLI extension to easily install binaries from GitHub releases.

> [!WARNING]
> Very much a work in progress. Currently only supports binaries, doesn't support compressed assets or archived assets. Also does not support system packages. Initial focus is on binaries (tested with Go/Rust binaries)

## Overview

`gh-install` is a command-line tool that simplifies the process of downloading binaries published on GitHub releases. It automates the following tasks:

- Detecting the appropriate binary for your operating system and architecture
- Downloading releases (latest or a specific version)
- Verifying checksums to ensure integrity
- Saving binaries to the appropriate location on your system

## Installation (When I actually cut a release)

```bash
gh extension install esacteksab/gh-install
```

## Usage

### Basic Usage (current API expected to be set, future flags may exist as things progress)

```bash
# Install the latest release
gh install owner/repo

# Install a specific version
gh install owner/repo@v1.2.3
```

```bash
# If you want more verbose logging
GH_INSTALL_INIT_DEBUG=true gh install owner/repo@latest
```

### Options

```bash
Flags:
  -b, --binName string   name to save binary as
  -h, --help             help for install
  -p, --path string      directory location to save binary. Default: $XDG_BIN_HOME
  -s, --sha string       SHA algorithm to use for checksum verification. Valid algorithms are: blake2b, blake2s, crc32, md5, sha224, sha384, sha256, sha1, sha512, sha3-224, sha3-384, sha3-256, sha3-512.
  -v, --version          version
```

### Examples

```bash
# Install the latest release of esacteksab/go-pretty-toml
gh install esacteksab/go-pretty-toml

# Install a specific version of esacteksab/go-pretty-toml
gh install esacteksab/go-pretty-toml@v0.1.3

# Install esacteksab/go-pretty-toml with a custom binary name
gh install esacteksab/go-pretty-toml -b toml-fmt

# Install to a specific directory
gh install esacteksab/go-pretty-toml -p /usr/local/bin -b toml-fmt

# Specify SHA algorithm for checksum verification sha256 is the default if no sha is passed
gh install esacteksab/go-pretty-toml -s sha256
```

## Features

- ✅ Automatic OS/architecture detection
- ✅ Checksum verification
- ✅ Custom binary name support
- ✅ Custom installation path support
- ✅ Latest or specific version installation
- ✅ Progress bar during downloads
- ⏳ Automatic extraction for archives (coming soon)
- ⏳ Post-installation steps (coming soon)

## Technical Details

- Automatically detects release assets matching your system
- Downloads selected assets with progress visualization
- Downloads and verifies checksums when available
- Supports various checksum algorithms
  - some attempt is made to detect algorithm used, but if verification fails, pass `-s/--sha algorithm`
- Configurable binary name and installation path

## License

MIT

## $XDG_BIN_HOME

| Platform   | Default Path                                    |
| ---------- | ----------------------------------------------- |
| Unix/Linux | `~/.local/bin`                                  |
| macOS      | `~/.local/bin`                                  |
| Plan 9     | `$home/bin`                                     |
| Windows    | `UserProgramFiles` or `%LOCALAPPDATA%\Programs` |

## Contributing

This project is still in development

Feel free to file issues, or suggest new features.
