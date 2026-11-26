# Spec Delta

## Purpose

Defines the command-line surface, layered configuration resolution, validation, output stream separation, and diagnostics that make the scraper binary predictable for scripts and operators.

## ADDED Requirements

### Requirement: Explicit subcommands
The binary SHALL expose `run`, `validate`, `config`, `list-sources`, `doctor`, `completion`, and `version` subcommands. The `config` subcommand SHALL provide `init`, `show`, `set`, `unset`, `edit`, `explain`, and `schema` actions. The `doctor` subcommand SHALL provide a `fix` action.

#### Scenario: Help lists the full surface
- **WHEN** a user runs the binary with --help
- **THEN** every supported subcommand and global flag is shown with a short description and an example

#### Scenario: Version is available everywhere
- **WHEN** a user runs `scrappy --version` or `scrappy run --version`
- **THEN** the binary prints the version and exits zero without loading config

#### Scenario: Subcommand help is independent of config
- **WHEN** a user runs `scrappy <subcommand> --help` with a broken config file present
- **THEN** help is printed and no config is loaded

#### Scenario: Unknown subcommand fails
- **WHEN** a user supplies an unrecognized subcommand
- **THEN** the binary exits 2 and prints valid subcommands to stderr

#### Scenario: Shell completion is generated
- **WHEN** a user requests completion for a supported shell
- **THEN** the binary prints a completion script for that shell and an unsupported shell exits 2

### Requirement: Layered configuration precedence
Every configurable field SHALL be resolved in this order: explicit CLI value, process environment, project config file, user config file, built-in default. Explicit files given by the user SHALL be layered, and a hermetic flag SHALL disable default config discovery. No environment value SHALL be used as a CLI flag default.

#### Scenario: Explicit CLI value wins
- **WHEN** a user sets a flag that is also set in the project config and the environment
- **THEN** the resolved value is the CLI value

#### Scenario: Both config files are read
- **WHEN** both a user config and a project config define different fields
- **THEN** the resolved configuration contains both sets of fields with project values taking precedence

#### Scenario: Hermetic run ignores defaults
- **WHEN** a user requests a hermetic run
- **THEN** no discovered config file or environment file is loaded

#### Scenario: Every resolved value names its layer
- **WHEN** a user asks the binary to explain a resolved field
- **THEN** the output names the winning layer, the file or flag, and the final value with secrets redacted

### Requirement: Tri-state boolean options
Boolean options SHALL distinguish unset, true, and false. An explicit false value from the CLI or config SHALL override a true value from a lower-precedence source. Every boolean flag SHALL have a negative counterpart.

#### Scenario: Explicit false overrides config true
- **WHEN** config sets is_remote to true and the CLI passes the negative remote flag
- **THEN** the resolved value is false

#### Scenario: Unset uses lower layer
- **WHEN** a boolean is absent from CLI and environment but set in the project config
- **THEN** the resolved value comes from the project config

#### Scenario: Explicit false in config stays false
- **WHEN** a config file sets a boolean to false and no higher layer sets it
- **THEN** the resolved value is false and the old default true is not restored

### Requirement: Scalar and list config values
Fields that accept multiple values SHALL accept both a single scalar and an array in TOML. A scalar SHALL be treated as a one-element list and SHALL NOT cause configuration loading to fail or drop other fields.

#### Scenario: Scalar search term loads
- **WHEN** a config file contains search as a single string
- **THEN** resolution succeeds and the search list contains that string

#### Scenario: Array search term loads
- **WHEN** a config file contains search as an array of strings
- **THEN** resolution succeeds and the search list contains every string

#### Scenario: Empty list is preserved
- **WHEN** a config file contains an empty array for a list field
- **THEN** resolution succeeds with an empty list instead of substituting a default

### Requirement: Strict validation and failure
Unknown config keys, unknown source IDs, invalid dates, invalid memory limits, invalid rate maps, and malformed config SHALL produce a diagnostic on stderr and a nonzero exit code. Warnings SHALL NOT be silently discarded.

#### Scenario: Unknown source fails
- **WHEN** a user requests a source ID that is not registered
- **THEN** the binary exits nonzero and lists the unknown ID

#### Scenario: Malformed config fails with a diagnostic
- **WHEN** a config file cannot be decoded by the runtime schema
- **THEN** the binary exits 2 and reports the file, field, and a remedy

#### Scenario: Unknown key suggests a remedy
- **WHEN** a config file contains an unknown key
- **THEN** validation names the key and the closest valid key from the schema

### Requirement: Machine-separated output streams
stdout SHALL contain only the serialized result in the requested format. Logs, warnings, progress, and diagnostics SHALL go to stderr. A warning SHALL NOT corrupt stdout output.

#### Scenario: Warning does not corrupt stdout
- **WHEN** a constraint produces a warning during a run
- **THEN** stdout still parses as the requested format and the warning appears on stderr

#### Scenario: Empty result is valid output
- **WHEN** a run finds no jobs
- **THEN** stdout contains a valid empty result in the requested format

### Requirement: Format independent of destination
Format selection SHALL apply identically to stdout and to files. There SHALL be an explicit way to write to stdout. Pretty and minified JSON settings SHALL behave the same for every destination.

#### Scenario: Same format for file and stdout
- **WHEN** a user runs the same scrape once to stdout and once to a file with the same format flag
- **THEN** both outputs use the same serialization

#### Scenario: Explicit stdout destination
- **WHEN** a user sets the output destination to stdout explicitly
- **THEN** the result is written to stdout in the selected format

### Requirement: Config template and editing workflow
The binary SHALL generate a commented config template with sane defaults through a config init action. The template SHALL document each field, its type, its default, and its environment variable. An interactive editing action MAY open an editor on the explicit config path. All config-writing actions SHALL fail cleanly when no explicit path is given and SHALL refuse to write credentials.

#### Scenario: Init writes a documented template
- **WHEN** a user runs `scrappy config init --path <path>`
- **THEN** the file contains documented fields, defaults, and environment variable names and contains no secret values

#### Scenario: Init refuses to overwrite
- **WHEN** a user runs config init against an existing file without a force option
- **THEN** the command exits 2 and the existing file remains unchanged

#### Scenario: Edit honors the explicit path
- **WHEN** a user runs a config editing action with an explicit path
- **THEN** the editor opens exactly that file and no discovered config is modified

### Requirement: Lossless atomic config writes
Config mutation SHALL read the existing document, change only the named keys, preserve all other sections including per-source sections, and write through a temporary file and rename. Config directories SHALL use mode 0700 and config files mode 0600. Credentials SHALL NOT be written into config files.

#### Scenario: Config set preserves sections
- **WHEN** a user changes one value in a config file that contains per-source sections
- **THEN** all other values and sections remain unchanged

#### Scenario: Config files are private
- **WHEN** a config file is written
- **THEN** its mode is 0600 and its parent directory mode is 0700

#### Scenario: Unset removes only the named key
- **WHEN** a user unsets a value in a config with per-source sections
- **THEN** that key is removed and all other keys and sections remain unchanged

### Requirement: Doctor parity with runtime
The doctor command SHALL use the same config schema as the runtime, report the exact files the runtime would load, and report API key availability as a count or name without revealing values.

#### Scenario: Doctor reports the loaded file
- **WHEN** a user config and a project config both exist
- **THEN** doctor reports both files and their precedence in the same order as the runtime

#### Scenario: Doctor hides key values
- **WHEN** all expected API keys are present
- **THEN** doctor reports key names or counts and never prints key values

### Requirement: Doctor check taxonomy
Doctor SHALL report named checks for config discovery and parsing, schema and source validation, credential presence without values, proxy reachability, browser dependency availability, source manifest and fixture parity, output destination writability, and feature availability.
Each check SHALL report pass, warn, fail, or skip with a reason. A missing optional credential SHALL NOT prevent unrelated checks from running. Doctor SHALL never print a secret, a raw cookie, or a query-string token.

#### Scenario: Doctor reports every named check
- **WHEN** doctor runs with a complete config
- **THEN** output includes a result for every check named in this requirement

#### Scenario: Missing key does not suppress proxy checks
- **WHEN** an optional API key is absent and proxies are configured
- **THEN** every configured proxy still receives a check result

### Requirement: Doctor output and fix mode
Doctor SHALL support JSON output and an explicit fix action that applies only documented safe repairs. A failed fix SHALL count as a failure and produce a nonzero exit.

#### Scenario: Doctor fix reports a failed repair
- **WHEN** a safe fix is attempted and fails
- **THEN** the check status is fail and doctor exits nonzero

#### Scenario: Doctor fix stays in scope
- **WHEN** doctor fix runs
- **THEN** it modifies only documented config files or generated artifacts and never changes source, fixture, or manifest files

#### Scenario: Doctor JSON is machine-readable
- **WHEN** a caller requests doctor output as JSON
- **THEN** stdout contains one JSON object with named checks and statuses and stderr contains no JSON unless diagnostics are explicitly requested as JSON

### Requirement: Config schema documentation
The binary SHALL print the complete config schema, including field names, types, defaults, environment variables, and accepted enum values, through a schema action on `config` or `validate`. The printed schema SHALL be the same schema used by the runtime parser and doctor. A user SHALL be able to ask for the accepted values of a named enum without reading source code.

#### Scenario: Schema output matches runtime parsing
- **WHEN** a user prints the schema
- **THEN** every field accepted by the runtime parser is listed with its type and default

#### Scenario: Enum values are discoverable
- **WHEN** a user asks for the values of an enum field
- **THEN** the binary prints every accepted value and exits zero

### Requirement: Honest rate semantics
Rate options SHALL enforce token-bucket requests per interval and concurrency options SHALL enforce parallel request caps. The two concepts SHALL be exposed separately.

#### Scenario: Rate option throttles requests
- **WHEN** a source is configured for a low requests-per-minute rate
- **THEN** requests to that source are spaced according to the token bucket

#### Scenario: Concurrency option caps parallel work
- **WHEN** a high concurrency value is set for a source
- **THEN** no more than the configured number of requests run in parallel

### Requirement: Non-interactive run contract
The binary SHALL support a non-interactive run mode that accepts a single JSON run-config document on stdin, starts the run at EOF, and emits one normalized job record per stdout line.
Every run-config field SHALL map to a documented CLI or config key, and the tri-state boolean rules SHALL apply to run-config booleans. A malformed run-config SHALL exit with the usage/validation code before any fetch and SHALL NOT emit partial stdout records.

#### Scenario: JSON run-config drives a run
- **WHEN** a caller writes one JSON run-config object to stdin and closes stdin
- **THEN** the binary begins a run using that document as the highest-precedence run-config layer

#### Scenario: Malformed run-config fails before fetching
- **WHEN** stdin contains an unknown key or an invalid value
- **THEN** the binary exits with the usage/validation code and prints the offending key to stderr

### Requirement: Structured progress events
Machine-readable progress lines on stderr SHALL be NDJSON events with a discriminated event field, a run identifier, a source identifier where applicable, counters, and a status. A terminal event SHALL report final emitted and skipped counts. Human-readable diagnostics SHALL remain separate non-JSON text on stderr, and consumers SHALL be able to distinguish the two by attempting JSON parsing.

#### Scenario: Consumer tracks progress
- **WHEN** a run processes two sources
- **THEN** stderr contains parseable JSON event lines for each source and one terminal event

#### Scenario: Human diagnostic stays separate
- **WHEN** a warning is emitted during a run
- **THEN** it appears as non-JSON stderr text and stdout remains parseable NDJSON

### Requirement: Published exit codes
The binary SHALL publish and honor distinct exit codes: 0 for clean success, 1 for internal failure, 2 for invalid usage or configuration including unknown sources, 3 for partial success with per-source failures, 4 for a run paused by challenges, and 5 for cancellation. stdout SHALL remain valid NDJSON for codes 0, 3, 4, and 5.

#### Scenario: Unknown source maps to usage error
- **WHEN** a run targets an unregistered source
- **THEN** the binary exits 2 and prints the source identifier on stderr

#### Scenario: Partial success is distinguishable
- **WHEN** one source fails but others emit records
- **THEN** the binary exits 3 and the terminal event names the failed source

### Requirement: Actionable validation diagnostics
Validation SHALL cover enums and ranges for job type, minimum score, timeout, log level, dates, memory limits, rate maps, and output formats. Unsupported filter and source combinations SHALL be reported before any fetch on stderr with the affected source and field named. Diagnostics SHALL name the field, file, and source where applicable and suggest a remedy that exists for that failure class.

#### Scenario: Invalid enum fails early
- **WHEN** a run supplies an unsupported job type or output format
- **THEN** the binary exits 2 and lists the accepted values on stderr

#### Scenario: Unsupported filter is preflighted
- **WHEN** a source cannot honor the hours-old filter
- **THEN** the source and filter field are named on stderr before any request to that source

### Requirement: Consistent config paths across subcommands
Every config-reading and config-writing subcommand SHALL honor the same explicit config path and the same discovery order. Secret files and environment files SHALL be preserved outside the keys being changed, and a proxy rejected by health checks SHALL NOT be reused by any fetcher.

#### Scenario: Setup honors the explicit config path
- **WHEN** a user passes an explicit config path to a config-writing subcommand
- **THEN** that path is both written and loaded by the runtime and doctor

#### Scenario: Environment file is parsed with a real parser
- **WHEN** an environment file contains quotes, inline comments, export syntax, or a proxy URL
- **THEN** the parser produces correct values and a malformed entry fails with a field-level diagnostic

#### Scenario: Rejected proxy is not reused
- **WHEN** a proxy fails its health check
- **THEN** no fetcher uses it for the remainder of the run

### Requirement: Doctor check integrity
Doctor SHALL derive its checks from the runtime schema and source manifests, emit a result for every check, exercise every configured proxy, count failed fixes as failures, and label each check by what it actually verifies. A missing optional credential SHALL NOT suppress unrelated checks.

#### Scenario: Fix failure fails doctor
- **WHEN** an automatic fix fails
- **THEN** doctor increments its failure count and exits nonzero

#### Scenario: Missing key does not hide proxy checks
- **WHEN** one optional API key is absent and proxies are configured
- **THEN** doctor still reports a result for every configured proxy
