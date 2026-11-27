# Email extraction

Email extraction runs in src/engine/pipeline.rs before a job is emitted.
observed_emails scans the job description for addresses and tags each one
with Description source and Observed provenance. Duplicates keep the first
address. Guessed addresses and commit-harvested addresses are not emitted.
src/output/record.rs carries the provenance into the final NDJSON record.
