#!/usr/bin/env bash

set -euo pipefail

export LANG="${LANG:-C.UTF-8}"
export LC_CTYPE="${LC_CTYPE:-C.UTF-8}"

# Openxhh 一键安装脚本
# 用法：curl -fsSL https://raw.githubusercontent.com/NoviceLevel/Openxhh/main/scripts/setup.sh | sudo bash

REPO_URL="${REPO_URL:-https://github.com/NoviceLevel/Openxhh.git}"
BRANCH="${BRANCH:-main}"
INSTALL_DIR="${INSTALL_DIR:-/opt/Openxhh}"
SERVICE_NAME="${SERVICE_NAME:-Openxhh}"
WEBUI_SERVICE_NAME="${WEBUI_SERVICE_NAME:-Openxhh-webui}"
WEBUI_BIN_NAME="${WEBUI_BIN_NAME:-Openxhh-webui}"
GOPROXY_VALUE="${GOPROXY:-https://goproxy.cn,direct}"
GOSUMDB_VALUE="${GOSUMDB:-sum.golang.google.cn}"
GO_BUILD_P="${GO_BUILD_P:-1}"
WEBUI_PORT="${WEBUI_PORT:-29173}"

log() { printf '\033[1;32m[Openxhh]\033[0m %s\n' "$*"; }
err() { printf '\033[1;31m[Openxhh]\033[0m %s\n' "$*" >&2; }

need_root() {
  if [ "$(id -u)" -ne 0 ]; then
    err "请使用 root 运行，或使用：curl -fsSL <脚本地址> | sudo bash"
    exit 1
  fi
}

install_build_deps() {
  if command -v git >/dev/null 2>&1 && command -v go >/dev/null 2>&1 && command -v gcc >/dev/null 2>&1; then
    log "构建依赖已就绪。"
    return
  fi

  if ! command -v apt-get >/dev/null 2>&1; then
    err "未检测到 apt-get，请手动安装 git、Go、gcc、libsqlite3-dev 后重试。"
    exit 1
  fi

  log "安装构建依赖：git、curl、gcc、libsqlite3-dev、snapd。"
  apt-get update -qq
  apt-get install -y -qq git curl ca-certificates build-essential libsqlite3-dev snapd

  if ! command -v go >/dev/null 2>&1; then
    log "安装 Go（通过 snap）。"
    systemctl enable --now snapd.socket >/dev/null 2>&1 || true
    snap install go --classic
  fi
}

build_openxhh() {
  local tmp_dir="$1"
  log "拉取源码：$REPO_URL ($BRANCH)"
  git clone --depth 1 --branch "$BRANCH" "$REPO_URL" "$tmp_dir/src"

  log "编译 Openxhh 主程序和 Web UI..."
  cd "$tmp_dir/src"
  export GOPROXY="$GOPROXY_VALUE"
  export GOSUMDB="$GOSUMDB_VALUE"
  export GOMAXPROCS="${GOMAXPROCS:-1}"
  export CGO_ENABLED=1

  go mod download
  go build -p "$GO_BUILD_P" -o "$tmp_dir/Openxhh" .
  go build -p "$GO_BUILD_P" -o "$tmp_dir/$WEBUI_BIN_NAME" ./cmd/webui-vps
  log "编译完成。"
}

install_binaries() {
  local tmp_dir="$1"
  local timestamp
  timestamp="$(date +%Y%m%d-%H%M%S)"

  mkdir -p "$INSTALL_DIR"

  if [ -f "$INSTALL_DIR/Openxhh" ]; then
    cp "$INSTALL_DIR/Openxhh" "$INSTALL_DIR/Openxhh.bak-$timestamp"
  fi
  if [ -f "$INSTALL_DIR/$WEBUI_BIN_NAME" ]; then
    cp "$INSTALL_DIR/$WEBUI_BIN_NAME" "$INSTALL_DIR/$WEBUI_BIN_NAME.bak-$timestamp"
  fi

  cp "$tmp_dir/Openxhh" "$INSTALL_DIR/Openxhh"
  chmod +x "$INSTALL_DIR/Openxhh"
  cp "$tmp_dir/$WEBUI_BIN_NAME" "$INSTALL_DIR/$WEBUI_BIN_NAME"
  chmod +x "$INSTALL_DIR/$WEBUI_BIN_NAME"
  log "二进制已安装到 $INSTALL_DIR"
}

install_manager_menu() {
  local menu_path="/usr/local/bin/xhh"
  local env_path="/etc/openxhh-manager.env"

  log "安装管理菜单：$menu_path"
  if [ -f "$INSTALL_DIR/scripts/openxhh.sh" ]; then
    cp "$INSTALL_DIR/scripts/openxhh.sh" "$menu_path"
  else
    curl -fsSL "https://raw.githubusercontent.com/NoviceLevel/Openxhh/main/scripts/openxhh.sh" -o "$menu_path"
  fi
  chmod +x "$menu_path"

  cat > "$env_path" <<EOF
REPO_RAW_URL="https://raw.githubusercontent.com/NoviceLevel/Openxhh/main"
INSTALL_DIR="$INSTALL_DIR"
SERVICE_NAME="$SERVICE_NAME"
WEBUI_SERVICE_NAME="$WEBUI_SERVICE_NAME"
WEBUI_PORT="$WEBUI_PORT"
WEBUI_BIN_NAME="$WEBUI_BIN_NAME"
EOF
}

generate_config() {
  cd "$INSTALL_DIR"
  if [ ! -f "$INSTALL_DIR/config.json" ]; then
    log "生成默认 config.json..."
    ./Openxhh 2>/dev/null || true
  fi
  if [ ! -f "$INSTALL_DIR/config.json" ]; then
    err "config.json 生成失败，请检查编译是否成功。"
    exit 1
  fi
}

create_systemd_services() {
  log "创建 systemd 服务..."

  if ! systemctl cat "$SERVICE_NAME" >/dev/null 2>&1; then
    cat > "/etc/systemd/system/${SERVICE_NAME}.service" <<EOF
[Unit]
Description=Openxhh
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=$INSTALL_DIR
ExecStart=$INSTALL_DIR/Openxhh -mode start
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF
    log "已创建 $SERVICE_NAME.service"
  else
    log "$SERVICE_NAME.service 已存在，跳过创建。"
  fi

  if ! systemctl cat "$WEBUI_SERVICE_NAME" >/dev/null 2>&1; then
    cat > "/etc/systemd/system/${WEBUI_SERVICE_NAME}.service" <<EOF
[Unit]
Description=Openxhh VPS Web UI
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=$INSTALL_DIR
ExecStart=$INSTALL_DIR/$WEBUI_BIN_NAME -addr :$WEBUI_PORT -root $INSTALL_DIR -service $SERVICE_NAME
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF
    log "已创建 $WEBUI_SERVICE_NAME.service"
  else
    log "$WEBUI_SERVICE_NAME.service 已存在，跳过创建。"
  fi

  systemctl daemon-reload
  systemctl enable "$SERVICE_NAME" "$WEBUI_SERVICE_NAME" >/dev/null 2>&1
}

start_services() {
  log "启动 Web UI..."
  systemctl start "$WEBUI_SERVICE_NAME" || true
}

print_summary() {
  local ip
  ip=$(curl -4 -s --max-time 5 ifconfig.me 2>/dev/null || echo "你的VPS公网IP")

  log ""
  log "========================================="
  log "  安装完成！接下来按 3 步启动："
  log "========================================="
  log ""
  log "  1. 打开 Web UI 填配置："
  log "     http://${ip}:${WEBUI_PORT}"
  log "     首次登录密码："
  log "     sudo journalctl -u $WEBUI_SERVICE_NAME -n 50 --no-pager"
  log "     至少填写 xhh.owner、ai.model、ai.baseUrl、ai.token。"
  log ""
  log "  2. 扫码登录小黑盒："
  log "     cd $INSTALL_DIR && sudo ./Openxhh -mode login"
  log ""
  log "  3. 启动机器人并看日志："
  log "     sudo systemctl start $SERVICE_NAME"
  log "     sudo journalctl -u $SERVICE_NAME -f"
  log ""
  log "========================================="
  log "  常用命令"
  log "========================================="
  log ""
  log "  sudo systemctl status $SERVICE_NAME        # 查看机器人状态"
  log "  sudo systemctl restart $SERVICE_NAME        # 重启机器人"
  log "  sudo journalctl -u $SERVICE_NAME -f         # 查看机器人日志"
  log "  sudo systemctl restart $WEBUI_SERVICE_NAME  # 重启 Web UI"
  log "  curl -fsSL https://raw.githubusercontent.com/NoviceLevel/Openxhh/main/scripts/update-installed.sh | sudo bash"
  log ""
}

main() {
  need_root
  install_build_deps

  local tmp_dir
  tmp_dir="$(mktemp -d)"
  trap 'rm -rf "$tmp_dir"' EXIT

  build_openxhh "$tmp_dir"
  install_binaries "$tmp_dir"
  mkdir -p "$INSTALL_DIR/scripts"
  cp "$tmp_dir/src/scripts/openxhh.sh" "$INSTALL_DIR/scripts/openxhh.sh"
  install_manager_menu
  generate_config
  create_systemd_services
  start_services
  print_summary
}

main "$@"
