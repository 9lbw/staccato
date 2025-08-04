# Staccato Music Server

A self-hosted music streaming server built with Go and vanilla JavaScript.

[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](https://www.gnu.org/licenses/gpl-3.0)
[![Go Version](https://img.shields.io/badge/Go-1.19+-blue.svg)](https://golang.org)
[![Release](https://img.shields.io/github/v/release/9lbw/staccato)](https://github.com/9lbw/staccato/releases)

## Features

- Music streaming via web browser
- Format support for FLAC, MP3, WAV, and M4A files
- Responsive design for desktop, tablet, and mobile
- Playlist management and search functionality
- Download integration with yt-dlp
- Ngrok integration for remote access
- Automatic library scanning and file monitoring
- Full audio controls with keyboard shortcuts

## Quick Start

Download the latest release from the [releases page](https://github.com/9lbw/staccato/releases), extract it, and create a `music` folder with your audio files.

```bash
# Windows
staccato.exe

# Linux/macOS
./staccato
```

Open your browser to `http://localhost:8080`

## Configuration

Staccato creates a `config.toml` file on first run:

```toml
[server]
port = "8080"
host = "0.0.0.0"

[music]
library_path = "./music"
scan_on_startup = true
watch_for_changes = true

[ngrok]
enabled = false
auth_token = ""
```

## Music Downloads

Install [yt-dlp](https://github.com/yt-dlp/yt-dlp) and paste URLs in the web interface. Downloaded music is automatically added to your library.

```bash
pip install yt-dlp
```

## Remote Access

1. Create an account at [ngrok.com](https://ngrok.com)
2. Add your auth token to `config.toml`
3. Set `enabled = true` under `[ngrok]`
4. Restart the server

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| Space | Play/Pause |
| → / N | Next track |
| ← / P | Previous track |

## Development

```bash
git clone https://github.com/9lbw/staccato.git
cd staccato
go mod download
go build -o bin/staccato ./cmd/staccato
./bin/staccato
```

## License

GPLv3 License, see [LICENSE](LICENSE) file for details.

## Contributing

Contributions welcome. Please submit pull requests or open issues for bugs and feature requests.
