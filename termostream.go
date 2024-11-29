package main

import (
	"bytes"
	"embed"
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

//go:embed templates/ui.html
var templates embed.FS

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

func main() {
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

	http.HandleFunc("/"+randomSuffix+"/ws", handleWebSocket)

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

	url := fmt.Sprintf("https://%s%s/%s", localIP, port, randomSuffix)
	qr, err := qrcode.New(url, qrcode.Low)
	if err != nil {
		fmt.Printf("Failed to generate QR code: %v\n", err)
		os.Exit(1)
	}

	clearScreen()
	fmt.Println(qr.ToString(false))

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
