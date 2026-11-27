// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Arin Balyan
// Last modified: 2026-11-27

// Rust guideline compliant 2026-10-09

use std::collections::BTreeMap;
use std::io;
use std::path::Path;

use scrappy::cli::{self, Cli, CommandKind, ConfigAction};
use scrappy::config::file as config_file;
use scrappy::config::{self, Config, OneOrMany, validate};
use scrappy::engine::contract::{
    self, EXIT_CANCELLED, EXIT_CHALLENGE, EXIT_OK, EXIT_PARTIAL, EXIT_USAGE,
};
use scrappy::engine::pipeline;
use scrappy::fetching::plain::PlainFetcher;
use scrappy::output::writer::{self, Format};
use scrappy::sources::custom;
use scrappy::sources::status;

fn main() {
    let cli = cli::parse();
    let code = match dispatch(cli) {
        Ok(code) => code,
        Err(message) => {
            eprintln!("{}", cli::diagnostic::safe_text(&message));
            EXIT_USAGE
        }
    };
    if code != EXIT_OK {
        std::process::exit(code);
    }
}

fn dispatch(cli: Cli) -> Result<i32, String> {
    match cli.command {
        CommandKind::Version => {
            println!("scrappy {}", env!("CARGO_PKG_VERSION"));
        }
        CommandKind::Completion { shell } => {
            cli::completion(shell, &mut io::stdout().lock());
        }
        CommandKind::Config { action } => dispatch_config(action)?,
        CommandKind::Run(args) if args.stdin_config => return dispatch_stdin_run(&args),
        CommandKind::Validate(args) => dispatch_validate(args)?,
        CommandKind::ListSources(args) if args.generate_status => {
            dispatch_generate_status(&args)?;
        }
        CommandKind::ListSources(args) => dispatch_list(&args)?,
        CommandKind::Run(args) => return dispatch_run(&args),
        CommandKind::Doctor(args) => {
            let report = if args.fix {
                cli::doctor::report_with_fix(&args.global.config)
            } else {
                cli::doctor::report(&args.global.config)
            };
            match args.format {
                cli::DoctorFormat::Json => println!(
                    "{}",
                    cli::doctor::render_json(&report)
                        .map_err(|error| format!("cannot encode doctor report: {error}"))?
                ),
                cli::DoctorFormat::Text => print!("{}", cli::doctor::render_text(&report)),
            }
        }
    }
    Ok(EXIT_OK)
}

fn dispatch_validate(args: cli::GlobalArgs) -> Result<(), String> {
    let sources = config::Sources {
        files: args.config,
        hermetic: args.no_default_config,
        ..config::Sources::default()
    };
    let resolved = config::resolve(&sources).map_err(|error| error.message)?;
    validate(&resolved).map_err(|error| error.message)?;
    println!("ok");
    Ok(())
}

fn dispatch_run(args: &cli::RunArgs) -> Result<i32, String> {
    let root = std::path::Path::new("src/sources/manifests");
    let registry = custom::load(root, &custom::directory()).map_err(|error| error.message)?;
    let sources = config::Sources {
        files: args.global.config.clone(),
        hermetic: args.global.no_default_config,
        ..config::Sources::default()
    };
    let config = config::resolve(&sources).map_err(|error| error.message)?;
    let secrets = scrappy::config::secret::SecretStore::from_config(&config);
    let fetcher = plain_fetcher(&config);
    let config_format = config.format.clone();
    let input = pipeline::RunInput {
        sources: args.source.clone(),
        fetched_at: pipeline::now_utc(),
        config,
        values: runtime_values(&registry, &secrets),
        ..pipeline::RunInput::default()
    };
    let runtime = tokio::runtime::Builder::new_current_thread()
        .enable_all()
        .build()
        .map_err(|error| format!("cannot create async runtime: {error}"))?;
    let (jobs, summary) = runtime.block_on(pipeline::run(&registry, &input, &fetcher))?;
    write_run_output(args, config_format.as_deref(), &jobs, &summary, false)
}

fn plain_fetcher(config: &Config) -> PlainFetcher {
    let proxies = config
        .proxies
        .clone()
        .map_or_else(Vec::new, scrappy::config::OneOrMany::into_vec);
    PlainFetcher::new()
        .with_proxies(proxies)
        .with_cookies(config.cookies.unwrap_or(false))
}

fn runtime_values(
    registry: &scrappy::sources::registry::Registry,
    secrets: &scrappy::config::secret::SecretStore,
) -> BTreeMap<String, scrappy::sources::adapter::TemplateValues> {
    let mut values = BTreeMap::new();
    for manifest in registry.iter() {
        let mut template = scrappy::sources::adapter::TemplateValues {
            slug: manifest.slugs.first().cloned(),
            tenant: manifest.tenant.clone(),
            site: manifest.site.clone(),
            credentials: secrets.values(&manifest.credentials),
            ..scrappy::sources::adapter::TemplateValues::default()
        };
        template.query.extend(manifest.query_defaults.clone());
        values.insert(manifest.id.clone(), template);
    }
    values
}

fn run_exit_code(summary: &pipeline::RunSummary) -> i32 {
    if summary.cancelled {
        EXIT_CANCELLED
    } else if !summary.attention.is_empty() {
        EXIT_CHALLENGE
    } else if !summary.failures.is_empty() {
        EXIT_PARTIAL
    } else {
        EXIT_OK
    }
}

fn write_run_output(
    args: &cli::RunArgs,
    _config_format: Option<&str>,
    jobs: &[scrappy::output::record::Job],
    summary: &pipeline::RunSummary,
    stdin_config: bool,
) -> Result<i32, String> {
    let format = if stdin_config {
        Format::Ndjson
    } else {
        match args.format {
            cli::OutputFormat::Ndjson => Format::Ndjson,
            cli::OutputFormat::Json => Format::Json,
            cli::OutputFormat::Csv => Format::Csv,
            cli::OutputFormat::Table => {
                return Err("table format is not supported for run output".to_owned());
            }
        }
    };
    let text = writer::encode(jobs, format)?;
    print!("{text}");
    for failure in &summary.failures {
        eprintln!("scrappy: {}", cli::diagnostic::safe_text(failure));
    }
    for event in &summary.attention {
        let encoded = serde_json::to_string(event)
            .map_err(|error| format!("cannot encode attention event: {error}"))?;
        eprintln!("{}", cli::diagnostic::safe_text(&encoded));
    }
    Ok(run_exit_code(summary))
}

fn dispatch_stdin_run(args: &cli::RunArgs) -> Result<i32, String> {
    let mut input = io::stdin().lock();
    let config: Config = contract::stdin_config(&mut input).map_err(|error| error.message)?;
    validate(&config).map_err(|error| error.message)?;
    let sources = config
        .sources
        .clone()
        .map_or_else(Vec::new, OneOrMany::into_vec);
    if sources.is_empty() {
        return Ok(EXIT_OK);
    }
    run_with_config(args, config, sources)
}

fn run_with_config(
    args: &cli::RunArgs,
    config: Config,
    sources: Vec<String>,
) -> Result<i32, String> {
    let root = std::path::Path::new("src/sources/manifests");
    let registry = custom::load(root, &custom::directory()).map_err(|error| error.message)?;
    let secrets = scrappy::config::secret::SecretStore::from_config(&config);
    let fetcher = plain_fetcher(&config);
    let config_format = config.format.clone();
    let input = pipeline::RunInput {
        sources,
        fetched_at: pipeline::now_utc(),
        config,
        values: runtime_values(&registry, &secrets),
        ..pipeline::RunInput::default()
    };
    let runtime = tokio::runtime::Builder::new_current_thread()
        .enable_all()
        .build()
        .map_err(|error| format!("cannot create async runtime: {error}"))?;
    let (jobs, summary) = runtime.block_on(pipeline::run(&registry, &input, &fetcher))?;
    write_run_output(args, config_format.as_deref(), &jobs, &summary, true)
}

fn dispatch_list(args: &cli::ListArgs) -> Result<(), String> {
    let root = std::path::Path::new("src/sources/manifests");
    let registry = custom::load(root, &custom::directory()).map_err(|error| error.message)?;
    let fixture_root = std::path::Path::new("tests/fixtures");
    let rows = status::list(&registry, fixture_root);
    match args.format {
        cli::OutputFormat::Json => println!(
            "{}",
            serde_json::to_string_pretty(&rows)
                .map_err(|error| format!("cannot encode source list: {error}"))?
        ),
        _ => {
            for row in &rows {
                println!(
                    "{} {} {:?} {:?}",
                    row.id, row.name, row.health, row.availability
                );
            }
        }
    }
    Ok(())
}

fn dispatch_generate_status(_args: &cli::ListArgs) -> Result<(), String> {
    let root = std::path::Path::new("src/sources/manifests");
    let registry = custom::load(root, &custom::directory()).map_err(|error| error.message)?;
    let text =
        scrappy::sources::status_doc::render(&registry, std::path::Path::new("tests/fixtures"));
    let path = std::path::Path::new("documentation/005-status.md");
    std::fs::write(path, text)
        .map_err(|error| format!("cannot write {}: {error}", path.display()))?;
    println!("wrote {}", path.display());
    Ok(())
}

fn dispatch_config(action: ConfigAction) -> Result<(), String> {
    match action {
        ConfigAction::Init { path, force } => {
            config_file::init(&path, force).map_err(|error| error.message)
        }
        ConfigAction::Show { path } => print_file(&path),
        ConfigAction::Set { path, key, value } => {
            config_file::set(&path, &key, &value).map_err(|error| error.message)
        }
        ConfigAction::Unset { path, key } => {
            config_file::unset(&path, &key).map_err(|error| error.message)
        }
        ConfigAction::Edit { path } => config_file::edit(&path).map_err(|error| error.message),
        ConfigAction::Explain { key, .. } => {
            println!("{}", config_file::explain(&key));
            Ok(())
        }
        ConfigAction::Schema { r#enum } => {
            if let Some(name) = r#enum {
                let values = scrappy::config::enum_values(&name)
                    .ok_or_else(|| format!("unknown enum field: {name}"))?;
                println!("{}", values.join(", "));
            } else {
                println!(
                    "{}",
                    config_file::print_schema().map_err(|error| error.message)?
                );
            }
            Ok(())
        }
    }
}

fn print_file(path: &Path) -> Result<(), String> {
    let document = config_file::read(path).map_err(|error| error.message)?;
    let text = serde_json::to_string_pretty(&document)
        .map_err(|error| format!("cannot encode config: {error}"))?;
    println!("{text}");
    Ok(())
}
