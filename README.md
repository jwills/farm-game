# 🌱 Grow Farm

A multiplayer emoji-based farming game. Plant crops, watch them grow, harvest, sell, and visit your neighbors!

## Playing the Game

The game is live at: https://growfarm.exe.xyz:8000/

## Project Structure

```
farm-game/
├── index.html          # The entire game client (HTML/CSS/JS)
├── server/
│   ├── main.go         # Multiplayer server (Go)
│   ├── go.mod          # Go module file
│   ├── neighborhoods.json  # Game state (player farms, etc.)
│   └── neighborhood.service # systemd service file
└── README.md
```

## Building & Running

### Prerequisites

- Go 1.22+ (for the server)
- A web browser (for playing)

### Server

```bash
cd server
go build -o farm-server main.go
./farm-server
```

The server runs on port 8000 and serves both the API and static files.

### Running as a Service (systemd)

```bash
sudo cp server/neighborhood.service /etc/systemd/system/farm-server.service
sudo systemctl daemon-reload
sudo systemctl enable farm-server
sudo systemctl start farm-server
```

### Development

For local development, just run the server and open http://localhost:8000 in your browser.

The game client is a single `index.html` file with embedded CSS and JavaScript. All graphics are emoji-based - no external images required!

## Game Features

- 🌱 Plant and grow crops
- 🌦️ Dynamic weather system with mutations
- 🏘️ Neighborhoods - visit and gift to friends
- 🐾 Collectible pets
- ⚡ Special events and chaos modes
- 💰 Economy with seeds, gear, and upgrades

## Backups

The game state is stored in `server/neighborhoods.json`. Back this up regularly!

```bash
cp server/neighborhoods.json server/neighborhoods-backup-$(date +%Y%m%d).json
```
