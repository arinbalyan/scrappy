# Spec Delta

## Purpose

Defines how job sources are declared, identified, validated, and reported so operators can see which sources are available, which require credentials, and which are blocked before a run starts.

## ADDED Requirements

### Requirement: Stable source identifiers
Each source SHALL have a stable, lowercase identifier that is unique across the registry. Identifiers SHALL NOT be derived from mutable display names or URLs.

#### Scenario: Duplicate identifiers are rejected
- **WHEN** two manifests declare the same source identifier
- **THEN** validation fails and names the duplicate identifier

#### Scenario: Identifier remains stable
- **WHEN** a source display name changes in a later release
- **THEN** the source identifier remains unchanged

### Requirement: Declarative source manifests
Each source SHALL be described by a versioned manifest containing its identifier, request template, parser kind, pagination strategy, emulation profile, whether JavaScript is required, requests per minute, maximum concurrency, robots policy, attribution, credential requirements, and fixture set.

#### Scenario: Required manifest fields are present
- **WHEN** a source manifest is validated
- **THEN** every required field is present and typed correctly

#### Scenario: Unknown manifest fields are rejected
- **WHEN** a manifest contains an undeclared field
- **THEN** validation fails and names the field

### Requirement: Credential requirements
A manifest SHALL declare every credential required by its source by variable name. A source with unmet credentials SHALL be skipped with a missing-credentials status and SHALL NOT make an unauthenticated request.

#### Scenario: Missing key skips the source
- **WHEN** a keyed source has no configured credential
- **THEN** the source is reported as missing-credentials and no request is sent to it

#### Scenario: Present key enables the source
- **WHEN** a keyed source has a configured credential
- **THEN** the source is eligible for scraping

### Requirement: Source health states
The source listing SHALL report each source as working, keyed, blocked, stale, requires-slug, browser-only, or unregistered. Documentation SHALL be generated from the registry so counts and status tables cannot drift.

#### Scenario: Health list matches the registry
- **WHEN** a user lists sources
- **THEN** every reported source corresponds to a registered manifest and the counts match the entries

#### Scenario: Unregistered source is visible as unavailable
- **WHEN** a source ID is declared in documentation but has no manifest
- **THEN** the listing reports it as unregistered and a run targeting it exits nonzero

### Requirement: Robots and attribution policy
Each manifest SHALL declare an explicit robots policy and attribution. A source SHALL NOT be fetched unless its policy permits the request.

#### Scenario: Disallowed robots policy blocks a source
- **WHEN** a source robots policy disallows a request
- **THEN** the engine skips the request and records a policy-skip status

#### Scenario: Attribution is present
- **WHEN** a source manifest is validated
- **THEN** attribution is non-empty

### Requirement: Fixture binding
Each manifest SHALL name a fixture set. Validation SHALL fail when the fixture set is missing or when declared selectors match nothing in the fixture.

#### Scenario: Missing fixture set fails validation
- **WHEN** a source manifest names a fixture set that does not exist
- **THEN** validation fails with the source identifier and expected fixture path

#### Scenario: Selector matches fixture content
- **WHEN** a source selector is validated against its fixture
- **THEN** the selector matches at least one node or record

### Requirement: Structured source metadata
The source listing SHALL support a JSON format containing stable source identifiers, health states, failure kinds, last failure reasons, fetcher kind, emulation profile, credential requirements, robots policy, attribution, fixture set, and slug requirements where applicable. An empty result and a blocked result SHALL be distinguishable by status, not by record count.

#### Scenario: Machine-readable listing
- **WHEN** a caller requests JSON source listing
- **THEN** every registered source appears with typed metadata fields

#### Scenario: Empty is not blocked
- **WHEN** a source returns zero jobs without a challenge or policy skip
- **THEN** its status indicates an empty result rather than a blocked state

### Requirement: Registry parity checks
Default source selection SHALL contain only registered manifests. Every registered source SHALL have a fixture set and a contract test. Generated status documentation SHALL assign each source exactly one state, and generated ATS slug counts SHALL match loaded manifests. Documentation SHALL NOT list a source that has no manifest.

#### Scenario: Unregistered source cannot be defaulted
- **WHEN** the registry is loaded
- **THEN** no unregistered identifier appears in the default source selection

#### Scenario: Fixture and contract coverage is complete
- **WHEN** registry parity checks run
- **THEN** every registered source has both a fixture set and a contract test

### Requirement: Slug and credential governance
Slug data and credential declarations SHALL live in source manifests. Explicit override paths SHALL be documented and validated. Empty slug sets SHALL have a deterministic skip policy. A source SHALL NOT read an undeclared credential from the environment, and email-style identifiers SHALL NOT be used as user agents.

#### Scenario: Undeclared credential is rejected
- **WHEN** a source attempts to read an environment credential absent from its manifest
- **THEN** validation fails and names the source and variable

#### Scenario: Empty slug set is deterministic
- **WHEN** a slug-dependent source has no valid seeds
- **THEN** the source is skipped with a missing-slug status and no request is sent

### Requirement: Custom source manifests
A user SHALL be able to add a source through a local manifest in a documented user config directory without rebuilding the binary or forking the repository. A custom manifest SHALL use the built-in parser kinds, pass the same validation as a built-in source, and SHALL NOT override a built-in source identifier.

#### Scenario: Custom source runs without rebuild
- **GIVEN** a valid custom manifest in the documented user config directory
- **WHEN** the user runs a scrape that selects the custom source
- **THEN** the source is listed and runs with the current binary

#### Scenario: Custom source cannot shadow a built-in
- **GIVEN** a custom manifest using a built-in source identifier
- **WHEN** the registry loads the manifest
- **THEN** validation fails and names the conflict

### Requirement: Scraper behavior documentation
Every registered source SHALL have one behavior document in documentation/scrapers/ keyed by its stable source identifier. Each document SHALL record the adapter family, health state, endpoint shape, parser kind, fetcher kind, emulation profile, credential and slug needs, rate and concurrency limits, robots policy, known quirks, and a dated break/fix log. When a scraper is repaired, its behavior document SHALL be updated in the same change with the observed failure and the fix.

#### Scenario: Every source has a behavior document
- **WHEN** registry parity checks run
- **THEN** every registered source has exactly one matching document in documentation/scrapers/

#### Scenario: Break and fix are recorded
- **WHEN** a scraper is repaired after a break
- **THEN** its behavior document contains a dated entry naming the failure and the fix

### Requirement: Adapter family placement
Each source SHALL be placed in exactly one adapter family directory:
src/sources/manifests/feeds_api/, ats/, html_static/, html_rendered/, or
keyed/. The family SHALL correspond to the fetch and parse behavior declared
in its manifest. Custom sources SHALL use one of these families and SHALL NOT
introduce a new top-level source directory.

#### Scenario: Manifest lives in its family directory
- **WHEN** a manifest declares a source family
- **THEN** the manifest file is located under the matching family directory

#### Scenario: Unknown family is rejected
- **WHEN** a manifest declares a family outside the five defined families
- **THEN** registry validation fails and names the invalid family
