// Rust guideline compliant 2026-10-09

use scrappy::output::record::{
    Email, EmailSource, JobInput, Provenance, SCHEMA_VERSION, from_record,
};
use scrappy::output::writer::{CSV_COLUMNS, Format, encode};
use scrappy::parsing::Record;

fn job(description: &str) -> scrappy::output::record::Job {
    let record = Record::new("https://example.test/jobs/1", "Rust Engineer", "Example Co")
        .expect("valid record")
        .with_description(Some(description.to_owned()));
    let input = JobInput {
        source_id: "remoteok".to_owned(),
        fetched_at: "2026-11-26T00:00:00Z".to_owned(),
        emails: vec![Email {
            addr: "jobs@example.test".to_owned(),
            verified: true,
            source: EmailSource::Description,
            role: None,
            provenance: Provenance::Observed,
        }],
        ..JobInput::default()
    };
    from_record(&record, &input, SCHEMA_VERSION).expect("valid job")
}

fn parse_rows(text: &str) -> Vec<Vec<String>> {
    let mut rows = Vec::new();
    let mut row = Vec::new();
    let mut field = String::new();
    let mut quoted = false;
    let mut chars = text.chars().peekable();
    while let Some(ch) = chars.next() {
        match (quoted, ch) {
            (true, '"') => {
                if chars.peek() == Some(&'"') {
                    chars.next();
                    field.push('"');
                } else {
                    quoted = false;
                }
            }
            (false, '"') => quoted = true,
            (false, ',') => row.push(std::mem::take(&mut field)),
            (false, '\n') => {
                row.push(std::mem::take(&mut field));
                rows.push(std::mem::take(&mut row));
            }
            (false, '\r') => {}
            (_, other) => field.push(other),
        }
    }
    if !field.is_empty() || !row.is_empty() {
        row.push(field);
        rows.push(row);
    }
    rows
}

#[test]
fn csv_has_stable_header_and_standard_escaping() {
    let description = "comma, quote \" and\nnewline";
    let csv = encode(&[job(description)], Format::Csv).expect("csv");
    let rows = parse_rows(&csv);
    assert_eq!(rows.len(), 2, "one header row and one record row");
    assert_eq!(rows[0], CSV_COLUMNS);
    assert_eq!(rows[0][0], "schema_version");
    assert_eq!(rows[1].len(), CSV_COLUMNS.len());
    let description_index = CSV_COLUMNS
        .iter()
        .position(|column| *column == "description")
        .expect("description column");
    assert_eq!(rows[1][description_index], description);
    assert!(csv.contains("\"comma, quote \"\" and\nnewline\""));
}
