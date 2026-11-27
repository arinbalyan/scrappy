#!/bin/sh
# Offline secret-leak check for scrappy commands and artifacts.
set -eu

BINARY="${1:-target/debug/scrappy}"

SECRETS_FILE=""

fail() {
    printf 'secret leak check failed: %s\n' "$1" >&2
    exit 1
}

# Report the first synthetic secret found in one captured surface.
check_surface() {
    surface="$1"
    path="$2"
    [ -f "$path" ] || {
        printf 'secret leak check failed: missing surface: %s\n' "$path" >&2
        return 1
    }
    while IFS= read -r secret; do
        if grep -F -q "$secret" "$path"; then
            printf 'artifact: %s\n' "$path" >&2
            printf 'secret leak check failed: %s contains synthetic secret: %s\n' \
                "$surface" "$secret" >&2
            return 1
        fi
    done <"$SECRETS_FILE"
}

if [ "${1:-}" = "--self-test" ]; then
    work="$(mktemp -d)"
    trap 'rm -rf "$work"' EXIT HUP INT TERM
    SECRETS_FILE="$work/secrets"
    printf 'LEAK-SELFTEST-TOKEN\n' >"$SECRETS_FILE"
    printf 'safe text\n' >"$work/clean"
    printf 'safe text with LEAK-SELFTEST-TOKEN\n' >"$work/leaked"
    check_surface 'clean surface' "$work/clean"
    set +e
    check_surface 'injected leak' "$work/leaked" 2>"$work/error"
    status=$?
    set -e
    if [ "$status" -eq 0 ]; then
        fail 'self-test did not detect the injected leak'
    fi
    grep -F -q 'LEAK-SELFTEST-TOKEN' "$work/error" ||
        fail 'self-test did not identify the leaked secret'
    printf 'self-test passed: injected leak detected and identified\n'
    exit 0
fi

[ -x "$BINARY" ] || fail "binary is not executable: $BINARY"

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT HUP INT TERM
SECRETS_FILE="$work/secrets"

# Distinct markers for each SCRAPPY_ config field and the challenge token.
SCRAPPY_SEARCH='LEAK-SEARCH-8f7a31c9d2'
SCRAPPY_SOURCES='LEAK-SOURCES-6b4e02f7a1'
SCRAPPY_FORMAT='LEAK-FORMAT-3c9d85e1f4'
SCRAPPY_RATES='LEAK-RATES-1e6a7b92d8'
SCRAPPY_CONCURRENCY='LEAK-CONCURRENCY-9a2f4c6e10'
SCRAPPY_TIMEOUT='LEAK-TIMEOUT-7d81b3e5c2'
SCRAPPY_MEMORY_MB='LEAK-MEMORY-2f5c9a7b41'
SCRAPPY_LOG_LEVEL='LEAK-LOGLEVEL-4b8e1d6a93'
SCRAPPY_IS_REMOTE='LEAK-ISREMOTE-8c3f7a1d52'
SCRAPPY_JOB_TYPE='LEAK-JOBTYPE-5e9b2c8f17'
SCRAPPY_MIN_SCORE='LEAK-MINSCORE-6d1a4e9b38'
SCRAPPY_HOURS_OLD='LEAK-HOURSOLD-1c7f5b2a94'
SCRAPPY_DATE='LEAK-DATE-9b3e6a2d75'
CHALLENGE_TOKEN='LEAK-CHALLENGE-TOKEN-7a4c9e2f61'

printf '%s\n' \
    "$SCRAPPY_SEARCH" \
    "$SCRAPPY_SOURCES" \
    "$SCRAPPY_FORMAT" \
    "$SCRAPPY_RATES" \
    "$SCRAPPY_CONCURRENCY" \
    "$SCRAPPY_TIMEOUT" \
    "$SCRAPPY_MEMORY_MB" \
    "$SCRAPPY_LOG_LEVEL" \
    "$SCRAPPY_IS_REMOTE" \
    "$SCRAPPY_JOB_TYPE" \
    "$SCRAPPY_MIN_SCORE" \
    "$SCRAPPY_HOURS_OLD" \
    "$SCRAPPY_DATE" \
    "$CHALLENGE_TOKEN" >"$SECRETS_FILE"

# Run a binary command with every synthetic secret and check both streams.
run_command() {
    name="$1"
    shift
    stdout="$work/$name.stdout"
    stderr="$work/$name.stderr"
    set +e
    env \
        SCRAPPY_SEARCH="$SCRAPPY_SEARCH" \
        SCRAPPY_SOURCES="$SCRAPPY_SOURCES" \
        SCRAPPY_FORMAT="$SCRAPPY_FORMAT" \
        SCRAPPY_RATES="$SCRAPPY_RATES" \
        SCRAPPY_CONCURRENCY="$SCRAPPY_CONCURRENCY" \
        SCRAPPY_TIMEOUT="$SCRAPPY_TIMEOUT" \
        SCRAPPY_MEMORY_MB="$SCRAPPY_MEMORY_MB" \
        SCRAPPY_LOG_LEVEL="$SCRAPPY_LOG_LEVEL" \
        SCRAPPY_IS_REMOTE="$SCRAPPY_IS_REMOTE" \
        SCRAPPY_JOB_TYPE="$SCRAPPY_JOB_TYPE" \
        SCRAPPY_MIN_SCORE="$SCRAPPY_MIN_SCORE" \
        SCRAPPY_HOURS_OLD="$SCRAPPY_HOURS_OLD" \
        SCRAPPY_DATE="$SCRAPPY_DATE" \
        CHALLENGE_TOKEN="$CHALLENGE_TOKEN" \
    "$BINARY" "$@" >"$stdout" 2>"$stderr"
    status=$?
    set -e
    check_surface "$name stdout" "$stdout"
    check_surface "$name stderr" "$stderr"
    printf 'checked %s (exit %s)\n' "$name" "$status"
}

config_path="$work/config.toml"
run_dir="$work/run"
mkdir -p "$run_dir"
printf 'format = "ndjson"\n' >"$config_path"

run_command version version
run_command validate validate
run_command doctor doctor --format json
run_command list-sources list-sources --format json
run_command run run --source remoteok --format ndjson
run_command config-init config init --path "$config_path"
run_command config-show config show --path "$config_path"
run_command config-explain config explain --key format

# stdin config supplies a synthetic challenge token; main only echoes a status.
printf '{"search":"%s"}\n' "$CHALLENGE_TOKEN" >"$work/stdin.json"
set +e
env \
    SCRAPPY_SEARCH="$SCRAPPY_SEARCH" \
    SCRAPPY_SOURCES="$SCRAPPY_SOURCES" \
    SCRAPPY_FORMAT="$SCRAPPY_FORMAT" \
    SCRAPPY_RATES="$SCRAPPY_RATES" \
    SCRAPPY_CONCURRENCY="$SCRAPPY_CONCURRENCY" \
    SCRAPPY_TIMEOUT="$SCRAPPY_TIMEOUT" \
    SCRAPPY_MEMORY_MB="$SCRAPPY_MEMORY_MB" \
    SCRAPPY_LOG_LEVEL="$SCRAPPY_LOG_LEVEL" \
    SCRAPPY_IS_REMOTE="$SCRAPPY_IS_REMOTE" \
    SCRAPPY_JOB_TYPE="$SCRAPPY_JOB_TYPE" \
    SCRAPPY_MIN_SCORE="$SCRAPPY_MIN_SCORE" \
    SCRAPPY_HOURS_OLD="$SCRAPPY_HOURS_OLD" \
    SCRAPPY_DATE="$SCRAPPY_DATE" \
    CHALLENGE_TOKEN="$CHALLENGE_TOKEN" \
    "$BINARY" run --stdin-config <"$work/stdin.json" >"$work/stdin.stdout" 2>"$work/stdin.stderr"
set -e
check_surface 'stdin stdout' "$work/stdin.stdout"
check_surface 'stdin stderr' "$work/stdin.stderr"

# Check every config and checkpoint file left behind by the commands.
find "$run_dir" "$work" -type f \
    \( -name 'config.toml' -o -name '.config.toml.tmp-*' -o -name 'checkpoint.json' \) \
    >"$work/artifacts"
while IFS= read -r artifact; do
    check_surface "artifact $artifact" "$artifact"
done <"$work/artifacts"

printf 'secret leak check passed\n'
