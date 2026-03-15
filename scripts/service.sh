#!/bin/bash
set -euo pipefail

LABEL="ai.vanadis.agent-chat-bridge"
PLIST_PATH="$HOME/Library/LaunchAgents/${LABEL}.plist"
PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
INSTALL_DIR="$HOME/.local/bin"
INSTALL_BINARY="$INSTALL_DIR/agent-chat-bridge"
CONFIG_SRC="$PROJECT_DIR/configs/config.yaml"
CONFIG_DIR="$HOME/.config/agent-chat-bridge"
CONFIG_DST="$CONFIG_DIR/config.yaml"
LOG_DIR="$HOME/.local/share/agent-chat-bridge/logs"

usage() {
    echo "Usage: $0 {install|uninstall|reload|status|logs}"
    echo ""
    echo "  install    Build, install binary and config, create launchd service"
    echo "  uninstall  Stop service, remove plist and installed binary"
    echo "  reload     Rebuild binary, copy config, restart service"
    echo "  status     Show service status"
    echo "  logs       Tail service logs"
    exit 1
}

build_and_install() {
    echo "Building binary..."
    (cd "$PROJECT_DIR" && go build -o agent-chat-bridge ./cmd/agent-chat-bridge)
    mkdir -p "$INSTALL_DIR"
    cp "$PROJECT_DIR/agent-chat-bridge" "$INSTALL_BINARY"
    echo "Binary installed: $INSTALL_BINARY"
}

install_config() {
    if [ ! -f "$CONFIG_SRC" ]; then
        echo "ERROR: Config not found: $CONFIG_SRC"
        echo "Copy configs/config.yaml.example to configs/config.yaml and configure it."
        exit 1
    fi
    mkdir -p "$CONFIG_DIR"
    cp "$CONFIG_SRC" "$CONFIG_DST"
    echo "Config installed: $CONFIG_DST"
}

stop_service() {
    if launchctl list "$LABEL" &>/dev/null; then
        launchctl unload "$PLIST_PATH" 2>/dev/null || true
        echo "Service stopped."
    fi
}

start_service() {
    if [ ! -f "$PLIST_PATH" ]; then
        echo "ERROR: Plist not found. Run '$0 install' first."
        exit 1
    fi
    launchctl load "$PLIST_PATH"
    echo "Service started."
}

install_service() {
    stop_service
    build_and_install
    install_config

    mkdir -p "$LOG_DIR"
    mkdir -p "$HOME/Library/LaunchAgents"

    cat > "$PLIST_PATH" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>${LABEL}</string>
    <key>ProgramArguments</key>
    <array>
        <string>${INSTALL_BINARY}</string>
        <string>--config</string>
        <string>${CONFIG_DST}</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>ThrottleInterval</key>
    <integer>10</integer>
    <key>StandardOutPath</key>
    <string>${LOG_DIR}/stdout.log</string>
    <key>StandardErrorPath</key>
    <string>${LOG_DIR}/stderr.log</string>
    <key>EnvironmentVariables</key>
    <dict>
        <key>PATH</key>
        <string>/usr/local/bin:/usr/bin:/bin:/opt/homebrew/bin:${HOME}/.local/bin</string>
        <key>HOME</key>
        <string>${HOME}</string>
    </dict>
</dict>
</plist>
EOF

    start_service
    echo ""
    echo "Service installed."
    echo "  Binary: $INSTALL_BINARY"
    echo "  Config: $CONFIG_DST"
    echo "  Plist:  $PLIST_PATH"
    echo "  Logs:   $LOG_DIR/"
}

uninstall_service() {
    stop_service

    if [ -f "$PLIST_PATH" ]; then
        rm "$PLIST_PATH"
        echo "Plist removed."
    fi

    if [ -f "$INSTALL_BINARY" ]; then
        rm "$INSTALL_BINARY"
        echo "Binary removed: $INSTALL_BINARY"
    fi

    echo "Service uninstalled."
    echo "Config left in place: $CONFIG_DST"
}

reload_service() {
    echo "Rebuilding and reloading service..."
    stop_service
    build_and_install
    install_config
    start_service
    echo "Service reloaded with updated binary and config."
}

show_status() {
    if launchctl list "$LABEL" &>/dev/null; then
        echo "Service: LOADED"
        launchctl list "$LABEL"
    else
        echo "Service: NOT LOADED"
    fi

    if pgrep -f "$INSTALL_BINARY" >/dev/null 2>&1; then
        echo ""
        echo "Process:"
        pgrep -fl "$INSTALL_BINARY"
    fi
}

show_logs() {
    if [ ! -d "$LOG_DIR" ]; then
        echo "No logs directory: $LOG_DIR"
        exit 1
    fi
    tail -f "$LOG_DIR/stdout.log" "$LOG_DIR/stderr.log"
}

case "${1:-}" in
    install)   install_service ;;
    uninstall) uninstall_service ;;
    reload)    reload_service ;;
    status)    show_status ;;
    logs)      show_logs ;;
    *)         usage ;;
esac
