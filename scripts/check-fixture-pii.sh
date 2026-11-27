#!/bin/sh
# Offline PII check for committed fixture files.
set -eu

FIXTURE_DIR="tests/fixtures"
SELF_TEST=0

if [ "${1:-}" = "--self-test" ]; then
    SELF_TEST=1
fi

fail() {
    printf 'fixture PII check failed: %s\n' "$1" >&2
    exit 1
}

# Report the fixture and match for each personal-data pattern.
check_file() {
    file="$1"
    if grep -n -E -i "$EMAIL_PATTERN" "$file" >"$MATCHES"; then
        report "$file" 'email address' "$MATCHES" || return 1
    fi
    if grep -n -E -i "$PHONE_PATTERN" "$file" >"$MATCHES"; then
        report "$file" 'phone number' "$MATCHES" || return 1
    fi
    # Known placeholder words are synthetic and stay allowed in fixtures.
    grep -n -E -i "$CREDENTIAL_PATTERN" "$file" >"$MATCHES" || true
    grep -v -E -i "(example|sample|synthetic|placeholder|test|fake)[-_]?(token|key|secret|value|password)" "$MATCHES" >"$MATCHES.filtered" || true
    if [ -s "$MATCHES.filtered" ]; then
        cp "$MATCHES.filtered" "$MATCHES"
        report "$file" 'credential value' "$MATCHES" || return 1
    fi
    if grep -n -E -i "$TOKEN_PATTERN" "$file" >"$MATCHES"; then
        report "$file" 'long token' "$MATCHES" || return 1
    fi
}

report() {
    file="$1"
    label="$2"
    matches="$3"
    printf 'fixture: %s\n' "$file" >&2
    printf 'fixture PII check failed: %s contains %s\n' "$file" "$label" >&2
    sed -n '1,3p' "$matches" >&2
    return 1
}

EMAIL_PATTERN='[[:alnum:]._%+-]+@[[:alnum:].-]+\.[[:alpha:]]{2,}'
# Phone-shaped runs with separators; ISO dates and 13-digit epochs are excluded.
PHONE_PATTERN='(\+[0-9]{1,3}[ .-]?)?\(?[0-9]{3}\)?[ .-][0-9]{3}[ .-][0-9]{4}([^0-9]|$)|\+[0-9]{9,15}([^0-9]|$)'
CREDENTIAL_PATTERN='(bearer|authorization|api[-_ ]?key|apikey|password|passwd|secret|cookie)[[:space:]]+[a-z0-9_-]{8,}'
TOKEN_PATTERN='[[:alnum:]_-]*[A-Z][[:alnum:]_-]*[0-9][[:alnum:]_-]*[A-Z][[:alnum:]_-]{19,}|[0-9][[:alnum:]_-]*[A-Z][[:alnum:]_-]*[0-9][[:alnum:]_-]{19,}'

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT HUP INT TERM
MATCHES="$work/matches"

if [ "$SELF_TEST" -eq 1 ]; then
    FIXTURE_DIR="$work/fixtures"
    mkdir -p "$FIXTURE_DIR/clean" "$FIXTURE_DIR/leaked"
    printf '{"title":"Backend Engineer","url":"https://jobs.example.test/1"}\n' \
        >"$FIXTURE_DIR/clean/response.body"
    printf '{"date":"2026-11-26","id":1764067200}\n' \
        >"$FIXTURE_DIR/clean/frozen.json"
    printf 'contact jane.doe@example.test at +1 555 010 0199\n' \
        >"$FIXTURE_DIR/leaked/response.body"
    printf '{"authorization":"Bearer LIVESECRETVALUE123"}\n' \
        >"$FIXTURE_DIR/leaked/request.json"

    check_file "$FIXTURE_DIR/clean/response.body"
    check_file "$FIXTURE_DIR/clean/frozen.json"
    status=0
    check_file "$FIXTURE_DIR/leaked/response.body" 2>"$work/error" || status=$?
    if [ "$status" -eq 0 ]; then
        fail 'self-test did not detect injected personal data'
    fi
    grep -F -q 'jane.doe@example.test' "$work/error" ||
        fail 'self-test did not identify the offending fixture and match'
    status=0
    check_file "$FIXTURE_DIR/leaked/request.json" 2>"$work/credential-error" || status=$?
    if [ "$status" -eq 0 ]; then
        fail 'self-test did not detect an injected credential value'
    fi
    grep -F -q 'LIVESECRETVALUE123' "$work/credential-error" ||
        fail 'self-test did not identify the injected credential'
    printf 'self-test passed: injected PII detected and identified\n'
    exit 0
fi

[ -d "$FIXTURE_DIR" ] || fail "missing fixture directory: $FIXTURE_DIR"

find "$FIXTURE_DIR" -type f | sort |
while IFS= read -r file; do
    check_file "$file"
done

printf 'fixture PII check passed\n'
