# Grove

Parallel dev environments from git worktrees. Every worktree gets reserved
ports, its own dependencies, its own database, and a background runner — so
many branches run at once without a terminal each.

```bash
cd your-repo
grove init                 # inspects the repo, writes .grove.json, reserves port bands
grove create payments      # branch + worktree + deps + database, seeded and verified
grove up payments          # detached; no terminal held
grove ps                   # what is running, where
grove down payments
grove delete payments
```

## Install

```bash
make install    # links grove into ~/.local/bin
```

Requires Go 1.26+. macOS and Linux; on Windows use WSL2.

## Two files, one rule

| | what | committed? |
|---|---|---|
| `.grove.json` | the project's **shape** — services, ports, package manager, database | yes |
| `~/.grove/registry.json` | this **machine's** allocation — which project owns which band, which worktree holds which slot | no |

Nothing machine-specific ever lands in git, so a teammate clones the repo and
`grove` works without editing anything. Two people on the same project get
different slots without coordinating.

## Ports

Each service owns a 100-port band; a worktree's slot is a `+1` offset into it.
The main checkout is slot 0, so a band is also its own port and the trailing
digits of any port are always the slot.

```
          main (slot 0)   worktree n
web       3100            3100 + n
admin     3200            3200 + n
api       8200            8200 + n
db        5500            5500 + n
```

Bands are reserved **globally** in `~/.grove`, so a second project cannot claim a
range that is already spoken for — and neither can it land on 3000, 5173, 5432
or any other well-known default. `grove ports -g` shows every project on the
machine.

## Configuration

`grove init` detects package manager, workspaces, dev scripts, how each service
takes its port, Prisma and docker-compose, then writes `.grove.json`. Repos
without a single workspace root — a Flutter app beside a Node server, tied
together by `just` — are detected too. Edit the file directly afterwards; it is
the contract, detection is only a convenience.

```jsonc
{
  "project": "acme",
  "maxSlots": 99,
  "services": [
    { "name": "api", "kind": "backend", "band": 8200,
      "cmd": "pnpm --filter ./api run dev",
      "port": { "via": "env", "key": "PORT" } },
    { "name": "web", "kind": "frontend", "band": 3100,
      "cmd": "pnpm --filter ./web run dev",
      "port": { "via": "flag", "key": "--port" } }
  ],
  "deps": {
    "steps": [{ "dir": ".", "manager": "pnpm", "install": "pnpm install" }],
    "shareable": false,
    "seed": ["node_modules/.prisma/client"]
  },
  "env": {
    "files": [{
      "path": ".env",
      "set":       { "VITE_WEB_PORT": "{{port.web}}" },
      "fullstack": { "PORT": "{{port.api}}",
                     "DATABASE_URL": "postgresql://u:p@localhost:{{port.db}}/{{db.name}}" }
    }]
  },
  "db": {
    "driver": "docker-postgres", "band": 5500, "image": "postgres:16",
    "name": "acme_wt{{slot}}", "container": "pg-acme-wt{{slot}}",
    "profiles": { "seed": "pnpm db:seed-baseline" }, "default": "seed",
    "verify": { "minTables": 10, "minRows": 1 }
  }
}
```

Values are templated with `{{port.<service>}}`, `{{slot}}`, `{{name}}`,
`{{db.name}}`, `{{main}}`, `{{worktree}}`. Unknown variables are a hard error, so
a typo fails at provisioning time rather than writing itself into a `.env`.

`set` applies in every mode; `fullstack` only when the worktree owns its backend
and database. Assignments are computed, never substituted from the main
checkout's current values — so changing the main ports can never leave worktrees
quietly pointing at them.

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

`delete` refuses to run when a worktree has uncommitted changes or unpushed
commits — worktree removal discards them — and says which flag overrides it.

`down` kills in three passes: recorded pids, then the slot's ports (dev servers
fork children that outlive their parent), then anything whose working directory
is inside the worktree. The third catches servers started before a port change
or by hand, which the first two miss.

## Status

Pre-1.0 and in use on two projects. Not yet: tests, a brew tap or Scoop bucket,
native Windows. See `CLAUDE.md` for the design rationale, the failure modes each
decision encodes, and the current gaps.

## Develop

```bash
make build   # bin/grove
make vet
make cross   # proves the platform stubs still compile for darwin/linux/windows
```
