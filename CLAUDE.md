# Grove — Claude Code Instructions

Grove is a project-agnostic CLI that gives every git worktree a fully provisioned
dev environment: reserved ports, its own dependencies, its own database, and a
background runner — so many branches run at once without a terminal each.

Go module `github.com/PrerakGada/grove`. The binary is **`grove`**. ~6,000 lines.

It is the rewrite of a 1,850-line bash predecessor. **That script is the
specification** — every non-obvious behaviour here exists because the bash
version got it wrong first, in daily use, on a machine running eight worktrees.
Read the "Failures this encodes" section before changing anything that looks
over-engineered.

---

## The one architectural rule

| | holds | committed? |
| --- | --- | --- |
| `.grove.json` in the repo | the project's **shape** — services, port bands, package manager, database, env files | **yes** |
| `~/.grove/registry.json` | **this machine's** allocation — which project owns which band, which worktree holds which slot | **no** |

Nothing machine-specific may leak into `.grove.json`, and nothing project-shaped
may live only in the registry. A teammate clones the repo and `grove` works with
no editing; two people on the same project get different slots without conflict.

`~/.grove` is overridable with `GROVE_HOME` (used by tests).

---

## Package map

| package | owns |
| --- | --- |
| `cmd/` | one file per command, cobra. `root.go` has `repoRoot()` / `loadConfig()`. |
| `internal/config` | `.grove.json` schema, `{{...}}` templating, validation |
| `internal/registry` | `~/.grove`, band reservation, slot allocation, flock |
| `internal/detect` | repository inspection → proposed config (`detect.go` = workspace repos, `polyglot.go` = everything else) |
| `internal/provision` | env files, symlinks, dependency isolation, copy-on-write seeding, markers |
| `internal/dbx` | the single verified path to a populated database |
| `internal/runner` | detached start, three-pass stop, status |
| `internal/worktree` | reads `git worktree list` + markers + change counts |
| `internal/platform` | **every** OS-specific operation. Nothing else may shell out to `lsof`, `pgrep`, `cp -c`, or touch `syscall`. |
| `site/` | the landing page (Next.js + Tailwind, deployed to Vercel). Not part of the Go build. |

Per-worktree markers, constants in `internal/config`: `.grove-mode`
(`frontend`|`fullstack`), `.grove-deps` (`real`|`light`), `.grove-db-profile`,
`.grove-run/` (pids + logs).

---

## Failures this encodes — do not "simplify" these away

**Ports: each service owns a 100-wide band; the slot is a `+1` offset; the main
checkout is slot 0.** The bash version used `band + slot*100`, which fanned one
worktree across 3000–6900 and collided with the database band at slot 25 — a hard
24-worktree ceiling. With bands, the trailing digits of any port are always the
slot. Bands are reserved **globally** in `~/.grove`, which is the only thing
stopping two repositories both claiming 3100.

**Env files are computed `key=value` assignments, never textual substitution.**
The bash version `sed`-ed the main checkout's *literal* port numbers into each
worktree. When the main ports changed, the patterns silently stopped matching and
every worktree kept pointing at the main backend and database. `provision/env.go`
rewrites keys in place, preserving order, comments and untouched keys.

**A port key used by more than one service cannot live in a shared env file.**
One file holds one value per key. The runner injects those per process at launch
(`runner.Command`). Only uniquely-owned keys are written to disk.

**A CLI port flag beats the environment variable.** Next.js `-p` overrides `PORT`,
so a project configured only through the environment binds whatever its script
hardcodes, in every worktree, forever. Hence `PortBinding{Via: env|flag}`.

**`down` needs three passes, and the third is the one everyone forgets:**
1. recorded pids;
2. anything holding the slot's current ports — dev servers fork children that
   outlive the process in the pid file;
3. anything whose **working directory** is inside the worktree — passes 1 and 2
   only cover ports the slot owns *now*, so a server started before a port change,
   or by hand, survives both. Proven: a vite on a pre-migration port outlived a
   full `wt down` of its own worktree.

**One compose file, and prove the database is not empty.** The bash version had
two generators emitting the same container name on the same port — one seeded, one
empty — so whichever ran last won, and the convenient command produced the empty
one. `dbx.AssertPopulated` is a hard error, not a warning.

**The data volume mount path is version-dependent (`db.DataMount`).** Postgres 18
moved PGDATA to `/var/lib/postgresql/<major>/docker`. Mounting the volume at the
historical `/var/lib/postgresql/data` on an 18 image is not an error — it creates
an unused empty directory while the real data sits in the container's writable
layer. Everything works until the container is recreated, and then the database is
silently empty and `VolumeExists()` still says it is not fresh, so no restore
happens either. Derived from the image tag, overridable with `db.volumeMount`.

**`db.initDir` exists because roles are cluster state that `pg_dump` never
carries.** A project whose privilege model lives in `docker-entrypoint-initdb.d`
(two-role Postgres RLS, say) gets a container that starts, passes its healthcheck
and restores a dump — while missing every role the dump's `ALTER ... OWNER` and
`GRANT` statements refer to. The failure surfaces as a working psql and a 401ing
API. When `initDir` is set the scripts OWN database creation, so `POSTGRES_DB`
stays `postgres` and their `CREATE DATABASE` cannot collide with the entrypoint's
(`db.BootstrapDB`). `db.env` feeds them their passwords and database name.

**Dependencies are per-worktree by default.** A Prisma-style generator with no
`output` writes into `node_modules`, so a shared tree means one `generate`
rewrites the client for the main checkout and every worktree. Vite's pre-bundle
cache lives there too, so parallel dev servers on different branches fight over
one cache. `--shared-deps` is an escape hatch, allowed only when
`deps.shareable` is true.

**Copy-on-write is a volume property, not a filesystem guarantee.** pnpm clones
from its store only within one filesystem; across volumes the same install
silently becomes full copies. `provision.cowViable` preflights and downgrades
rather than eating the disk. Note `du` cannot see shared blocks and reports many
times the real cost — measure with `df`.

**`prune` must free slots.** A registry that only hands slots out shrinks the pool
on every delete until it runs out for no visible reason.

**`delete` must refuse to destroy work.** Removal uses `git worktree remove
--force` internally, so it checks for uncommitted changes and unpushed commits
first. The bash version forced unconditionally *and* deleted the branch by default.

---

## Detection rules that must not regress

`internal/detect` proposes; the config decides. But these were each found by
running against a real repository, and re-breaking them is easy:

- **Band assignment is two-pass.** Services with a known current port claim their
  band first; leftovers take free bands. One pass lets whichever service iterates
  first steal a band another already uses and shuffles every port in the project.
- **Services whose current ports share one 100-block all get fresh bands.**
  Otherwise one keeps a familiar number while its siblings scatter.
- **Resolve symlinks before treating a file as a service's own env file.** A
  monorepo commonly links every workspace's `.env` back to one root file. Treating
  those as distinct had six services in one monorepo read the same `PORT` from that
  one file, collide, and all move off their correct bands.
- **Credentials come only from committed compose files**, resolving
  `${VAR:-default}` to its inline default. `.grove.json` is committed, so it must
  never absorb a secret the repo keeps out of git.
- **Compose port specs split on the LAST colon.** `${DB_PORT:-25446}:5432`
  contains a colon inside the substitution.
- **`POSTGRES_DB` is often just the bootstrap database** — prefer another
  `*_DB_NAME` the compose file declares.
- **Reserved-port overlaps are soft.** A band containing 3306 or 5173 bites only
  at the one slot whose offset lands there. Warn; never silently relocate a band
  the project already uses. A collision with another *project* is hard.

---

## The landing page

`site/` argues for the tool by showing it. Two rules make that work:

- **Every terminal block is captured, not written.** They live in
  `site/src/lib/content.ts`, taken from a real binary run against a scratch
  monorepo, with only the checkout path rewritten. If output formatting
  changes, re-capture — do not hand-edit a string to match.
- **The palette is the CLI's palette.** `internal/ui/ui.go` styles output with
  ANSI 1–6; the site's tokens are those five roles and nothing else. A new
  colour on the site means a new colour in the CLI first.

The page also states the gaps — no tests, Postgres-only, WSL2 on Windows —
because the "Known gaps" list below is the honest version and the site must not
drift from it.

## Conventions

- `make build` · `make install` (symlinks into `~/.local/bin`) · `make vet` ·
  `make cross` (proves the platform stubs still compile for darwin/linux/windows).
- Run `gofmt -w .` and `go vet ./...` before every commit. `go vet` has caught
  real bugs here (a format verb with no argument).
- Dependencies are deliberately few: cobra, charmbracelet/huh + lipgloss,
  yaml.v3. Do not add more without a reason.
- OS-specific code goes in `internal/platform` behind a build tag. Nothing else.
- Errors are user-facing. Say what went wrong *and* the command that fixes it.

---

## Current state

**All 15 commands built:** init, create, track, setup, delete, db, up, down, ps,
logs, dev, list, ports, doctor, prune.

**Adopted on two real projects**, both registered in `~/.grove`: a pnpm/Turbo
monorepo (six services + Postgres, where detection reproduced a hand-written port
scheme exactly), and a polyglot repository (Flutter app + NestJS API + a pnpm
codegen package, tied together by `just`) — which is what forced `deps.steps`,
`env.files` and sub-project database discovery to exist.

**Verified end-to-end in a scratch repo** rather than against either of those, since
a real `create` there costs ~500 MB of dependencies plus a database restore. create installs and
allocates; up binds and reparents to PID 1; ps/logs/dev work; the delete guards
fire; forced delete frees slot and branch; prune reclaims an orphan. The
three-pass kill was proven explicitly: a hand-started server on a port the slot
does *not* own died via the cwd sweep, while a server in the main checkout
survived.

### Known gaps

- **No tests.** 0 `_test.go` files against ~6,000 lines. `internal/config`
  (templating, validation, band overlap) and `internal/detect` are pure functions
  with obvious table tests — that is the highest-value next work.
- **Windows is stubbed on purpose.** `internal/platform/platform_windows.go`
  documents what a native port needs; `ProcCwd` is the blocker (no public API —
  it needs `NtQueryInformationProcess`). WSL2 is the answer, and a repo relying on
  symlinks needs Developer Mode on native Windows anyway.
- **Not distributed.** No GoReleaser config, no brew tap, no Scoop bucket, no
  git remote. `make install` is the only install path.
- **`Links` is implemented but nothing populates it.** Detection never proposes
  symlinks, so gitignored root directories a worktree needs (an agent config
  directory, a framework directory) have to be added to `.grove.json` by hand.
- **Legacy adoption is read-only.** `grove list` falls back to the old tool's
  `worktrees/.slots` and marks those rows `*`, but nothing imports them into the
  registry. Existing worktrees need `grove setup <name>` to be fully managed.

### Migrating off the bash predecessor

Two hand-written bash versions of this tool are still in circulation, each with
its own `worktrees/.slots` file. Those are independent allocators: running one
alongside Grove hands the same slot different ports, which is the exact collision
Grove exists to prevent. `grove list` reads a legacy `.slots` and marks those rows
`*` so the state is visible, but nothing imports them — an existing worktree needs
`grove setup <name>` to become managed, and the old script should then be deleted.
