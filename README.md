# FRC Computer Setup

**One download. One walkthrough. Official installers only.**

This is a setup wizard for a new FRC programming computer. It does **not** ship WPILib, NI Game Tools, PathPlanner, AdvantageScope, Choreo, Git, or anyone else’s binaries. It downloads the official latest files (SHA-256 / ETag) and walks you through them.

Driver Station is **Windows-only**. On a Mac or Linux box the wizard says so and skips it.

## Run it (that’s the whole job)

1. Download the file for **this** computer from [Releases](https://github.com/sahil-patel-2011/frc-computer-setup/releases/latest).
2. Run it.
3. Click **Start**.
4. Stay with it. Each tool is: **download → install → we check it → next**.

| This computer | Download this |
| --- | --- |
| Windows 10/11 x64 | `FRC-Computer-Setup.exe` |
| Windows ARM | `FRC-Computer-Setup-arm64.exe` |
| macOS Apple silicon | `frc-computer-setup-macos-arm64` |
| macOS Intel | `frc-computer-setup-macos-intel` |
| Linux x64 | `frc-computer-setup-linux-amd64` |
| Linux ARM | `frc-computer-setup-linux-arm64` |

Windows SmartScreen: More info → Run anyway. That file is our orchestrator, not a vendor installer.

macOS: if Gatekeeper blocks it, right-click → Open.

Linux: `chmod +x` the file, then run it.

Preview without installing anything:

```text
go run ./cmd/frc-computer-setup --demo
```

## What you get

| Tool | Windows | macOS | Linux |
| --- | --- | --- | --- |
| Git | Silent Git for Windows | Official git-scm page (often already installed) | Official git-scm page (often already installed) |
| WPILib | Official ISO → installer GUI | Official DMG → installer GUI | Official tar.gz → installer GUI |
| FRC Driver Station / NI Game Tools | Official NI page (no public latest URL) | **Windows only** | **Windows only** |
| PathPlanner | Silent Inno setup | Unattended copy into Applications | Unattended zip extract (x64; no ARM pin) |
| AdvantageScope | Silent NSIS `/S` | Unattended copy into Applications | Unattended AppImage → `~/.local/bin` |
| Choreo | Silent NSIS `/S` | Unattended copy into Applications | Unattended AppImage → `~/.local/bin` |

Optional checkboxes: Elastic, REV Hardware Client, Phoenix Tuner X, LabVIEW, old OM5P radio tool.

If we cannot prove a latest download URL, that step opens the **vendor page**. We never invent versions.

Windows ARM: WPILib does not publish an installer for that arch (their notes say Arm Windows is unsupported). The wizard will not fake one.

## Build

```text
# Windows x64 (typical programming laptop)
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w -H windowsgui" -o dist/FRC-Computer-Setup.exe ./cmd/frc-computer-setup

# This computer
go build -o dist/frc-computer-setup ./cmd/frc-computer-setup
```

CI builds every OS/arch above. GitHub Releases attach **only those orchestrator files** — never vendor zips.

## Refresh pins

```text
GITHUB_TOKEN=… go run ./cmd/refresh-manifest
```

Pulls GitHub `releases/latest` (stable only), per OS/arch. Missing asset → that platform stays a vendor step. No guessing.

## Rules we will not break

- No vendor binaries in this git repo or its GitHub Release.
- HTTPS + SHA-256 (or ETag + size). Mismatch → delete the file and stop that step.
- Silent/unattended flags only when the vendor actually supports them. WPILib stays a GUI installer.
- NI / LabVIEW / Store apps stay vendor pages until they publish a pin-able latest URL.
