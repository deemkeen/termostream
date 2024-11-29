# termoStream

termoStream is a command-line tool that allows you to stream your mobile device's camera to your terminal using QR codes, creating a unique ASCII art video stream.

## Features

- Live camera streaming to terminal
- ASCII art conversion in real-time
- Switch between front and back cameras
- Clean and responsive web interface
- Secure HTTPS connection
- Cross-platform (Windows, macOS, Linux)

## Requirements

- Go 1.23 or higher
- `chafa` command-line tool for ASCII art conversion
- Modern web browser with camera access

## Installation

```bash
# Clone the repository
git clone <this repo>

cd termostream

# Build the binary and generate SSL certificates
./build.sh

# Optional: Copy binary to your PATH to run from anywhere
sudo cp termostream /usr/local/bin/
```

## Usage

1. Run the `termostream` binary in your terminal:
```bash
./termostream
```

2. A QR code will appear in your terminal

3. Scan the QR code with your mobile device's camera

4. Your browser will open with camera controls:
   - Click "Start Video Stream" to begin streaming
   - Use "Switch Camera" to toggle between front/back cameras
   - Click "Stop Video Stream" to end the stream

## Environment Variables

- `HOST_ADDR`: Set a specific IP address or hostname (default: auto-detect)
- `HOST_PORT`: Set a specific port (default: 8443)

Example:
```bash
HOST_ADDR=192.168.1.100 HOST_PORT=3000 ./termostream
```

## Notes

- HTTPS is required for camera access in modern browsers
- The stream URL contains a random suffix for basic security
- Works on any device with a modern browser and camera
- No need to be on the same Wi-Fi network as long as the device can reach the host IP
- Video stream is converted to ASCII art using the `chafa` utility

## Technical Details

- Uses WebSocket for real-time video streaming
- Converts frames to JPEG before transmission
- Optimized for terminal display using `chafa`
- Responsive web interface with mobile-first design

## Credits

- ASCII art conversion powered by [Chafa](https://hpjansson.org/chafa/)
- QR code generation using [github.com/skip2/go-qrcode](https://github.com/skip2/go-qrcode)

## License

MIT License
