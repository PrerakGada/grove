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

## Status

Working: `init` (detection + wizard), `ports`, `doctor`.

Not built yet: `create`, `track`, `delete`, `setup`, `up`, `down`, `ps`,
`logs`, `db`, `dev`. The reference implementation for all of those is the bash
predecessor at `web/monorepo/.claude/scripts/wt.sh`.

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
