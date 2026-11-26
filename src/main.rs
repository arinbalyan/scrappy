// Rust guideline compliant 2026-10-09

use std::io;
use std::path::Path;

use scrappy::cli::{self, Cli, CommandKind, ConfigAction};
use scrappy::config::file as config_file;
use scrappy::config::{Config, validate};
use scrappy::engine::contract::{self, EXIT_USAGE};

fn main() {
    let cli = cli::parse();
    if let Err(message) = dispatch(cli) {
        eprintln!("{message}");
        std::process::exit(EXIT_USAGE);
    }
}

fn dispatch(cli: Cli) -> Result<(), String> {
    match cli.command {
        CommandKind::Version => println!("scrappy {}", env!("CARGO_PKG_VERSION")),
        CommandKind::Completion { shell } => {
            cli::completion(shell, &mut io::stdout().lock());
        }
        CommandKind::Config { action } => dispatch_config(action)?,
        CommandKind::Run(args) if args.stdin_config => {
            let mut input = io::stdin().lock();
            let config: Config =
                contract::stdin_config(&mut input).map_err(|error| error.message)?;
            validate(&config).map_err(|error| error.message)?;
            eprintln!("scrappy: source selection is unavailable in this build");
        }
        CommandKind::Run(_)
        | CommandKind::Validate(_)
        | CommandKind::ListSources(_)
        | CommandKind::Doctor(_) => {
            cli::print_help();
        }
    }
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
