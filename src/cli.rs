//! Command-line surface for the scrappy binary.
//!
//! The command tree is described in documentation/003-cli-reference.md.
//! Commands that need configuration or fetching are wired in later sections.

// Rust guideline compliant 2026-10-09

use std::io;
use std::path::PathBuf;

use clap::{Args, CommandFactory, Parser, Subcommand, ValueEnum};
use clap_complete::{Shell, generate};

/// Parsed command-line input.
#[derive(Debug, Parser)]
#[command(
    name = "scrappy",
    version,
    propagate_version = true,
    about = "Scrape public job sources into deterministic records",
    after_help = "Example: scrappy run --source remoteok --format ndjson"
)]
pub struct Cli {
    /// Select a command.
    #[command(subcommand)]
    pub command: CommandKind,
}

/// Top-level commands.
#[derive(Debug, Subcommand)]
pub enum CommandKind {
    /// Run a scrape against selected sources.
    #[command(after_help = "Example: scrappy run --source remoteok --format ndjson")]
    Run(RunArgs),
    /// Validate configuration and source selection without fetching.
    #[command(after_help = "Example: scrappy validate --config config.toml")]
    Validate(GlobalArgs),
    /// Inspect or change configuration files.
    #[command(after_help = "Example: scrappy config show --path config.toml")]
    Config {
        /// Configuration action.
        #[command(subcommand)]
        action: ConfigAction,
    },
    /// List registered sources and their health.
    #[command(after_help = "Example: scrappy list-sources --format json")]
    ListSources(ListArgs),
    /// Check local setup and dependencies.
    #[command(after_help = "Example: scrappy doctor --format json")]
    Doctor(DoctorArgs),
    /// Generate a shell completion script.
    #[command(after_help = "Example: scrappy completion bash")]
    Completion {
        /// Target shell.
        shell: Shell,
    },
    /// Print version information without loading configuration.
    #[command(after_help = "Example: scrappy version")]
    Version,
}

/// Options shared by commands that read configuration.
#[derive(Debug, Args, Default)]
pub struct GlobalArgs {
    /// Explicit config file; may be repeated.
    #[arg(long, global = true)]
    pub config: Vec<PathBuf>,
    /// Disable discovered config files and environment layers.
    #[arg(long, global = true)]
    pub no_default_config: bool,
}

/// Arguments for a scrape run.
#[derive(Debug, Args, Default)]
pub struct RunArgs {
    /// Shared config options.
    #[command(flatten)]
    pub global: GlobalArgs,
    /// Source id to include; may be repeated.
    #[arg(long)]
    pub source: Vec<String>,
    /// Read a single JSON run-config document from stdin.
    #[arg(long)]
    pub stdin_config: bool,
    /// Output format.
    #[arg(long, value_enum, default_value_t = OutputFormat::Ndjson)]
    pub format: OutputFormat,
}

/// Arguments for source listing.
#[derive(Debug, Args, Default)]
pub struct ListArgs {
    /// Shared config options.
    #[command(flatten)]
    pub global: GlobalArgs,
    /// Output format.
    #[arg(long, value_enum, default_value_t = OutputFormat::Table)]
    pub format: OutputFormat,
}

/// Arguments for doctor.
#[derive(Debug, Args, Default)]
pub struct DoctorArgs {
    /// Shared config options.
    #[command(flatten)]
    pub global: GlobalArgs,
    /// Output format.
    #[arg(long, value_enum, default_value_t = DoctorFormat::Text)]
    pub format: DoctorFormat,
    /// Apply documented safe repairs.
    #[arg(long)]
    pub fix: bool,
}

/// Configuration actions.
#[derive(Debug, Subcommand)]
pub enum ConfigAction {
    /// Write a commented config template.
    #[command(after_help = "Example: scrappy config init --path config.toml")]
    Init {
        /// Explicit destination path.
        #[arg(long)]
        path: PathBuf,
        /// Replace an existing file.
        #[arg(long)]
        force: bool,
    },
    /// Show resolved configuration with secrets redacted.
    #[command(after_help = "Example: scrappy config show --path config.toml")]
    Show {
        /// Explicit config path.
        #[arg(long)]
        path: PathBuf,
    },
    /// Set one config key.
    #[command(
        after_help = "Example: scrappy config set --path config.toml --key timeout --value 30"
    )]
    Set {
        /// Explicit config path.
        #[arg(long)]
        path: PathBuf,
        /// Key to change.
        #[arg(long)]
        key: String,
        /// New value.
        #[arg(long)]
        value: String,
    },
    /// Remove one config key.
    #[command(after_help = "Example: scrappy config unset --path config.toml --key timeout")]
    Unset {
        /// Explicit config path.
        #[arg(long)]
        path: PathBuf,
        /// Key to remove.
        #[arg(long)]
        key: String,
    },
    /// Open the explicit config path in an editor.
    #[command(after_help = "Example: scrappy config edit --path config.toml")]
    Edit {
        /// Explicit config path.
        #[arg(long)]
        path: PathBuf,
    },
    /// Explain which layer supplied a value.
    #[command(after_help = "Example: scrappy config explain --key timeout")]
    Explain {
        /// Shared config options.
        #[command(flatten)]
        global: GlobalArgs,
        /// Field to explain.
        #[arg(long)]
        key: String,
    },
    /// Print the runtime config schema.
    #[command(after_help = "Example: scrappy config schema")]
    Schema,
}

/// Record output formats.
#[derive(Debug, Clone, Copy, Default, PartialEq, Eq, ValueEnum)]
pub enum OutputFormat {
    /// Newline-delimited JSON, the default.
    #[default]
    Ndjson,
    /// A single JSON document.
    Json,
    /// Comma-separated values.
    Csv,
    /// Human-readable table.
    Table,
}

/// Doctor output formats.
#[derive(Debug, Clone, Copy, Default, PartialEq, Eq, ValueEnum)]
pub enum DoctorFormat {
    /// Plain text.
    #[default]
    Text,
    /// Machine-readable JSON.
    Json,
}

/// Parse arguments from the process environment.
#[must_use]
pub fn parse() -> Cli {
    Cli::parse()
}

/// Generate a completion script for the selected shell.
pub fn completion(shell: Shell, out: &mut impl io::Write) {
    let mut command = Cli::command();
    command = command.bin_name("scrappy");
    generate(shell, &mut command, "scrappy", out);
}

/// Print the root help text.
pub fn print_help() {
    let _ = Cli::command().print_help();
    println!();
}
