# Browser automation

src/fetching/browser.rs is compiled only with the browser feature in
Cargo.toml. It uses a pinned browser and is selected only by manifests marked
requires_js. Diagnosis lives in the planned `doctor` command.
