# inl-cli

Industrial Netline CLI (NRC protocol) — AI-native PROFINET configuration tool for industrial PCs running `nrc2.out`.

This is the **npm thin-shell wrapper** for [inl](https://github.com/hqy2435662352/inl). The actual binary is downloaded from the platform-specific sub-package (`@inl/cli-<os>-<arch>`).

## Install

```bash
npm install -g inl-cli
```

Then verify:

```bash
inl --version
# 输出: inl version 0.1.0
```

## Quick Start

```bash
# 1. AI self-discovery
inl schema list

# 2. Read GSD device drivers
inl --target 192.168.3.15 gsd list

# 3. DCP discover online devices
inl --target 192.168.3.15 topology scan --interface enp4s0

# 4. Write config (requires --yes)
inl --target 192.168.3.15 config add-device --yes
```

## Supported Platforms

| OS | Arch | Package |
|---|---|---|
| Linux | x64 | `@inl/cli-linux-x64` |
| Linux | arm64 | `@inl/cli-linux-arm64` |
| macOS | x64 | `@inl/cli-darwin-x64` |
| macOS | arm64 | `@inl/cli-darwin-arm64` |
| Windows | x64 | `@inl/cli-win32-x64` |
| Windows | arm64 | `@inl/cli-win32-arm64` |

The 6 platform sub-packages are `optionalDependencies`. npm will install only the one matching your platform.

## How It Works

1. `npm install -g inl-cli` installs the entry package + 1 platform sub-package
2. `postinstall` script (`install.js`) copies the platform binary to `npm/inl-cli/bin/inl`
3. Running `inl ...` spawns `npm/inl-cli/bin/inl.js` → `npm/inl-cli/bin/inl` (real binary)

## Documentation

- Repository: <https://github.com/hqy2435662352/inl>
- Source docs: see `AGENTS.md` and `docs/` in the repository

## License

MIT
