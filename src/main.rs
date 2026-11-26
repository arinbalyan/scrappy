// Rust guideline compliant 2026-10-09

use std::io;

use clap::CommandFactory;
use scrappy::cli::{self, Cli, CommandKind};

fn main() {
    let cli = cli::parse();
    match cli.command {
        CommandKind::Version => println!("scrappy {}", env!("CARGO_PKG_VERSION")),
        CommandKind::Completion { shell } => {
            cli::completion(shell, &mut io::stdout().lock());
        }
        CommandKind::Run(_)
        | CommandKind::Validate(_)
        | CommandKind::Config { .. }
        | CommandKind::ListSources(_)
        | CommandKind::Doctor(_) => {
            Cli::command().print_help().expect("help is writable");
            println!();
        }
    }
}
