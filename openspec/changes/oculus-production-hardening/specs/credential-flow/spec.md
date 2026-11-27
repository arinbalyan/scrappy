# Spec Delta

## Purpose

Define how keyed sources receive credentials at runtime without writing secrets into config, fixtures, or output.

## ADDED Requirements

### Requirement: Credentials reach runtime substitutions
The engine SHALL resolve declared credential names from environment variables and the credential layer into per-source template values before building a request.

#### Scenario: Environment credential present
- **WHEN** a keyed source declares `ADZUNA_APP_KEY` and `SCRAPPY_ADZUNA_APP_KEY` is set
- **THEN** the request includes the key and the run proceeds

#### Scenario: Credential absent
- **WHEN** a declared credential is absent
- **THEN** the source is skipped with a missing-credential outcome and the rest of the run continues

### Requirement: Credential values never persist
Credentials SHALL NOT appear in config files, fixtures, logs, errors, progress events, or normalized records.

#### Scenario: Error redaction
- **WHEN** a credential-bearing request fails
- **THEN** the diagnostic contains the credential name and no credential value

### Requirement: Credential precedence is deterministic
An explicit CLI or run-config credential SHALL override an environment credential, and an environment credential SHALL override a discovered file value.

#### Scenario: Explicit override
- **WHEN** the same credential exists in the environment and in an explicit run config
- **THEN** the run-config value is used

### Requirement: Doctor reports credential availability
Doctor SHALL report each declared credential name as present or absent without printing its value.

#### Scenario: Doctor with missing keys
- **WHEN** keyed sources declare credentials that are not set
- **THEN** doctor reports an absent count and names, and still runs unrelated checks
