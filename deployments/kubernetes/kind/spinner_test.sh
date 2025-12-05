#!/usr/bin/env bash
# Spinner debug test

BOLD='\033[1m'
BLUE='\033[38;5;39m'
DIM='\033[2m'
NC='\033[0m'

SPINNER_PID=""
TEMP_MSG_FILE=$(mktemp)

cleanup() {
    if [ -n "$SPINNER_PID" ]; then kill "$SPINNER_PID" 2>/dev/null; fi
    rm -f "$TEMP_MSG_FILE"
    tput cnorm
}
trap cleanup EXIT

start_spinner() {
    local msg="$1"
    echo "$msg" > "$TEMP_MSG_FILE"
    
    tput civis
    (
        local frames=('⠋' '⠙' '⠹' '⠸' '⠼' '⠴' '⠦' '⠧' '⠇' '⠏')
        local i=0
        while true; do
            local current_msg=$(cat "$TEMP_MSG_FILE")
            printf "\r\033[2K${BLUE}${frames[i]}${NC} ${DIM}%s${NC}" "$current_msg"
            sleep 0.1
            i=$(( (i + 1) % ${#frames[@]} ))
        done
    ) &
    SPINNER_PID=$!
}

update_spinner() {
    echo "$1" > "$TEMP_MSG_FILE"
}

stop_spinner() {
    if [ -n "$SPINNER_PID" ]; then
        kill "$SPINNER_PID" 2>/dev/null
        wait "$SPINNER_PID" 2>/dev/null
        SPINNER_PID=""
        printf "\r\033[2K"
    fi
    tput cnorm
}

echo "Starting test..."
start_spinner "Initial message..."
sleep 2
update_spinner "Updated message (1/3)..."
sleep 2
update_spinner "Updated message (2/3)..."
sleep 2
stop_spinner
echo "Done."
