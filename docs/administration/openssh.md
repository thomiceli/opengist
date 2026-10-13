# Serve Git over SSH with OpenSSH

Opengist can serve git over SSH (clone, pull, push) in two ways, selected with
the [`ssh.git-enabled`](/docs/configuration/cheat-sheet) config key:

- **`builtin`** (default): Opengist runs its own embedded SSH server, by default
  on port `2222`. Self-contained, works in Docker and rootless setups, and needs
  no extra configuration. This is recommended for most installs.
- **`host`**: Opengist delegates SSH access to the machine's own OpenSSH server
  (`sshd`), so clients connect on the standard port `22` through the same SSH
  daemon that already runs on the host. Best for installs where you
  want a single SSH entry point, that can be shared with other services like Gitea.
- **`disabled`**: no SSH git access at all.

This page covers the **`host`** mode.

## Requirements

Host mode runs an Opengist subcommand from inside `sshd`, so a few things must be
true:

- **`sshd`, Opengist and the gist repositories share the same machine and OS user.**
  The connection authenticates as a single OS account (e.g. `opengist` or `git`),
  and git runs directly against the repositories on disk. A split setup -
  host `sshd` in front of a containerized Opengist - is **not** supported, because
  the git data must be on the same filesystem.
- **Opengist is started with a config file via `--config`** (for example
  `opengist --config /etc/opengist/config.yml`). Host mode resolves the daemon's
  settings through that file when `sshd` invokes Opengist; a pure environment-only
  configuration is not supported in this mode.
- A **dedicated OS user** for Opengist is strongly recommended.
- Users have added their SSH keys in Opengist as usual (**Settings → SSH keys**);
  the keys come from Opengist's database.

## How it works

When a client connects, `sshd` has to (1) recognize the public key and (2) limit
what the session may do. Opengist plugs into both:

- **Key lookup** - `sshd` asks Opengist whether the offered key belongs to an
  Opengist user.
- **Forced command** - the matching `authorized_keys` entry pins the session to
  `opengist shell <key-id>`. The connection can do nothing but run a single git
  command, which Opengist authorizes against the target gist before streaming it.
  Shells, port forwarding, PTYs, etc. are all denied.

There are two ways to wire this up. Pick **one**.

## Option A - `AuthorizedKeysCommand`

`sshd` calls Opengist on each connection to resolve the offered key. Nothing is
written to disk, and keys added or removed in the web UI take effect immediately.

1. Configure Opengist (`/etc/opengist/config.yml`):

   ```yaml
   ssh.git-enabled: host
   ssh.external-domain: gist.example.com   # host clients will SSH to
   ssh.port: "22"
   ssh.username: gist                      # the OS account clients log in as
   ```

2. Configure `sshd` (`/etc/ssh/sshd_config`). Use a `Match` block so only the
   Opengist account is affected:

   ```
   Match User gist
       AuthorizedKeysCommand /usr/local/bin/opengist --config /path/to/config.yml keys -t %t -k %k
       AuthorizedKeysCommandUser gist
   ```

   > `--config` is a global flag, so it must come **before** the `keys`
   > subcommand. `%t` and `%k` are the key type and content `sshd` passes in.

3. Reload `sshd`:

   ```shell
   sudo systemctl reload ssh
   ```

Notes:

- `sshd` requires the `AuthorizedKeysCommand` binary (and every parent directory)
  to be owned by `root` and not writable by group/others. A binary installed at
  `/usr/local/bin/opengist` satisfies this.
- `AuthorizedKeysCommandUser` should be the Opengist OS user, so the command can
  reach the running daemon and read its secret key.

## Option B - Managed `authorized_keys` file

Opengist maintains a managed block inside the OS user's `authorized_keys`. Use
this when you can't edit `sshd_config` (e.g. no `AuthorizedKeysCommand`), or
prefer a static file.

1. Configure Opengist:

   ```yaml
   ssh.git-enabled: host
   ssh.authorized-keys-file: /home/gist/.ssh/authorized_keys
   ssh.external-domain: gist.example.com
   ssh.port: "22"
   ssh.username: gist                  # the OS account clients log in as
   ```

2. Restart Opengist. It (re)generates the managed block:
   - whenever an SSH key or a user is added or removed,
   - and every 72 hours, in case the file drifts.

Opengist writes the file with the strict permissions `sshd` expects under
`StrictModes` (`.ssh` `0700`, `authorized_keys` `0600`). Only the block between
the markers is managed - any keys you add outside it are preserved:

```
# --- opengist managed keys start (do not edit) ---
command="/home/gist/.opengist/symlinks/opengist --config /home/gist/.opengist/symlinks/config.yml shell 1",no-port-forwarding,no-x11-forwarding,no-agent-forwarding,no-pty ssh-ed25519 AAAA... thomas@laptop
# --- opengist managed keys end ---
```

## Cloning

Clone URLs in the UI are built from `ssh.external-domain`, `ssh.port` and
`ssh.username`. Because OpenSSH authenticates by key rather than by Opengist
username, set `ssh.username` to the OS account clients log in as so the displayed
URL carries it:

```shell
git clone gist@gist.example.com:thomas/mygist.git
```

If you leave `ssh.username` empty the URL has no account name, and clients fall
back to their local username - usually not what you want in `host` mode. Either
set `ssh.username`, or have each client set it once in `~/.ssh/config`:

```
Host gist.example.com
    User gist
```

## Disabling SSH

Set `ssh.git-enabled: disabled` to turn off SSH git access entirely: the embedded
server won't start and SSH clone URLs are hidden in the UI.
