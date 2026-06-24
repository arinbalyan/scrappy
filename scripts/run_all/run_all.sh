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
OUTFILE="$OUTDIR/results.json"
LOGFILE="$OUTDIR/run_all.log"
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

# Start fresh
rm -f "$LOGFILE"
echo "run_all started at $(date)" | tee -a "$LOGFILE"
echo "" | tee -a "$LOGFILE"

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

echo '[' > "$OUTFILE"
first=true
total=${#SITES[@]}
count=0

for site in "${SITES[@]}"; do
  count=$((count + 1))
  echo "" | tee -a "$LOGFILE"
  echo "[$count/$total] scraping $site ..." | tee -a "$LOGFILE"
  site_out="$TMPDIR/$site.json"
  site_err="$TMPDIR/$site.err"

  set +e
  timeout 60 "$BINARY" \
    --non-interactive \
    --search "software engineer" \
    --sites "$site" \
    --results-wanted 100 \
    --format jsonl \
    --out "$site_out" \
    --log-level INFO \
    2>"$site_err"
  exit_code=$?
  set -e

  # Append this site's stderr logs to the combined log
  if [ -s "$site_err" ]; then
    cat "$site_err" >> "$LOGFILE"
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

  echo "  status=$status jobs=$jobs exit=$exit_code" | tee -a "$LOGFILE"

  if [ "$first" = true ]; then
    first=false
  else
    echo "," >> "$OUTFILE"
  fi

  cat >> "$OUTFILE" <<RESULT
  {
    "site": "$site",
    "status": "$status",
    "jobs": $jobs,
    "fields": "$fields",
    "exit_code": $exit_code
  }
RESULT
done

echo ']' >> "$OUTFILE"
echo "" | tee -a "$LOGFILE"
echo "Done — $(date)" | tee -a "$LOGFILE"
echo "Results: $OUTFILE" | tee -a "$LOGFILE"
echo "Logs:    $LOGFILE" | tee -a "$LOGFILE"
jq '.' "$OUTFILE" > /dev/null 2>&1 && echo "Valid JSON ✓" | tee -a "$LOGFILE" || echo "Invalid JSON ✗" | tee -a "$LOGFILE"
