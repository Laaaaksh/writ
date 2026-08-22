<!--
Thanks for contributing to writ! Every change lands through a pull request:
`master` is protected, the `Test` check must pass, and all conversations
must be resolved before merge (see CONTRIBUTING.md).
-->

## What does this PR change?

<!-- A short summary of the change and why it is needed. Link to an issue if one exists. -->

## How was it verified?

Run all three locally before opening - CI runs exactly these on every PR:

```bash
go build ./...
go vet ./...
go test ./...
```

- [ ] All three commands pass on my machine
- [ ] The fixed lifecycle (`propose` -> `approve` -> implement -> `attest`/`unattest` -> `status`/`merge`, `discard`) still works if this change touches it
- [ ] No new dependencies beyond `spf13/cobra` and `BurntSushi/toml`
