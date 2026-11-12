# eino-cli (npm)

Installs the [Eino CLI](https://github.com/tk103331/eino-cli) binary for your
platform and exposes two identical commands: `eino` and `eino-cli`.

```bash
npm install -g eino-cli

eino                 # interactive TUI (last agent / built-in agent)
eino agent --agent minimax_agent
eino acp --agent minimax_agent
```

The platform binary (`darwin`/`linux`, `x64`/`arm64`) is downloaded from the
GitHub release matching this package version on first install (or first run).
Homebrew and `go install` are also supported.
