# Staccato Music Server
A self-hosted music streaming server built with Go and vanilla JavaScript. Stream your local music library from anywhere with a web interface.

## Features
- Music streaming via web browser
- Format support for FLAC, MP3, WAV, and M4A files
- Responsive design for desktop, tablet, and mobile
- Playlist management
- Search and sort by title, artist, or album
- Download integration with yt-dlp
- Ngrok integration for remote access
- Discord Rich Presence
- Advanced Session Management with 3 Priority Modes
- Automatic music library scanning
- Audio controls: play, pause, skip, shuffle, repeat, and volume

## Session Management
Staccato supports multiple clients (devices) connected simultaneously with intelligent session priority handling:

### Priority Modes
1. **Play Priority** (`priority_mode = "play"`)
   - Works like Spotify: when one device starts playing, it pauses all others
   - Perfect for preventing music conflicts between devices
   - Discord RPC shows the currently playing device

2. **Session Priority** (`priority_mode = "session"`) - *Default*
   - Most recent session controls Discord RPC
   - All devices can play simultaneously
   - Original behavior preserved

3. **Session + Play Priority** (`priority_mode = "session_play"`)
   - Best of both worlds: new playing device gets Discord RPC priority
   - Other devices continue playing in background
   - Useful for having music in multiple rooms

### Configuration
```toml
[session]
priority_mode = "play"          # "play", "session", or "session_play"
activity_timeout = 30           # Session timeout in seconds
discord_rpc_mode = "active_only" # Discord integration mode
```

## Quick Start
1. Download the latest release from the [releases page](https://github.com/9lbw/musicserver/releases)
2. Extract the archive
3. Create a `music` folder and add your music files
4. Run the executable:
   - Windows: `staccato.exe`
   - Linux/macOS: `./staccato`
5. Open your browser to `http://localhost:8080`

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
[discord]
enabled = false
application_id = "your_discord_app_id"
```

## Discord Rich Presence
Display your currently playing music in Discord. See [config.example.toml](config.example.toml) for configuration.

## Music Downloads
Download music from YouTube and other sites using yt-dlp:
1. Install yt-dlp
2. Paste URLs in the download section
3. Downloaded music is automatically added to your library

## Remote Access
Access your server remotely using ngrok:
1. Create an account at [ngrok.com](https://ngrok.com)
2. See [config.example.toml](config.example.toml) for configuration

## Supported Formats
- FLAC - Lossless audio
- MP3 - Common format
- WAV - Uncompressed audio
- M4A - Apple's audio format

## Keyboard Shortcuts
- Space - Play/Pause
- Arrow Right / N - Next track
- Arrow Left / P - Previous track

## Performance
- Lightweight Go backend
- Efficient audio streaming with range request support
- SQLite database for metadata queries
- Real-time file system monitoring

## Development
```bash
git clone https://github.com/9lbw/staccato.git
cd staccato
go mod download
go build -o bin/staccato ./cmd/staccato
./bin/staccato
```

## License
This project is available under the MIT License.

## Contributing
Contributions are welcome. Please submit Pull Requests.