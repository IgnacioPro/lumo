#!/bin/bash

# Colors and Formatting
RESET='\033[0m'
BOLD='\033[1m'
GREEN='\033[32m'
RED='\033[31m'
BLUE='\033[34m'
YELLOW='\033[33m'

MESSAGE="$1"
shift
COMMAND="$@"

# Print the starting message
printf "${BLUE}⏳ %s...${RESET}" "$MESSAGE"

# Temp file to capture output (to check if any output was produced)
OUTPUT_FILE=$(mktemp)

# Use pipefail to get the exit code of the command, not tee/sed
set -o pipefail

# Execute command:
# 1. Redirect stderr to stdout (2>&1)
# 2. Tee output to temp file (so we can check if it was empty later)
# 3. Use awk to prepend a newline to the *first* line of output only.
#    This ensures that if output exists, it starts on a new line below "Running...".
#    If no output exists, awk prints nothing, preserving the "Running..." line.
eval "$COMMAND" 2>&1 | tee "$OUTPUT_FILE" | awk 'NR==1{print ""; print $0; next} {print}'

EXIT_CODE=$?

# If exit code is 0 (Success)
if [ $EXIT_CODE -eq 0 ]; then
    if [ -s "$OUTPUT_FILE" ]; then
        # Output was produced (and printed with a leading newline), so print Done on a new line
        printf "${GREEN}✅ %s${RESET}\n" "$MESSAGE"
    else
        # No output was produced, so overwrite the "Running..." line
        printf "\r\033[K${GREEN}✅ %s${RESET}\n" "$MESSAGE"
    fi
else
    # Failure
    if [ -s "$OUTPUT_FILE" ]; then
        # Output was produced, so failure message goes on new line
        printf "${RED}❌ %s failed${RESET}\n" "$MESSAGE"
    else
        # No output (silent failure?), overwrite line
        printf "\n${RED}❌ %s failed${RESET}\n" "$MESSAGE"
    fi
    rm -f "$OUTPUT_FILE"
    exit $EXIT_CODE
fi

# Cleanup
rm -f "$OUTPUT_FILE"
