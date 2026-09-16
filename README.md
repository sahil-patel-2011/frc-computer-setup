# FRC Computer Setup

**One download. Check what you want. Click Install. The wizard does the rest.**

This is an orchestrator only. It does **not** ship WPILib, NI, Limelight, PathPlanner, or anyone else’s binaries. It downloads official latest stable files (SHA-256 or ETag + size).

Driver Station is **Windows-only**. There is no Vercel app for this installer.

## Run it

1. Download the file for **this** computer from [Releases](https://github.com/sahil-patel-2011/frc-computer-setup/releases/latest).
2. Run it. First screen is a **checklist**. Nothing installs unless you check it.
3. Click **Install**. Windows asks for **admin once**. After that: no per-app UAC, no vendor Next/Next, no extra yes/no.
4. Each checked tool installs silently (`/S`, `/VERYSILENT`, WPILib `--force` if the ISO has a CLI). Then a self-check (path/version). Fail → one silent retry → report.
5. If a vendor has no silent installer (NI Game Tools, Phoenix, REV, Limelight Windows, Git on mac/Linux, WPILib without a CLI), that row is **needs vendor page** — we do not pop junk dialogs.

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

CLI (nothing runs unless you pass ids):

```text
go run ./cmd/frc-computer-setup --cli --tools=git,wpilib
```

## 6925 stack

Check the ones you want: Git, WPILib, Driver Station (Windows / vendor page), PathPlanner, AdvantageScope, Choreo, Elastic, Limelight Hardware Manager, Phoenix Tuner X (vendor page), optional Microsoft VS Code, optional public clone of `6925-RobotCodeUpdated`.

VS Code is **not** asked again after the checklist. WPILib already ships VS Code — check Microsoft VS Code only if you want a second copy.

PhotonVision is **not** installed — 6925 robot code uses Limelight.

NI / Limelight OS images / WPILib ISOs are never packed in this git repo.

## Refresh pins

```text
GITHUB_TOKEN=… go run ./cmd/refresh-manifest
```

GitHub `releases/latest` (stable) + Microsoft VS Code `latest/stable` + proven WPILib installer URLs from release notes. No invented versions.
