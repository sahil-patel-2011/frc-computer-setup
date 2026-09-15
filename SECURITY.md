# Security

This repo is an **orchestrator**. It does not vendor, repack, or ship NI, WPILib, PathPlanner, AdvantageScope, Choreo, Git, or other vendor binaries.

## What we download

- Only HTTPS URLs from the pinned manifest (`manifest/tools.json`).
- GitHub `releases/latest` (stable / non-prerelease) or a vendor page when a latest URL cannot be proven.
- After download: SHA-256 when the vendor published one; otherwise ETag + size. No checksum and no ETag → we **do not guess**. The wizard opens the official vendor page instead.

## What we never do

- Invent version numbers.
- Mirror vendor installers in GitHub Releases of this repo.
- Disable TLS verification.
- Run a random script from the internet. The only executable we ship is this wizard.

If a checksum fails, the file is deleted and setup stops on that step.
