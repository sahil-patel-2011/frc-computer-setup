# FRC Computer Setup

One Windows wizard. One new FRC programming laptop.

This repo does **not** ship WPILib, NI Game Tools, PathPlanner, or anyone else’s installer. It downloads the official latest files (checksums / ETags) and walks you through them.

## Run it

1. Download **`FRC-Computer-Setup.exe`** from [Releases](https://github.com/sahil-patel-2011/frc-computer-setup/releases/latest).
2. Double-click it.
3. Click **Start**.
4. Stay with it. Each tool is: **download → install → we check it → next**.

That’s the whole job.

Windows 10/11, 64-bit. Driver Station is Windows-only.

If SmartScreen complains: More info → Run anyway. The file is our orchestrator, not a vendor binary.

## What it installs

| On by default | What |
| --- | --- |
| Git | Official Git for Windows |
| WPILib | Official ISO (VS Code, JDK, FRC tools) |
| FRC Driver Station / NI Game Tools | Opens NI’s official page (no public latest URL to pin) |
| PathPlanner | Official Windows setup |
| AdvantageScope | Official Windows setup (newer than the WPILib copy) |
| Choreo | Official Windows setup |

Optional checkboxes: Elastic, REV Hardware Client, Phoenix Tuner X, LabVIEW, old OM5P radio tool. Vendor pages when we cannot prove a latest URL — we never invent versions.

## Preview without Windows

```text
go run ./cmd/frc-computer-setup --demo
```

Opens the same walkthrough. Does not download vendor installers.

## Build the Windows exe

```text
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w -H windowsgui" -o dist/FRC-Computer-Setup.exe ./cmd/frc-computer-setup
```

CI does this on every push. The GitHub Release attaches only that exe — never vendor zips.

## Refresh pins

```text
GITHUB_TOKEN=… go run ./cmd/refresh-manifest
```

Pulls GitHub `releases/latest` (stable only). If a latest URL cannot be proven, that tool becomes a vendor step.

## Rules we will not break

- No vendor binaries in this git repo or its release zip.
- HTTPS + SHA-256 (or ETag + size). Mismatch → delete the file and stop that step.
- NI / LabVIEW / Store apps stay vendor pages until they publish a pin-able latest URL.
