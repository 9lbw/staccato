# Testing Graceful Shutdown

## Changes Made

The following changes have been implemented for graceful shutdown:

### 1. **MusicServer Struct Updates**
- Added `server *http.Server` field to store HTTP server instance
- Added `shutdownCh chan struct{}` for coordinating shutdown

### 2. **Configuration Enhancements**
- Added `WriteTimeout` and `IdleTimeout` to server config
- Default values: Write timeout 30s, Idle timeout 120s
- Updated both `config.toml` and `config.example.toml`

### 3. **Start Method Improvements**
- Server now starts in a goroutine
- Proper HTTP server configuration with all timeouts
- Waits for shutdown signal or server error
- No longer blocks the main thread

### 4. **Shutdown Method Rewrite**
- Implements proper graceful shutdown with 30-second timeout
- Shuts down HTTP server gracefully with connection draining
- Stops file watcher, ngrok service, and closes database
- Force closes server if graceful shutdown fails
- Thread-safe shutdown channel handling

### 5. **Main Function Updates**
- Server starts in goroutine
- Signal handling remains the same
- Proper coordination between signal and shutdown

## How to Test

1. **Start the server:**
   ```bash
   ./bin/staccato.exe
   ```

2. **Make some requests:**
   ```bash
   curl http://localhost:8000/health
   curl http://localhost:8000/api/tracks
   ```

3. **Trigger graceful shutdown:**
   - Send SIGINT (Ctrl+C) or SIGTERM
   - Watch the logs for graceful shutdown messages

4. **Expected log output:**
   ```
   Received shutdown signal
   Shutting down music server...
   Shutting down HTTP server...
   HTTP server shut down gracefully
   Stopping file watcher...
   Stopping ngrok service...
   Closing database connection...
   Music server shutdown complete
   ```

## Key Benefits

- **Zero Downtime**: Existing connections are allowed to complete
- **Resource Cleanup**: All resources are properly released
- **Timeout Protection**: Force shutdown after 30 seconds prevents hanging
- **Thread Safety**: Safe to call from multiple goroutines
- **Better Logging**: Clear visibility into shutdown process
- **Production Ready**: Follows HTTP server best practices

## Configuration Options

New timeout settings in `config.toml`:
```toml
[server]
read_timeout_seconds = 30   # Maximum time to read request
write_timeout_seconds = 30  # Maximum time to write response  
idle_timeout_seconds = 120  # Maximum time for keep-alive connections
```

The implementation ensures your music server can be safely deployed in production environments with proper process management.
