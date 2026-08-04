# Grove

Parallel dev environments from git worktrees. Every worktree gets reserved
ports, its own dependencies, its own database, and a background runner — so
many branches run at once without a terminal each.

```
grove init          # detect this repo, write .grove.json, reserve port bands
grove ports         # the allocation table
grove ports -g      # every project registered on this machine
grove doctor        # config + registry + machine checks
```


## Install

```
make install    # links grove into ~/.local/bin
```

## Two files, one rule

| | what | committed? |
|---|---|---|
| `.grove.json` | the project's **shape** — services, ports, package manager, database | yes |
| `~/.grove/registry.json` | this **machine's** allocation — which project owns which band, which worktree holds which slot | no |

Nothing machine-specific ever lands in git, so a teammate clones the repo and
`grove` works without editing anything.

## Ports

Each service owns a 100-port band; a worktree's slot is a `+1` offset into it.
The main checkout is slot 0, so a band is also its own port and the trailing
digits of any port are always the slot.

```
          main (slot 0)   worktree n
web  3100            3100 + n
admin   3200            3200 + n
backend   8200            8200 + n
db        5500            5500 + n
```

Bands are reserved **globally** in `~/.grove`, so a second project cannot claim a
range that is already spoken for — and neither can it land on 3000, 5173, 5432
or any other well-known default.

## Commands

```
grove init                    detect this repo, write .grove.json, reserve bands
grove create <name> [base]    new branch + worktree, provisioned and seeded
grove track <branch> [name]   check out an existing branch (local or remote)
grove setup <name>            re-provision: repair ports/links, convert mode or deps
grove delete <name>           remove worktree, database, slot and branch
grove db <name> [profile]     provision its database, verified non-empty
grove up <name> [service...]  start detached
grove down <name>             stop (pids, ports, and working directory)
grove ps                      what is running
grove logs <name> <service>   captured output
grove dev [service...]        run in the foreground, on this worktree's ports
grove list                    worktrees, slots, modes, what is listening
grove ports [-g]              port table; -g for every project on this machine
grove doctor                  config, registry and machine checks
grove prune                   drop stale worktrees and free their slots
```

`create` defaults to a fully isolated worktree: own dependencies, own backend
port, own database seeded and **verified non-empty before the command returns**.
`--frontend` shares the main checkout's backend and database instead;
`--shared-deps` symlinks dependencies where the project allows it.

## Platforms

macOS and Linux. Windows is stubbed — use WSL2, which is what a repository
relying on symlinks needs there anyway. See `internal/platform` for what a
native port would take; `ProcCwd` is the hard one.

## Develop

```
make build   # bin/grove
make vet
make cross   # proves the platform stubs still compile for darwin/linux/windows
```
