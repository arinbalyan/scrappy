#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/../.."

BINARY="./scrappy"
if [ ! -x "$BINARY" ]; then
  echo "Building scrappy first..."
  go build -o "$BINARY" ./cmd/scrappy
fi

OUTDIR="scripts/run_all"
mkdir -p "$OUTDIR"
JSON_TMP=$(mktemp)
RESULT_JSON="$OUTDIR/results.json"
LOGFILE="$OUTDIR/run_all.log"
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR" "$JSON_TMP"' EXIT

rm -f "$LOGFILE" "$RESULT_JSON"
exec > >(tee -a "$LOGFILE") 2>&1

echo "run_all started at $(date)"
echo ""

# ── Sites that need API keys (skip automatically, listed for reference) ──
# These will be tried but flagged in the summary

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

# Build JSON entries in memory (tmpfile), write atomically at end
echo '[' > "$JSON_TMP"
first=true
total=${#SITES[@]}
count=0
declare -A SITE_JOBS SITE_STATUS SITE_EXIT SITE_FIELDS SITE_ELAPSED SITE_METHOD

for site in "${SITES[@]}"; do
  count=$((count + 1))
  echo ""
  echo "[$count/$total] scraping $site ..."
  site_out="$TMPDIR/$site.json"
  site_err="$TMPDIR/$site.err"

  site_start=$(date +%s%N)
  set +e
  timeout 60 "$BINARY" \
    --non-interactive \
    --search "software engineer" \
    --sites "$site" \
    --results-wanted 100 \
    --format jsonl \
    --out "$site_out" \
    --log-level INFO \
    --verify-concurrency 0 \
    2>"$site_err"
  exit_code=$?
  site_end=$(date +%s%N)
  set -e

  # Append site stderr to combined log
  if [ -s "$site_err" ]; then
    cat "$site_err" >> "$LOGFILE"
  fi

  elapsed_ms=$(( (site_end - site_start) / 1000000 ))
  if [ "$elapsed_ms" -lt 1000 ]; then
    elapsed_str="${elapsed_ms}ms"
  else
    elapsed_str="$(echo "scale=1; $elapsed_ms / 1000" | bc)s"
  fi

  status="success"
  [ $exit_code -eq 124 ] && status="timeout"
  [ $exit_code -ne 0 ] && [ $exit_code -ne 124 ] && status="failed"

  jobs=0
  fields=""
  if [ -f "$site_out" ] && [ -s "$site_out" ]; then
    jobs=$(wc -l < "$site_out")
    fields=$(head -1 "$site_out" 2>/dev/null | jq -r 'keys | join(",")' 2>/dev/null || echo "")
  fi

  # Store for summary
  SITE_JOBS[$site]=$jobs
  SITE_STATUS[$site]=$status
  SITE_EXIT[$site]=$exit_code
  SITE_FIELDS[$site]=$fields
  SITE_ELAPSED[$site]=$elapsed_str

  echo "  status=$status jobs=$jobs elapsed=$elapsed_str exit=$exit_code"

  # Build JSON entry
  if [ "$first" = true ]; then
    first=false
  else
    echo "," >> "$JSON_TMP"
  fi

  cat >> "$JSON_TMP" <<RESULT
  {
    "site": "$site",
    "status": "$status",
    "jobs": $jobs,
    "fields": "$fields",
    "elapsed": "$elapsed_str",
    "exit_code": $exit_code
  }
RESULT
done

echo ']' >> "$JSON_TMP"
mv "$JSON_TMP" "$RESULT_JSON"

echo ""
echo "══════════════════════════════════════════════"
echo "  DONE — $(date)"
echo "══════════════════════════════════════════════"
echo ""

# ── Summary ────────────────────────────────────────────────────
SITES_WITH_JOBS=()
SITES_ZERO_JOBS=()
SITES_TIMEOUT=()
SITES_FAILED=()

for site in "${SITES[@]}"; do
  jobs=${SITE_JOBS[$site]:-0}
  status=${SITE_STATUS[$site]:-unknown}
  if [ "$jobs" -gt 0 ]; then
    SITES_WITH_JOBS+=("$site")
  elif [ "$status" = "timeout" ]; then
    SITES_TIMEOUT+=("$site")
  elif [ "$status" = "failed" ]; then
    SITES_FAILED+=("$site")
  else
    SITES_ZERO_JOBS+=("$site")
  fi
done

echo "┌─────────────────────────────────────────────────────────┐"
echo "│                    SUMMARY                              │"
echo "├─────────────────────────────────────────────────────────┤"
printf "│ %-25s %6d / %-3d                          │\n" "Sites with jobs"    "${#SITES_WITH_JOBS[@]}" "$total"
printf "│ %-25s %6d                                   │\n" "Zero jobs (success)" "${#SITES_ZERO_JOBS[@]}"
printf "│ %-25s %6d                                   │\n" "Timeout"             "${#SITES_TIMEOUT[@]}"
printf "│ %-25s %6d                                   │\n" "Failed"              "${#SITES_FAILED[@]}"
echo "└─────────────────────────────────────────────────────────┘"

if [ "${#SITES_WITH_JOBS[@]}" -gt 0 ]; then
  echo ""
  echo "── Working (sorted by job count) ──"
  printf "  %-25s %s\n" "SITE" "JOBS"
  for site in "${SITES_WITH_JOBS[@]}"; do
    printf "  %-25s %s\n" "$site" "${SITE_JOBS[$site]}"
  done | sort -k2 -rn
fi

if [ "${#SITES_TIMEOUT[@]}" -gt 0 ]; then
  echo ""
  echo "── Timeout ──"
  for site in "${SITES_TIMEOUT[@]}"; do
    echo "  $site"
  done | sort
fi

if [ "${#SITES_ZERO_JOBS[@]}" -gt 0 ]; then
  echo ""
  echo "── Zero jobs (may need API keys or company seeds) ──"
  # Known API-key sites
  API_KEYS="adzuna careerjet infojobs findwork arbeitsagentur web3career jobtechdev authenticjobs exa francetravail talroo upwork indeed dice"
  for site in "${SITES_ZERO_JOBS[@]}"; do
    label=""
    for k in $API_KEYS; do
      [ "$site" = "$k" ] && label=" (needs API key)" && break
    done
    [[ "$site" == ats-* ]] && label=" (needs company seed env var)"
    echo "  $site$label"
  done | sort
fi

echo ""
echo "Results: $RESULT_JSON"
echo "Logs:    $LOGFILE"
# Validate
python3 -c "import json; json.load(open('$RESULT_JSON'))" 2>/dev/null && echo "Valid JSON ✓" || echo "Invalid JSON ✗"
