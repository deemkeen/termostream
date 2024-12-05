package main

import (
	"bytes"
	"embed"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nathan-fiscaletti/consolesize-go"
	"github.com/skip2/go-qrcode"
)

//go:embed templates/ui.html templates/stream.html
var templates embed.FS

var (
	viewers    = make(map[*websocket.Conn]bool)
	viewersMux sync.RWMutex
)

const (
	letters      = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	suffixLength = 10
	defaultPort  = ":8443"
	ipPrefix     = "192.168."
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func init() {
	_, err := exec.LookPath("chafa")
	if err != nil {
		fmt.Println("Error: chafa is not installed on this system")
		os.Exit(1)
	}
	log.SetOutput(io.Discard)
}

func chafaRender(data []byte) (string, error) {
	cols, _ := consolesize.GetConsoleSize()
	targetWidth := int(float64(cols) / 1.2)
	targetHeight := int(float64(targetWidth) / 2 / 1.2)

	sizeStr := fmt.Sprintf("%dx%d", targetWidth, targetHeight)

	cmd := exec.Command("chafa",
		"--clear",
		"--size", sizeStr,
		"--symbols", "all",
		"--center", "on",
		"--align", "bottom",
		"--format=symbols",
		"-")

	cmd.Stdin = bytes.NewReader(data)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}

	return string(output), nil
}

var renderMutex sync.Mutex

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Time{})
	conn.SetWriteDeadline(time.Time{})
	conn.EnableWriteCompression(false)

	for {
		messageType, data, err := conn.ReadMessage()
		if err != nil {
			break
		}

		if messageType != websocket.BinaryMessage {
			continue
		}

		renderMutex.Lock()
		ascii, err := chafaRender(data)
		renderMutex.Unlock()
		if err != nil {
			continue
		}

		fmt.Print("\033[H\033[2J")
		fmt.Print(ascii)
	}
}

func handleWebSocketWeb(w http.ResponseWriter, r *http.Request) {
	log.Printf("Web client connecting from %s", r.RemoteAddr)
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Upgrade error: %v", err)
		return
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Time{})
	conn.SetWriteDeadline(time.Time{})
	conn.EnableWriteCompression(false)

	viewersMux.Lock()
	viewers[conn] = true
	viewersMux.Unlock()
	defer func() {
		viewersMux.Lock()
		delete(viewers, conn)
		viewersMux.Unlock()
	}()

	log.Printf("Web client connected from %s", r.RemoteAddr)

	for {
		messageType, data, err := conn.ReadMessage()
		if err != nil {
			log.Printf("Read error: %v", err)
			break
		}

		if messageType != websocket.BinaryMessage {
			continue
		}

		viewersMux.RLock()
		for viewer := range viewers {
			if viewer != conn {
				err := viewer.WriteMessage(websocket.BinaryMessage, data)
				if err != nil {
					log.Printf("Write error to viewer: %v", err)
				}
			}
		}
		viewersMux.RUnlock()
	}

	log.Printf("Web client disconnected from %s", r.RemoteAddr)
}

func main() {
	webView := flag.Bool("web", false, "View stream in browser instead of console")
	flag.Parse()

	src := rand.NewSource(time.Now().UnixNano())
	rand.New(src)

	b := make([]byte, suffixLength)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	randomSuffix := string(b)

	http.HandleFunc("/"+randomSuffix, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			template, err := templates.ReadFile("templates/ui.html")
			if err != nil {
				http.Error(w, "Failed to load template", http.StatusInternalServerError)
				return
			}
			w.Write(template)
		}
	})

	http.HandleFunc("/"+randomSuffix+"/view", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		template, err := templates.ReadFile("templates/stream.html")
		if err != nil {
			http.Error(w, "Failed to load template", http.StatusInternalServerError)
			return
		}
		w.Write(template)
	})

	http.HandleFunc("/"+randomSuffix+"/ws", func(w http.ResponseWriter, r *http.Request) {
		if *webView {
			handleWebSocketWeb(w, r)
		} else {
			handleWebSocket(w, r)
		}
	})

	var localIP string
	if envHost := os.Getenv("HOST_ADDR"); envHost != "" {
		localIP = envHost
	} else {
		addrs, err := net.InterfaceAddrs()
		if err != nil {
			fmt.Printf("Failed to get interface addresses: %v\n", err)
			os.Exit(1)
		}

		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok {
				ipStr := ipnet.IP.String()
				if len(ipStr) >= 8 && ipStr[:8] == ipPrefix {
					if ipnet.IP.To4() != nil {
						localIP = ipStr
						break
					}
				}
			}
		}

		if localIP == "" {
			fmt.Println("Could not find local networks IP address")
			os.Exit(1)
		}
	}

	port := defaultPort
	if envPort := os.Getenv("HOST_PORT"); envPort != "" {
		port = ":" + envPort
	}

	captureUrl := fmt.Sprintf("https://%s%s/%s", localIP, port, randomSuffix)
	viewUrl := captureUrl + "/view"
	qr, err := qrcode.New(captureUrl, qrcode.Low)
	if err != nil {
		fmt.Printf("Failed to generate QR code: %v\n", err)
		os.Exit(1)
	}

	clearScreen()
	fmt.Println(qr.ToString(false))
	fmt.Printf("\nCapture URL (scan with phone):\n%s\n", captureUrl)

	if *webView {
		fmt.Printf("\nView the stream in your browser at:\n%s\n", viewUrl)
	}

	if err := http.ListenAndServeTLS(port, "certificate.pem", "private.pem", nil); err != nil {
		fmt.Printf("Server error: %v\n", err)
		os.Exit(1)
	}
}

func clearScreen() {
	switch runtime.GOOS {
	case "linux", "darwin":
		cmd := exec.Command("clear")
		cmd.Stdout = os.Stdout
		cmd.Run()
	case "windows":
		cmd := exec.Command("cmd", "/c", "cls")
		cmd.Stdout = os.Stdout
		cmd.Run()
	}
}
