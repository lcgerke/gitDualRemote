# GitHelper - Unified Git Management Tool

A comprehensive Go CLI tool that manages both bare repository workflows and GitHub dual-remote synchronization. GitHelper combines repository lifecycle management with seamless GitHub backup integration.

## Status

**Current Version**: 3.2 (Phase 1 Complete)

- ✅ **Phase 0**: Go-git validation spike (Complete)
- ✅ **Phase 1**: Core infrastructure (Complete)
- ⬜ **Phase 2**: GitHub integration
- ⬜ **Phase 3**: Sync & recovery
- ⬜ **Phase 4**: Diagnostics & polish
- ⬜ **Phase 5**: Testing & documentation

## Features (Phase 1)

- 🚀 **Bare Repository Management**: Create and manage bare repositories
- 📂 **Local Clones**: Automatically clone to local working directories
- ⚙️ **Configuration Management**: Vault-backed config with 24h caching
- 💾 **State Tracking**: YAML state file for repository inventory
- 🎨 **Smart Output**: TTY detection with human/JSON formats
- 🔧 **Git CLI Wrapper**: Native git operations via CLI
- 📦 **Repository Types**: Extensible initialization (Go, Python, etc.)

## Quick Start

### Build

```bash
make build
# or
go build -o githelper ./cmd/githelper
```

### Setup

Create a config cache (simulates Vault for testing):

```bash
mkdir -p ~/.githelper/cache
cat > ~/.githelper/cache/config.json << 'EOF'
{
  "config": {
    "github_username": "lcgerke",
    "bare_repo_pattern": "/tmp/bare-repos/{repo}.git",
    "default_visibility": "private",
    "auto_create_github": false,
    "test_before_push": true,
    "sync_on_setup": true,
    "retry_on_partial_failure": true
  },
  "fetched_at": "2025-11-16T21:00:00Z"
}
EOF
```

### Usage

```bash
# Create a repository
./githelper repo create myproject --type go --format human

# List repositories
./githelper repo list

# JSON output
./githelper repo list --format json
```

## Documentation

- [Implementation Plan (v3.2)](GITHELPER_PLAN_V3.md) - Complete architecture and design
- [Phase 0 Spike Results](spike/FINDINGS.md) - Go-git evaluation and decision
- [Phase 1 Testing Guide](docs/PHASE1_TESTING.md) - How to test Phase 1 features

## Architecture

### Project Structure

```
githelper/
├── cmd/githelper/          # CLI entry point and commands
│   ├── main.go            # Root command
│   ├── repo.go            # Repo subcommand
│   ├── repo_create.go     # Create command
│   └── repo_list.go       # List command
├── internal/
│   ├── config/            # Configuration management
│   │   └── config.go      # Vault config with caching
│   ├── vault/             # Vault integration
│   │   ├── client.go      # Vault client wrapper
│   │   └── types.go       # Config and SSH key types
│   ├── git/               # Git operations
│   │   └── cli.go         # Git CLI wrapper
│   ├── state/             # State file management
│   │   └── state.go       # Repository state tracking
│   └── ui/                # Output formatting
│       └── output.go      # TTY detection and formatting
├── spike/                  # Phase 0 validation
│   ├── main.go            # Go-git tests
│   └── FINDINGS.md        # Spike results
└── docs/
    └── PHASE1_TESTING.md  # Testing guide
```

### Technology Stack

**Go Dependencies**:
- `github.com/spf13/cobra` - CLI framework
- `github.com/hashicorp/vault/api` - Vault client
- `gopkg.in/yaml.v3` - State file
- `go.uber.org/zap` - Logging
- `github.com/fatih/color` - Colorized output

**External Requirements**:
- Git >= 2.0 (for pushurl support)
- Vault server (or cached config for testing)

**Git Operations**: Git CLI wrapper using `os/exec` (no go-git dependency)

## Key Design Decisions

1. **Git CLI Wrapper over go-git**: After Phase 0 spike, chose CLI wrapper for native dual-push support
2. **Hybrid State Management**: Git config authoritative, state file for metadata
3. **Vault with Caching**: 24h cache enables offline operation
4. **TTY Detection**: Automatic JSON output for pipes/scripts
5. **Hook Backup**: Auto-backup existing hooks before installation

See [GITHELPER_PLAN_V3.md](GITHELPER_PLAN_V3.md) for complete architectural decisions.

## Phase 1 Accomplishments

✅ **Core Infrastructure**:
- Cobra CLI scaffolding with subcommands
- Vault integration with 24h caching
- State file management (~/.githelper/state.yaml)
- TTY detection and dual output formats
- Git CLI wrapper with all basic operations
- Repository creation with type-specific initialization

✅ **Commands Implemented**:
- `githelper repo create <name> [--type TYPE] [--clone-dir DIR]`
- `githelper repo list [--format human|json]`

✅ **Tested and Working**:
- Bare repository creation
- Local cloning
- Initial commit and push
- Go repository initialization
- State tracking
- Configuration caching
- Output formatting

## Coming in Phase 2

- GitHub API integration (go-github)
- SSH key retrieval from Vault
- Dual-push remote configuration
- `githelper github setup` command
- Hook installation (pre-push, post-push)
- Repository-local SSH configuration

## Development

### Build and Test

```bash
# Build
make build

# Run tests
make test

# Clean
make clean

# Install to GOPATH/bin
make install
```

### Testing

See [docs/PHASE1_TESTING.md](docs/PHASE1_TESTING.md) for comprehensive testing guide.

## Contributing

This is an active development project. Phase 1 is complete and Phase 2 is next.

## License

[To be determined]

---

**Version**: 3.2 (Post-Phase 1)
**Status**: Phase 1 Complete, Ready for Phase 2
**Author**: lcgerke + Claude
