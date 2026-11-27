# Installation

The default build uses reqwest with rustls and has no browser runtime
dependency. Optional builds enable the emulation and browser features in
Cargo.toml. See documentation/015-security.md for dependency review guidance.

## Prebuilt binaries

Every tagged release publishes four archives and a SHA256SUMS file:

- scrappy_linux_amd64.tar.gz
- scrappy_darwin_arm64.tar.gz
- scrappy_darwin_amd64.tar.gz
- scrappy_windows_amd64.zip

The shell installer selects the Linux or macOS archive, verifies its checksum
against SHA256SUMS, and installs the binary into SCRAPPY_INSTALL_DIR or
$HOME/.local/bin. The PowerShell installer performs the same checks on Windows
and installs into %LOCALAPPDATA%\scrappy.

```bash
curl -fsSL https://raw.githubusercontent.com/arinbalyan/scrappy/main/scripts/install.sh | bash
```

```powershell
irm https://raw.githubusercontent.com/arinbalyan/scrappy/main/scripts/install.ps1 | iex
```

Both installers download SHA256SUMS from the same release and refuse to install
when the archive hash does not match. The release workflow builds each target,
smoke-tests it with --version, generates the checksum file, and verifies it
before publishing.

## Build from source

A stable Rust toolchain and a Cargo lockfile are required.

```bash
cargo build --release --locked
cargo test --locked
```

Enable optional fetchers only when needed. The browser feature needs an
available Chromium runtime at run time.

```bash
cargo check --locked --features emulation
cargo check --locked --features browser
```

## Verify the install

```bash
scrappy --version
scrappy doctor --format json
scrappy list-sources --format json
```

See documentation/003-cli-reference.md for the command tree and
documentation/016-config-migration.md when replacing an old binary config.

