# Browser automation

Browser rendering is opt-in. Build with `cargo build --features browser`. The
default build has no browser runtime dependency.

src/fetching/browser.rs uses Chromium revision 1585606 through
crawlberg-chromiumoxide. `SCRAPPY_BROWSER_EXECUTABLE` selects an explicit
executable. Otherwise the pinned cache under the XDG cache home or the
user cache directory is checked before the installed browser.
`fetch_pinned_executable()` downloads the pinned revision into that cache.

The doctor `browser` check uses the same resolver. A browser-required source
with no executable is skipped with a clear status. There is no plain HTTP
fallback.

Each fetch validates the target host and every resolved address, rejects
localhost and private addresses, sets configured proxy arguments and profile
headers, caps rendered HTML at 8 MiB, rejects cookie data above 4096 bytes,
and refuses challenge pages. Cookies are read only to enforce the cap and are
never written to records, logs, or resume state.
