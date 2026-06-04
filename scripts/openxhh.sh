#!/usr/bin/env bash

set -euo pipefail

ENV_FILE="${OPENXHH_MANAGER_ENV:-/etc/openxhh-manager.env}"
if [ -f "$ENV_FILE" ]; then
  # shellcheck disable=SC1090
  . "$ENV_FILE"
fi

REPO_RAW_URL="${REPO_RAW_URL:-https://raw.githubusercontent.com/NoviceLevel/Openxhh/main}"
INSTALL_DIR="${INSTALL_DIR:-/opt/Openxhh}"
SERVICE_NAME="${SERVICE_NAME:-Openxhh}"
WEBUI_SERVICE_NAME="${WEBUI_SERVICE_NAME:-Openxhh-webui}"
WEBUI_PORT="${WEBUI_PORT:-29173}"
WEBUI_BIN_NAME="${WEBUI_BIN_NAME:-Openxhh-webui}"

green() { printf '\033[1;32m%s\033[0m\n' "$*"; }
red() { printf '\033[1;31m%s\033[0m\n' "$*" >&2; }
muted() { printf '\033[2m%s\033[0m\n' "$*"; }

need_root() {
  if [ "$(id -u)" -ne 0 ]; then
    red "请使用 sudo 运行：sudo xhh"
    exit 1
  fi
}

pause() {
  printf '\n按回车返回菜单...'
  read -r _ || true
}

public_ip() {
  curl -4 -s --max-time 5 ifconfig.me 2>/dev/null || hostname -I 2>/dev/null | awk '{print $1}' || printf '服务器IP'
}

run_update() {
  green "开始更新 Openxhh..."
  curl -fsSL "$REPO_RAW_URL/scripts/update-installed.sh" | bash
}

restart_service() {
  green "重启机器人服务：$SERVICE_NAME"
  systemctl restart "$SERVICE_NAME"
  systemctl status "$SERVICE_NAME" --no-pager || true
}

start_service() {
  green "启动机器人服务：$SERVICE_NAME"
  systemctl start "$SERVICE_NAME"
  systemctl status "$SERVICE_NAME" --no-pager || true
}

stop_service() {
  green "停止机器人服务：$SERVICE_NAME"
  systemctl stop "$SERVICE_NAME"
  systemctl status "$SERVICE_NAME" --no-pager || true
}

service_status() {
  green "机器人服务状态：$SERVICE_NAME"
  systemctl status "$SERVICE_NAME" --no-pager || true
}

follow_logs() {
  green "按 Ctrl+C 退出日志跟随。"
  journalctl -u "$SERVICE_NAME" -f
}

restart_webui() {
  green "重启 Web UI 服务：$WEBUI_SERVICE_NAME"
  systemctl restart "$WEBUI_SERVICE_NAME"
  systemctl status "$WEBUI_SERVICE_NAME" --no-pager || true
}

latest_webui_password() {
  local password_file="$INSTALL_DIR/webui_password.txt"
  if [ -s "$password_file" ]; then
    sed -n '1{s/[[:space:]]*$//;p;q;}' "$password_file"
    return 0
  fi
  journalctl -u "$WEBUI_SERVICE_NAME" --since "30 days ago" --no-pager 2>/dev/null |
    sed -n 's/.*登录密码:[[:space:]]*//p' |
    tail -n 1
}

print_webui_password() {
  local password
  password="$(latest_webui_password || true)"
  if [ -n "$password" ]; then
    printf '%s\n' "$password"
    return 0
  fi
  red "没有找到明文密码文件：$INSTALL_DIR/webui_password.txt"
  muted "旧版本只保存 webui_auth.json 哈希，无法反推出原密码；请重置后再查看。"
  return 1
}

reset_webui_password() {
  local auth_path="$INSTALL_DIR/webui_auth.json"
  local backup_path
  backup_path="$INSTALL_DIR/webui_auth.json.bak-$(date +%Y%m%d-%H%M%S)"

  if [ -f "$auth_path" ]; then
    cp "$auth_path" "$backup_path"
    rm -f "$auth_path"
    muted "已备份旧认证文件：$backup_path"
  fi

  green "重启 Web UI 生成新密码..."
  systemctl restart "$WEBUI_SERVICE_NAME"
  sleep 2
  print_webui_password
}

webui_password_and_logs() {
  if print_webui_password; then
    return
  fi

  printf '\n是否重置 Web UI 登录密码？输入 RESET 确认，其他内容取消：'
  read -r answer || true
  if [ "$answer" = "RESET" ]; then
    reset_webui_password
  else
    muted "已取消。"
  fi
}

login_xhh() {
  if [ ! -x "$INSTALL_DIR/Openxhh" ]; then
    red "未找到 $INSTALL_DIR/Openxhh，请先安装或更新。"
    return 1
  fi
  green "开始扫码登录。二维码会输出在终端，也会保存到 $INSTALL_DIR/qrcode.png"
  cd "$INSTALL_DIR"
  ./Openxhh -mode login
}

check_ok() {
  local name="$1" test_expr="$2" detail="$3"
  if bash -c "$test_expr"; then
    printf '  [OK] %s：%s\n' "$name" "$detail"
    return 0
  fi
  printf '  [WARN] %s异常：%s\n' "$name" "$detail"
  return 1
}

config_hint() {
  local cfg="$INSTALL_DIR/config.json"
  if command -v grep >/dev/null 2>&1; then
    if grep -Eq '"owner"[[:space:]]*:[[:space:]]*"[^0-9, ]' "$cfg"; then
      printf '  [WARN] owner 可能不是纯数字 UID，请在 Web UI 配置体检里确认。\n'
    fi
    if ! grep -Eq '/v1/(chat/completions|responses)' "$cfg"; then
      printf '  [WARN] ai.baseUrl 可能没有填写完整 /v1/chat/completions 或 /v1/responses。\n'
    fi
  fi
}

health_check() {
  local errors=0 warnings=0
  green "Openxhh 基础体检"
  muted "更完整的体检请打开 Web UI -> 配置体检。"
  printf '\n'

  check_ok "安装目录" "[ -d '$INSTALL_DIR' ]" "$INSTALL_DIR" || errors=$((errors + 1))
  check_ok "主程序" "[ -x '$INSTALL_DIR/Openxhh' ]" "$INSTALL_DIR/Openxhh" || errors=$((errors + 1))
  check_ok "Web UI 程序" "[ -x '$INSTALL_DIR/$WEBUI_BIN_NAME' ]" "$INSTALL_DIR/$WEBUI_BIN_NAME" || warnings=$((warnings + 1))
  check_ok "config.json" "[ -s '$INSTALL_DIR/config.json' ]" "$INSTALL_DIR/config.json" || errors=$((errors + 1))
  check_ok "cookie.json" "[ -s '$INSTALL_DIR/cookie.json' ]" "$INSTALL_DIR/cookie.json" || warnings=$((warnings + 1))

  if command -v systemctl >/dev/null 2>&1; then
    if systemctl is-active --quiet "$SERVICE_NAME"; then
      printf '  [OK] 机器人服务运行中：%s\n' "$SERVICE_NAME"
    else
      printf '  [WARN] 机器人服务未运行：%s\n' "$SERVICE_NAME"
      warnings=$((warnings + 1))
    fi
    if systemctl is-active --quiet "$WEBUI_SERVICE_NAME"; then
      printf '  [OK] Web UI 服务运行中：%s\n' "$WEBUI_SERVICE_NAME"
    else
      printf '  [WARN] Web UI 服务未运行：%s\n' "$WEBUI_SERVICE_NAME"
      warnings=$((warnings + 1))
    fi
  fi

  if [ -s "$INSTALL_DIR/config.json" ]; then
    config_hint
  fi

  printf '\n体检完成：%s 个错误，%s 个提醒。\n' "$errors" "$warnings"
  printf 'Web UI 地址：http://%s:%s\n' "$(public_ip)" "$WEBUI_PORT"
}

show_menu() {
  clear || true
  green "Openxhh VPS 管理菜单"
  printf '安装目录：%s\n机器人服务：%s\nWeb UI：%s\n\n' "$INSTALL_DIR" "$SERVICE_NAME" "$WEBUI_SERVICE_NAME"
  printf '  1) 更新到最新版\n'
  printf '  2) 重启机器人\n'
  printf '  3) 查看机器人日志\n'
  printf '  4) 查看机器人状态\n'
  printf '  5) 启动机器人\n'
  printf '  6) 停止机器人\n'
  printf '  7) 扫码登录小黑盒\n'
  printf '  8) 重启 Web UI\n'
  printf '  9) 直接输出 Web UI 密码\n'
  printf ' 10) 基础配置体检\n'
  printf '  0) 退出\n\n'
}

main() {
  need_root
  while true; do
    show_menu
    printf '请输入数字：'
    read -r choice || exit 0
    case "$choice" in
      1) run_update; pause ;;
      2) restart_service; pause ;;
      3) follow_logs ;;
      4) service_status; pause ;;
      5) start_service; pause ;;
      6) stop_service; pause ;;
      7) login_xhh; pause ;;
      8) restart_webui; pause ;;
      9) webui_password_and_logs; pause ;;
      10) health_check; pause ;;
      0|q|Q) exit 0 ;;
      *) red "无效选项：$choice"; sleep 1 ;;
    esac
  done
}

main "$@"
