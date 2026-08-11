#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND_DIR="$ROOT_DIR/backend"
LOG_FILE="$BACKEND_DIR/auto_pro.log"
PID_FILE="$BACKEND_DIR/auto_pro.pid"
PORT="${PORT:-19127}"
export GOCACHE="${GOCACHE:-$ROOT_DIR/.cache/go-build}"
mkdir -p "$GOCACHE"

cd "$BACKEND_DIR"

if [[ -f "$PID_FILE" ]]; then
  OLD_PID="$(cat "$PID_FILE" || true)"
  if [[ -n "$OLD_PID" ]] && kill -0 "$OLD_PID" 2>/dev/null; then
    kill "$OLD_PID" || true
    sleep 1
  fi
fi

PORT_PIDS="$(lsof -tiTCP:"$PORT" -sTCP:LISTEN 2>/dev/null || true)"
if [[ -n "$PORT_PIDS" ]]; then
  kill $PORT_PIDS || true
  sleep 1
fi

go build -o auto_pro .

: > "$LOG_FILE"
AUTO_PRO_DATA_DIR="$BACKEND_DIR" PORT="$PORT" nohup ./auto_pro >> "$LOG_FILE" 2>&1 &
NEW_PID="$!"
echo "$NEW_PID" > "$PID_FILE"

sleep 1
if ! kill -0 "$NEW_PID" 2>/dev/null; then
  echo "后端启动失败，日志如下："
  tail -n 80 "$LOG_FILE"
  exit 1
fi

echo "后端已启动：PID=$NEW_PID PORT=$PORT"
echo "日志文件：$LOG_FILE"
