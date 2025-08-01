# 🎵 Staccato Music Server

A self-hosted music streaming server built with Go and vanilla JavaScript. Stream your local music library from anywhere with a clean, modern web interface.

## ✨ Features

- **🎧 Music Streaming**: Stream your local music library via web browser
- **🎵 Format Support**: Supports FLAC, MP3, WAV, and M4A files
- **📱 Responsive Design**: Works on desktop, tablet, and mobile devices
- **🎶 Playlist Management**: Create and manage custom playlists
- **🔍 Search & Sort**: Search by title, artist, or album with sorting options
- **⬇️ Download Integration**: Download music from YouTube and other platforms (yt-dlp required)
- **🌐 Ngrok Integration**: Expose your server securely over the internet
- **🎮 Discord Rich Presence**: Show what you're listening to in Discord
- **📁 Auto-Scanning**: Automatically detects new music files
- **🎚️ Audio Controls**: Play, pause, skip, shuffle, repeat, and volume control

## 🚀 Quick Start

1. **Download** the latest release for your platform from the [releases page](https://github.com/9lbw/musicserver/releases)
2. **Extract** the archive to a folder of your choice
3. **Create** a `music` folder and add your music files
4. **Run** the executable:
   - Windows: `staccato.exe`
   - Linux/macOS: `./staccato`
5. **Open** your browser to `http://localhost:8080`

## ⚙️ Configuration

Staccato creates a `config.toml` file on first run. Key settings:

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

## 🎮 Discord Rich Presence

Show your currently playing music in Discord! See [DISCORD_SETUP.md](DISCORD_SETUP.md) for setup instructions.

Features:
- Display current track, artist, and album
- Show playback progress and status
- Clickable button to open your music server
- Automatic status updates

## 📥 Music Downloads

Staccato can download music from YouTube and 500+ other sites using yt-dlp:

1. Install yt-dlp: See [INSTALL_YTDLP.md](INSTALL_YTDLP.md)
2. Paste any supported URL in the download section
3. Downloaded music is automatically added to your library

## 🌐 Remote Access

Access your music server from anywhere using ngrok:

1. Get a free ngrok account at [ngrok.com](https://ngrok.com)
2. See [NGROK_SETUP.md](NGROK_SETUP.md) for configuration
3. Share your music with friends securely

## 📁 Supported Formats

- **FLAC** - Lossless audio
- **MP3** - Most common format
- **WAV** - Uncompressed audio
- **M4A** - Apple's audio format

## 🎹 Keyboard Shortcuts

- **Space** - Play/Pause
- **Arrow Right / N** - Next track
- **Arrow Left / P** - Previous track

## ⚡ Performance

- Lightweight Go backend
- Efficient audio streaming with range request support
- SQLite database for fast metadata queries
- Real-time file system monitoring

## 🛠️ Development

```bash
# Clone the repository
git clone https://github.com/9lbw/musicserver.git
cd musicserver

# Install dependencies
go mod download

# Build
go build -o bin/staccato ./cmd/staccato

# Run
./bin/staccato
```

## 📝 License

This project is open source and available under the MIT License.

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## 🐛 Issues

If you encounter any problems, please create an issue on GitHub with:
- Your operating system
- Steps to reproduce
- Error messages or logs
- Configuration details

## 🙏 Acknowledgments

- Built with Go and vanilla JavaScript
- Uses SQLite for data storage
- yt-dlp for download functionality
- ngrok for secure tunneling
- Discord Rich Presence for status integration
