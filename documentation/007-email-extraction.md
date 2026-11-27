# Email extraction

Email extraction runs in src/engine/pipeline.rs before a job is emitted.
observed_emails scans the job description for addresses and tags each one
with Description source and Observed provenance. Duplicates keep the first
address. Guessed addresses and commit-harvested addresses are not emitted by
default. Setting `email_discovery = true`, or
`SCRAPPY_EMAIL_DISCOVERY=true`, allows inferred addresses from a dedicated
source manifest into the output. Each inferred address keeps provenance
`Inferred` and source `Guessed`. src/output/record.rs carries the provenance
into the final NDJSON record.
