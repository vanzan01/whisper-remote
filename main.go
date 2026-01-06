package main

import (
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/getlantern/systray"
	"github.com/skip2/go-qrcode"
)

//go:embed icon.ico
var iconData []byte

var (
	port              int
	hotkey            string
	serverURL         string
	user32            = syscall.NewLazyDLL("user32.dll")
	procKeyboardEvent = user32.NewProc("keybd_event")
	procMessageBoxW   = user32.NewProc("MessageBoxW")
)

const (
	VK_RETURN    = 0x0D
	VK_TAB       = 0x09
	VK_ESCAPE    = 0x1B
	VK_SPACE     = 0x20
	VK_LWIN      = 0x5B
	VK_CONTROL   = 0x11
	VK_SHIFT     = 0x10
	VK_MENU      = 0x12 // Alt key

	KEYEVENTF_KEYDOWN = 0x0000
	KEYEVENTF_KEYUP   = 0x0002

	MB_OK              = 0x00000000
	MB_ICONINFORMATION = 0x00000040
)

// Map of key names to virtual key codes
var keyMap = map[string]byte{
	"space":  VK_SPACE,
	"enter":  VK_RETURN,
	"return": VK_RETURN,
	"tab":    VK_TAB,
	"esc":    VK_ESCAPE,
	"escape": VK_ESCAPE,
}

func keyDown(vk byte) {
	procKeyboardEvent.Call(uintptr(vk), 0, KEYEVENTF_KEYDOWN, 0)
}

func keyUp(vk byte) {
	procKeyboardEvent.Call(uintptr(vk), 0, KEYEVENTF_KEYUP, 0)
}

func pressKey(vk byte) {
	keyDown(vk)
	time.Sleep(10 * time.Millisecond)
	keyUp(vk)
}

func pressCtrlWinSpace() {
	keyDown(VK_CONTROL)
	keyDown(VK_LWIN)
	time.Sleep(10 * time.Millisecond)
	keyDown(VK_SPACE)
	time.Sleep(10 * time.Millisecond)
	keyUp(VK_SPACE)
	keyUp(VK_LWIN)
	keyUp(VK_CONTROL)
}

// parseHotkey parses a hotkey string like "ctrl+win+space" or "ctrl+shift+a"
func parseHotkey(hk string) (modifiers []byte, key byte, ok bool) {
	parts := strings.Split(strings.ToLower(hk), "+")
	if len(parts) == 0 {
		return nil, 0, false
	}

	for i, part := range parts {
		part = strings.TrimSpace(part)
		if i == len(parts)-1 {
			// Last part is the main key
			if vk, exists := keyMap[part]; exists {
				key = vk
			} else if len(part) == 1 && part[0] >= 'a' && part[0] <= 'z' {
				// Single letter: A=0x41, B=0x42, etc.
				key = byte(strings.ToUpper(part)[0])
			} else {
				return nil, 0, false
			}
		} else {
			// Modifier keys
			switch part {
			case "ctrl", "control":
				modifiers = append(modifiers, VK_CONTROL)
			case "win", "cmd", "command", "super":
				modifiers = append(modifiers, VK_LWIN)
			case "shift":
				modifiers = append(modifiers, VK_SHIFT)
			case "alt":
				modifiers = append(modifiers, VK_MENU)
			default:
				return nil, 0, false
			}
		}
	}
	return modifiers, key, true
}

func pressHotkey(hk string) {
	modifiers, key, ok := parseHotkey(hk)
	if !ok {
		return
	}

	// Press modifiers
	for _, mod := range modifiers {
		keyDown(mod)
	}
	time.Sleep(10 * time.Millisecond)

	// Press and release main key
	keyDown(key)
	time.Sleep(10 * time.Millisecond)
	keyUp(key)

	// Release modifiers in reverse order
	for i := len(modifiers) - 1; i >= 0; i-- {
		keyUp(modifiers[i])
	}
}

func getPrimaryIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err == nil {
		defer conn.Close()
		localAddr := conn.LocalAddr().(*net.UDPAddr)
		return localAddr.IP.String()
	}

	interfaces, err := net.Interfaces()
	if err != nil {
		return "127.0.0.1"
	}

	for _, iface := range interfaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ip4 := ipnet.IP.To4(); ip4 != nil {
					if ip4[0] == 192 || ip4[0] == 10 {
						return ip4.String()
					}
				}
			}
		}
	}
	return "127.0.0.1"
}

func jsonResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			return
		}
		next(w, r)
	}
}

var startTime = time.Now()

func init() {
	flag.IntVar(&port, "port", 8080, "Port to listen on")
	flag.StringVar(&hotkey, "hotkey", "ctrl+win+space", "Hotkey for whisper button (e.g., ctrl+win+space, ctrl+shift+d)")
}

func showQRCode() {
	// Generate QR code as PNG
	png, err := qrcode.Encode(serverURL, qrcode.Medium, 256)
	if err != nil {
		return
	}

	// Save to temp file
	tmpDir := os.TempDir()
	qrPath := filepath.Join(tmpDir, "whisper-remote-qr.png")
	if err := os.WriteFile(qrPath, png, 0644); err != nil {
		return
	}

	// Create HTML file with QR code and info
	htmlContent := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>Whisper Remote</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
            background: #1a1a1a;
            color: white;
            display: flex;
            flex-direction: column;
            align-items: center;
            justify-content: center;
            min-height: 100vh;
            margin: 0;
            padding: 20px;
            box-sizing: border-box;
        }
        h1 { margin: 0 0 10px 0; font-size: 24px; }
        .url {
            font-family: monospace;
            background: #333;
            padding: 10px 20px;
            border-radius: 8px;
            margin: 15px 0;
            font-size: 18px;
        }
        img {
            background: white;
            padding: 20px;
            border-radius: 16px;
            margin: 20px 0;
        }
        .info {
            color: #888;
            font-size: 14px;
            text-align: center;
            max-width: 400px;
        }
        .hotkey {
            background: #5856d6;
            padding: 5px 12px;
            border-radius: 6px;
            font-family: monospace;
            margin: 10px 0;
        }
    </style>
</head>
<body>
    <h1>Whisper Remote</h1>
    <div class="url">%s</div>
    <img src="file:///%s" alt="QR Code">
    <div class="hotkey">%s</div>
    <p class="info">
        Scan the QR code with your phone to open the remote control.<br>
        Close this window - the server runs in the background.<br>
        Right-click the system tray icon to quit.
    </p>
</body>
</html>`, serverURL, strings.ReplaceAll(qrPath, "\\", "/"), hotkey)

	htmlPath := filepath.Join(tmpDir, "whisper-remote.html")
	if err := os.WriteFile(htmlPath, []byte(htmlContent), 0644); err != nil {
		return
	}

	// Open in default browser
	exec.Command("cmd", "/c", "start", htmlPath).Start()
}

func onReady() {
	systray.SetIcon(iconData)
	systray.SetTitle("WR")
	systray.SetTooltip(fmt.Sprintf("Whisper Remote - %s", serverURL))

	mQR := systray.AddMenuItem("Show QR Code", "Open QR code to connect your phone")
	mURL := systray.AddMenuItem(serverURL, "Server address")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit", "Quit the application")

	go func() {
		for {
			select {
			case <-mQR.ClickedCh:
				showQRCode()
			case <-mURL.ClickedCh:
				showQRCode()
			case <-mQuit.ClickedCh:
				systray.Quit()
			}
		}
	}()
}

func onExit() {
	// Cleanup temp files
	tmpDir := os.TempDir()
	os.Remove(filepath.Join(tmpDir, "whisper-remote-qr.png"))
	os.Remove(filepath.Join(tmpDir, "whisper-remote.html"))
}

func setupHTTP() {
	http.HandleFunc("/whisper", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", 405)
			return
		}
		pressHotkey(hotkey)
		jsonResponse(w, map[string]string{
			"status": "success",
			"action": hotkey,
		})
	}))

	http.HandleFunc("/enter", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", 405)
			return
		}
		pressKey(VK_RETURN)
		jsonResponse(w, map[string]string{
			"status": "success",
			"action": "enter",
		})
	}))

	http.HandleFunc("/ping", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, map[string]interface{}{
			"status":    "alive",
			"uptime":    time.Since(startTime).Seconds(),
			"timestamp": time.Now().Format(time.RFC3339),
		})
	}))

	http.HandleFunc("/ip", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, map[string]string{
			"address": getPrimaryIP(),
		})
	}))

	http.Handle("/", http.FileServer(http.Dir("public")))
}

func main() {
	flag.Parse()

	// Build server URL
	ip := getPrimaryIP()
	serverURL = fmt.Sprintf("http://%s:%d", ip, port)

	setupHTTP()

	// Start HTTP server in background
	go func() {
		addr := fmt.Sprintf("0.0.0.0:%d", port)
		http.ListenAndServe(addr, nil)
	}()

	// Show QR code on startup
	showQRCode()

	// Handle Ctrl+C
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		systray.Quit()
	}()

	// Run systray (this blocks)
	systray.Run(onReady, onExit)
}
