# Needs Company Slug

These ATS (Applicant Tracking System) scrapers need company seed slugs to search for jobs. They use either `SCRAPPY_{PROVIDER}_SEEDS` env vars or `config/company_slugs.toml`.

## How ATS resolution works

1. Check `SCRAPPY_{PROVIDER}_SEEDS` env var (if set → use comma-separated slugs)
2. Check `config/company_slugs.toml` (embedded in binary, override with local file)
3. Fall back to `--search` term as a single company slug

## Populated (slugs exist in embedded file)

These providers have company slugs embedded in the binary. Set the env var OR let it fall through to the embedded file:

| Site | Slugs | Notes |
|------|-------|-------|
| ats-ashby | 566 | Includes OpenAI, Stripe, Notion, Vercel, etc. |
| greenhouse | 147 | Includes Stripe, Airbnb, Anthropic, MongoDB, etc. |
| ats-jazzhr | 21 | Includes Datadog, MongoDB, Cisco, etc. |
| ats-crelate | 20 | Staffing agencies |
| ats-deel | 20 | Deel, OpenAI, Anthropic, Stripe, etc. |
| ats-gem | 20 | Gem, Datadog, MongoDB, etc. |
| ats-hiringthing | 20 | Amazon, Google, Meta, Apple, etc. |
| ats-jobvite | 20 | LinkedIn, Twitter, Square, etc. |
| ats-loxo | 19 | Vercel, Stripe, Anthropic, OpenAI, etc. |
| ats-mercor | 20 | Mercor, OpenAI, Anthropic, Scale AI, etc. |
| ats-oracle | 20 | Oracle, FedEx, Marriott, etc. |
| ats-pinpoint | 20 | Vercel, Linear, Notion, Figma, etc. |
| ats-recruiterflow | 20 | Bain, McKinsey, BCG, Deloitte, etc. |
| ats-smartrecruiters | 9 | Palantir, Uber, ServiceNow, etc. |
| ats-successfactors | 15 | SAP, Siemens, Bosch, Bayer, etc. |
| ats-taleo | 15 | Amazon, Target, Walmart, etc. |
| ats-trakstar | 13 | Automattic, WordPress, Drift, etc. |
| ats-bamboohr | 8 | Palantir, Asana, OpenAI, Docker, etc. |
| ats-personio | 6 | Delivery Hero, Flixbus, N26, etc. |
| ats-breezyhr | 5 | Gusto, Roblox, Checkr, etc. |
| ats-icims | 3 | Costco, Quest, Vanguard |
| ats-bullhorn | 20 | Staffing agencies (aerotek, kforce, etc.) |
| ats-ismartrecruit | 20 | Healthcare companies |

## Empty (no slugs — will return 0 unless env var is set)

These providers have no company slugs in the embedded file. Set `SCRAPPY_{PROVIDER}_SEEDS` to use them:

| Site |
|------|
| ats-adp |
| ats-avature |
| ats-comeet |
| ats-fountain |
| ats-freshteam |
| ats-jobscore |
| ats-jobylon |
| ats-joincom |
| ats-manatal |
| ats-phenom |
| ats-rippling |
| ats-talentlyft |
| ats-teamtailor |
| ats-ukg |
| ats-workable |
| ats-workday |

## How to add slugs

1. Edit `config/company_slugs.toml` (local override, gitignored)
2. Or edit `internal/scraper/ats/company_slugs.toml` (embedded in binary, tracked)
3. Format: `provider_name = ["slug1", "slug2", "slug3"]`
