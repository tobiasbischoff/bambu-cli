# bambu-cli

CLI for controlling BambuLab printers directly over MQTT/FTPS/camera.

## Build

```bash
go build -o bambu-cli ./cmd/bambu-cli
```

## Install (Homebrew)

```bash
brew install tobiasbischoff/tap/bambu-cli
```

## Quick start

```bash
# Create a profile
bambu-cli config set --printer lab \
  --ip 192.168.1.200 \
  --serial AC12309BH109 \
  --access-code-file ~/.config/bambu/lab.code \
  --default

# Status
bambu-cli status

# Start a print
bambu-cli print start ./benchy.3mf --plate 1
```

## Config

- User config: `~/.config/bambu/config.json`
- Project config: `./.bambu.json`
- Precedence: flags > env > project config > user config

### Env vars

- `BAMBU_PROFILE`
- `BAMBU_IP`
- `BAMBU_SERIAL`
- `BAMBU_ACCESS_CODE_FILE`
- `BAMBU_TIMEOUT`
- `BAMBU_NO_CAMERA`
- `BAMBU_MQTT_PORT`
- `BAMBU_FTP_PORT`
- `BAMBU_CAMERA_PORT`

## Notes

- Printer must be reachable on ports 8883 (MQTT), 990 (FTPS), 6000 (camera).
- Avoid passing access codes via flags; use `--access-code-file` or `--access-code-stdin`.
