# Tailscale with WebSocket fallbacks

Builds of the open source [Tailscale](https://github.com/tailscale/tailscale)
client that can reach a control server through an HTTP proxy that only lets
WebSocket upgrades through. The case it was made for is a
[Headscale](https://headscale.net) server behind a Cloudflare Tunnel.

This is an unofficial fork for personal use. It is not made, endorsed, or
supported by Tailscale Inc. If you don't need the changes below, use the
[official client](https://tailscale.com/download).

## What is different

*   **WebSocket fallback for the control connection.** The client first tries
    Tailscale's own HTTP upgrade, as usual. If the proxy refuses it, the
    client connects over a WebSocket instead. Headscale accepts that already,
    so the server needs no change. Proposed upstream in
    [tailscale/tailscale#22643](https://github.com/tailscale/tailscale/pull/22643).
*   **WebSocket fallback for DERP** is linked in. It is upstream code that the
    official builds leave out.
*   **A system tray app for Windows** (`tailscale-systray.exe`). The official
    Windows app is not open source, so this build has upstream's tray app
    instead, changed to run on Windows, with notifications, a device list,
    and "Start at login".
*   **No self-updater.** `tailscale update` and auto-update are left out,
    because they install the official packages. Update with the install
    scripts below.

Everything else is the upstream release of the same version number.

## Install

Releases are on the
[releases page](https://github.com/NightFeather0615/tailscale/releases). The
same commands update an existing installation, and keep its login.

### Windows

64-bit x86 only. Uninstall the official Tailscale client first; your login is
kept. Then, in PowerShell:

```powershell
irm https://raw.githubusercontent.com/NightFeather0615/tailscale/fork/packaging/windows/install.ps1 | iex
```

The script downloads the latest release, checks its SHA-256, asks for
administrator rights, and installs to `Program Files\Tailscale` with
`tailscaled` as the `Tailscale` service. The binaries are not code signed.

To remove it, run these as administrator, then delete the directory:

```powershell
Stop-Process -Name tailscale-systray -ErrorAction SilentlyContinue
& "$env:ProgramFiles\Tailscale\tailscaled.exe" uninstall-system-daemon
```

### Linux

amd64 and arm64, with systemd:

```shell
curl -fsSL https://raw.githubusercontent.com/NightFeather0615/tailscale/fork/packaging/linux/install.sh | sudo sh
```

The binaries go to `/usr/local/bin` and `/usr/local/sbin`. If the `tailscale`
package of your distribution is installed, it stays, and a systemd drop-in
(`/etc/systemd/system/tailscaled.service.d/fork.conf`) makes its unit run the
fork build. Remove that file to go back.

### Android

See [NightFeather0615/tailscale-android](https://github.com/NightFeather0615/tailscale-android).

### First login

```shell
tailscale up --login-server https://headscale.example.com
```

## How releases are made

The [Release workflow](.github/workflows/release.yml) runs every day. When
upstream has a new stable release, it puts this fork's patches on top of the
release tag, runs the tests of the changed packages, builds, and publishes a
release named after the upstream version plus a build number, such as
`1.106.1-nf.1`.

A release holds the archives, a `SHA256SUMS` file, and a `.patch` file with
the changes made to the upstream source. Its tag points at this branch, which
holds the tooling. The source is the upstream tag plus that patch.

## Branches

| Branch | Holds |
| --- | --- |
| `fork` | This README, the release workflow, and the install scripts. |
| `stack` | The patches, as commits on top of an upstream release tag. |
| `controlwebsocket-upstream` | The upstream pull request. |
| `main` | A copy of upstream's `main`. |

## Maintaining

The workflow fails when the patches no longer apply to a new release. Then:

```shell
git fetch upstream --tags
git switch stack
git rebase --onto v1.108.0 v1.106.1   # new tag, then the tag stack was on
# resolve conflicts, build, test
git push --force-with-lease origin stack
gh workflow run release.yml
```

To change a patch between upstream releases, push the change to `stack` and
run the workflow with `rebuild` turned on. To try a build without publishing
it, turn `publish` off; the files are attached to the workflow run.

## License

BSD 3-Clause, the same as upstream; see [LICENSE](LICENSE). Tailscale is a
registered trademark of Tailscale Inc. WireGuard is a registered trademark of
Jason A. Donenfeld. The Windows archive includes `wintun.dll` under its own
licence, which is in the archive.
