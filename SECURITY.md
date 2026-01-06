# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 1.0.x   | :white_check_mark: |

## Reporting a Vulnerability

If you discover a security vulnerability, please report it by:

1. **Opening a GitHub Issue**: https://github.com/vanzan01/whisper-remote/issues/new
2. **Email**: Contact the maintainer directly via GitHub profile

### What to Include

- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

### Response Timeline

- **Initial Response**: Within 48 hours
- **Status Update**: Within 7 days
- **Resolution**: Depends on severity

## Security Model

### Threat Model

This application is designed for **local network use only**. It assumes:

- Your home/office WiFi network is trusted
- Devices on your network are not malicious
- The application runs on a single user's PC

### What This Application Does

- Runs a local HTTP server (default port 8080)
- Accepts commands from devices on the same network
- Simulates keyboard input via Windows API (`keybd_event`)
- Does **not** transmit any data to external servers
- Does **not** store any user data

### Known Limitations

- **No Authentication**: Any device on your network can send commands
- **No Encryption**: Traffic is HTTP, not HTTPS (acceptable for local network)
- **Keyboard Simulation**: Uses Windows API which may trigger antivirus heuristics

### Antivirus False Positives

Some antivirus tools may flag this application due to:

- Keyboard simulation (common in automation tools)
- Network server functionality
- Go runtime embedding

This is expected behavior. The application is open source (~400 lines of Go) and can be audited in [main.go](main.go).

## Security Best Practices

1. Only run on trusted networks
2. Use Windows Firewall to restrict access if needed
3. Stop the application when not in use
4. Build from source if you prefer: `go build -ldflags="-H=windowsgui" -o whisper-remote.exe .`
