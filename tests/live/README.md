# Live validation

Run the ignored suite opt-in:

```bash
cargo test --test live_validate -- --ignored --nocapture
```

The report prints working, keyed, blocked, stale, and unregistered sections.
Save it outside version control. See documentation/014-testing.md.
