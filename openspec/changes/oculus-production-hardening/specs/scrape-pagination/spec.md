# Spec Delta

## Purpose

Define how manifest-declared pagination is executed so a source can return all reachable jobs instead of one page.

## ADDED Requirements

### Requirement: Declared pagination is executed
The engine SHALL execute the pagination strategy declared by a manifest when the request template consumes the strategy parameter.

#### Scenario: Page range
- **WHEN** a manifest declares `page:1:8` and its request contains `{query:page}`
- **THEN** the engine requests pages 1 through 8 in order and merges their records

#### Scenario: Offset sequence
- **WHEN** a manifest declares `offset:20:10` and its request contains `{query:offset}`
- **THEN** the engine requests offsets 0 through 180 in steps of 20

#### Scenario: Unsupported strategy
- **WHEN** a manifest declares a strategy the engine cannot execute
- **THEN** the engine makes one request and reports the unsupported strategy in the source outcome

### Requirement: Pagination stops safely
The engine SHALL stop pagination on an empty page, a repeated page, a cancelled run, an exhausted result cap, or a configured request budget.

#### Scenario: Empty page ends a source
- **WHEN** a page returns no records
- **THEN** the engine stops requesting further pages for that source

#### Scenario: Budget exhausted
- **WHEN** the per-source request budget is reached
- **THEN** the engine stops pagination and reports the source as partial

### Requirement: Later-page failure preserves collected records
A failure after the first page SHALL return the records already collected and SHALL NOT discard the source.

#### Scenario: Second page fails
- **WHEN** page one succeeds and page two returns a transport or parse failure
- **THEN** the source emits page-one records and records a partial outcome

### Requirement: Harvest matches engine pagination
The live harvest SHALL use the same pagination executor as normal runs.

#### Scenario: Harvest total equals run total
- **WHEN** the same source and seed are used for a run and a harvest
- **THEN** both report the same record count and stop reason
