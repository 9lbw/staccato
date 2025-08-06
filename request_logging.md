# Request Logging Implementation

## What Was Added

### 1. **Configuration Options**
Added `request_logging` option to the `[logging]` section:
```toml
[logging]
  level = "debug"
  format = "text"
  file = ""
  request_logging = true  # Enable/disable request logging
```

### 2. **Request Logging Middleware**
- **Captures**: Method, URL, client IP, status code, response size, duration
- **Smart filtering**: Skips noisy static assets (CSS, JS, images)
- **Human readable**: Response sizes formatted as KB/MB/GB
- **Performance aware**: Minimal overhead when disabled

### 3. **Panic Recovery Middleware**
- **Prevents crashes**: Recovers from panics in handlers
- **Logs details**: Records panic information for debugging
- **Graceful response**: Returns 500 error instead of crashing

## Example Log Output

With `request_logging = true`, you'll see logs like:
```
[GET] /api/tracks 192.168.1.100:52341 - 200 15KB (23ms)
[POST] /api/playlists/create 192.168.1.100:52341 - 201 < 1KB (5ms)
[GET] /stream/123 192.168.1.100:52341 - 206 2MB (156ms)
[GET] /albumart/abc123 192.168.1.100:52341 - 200 45KB (12ms)
PANIC in POST /api/download: runtime error: nil pointer dereference
```

## Configuration Examples

### **Development/Debugging** (current config.toml):
```toml
[logging]
  level = "debug"
  request_logging = true
```

### **Production/Quiet**:
```toml
[logging]
  level = "info"
  request_logging = false
```

## Benefits for Local Server

- ✅ **Debug streaming issues**: See which requests are slow
- ✅ **Monitor downloads**: Track yt-dlp download requests
- ✅ **Identify problems**: Spot 404s, 500s, slow responses
- ✅ **Usage patterns**: See what family members are listening to
- ✅ **Performance insights**: Identify bottlenecks
- ✅ **Crash protection**: Panic recovery prevents server crashes

## Testing

1. **Start server with logging enabled**:
   ```bash
   ./bin/staccato.exe
   ```

2. **Make some requests**:
   ```bash
   curl http://localhost:8000/health
   curl http://localhost:8000/api/tracks
   curl http://localhost:8000/api/playlists
   ```

3. **Check logs** for request entries

4. **Disable logging** by setting `request_logging = false` and restart

The implementation is lightweight and designed specifically for local network debugging rather than enterprise monitoring.
