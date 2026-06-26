#!/usr/bin/env bash
set -euo pipefail

FILTER_SITE="${1:-}"

cd "$(dirname "$0")/../.."

BINARY="./scrappy"
if [ ! -x "$BINARY" ]; then
  echo "Building scrappy first..."
  go build -o "$BINARY" ./cmd/scrappy
fi

OUTDIR="scripts/run_all"
mkdir -p "$OUTDIR"
RESULT_JSON="$OUTDIR/results.json"
LOGFILE="$OUTDIR/run_all.log"
TMPDIR=$(mktemp -d)
RESDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR" "$RESDIR"' EXIT

rm -f "$LOGFILE" "$RESULT_JSON"

SITES=(
  linkedin indeed internshala builtin startupjobs greenhouse gunio himalayas
  hiringcafe huggingfacejobs jobindex remoteok remotive remotefirstjobs jobspresso
  hasjob vuejobs larajobs arbeitnow hackernews cryptocurrencyjobs androidjobs
  jobicy devopsjobs crunchboard cryptojobslist aijobs workingnomads ycjobs
  ukvisajobs google adzuna simplyhired careerbuilder careerjet dice monster
  infojobs reed themuse jobsdb snagajob djinni headhunter mycareersfuture
  jobstreet 4dayweek eurojobs findwork arbeitsagentur web3career echojobs
  jobtechdev authenticjobs ecojobs golangjobs landingjobs realworkfromanywhere
  bayt berlinstartupjobs canadajobbank careeronestop conservationjobs coroflot
  devitjobs drupaljobs higheredjobs icrunchdata jobdataapi jobsch jobsinjapan
  joinrise exa fossjobs francetravail freelancercom functionalworks
  germantechjobs getonboard greenjobsboard guardianjobs nofluffjobs
  opensourcedesignjobs powertofly pyjobs pythonjobs railsjobs stepstone
  swissdevjobs talroo ats-adp ats-ashby ats-avature ats-bamboohr ats-breezyhr
  ats-bullhorn ats-comeet ats-crelate ats-deel ats-fountain ats-freshteam ats-gem
  ats-hiringthing ats-icims ats-ismartrecruit ats-jazzhr ats-jobscore ats-jobvite
  ats-jobylon ats-joincom ats-loxo ats-manatal ats-mercor ats-oracle ats-personio
  ats-phenom ats-pinpoint ats-recruitee ats-recruiterflow ats-rippling
  ats-smartrecruiters ats-successfactors ats-talentlyft ats-taleo ats-teamtailor
  ats-trakstar ats-ukg ats-workable ats-workday techcareers tesla undpjobs upwork
  usajobs virtualvocations wellfound weworkremotely wordpressjobs ziprecruiter
  academiccareers wuzzuf
)

CONCURRENCY=16
total=${#SITES[@]}

echo "run_all started at $(date) — $total sites, $CONCURRENCY concurrent"
echo ""

# ── Worker Pool ──────────────────────────────────────────
running=0
pids=()
indices=()

for site in "${SITES[@]}"; do
  [ -n "$FILTER_SITE" ] && [ "$site" != "$FILTER_SITE" ] && continue

  # Wait if pool is full
  while [ "$running" -ge "$CONCURRENCY" ]; do
    wait -n 2>/dev/null || true
    running=$((running - 1))
  done

  running=$((running + 1))

  (
    site_out="$TMPDIR/$site.json"
    site_err="$TMPDIR/$site.err"
    start=$(date +%s%N)

    set +e
    timeout 120 "$BINARY" \
      --non-interactive \
      --search "software engineer" \
      --sites "$site" \
      --results-wanted 100 \
      --format jsonl \
      --out "$site_out" \
      --log-level ERROR \
      --verify-concurrency 0 \
      2>"$site_err"
    exit_code=$?
    set -e
    end=$(date +%s%N)

    elapsed_ms=$(( (end - start) / 1000000 ))
    [ "$elapsed_ms" -lt 1000 ] && elapsed_str="${elapsed_ms}ms" || elapsed_str="$(echo "scale=1; $elapsed_ms / 1000" | bc)s"

    status="success"
    [ $exit_code -eq 124 ] && status="timeout"
    [ $exit_code -ne 0 ] && [ $exit_code -ne 124 ] && status="failed"

    jobs=0
    if [ -f "$site_out" ] && [ -s "$site_out" ]; then
      jobs=$(wc -l < "$site_out")
    fi

    cat > "$RESDIR/$site" <<RES
status=$status
jobs=$jobs
elapsed=$elapsed_str
exit_code=$exit_code
RES

    if [ -s "$site_err" ]; then
      {
        echo ""
        echo "[$(date +%H:%M:%S)] scraping $site ..."
        cat "$site_err"
        echo "  status=$status jobs=$jobs elapsed=$elapsed_str exit=$exit_code"
      } >> "$LOGFILE"
    fi

    printf "\033[36m[%s]\033[0m %-25s \033[33m%-7s\033[0m jobs=%-4d %s\n" \
      "$(date +%H:%M:%S)" "$site" "$status" "$jobs" "$elapsed_str"
  ) &
  pids+=($!)

done

# Wait for all
for pid in "${pids[@]}"; do
  wait "$pid" 2>/dev/null || true
done

echo ""
echo "══════════════════════════════════════════════"
echo "  DONE — $(date)"
echo "══════════════════════════════════════════════"
echo ""

# ── Aggregate results ────────────────────────────────
echo '[' > "$RESULT_JSON"
first=true
SITES_WITH_JOBS=()
SITES_ZERO_JOBS=()
SITES_TIMEOUT=()
SITES_FAILED=()

for site in "${SITES[@]}"; do
  res="$RESDIR/$site"
  [ ! -f "$res" ] && continue
  status=$(grep '^status=' "$res" | cut -d= -f2-)
  jobs=$(grep '^jobs=' "$res" | cut -d= -f2-)
  elapsed=$(grep '^elapsed=' "$res" | cut -d= -f2-)
  exit_code=$(grep '^exit_code=' "$res" | cut -d= -f2-)

  [ -z "$exit_code" ] && exit_code=-1

  if [ "$jobs" -gt 0 ] 2>/dev/null; then
    SITES_WITH_JOBS+=("$site")
  elif [ "$status" = "timeout" ]; then
    SITES_TIMEOUT+=("$site")
  elif [ "$status" = "failed" ]; then
    SITES_FAILED+=("$site")
  else
    SITES_ZERO_JOBS+=("$site")
  fi

  [ "$first" = true ] && first=false || echo "," >> "$RESULT_JSON"
  cat >> "$RESULT_JSON" <<RES
  {"site":"$site","status":"$status","jobs":$jobs,"elapsed":"$elapsed","exit_code":$exit_code}
RES
done

echo ']' >> "$RESULT_JSON"

# ── Summary ────────────────────────────────────────────
echo "┌─────────────────────────────────────────────────────────┐"
printf "│ %-54s│\n" "Sites with jobs: ${#SITES_WITH_JOBS[@]} / $total"
printf "│ %-54s│\n" "Zero jobs: ${#SITES_ZERO_JOBS[@]}"
printf "│ %-54s│\n" "Timeout: ${#SITES_TIMEOUT[@]}"
printf "│ %-54s│\n" "Failed: ${#SITES_FAILED[@]}"
echo "└─────────────────────────────────────────────────────────┘"

[ "${#SITES_WITH_JOBS[@]}" -gt 0 ] && {
  echo ""
  echo "── Working ──"
  for site in "${SITES_WITH_JOBS[@]}"; do
    echo "$(grep '^jobs=' "$RESDIR/$site" | cut -d= -f2-) $site"
  done | sort -rn | while read -r j s; do printf "  %-25s %s\n" "$s" "$j"; done
}

[ "${#SITES_TIMEOUT[@]}" -gt 0 ] && { echo ""; echo "── Timeout ──"; printf '  %s\n' "${SITES_TIMEOUT[@]}" | sort; }

[ "${#SITES_ZERO_JOBS[@]}" -gt 0 ] && {
  echo ""
  echo "── Zero jobs ──"
  API_KEYS="adzuna careerjet infojobs findwork arbeitsagentur web3career jobtechdev authenticjobs exa francetravail talroo upwork indeed dice"
  for site in "${SITES_ZERO_JOBS[@]}"; do
    label=""
    for k in $API_KEYS; do
      [ "$site" = "$k" ] && label=" (needs API key)" && break
    done
    [[ "$site" == ats-* ]] && label=" (needs company seed)"
    echo "  $site$label"
  done | sort
}

echo ""
echo "Results: $RESULT_JSON"
echo "Logs:    $LOGFILE"
python3 -c "import json; json.load(open('$RESULT_JSON'))" 2>/dev/null && echo "Valid JSON ✓" || echo "Invalid JSON ✗"
