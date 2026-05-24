#!/usr/bin/env bash
set -euo pipefail

# =============================================================================
# scrappy — Site Health & Relevancy Checker
#
# Runs each site individually with a 5-minute timeout, searches for
# "software engineer", captures output/errors, and generates a report
# showing which sites work, which fail, and whether the returned jobs
# are relevant to the search query.
#
# Usage:
#   ./scripts/site-check.sh                  # test all 66 sites
#   ./scripts/site-check.sh --site indeed    # test a single site
#   ./scripts/site-check.sh --verbose        # show per-site progress live
#
# Report output: testruns/site-check-{YYYYMMDD-HHMMSS}/
# =============================================================================

REPORT_DIR="testruns/site-check-$(date +%Y%m%d-%H%M%S)"
RESULTS_WANTED=5
TIMEOUT_SECONDS=300  # 5 minutes per site
SEARCH_TERM="software engineer"
VERBOSE=false
SINGLE_SITE=""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

# Parse args
while [[ $# -gt 0 ]]; do
  case "$1" in
    --site|--site=*)
      if [[ "$1" == --site=* ]]; then SINGLE_SITE="${1#*=}"; else SINGLE_SITE="$2"; shift; fi
      shift
      ;;
    --verbose) VERBOSE=true; shift ;;
    --help|-h)
      grep "^#" "$0" | head -20 | sed 's/^# //; s/^#$//'
      exit 0
      ;;
    *) echo "Unknown option: $1"; exit 1 ;;
  esac
done

# ---------- All 66 sites ----------
ALL_SITES=(
  linkedin indeed zip_recruiter bayt bdjobs naukri internshala
  builtin startupjobs greenhouse gunio himalayas hiringcafe
  huggingfacejobs jobindex remoteok remotive remotefirstjobs
  jobspresso hasjob vuejs larajobs arbeitnow arbeitsagentur
  hackernews cryptocurrencyjobs androidjobs jobicy devopsjobs
  crunchboard iosdevjobs swissdevjobs cryptojobslist devitjobs
  dribbble aijobs workingnomads wuzzuf ycjobs ukvisajobs google
  glassdoor adzuna simplyhired careerbuilder careerjet jooble
  dice monster stepstone infojobs reed themuse jobsdb snagajob
  djinni headhunter mycareersfuture jobstreet upwork 4dayweek
  academiccareers eurojobs findwork web3career glassdoor
)

# Relevancy keywords for tech/software jobs (lowercase match on job title)
RELEVANCY_KEYWORDS=(
  engineer developer software architect devops backend frontend
  fullstack full-stack sre platform infrastructure cloud ai ml
  machine.learning data.scientist data.engineer security systems
  analyst technical qa test automation site.reliability dev
  tech lead staff principal solutions integration database
  network devsecops bi.analyst bi.developer appian salesforce
  sap oracle aws azure gcp kubernetes docker terraform ansible
  linux unix scala kotlin swift golang rust react angular vue
  node nodejs typescript python java c++ c# dotnet php ruby
  blockchain web3 smart.contract solidity product.manager
  scrum.master agile program.manager it director vp cto cio
)

# ---------- Preliminaries ----------

build_binary() {
  echo -e "${CYAN}▸ Building scrappy binary...${NC}"
  go build -o /tmp/scrappy-sitecheck ./cmd/scrappy/
  echo -e "${GREEN}  ✓ Built /tmp/scrappy-sitecheck${NC}"
}

init_report() {
  mkdir -p "$REPORT_DIR/sites"
  SUMMARY="$REPORT_DIR/summary.txt"
  REPORT="$REPORT_DIR/report.md"
  JSON_ALL="$REPORT_DIR/all.jsonl"

  {
    echo "# scrappy Site Health Report"
    echo "Date: $(date '+%Y-%m-%d %H:%M:%S')"
    echo "Search term: \`$SEARCH_TERM\`"
    echo "Results wanted per site: $RESULTS_WANTED"
    echo "Timeout: ${TIMEOUT_SECONDS}s per site"
    echo ""
    echo "## Summary"
    echo ""
    echo "| Status | Count |"
    echo "|--------|------:|"
  } > "$REPORT"
}

# ---------- Relevancy checking ----------

check_relevancy() {
  local site="$1" jsonl="$2"
  local total=0 relevant=0
  local titles=()

  while IFS= read -r line; do
    title=$(echo "$line" | python3 -c "
import sys, json
try:
    d = json.loads(sys.stdin.read())
    print(d.get('title', '') or '')
except: pass
" 2>/dev/null || true)
    if [[ -z "$title" ]]; then continue; fi
    total=$((total + 1))
    titles+=("$title")
    title_lower=$(echo "$title" | tr '[:upper:]' '[:lower:]')
    matched=false
    for kw in "${RELEVANCY_KEYWORDS[@]}"; do
      # Convert keyword pattern to regex-safe
      kw_regex=$(echo "$kw" | sed 's/\./\\./g')
      if echo "$title_lower" | grep -Eq "$kw_regex"; then
        relevant=$((relevant + 1))
        matched=true
        break
      fi
    done
  done < <(grep -c . "$jsonl" > /dev/null 2>&1 && cat "$jsonl" || true)

  # Save titles for report
  if [[ ${#titles[@]} -gt 0 ]]; then
    printf '%s\n' "${titles[@]}" > "$REPORT_DIR/sites/${site}_titles.txt"
  fi

  echo "$total|$relevant"
}

# ---------- Run a single site ----------

run_site() {
  local site="$1"
  local site_out="$REPORT_DIR/sites/${site}.jsonl"
  local site_err="$REPORT_DIR/sites/${site}.err"
  local start elapsed exit_code

  if $VERBOSE; then
    echo -e "  ${CYAN}${site}${NC} ... " | tr -d '\n'
  fi

  start=$(date +%s)

  set +e
  timeout "$TIMEOUT_SECONDS" /tmp/scrappy-sitecheck scrape \
    --sites "$site" \
    --search "$SEARCH_TERM" \
    --results-wanted "$RESULTS_WANTED" \
    --format jsonl \
    --out "$site_out" \
    > /dev/null 2>"$site_err"
  exit_code=$?
  set -e

  elapsed=$(( $(date +%s) - start ))

  echo "$site|$exit_code|$elapsed" >> "$SUMMARY"
}

# ---------- Generate report ----------

generate_report() {
  local total_sites=0 working=0 partial=0 failing=0 timeout_count=0

  {
    echo ""
    echo "## Per-Site Results"
    echo ""
    echo "| Site | Status | Jobs | Relevant | Score | Time | Notes |"
    echo "|------|--------|------|----------|-------|------|-------|"
  } >> "$REPORT"

  while IFS='|' read -r site exit_code elapsed; do
    [[ -z "$site" ]] && continue
    total_sites=$((total_sites + 1))

    local jsonl="$REPORT_DIR/sites/${site}.jsonl"
    local err_file="$REPORT_DIR/sites/${site}.err"
    local err_msg=""

    # Determine status
    if [[ "$exit_code" -eq 124 ]]; then
      status="TIMEOUT"
      status_icon="⏰"
      timeout_count=$((timeout_count + 1))
      failing=$((failing + 1))
      err_msg="Timed out after ${TIMEOUT_SECONDS}s"
    elif [[ "$exit_code" -ne 0 ]]; then
      status="FAIL"
      status_icon="❌"
      failing=$((failing + 1))
      err_msg=$(head -5 "$err_file" 2>/dev/null | tr '\n' '; ' | head -c 200)
    else
      # Check if we got results
      if [[ ! -f "$jsonl" ]]; then
        status="NO_DATA"
        status_icon="⚠️"
        partial=$((partial + 1))
        err_msg="No output file produced"
      else
        local job_count
        job_count=$(wc -l < "$jsonl" 2>/dev/null || echo 0)
        if [[ "$job_count" -eq 0 ]]; then
          status="EMPTY"
          status_icon="⚠️"
          partial=$((partial + 1))
          err_msg=$(head -3 "$err_file" 2>/dev/null | tr '\n' '; ' | head -c 200)
          if [[ -z "$err_msg" ]]; then err_msg="No jobs returned (0 results)"; fi
        else
          working=$((working + 1))
          # Relevancy
          local rel_result
          rel_result=$(check_relevancy "$site" "$jsonl")
          local total_jobs relevant_jobs
          total_jobs=$(echo "$rel_result" | cut -d'|' -f1)
          relevant_jobs=$(echo "$rel_result" | cut -d'|' -f2)
          local score=0
          if [[ "$total_jobs" -gt 0 ]]; then
            score=$(( relevant_jobs * 100 / total_jobs ))
          fi
          if [[ "$score" -ge 80 ]]; then
            status_icon="✅"
            status="GOOD"
          elif [[ "$score" -ge 50 ]]; then
            status_icon="🔶"
            status="MIXED"
          else
            status_icon="🔴"
            status="OFF_TOPIC"
          fi
          echo "| $site | $status_icon $status | $total_jobs/$RESULTS_WANTED | ${relevant_jobs}/${total_jobs} | ${score}% | ${elapsed}s |" >> "$REPORT"
          # Skip to next site
          [[ "$status" != "TIMEOUT" && "$status" != "FAIL" && "$status" != "NO_DATA" && "$status" != "EMPTY" ]] && continue
        fi
      fi
    fi

    # Write row for non-working sites
    local note="${err_msg:-see error log}"
    echo "| $site | $status_icon $status | — | — | — | ${elapsed}s | ${note:0:80} |" >> "$REPORT"
  done < <(tail -n +1 "$SUMMARY" 2>/dev/null; echo)

  # Append error details section
  {
    echo ""
    echo "---"
    echo ""
    echo "## Error Details"
    echo ""
  } >> "$REPORT"

  while IFS='|' read -r site exit_code elapsed; do
    [[ -z "$site" ]] && continue
    local err_file="$REPORT_DIR/sites/${site}.err"
    if [[ -s "$err_file" ]]; then
      echo "### $site (exit $exit_code, ${elapsed}s)" >> "$REPORT"
      echo '```' >> "$REPORT"
      head -20 "$err_file" >> "$REPORT"
      echo '```' >> "$REPORT"
      echo "" >> "$REPORT"
    fi
  done < <(tail -n +1 "$SUMMARY" 2>/dev/null; echo)

  # Update summary table at top
  {
    echo "| ✅ Working | $working |"
    echo "| ⚠️ Partial/Empty | $partial |"
    echo "| ❌ Failing | $failing |"
    echo "| ⏰ Timeout | $timeout_count |"
    echo "| **Total** | **$total_sites** |"
    echo ""
    echo "### Legend"
    echo "- **GOOD** = ≥80% of job titles contain tech keywords"
    echo "- **MIXED** = 50-79% relevant"
    echo "- **OFF_TOPIC** = <50% relevant (many non-tech jobs returned)"
    echo "- **EMPTY** = Site returned 0 jobs"
    echo "- **FAIL** = Site returned an error"
    echo "- **TIMEOUT** = Site did not respond within ${TIMEOUTS_SECONDS}s"
    echo "- **NO_DATA** = No output file was created"
  } > /tmp/summary_header.txt

  # Insert header at top of report (after h1/h2 lines)
  local temp_report="$REPORT.tmp"
  head -6 "$REPORT" > "$temp_report"
  cat /tmp/summary_header.txt >> "$temp_report"
  local total_lines
  total_lines=$(wc -l < "$REPORT")
  tail -n $((total_lines - 6)) "$REPORT" >> "$temp_report"
  mv "$temp_report" "$REPORT"

  echo ""
  echo -e "${GREEN}✓ Report written to:${NC} $REPORT"
  echo ""
  echo -e "${BOLD}Quick numbers:${NC}"
  echo -e "  ${GREEN}Working:${NC} $working  ${YELLOW}Partial:${NC} $partial  ${RED}Failing:${NC} $failing  ${YELLOW}Timeout:${NC} $timeout_count"
}

# ---------- Main ----------

echo ""
echo -e "${BOLD}╔═══════════════════════════════════════════╗${NC}"
echo -e "${BOLD}║   scrappy — Site Health & Relevancy Check ${NC}"
echo -e "${BOLD}╚═══════════════════════════════════════════╝${NC}"
echo ""

build_binary
init_report

SITES=("${ALL_SITES[@]}")
if [[ -n "$SINGLE_SITE" ]]; then
  SITES=("$SINGLE_SITE")
  echo -e "${YELLOW}▸ Testing single site: ${SINGLE_SITE}${NC}"
else
  echo -e "${YELLOW}▸ Testing all ${#SITES[@]} sites${NC}"
fi
echo -e "${YELLOW}▸ Report directory: ${REPORT_DIR}${NC}"
echo -e "${YELLOW}▸ Timeout: ${TIMEOUT_SECONDS}s per site${NC}"
echo ""

for site in "${SITES[@]}"; do
  run_site "$site"
done

echo ""
echo -e "${CYAN}▸ Generating report...${NC}"
generate_report

# Final summary
echo ""
echo -e "${GREEN}═══════════════════════════════════════════${NC}"
echo -e "${GREEN}  Report ready: ${BOLD}$REPORT${NC}"
echo -e "${GREEN}  Raw data:     ${BOLD}$REPORT_DIR/sites/${NC}"
echo -e "${GREEN}═══════════════════════════════════════════${NC}"
