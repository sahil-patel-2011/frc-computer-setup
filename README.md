# FRC Computer Setup

**One download. Official latest only. Detect first — do not blindly reinstall.**

This wizard sets up an FRC programming computer (Team 6925 stack). It does **not** ship WPILib, NI, Limelight, PathPlanner, or anyone else’s binaries. It downloads the official latest stable files (SHA-256 or ETag + size).

Driver Station is **Windows-only**.

## Run it

1. Download the file for **this** computer from [Releases](https://github.com/sahil-patel-2011/frc-computer-setup/releases/latest).
2. Run it. Click **Start**.
3. We **check what is already installed**. Current → skip. Older/unknown → **Update** or keep it.
4. **Install VS Code?** Default **no**. WPILib already ships VS Code. Check the box only if you want a separate Microsoft VS Code.

| This computer | Download this |
| --- | --- |
| Windows 10/11 x64 | `FRC-Computer-Setup.exe` |
| Windows ARM | `FRC-Computer-Setup-arm64.exe` |
| macOS Apple silicon | `frc-computer-setup-macos-arm64` |
| macOS Intel | `frc-computer-setup-macos-intel` |
| Linux x64 | `frc-computer-setup-linux-amd64` |
| Linux ARM | `frc-computer-setup-linux-arm64` |

Preview (no vendor downloads):

```text
go run ./cmd/frc-computer-setup --demo
```

## 6925 stack (get robot code working)

On by default: Git, WPILib, Driver Station (Windows / NI page), PathPlanner, AdvantageScope, Choreo, Elastic, Limelight Hardware Manager, Phoenix Tuner X (vendor page).

Ask first: **VS Code** (default off). Optional: clone public `6925-RobotCodeUpdated` after Git works (no secrets). REV Hardware Client is a vendor page if you need SPARKs.

PhotonVision is **not** installed — 6925 robot code uses Limelight.

NI / Limelight OS images / WPILib ISOs are never packed in this git repo.

## Refresh pins

```text
GITHUB_TOKEN=… go run ./cmd/refresh-manifest
```

GitHub `releases/latest` (stable) + Microsoft VS Code `latest/stable` + proven WPILib installer URLs from release notes. No invented versions.
