// Rust guideline compliant 2026-10-09

use std::io;
use std::path::Path;

use scrappy::cli::{self, Cli, CommandKind, ConfigAction};
use scrappy::config_file::{self, FileError};

fn main() {
    let cli = cli::parse();
    if let Err(error) = dispatch(cli) {
        eprintln!("{error}");
        std::process::exit(2);
    }
}

fn dispatch(cli: Cli) -> Result<(), FileError> {
    match cli.command {
        CommandKind::Version => println!("scrappy {}", env!("CARGO_PKG_VERSION")),
        CommandKind::Completion { shell } => {
            cli::completion(shell, &mut io::stdout().lock());
        }
        CommandKind::Config { action } => match action {
            ConfigAction::Init { path, force } => config_file::init(&path, force)?,
            ConfigAction::Show { path } => print_file(&path)?,
            ConfigAction::Set { path, key, value } => config_file::set(&path, &key, &value)?,
            ConfigAction::Unset { path, key } => config_file::unset(&path, &key)?,
            ConfigAction::Edit { path } => config_file::edit(&path)?,
            ConfigAction::Explain { key, .. } => println!("{}", config_file::explain(&key)),
            ConfigAction::Schema => println!("{}", config_file::print_schema()?),
        },
        CommandKind::Run(_)
        | CommandKind::Validate(_)
        | CommandKind::ListSources(_)
        | CommandKind::Doctor(_) => {
            cli::print_help();
        }
    }
    Ok(())
}

fn print_file(path: &Path) -> Result<(), FileError> {
    let document = config_file::read(path)?;
    println!(
        "{}",
        serde_json::to_string_pretty(&document).map_err(|error| FileError {
            message: format!("cannot encode config: {error}"),
        })?
    );
    Ok(())
}
