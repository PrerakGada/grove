/*
  Everything on this page is real.

  The terminal blocks below are literal stdout, captured by running the binary
  from `go install github.com/PrerakGada/grove@latest` against a scratch pnpm
  monorepo. Only two cosmetic edits were made: the temporary checkout path was
  rewritten to ~/code/acme, and GROVE_HOME back to ~/.grove.
*/

export const REPO = "https://github.com/PrerakGada/grove";
export const INSTALL = "go install github.com/PrerakGada/grove@latest";

export const nav = [
  { href: "#ports", label: "Ports" },
  { href: "#lifecycle", label: "Lifecycle" },
  { href: "#config", label: "Config" },
  { href: "#failures", label: "Why" },
  { href: "#install", label: "Install" },
];

/* Verified: `find . -name '*.go' -not -path './site/*' | xargs wc -l` → 5964.
   "0 tests" is on the page on purpose — it is the most useful thing a reader
   could pick up, and hiding it would make every other number suspect. */
export const stats = [
  ["5,964", "lines of Go"],
  ["15", "commands"],
  ["2", "real projects"],
  ["0", "tests"],
] as const;

/* ---------------------------------------------------------------- terminals */

export const term = {
  create: `$ grove create payments
→ creating worktree "payments" on branch feat/payments (from main)
Preparing worktree (new branch 'feat/payments')
✓ env written (slot 1, fullstack mode)
→ installing pnpm in . ...
✓ dependencies installed — builds and code generation are safe here
→ starting database for "payments" on port 5501 ...
→ restoring the seed dump into acme_wt1 ...
→ applying pending migrations ...
✓ verified: 41 tables, ~128432 rows
✓ "payments" is serving seed data — acme_wt1 on localhost:5501

✓ "payments" is ready at ~/code/acme/worktrees/payments
→ slot 1 — api:8201  web:3101  db:5501

→ Start it: grove up payments`,

  up: `$ grove up payments
→ starting "payments" (fullstack) — [web]
✓   api up on 8201
✓   web up on 3101

→ Detached — close this terminal freely.  Status: grove ps   Logs: grove logs <name> <service>`,

  ports: `$ grove ports

acme  ~/code/acme

             band     api       web       db
  main       0        8200      3100      5500
  payments   1        8201      3101      5501
  auth-v2    2        8202      3102      5502
  refactor   3        8203      3103      5503

→ 3 of 99 slots in use. Bands: 3100, 5500, 8200`,

  list: `$ grove list

acme  ~/code/acme

  WORKTREE     BRANCH         SLOT  MODE       DIRTY  DEPS  RUNNING
  acme (main)  main           0     -          3      -     db:5500
  payments     feat/payments  1     fullstack  6      real  api:8201 web:3101
  auth-v2      feat/auth-v2   2     fullstack  6      real  api:8202 web:3102
  refactor     feat/refactor  3     fullstack  5      real  —`,

  ps: `$ grove ps

acme  ~/code/acme

  WORKTREE  SERVICE  PORT  PID    STATE
  auth-v2   api      8202  36056  up
  auth-v2   web      3102  36217  up
  payments  api      8201  35882  up
  payments  web      3101  36020  up

→ 4 service(s) running`,

  doctor: `$ grove doctor

✓ repository root: ~/code/acme
✓ .grove.json is valid — 2 services, maxSlots 99
✓ registered in ~/.grove — 3 slot(s) allocated
✓ port bands 3100, 5500, 8200 are clear of other projects and well-known ports
✓ pnpm store shares a volume with worktrees — installs stay copy-on-write

✓ no problems found`,

  init: `$ grove init

Detected

  root           ~/code/acme
  package manager pnpm
  workspaces     2
  env files      apps/api/.env
  database       docker-postgres (postgres:16, host port 5500)

  services
    api          backend   unknown  PORT           currently :8200
    web          frontend  next     -p (flag)      currently :3100

✓ wrote ~/code/acme/.grove.json
✓ reserved bands 3100, 5500, 8200 in ~/.grove

→ Next: \`grove create <name>\` to provision a worktree, \`grove ports\` to see the table.`,

  down: `$ grove down auth-v2
→ stopping "auth-v2" ...
✓   "auth-v2" stopped`,

  guard: `$ grove delete payments
✗ "payments" has 6 uncommitted change(s) — commit them, or re-run with --force`,
} as const;

/* ------------------------------------------------------------------ lifecycle */

export const lifecycle = [
  {
    cmd: "grove init",
    label: "Detect",
    blurb:
      "Inspects package manager, workspaces, dev scripts, how each service takes its port, Prisma and docker-compose. Proposes a config; you confirm it.",
    term: term.init,
  },
  {
    cmd: "grove create",
    label: "Provision",
    blurb:
      "Branch, worktree, env files, dependencies and a database — seeded and proven non-empty before the command returns.",
    term: term.create,
  },
  {
    cmd: "grove up",
    label: "Run",
    blurb:
      "Starts detached: new session, no controlling terminal, output to a per-service log, reparented to PID 1. Close the terminal; it keeps running.",
    term: term.up,
  },
  {
    cmd: "grove ps",
    label: "Observe",
    blurb:
      "What is running, on which port, under which pid — across every worktree at once.",
    term: term.ps,
  },
  {
    cmd: "grove down",
    label: "Stop",
    blurb:
      "Three passes: recorded process trees, whatever holds the slot's ports, then anything whose working directory is inside the worktree.",
    term: term.down,
  },
] as const;

/* -------------------------------------------------------------------- pillars */

export const pillars = [
  {
    k: "Ports",
    h: "Every port is band + slot",
    b: "Each service owns a 100-wide band, each worktree owns one slot, the main checkout is slot 0. The trailing digits of any port are always the slot number.",
  },
  {
    k: "Isolation",
    h: "Its own dependencies",
    b: "A generator with no output path writes into node_modules. Per-worktree installs by default, so one generate cannot rewrite every branch's client at once.",
  },
  {
    k: "Database",
    h: "Proven, not assumed",
    b: "A restore is not finished until it is counted. Grove runs ANALYZE, checks tables and rows, and fails hard with the exact reseed command if it came back empty.",
  },
  {
    k: "Runner",
    h: "Nothing holds a terminal",
    b: "Services start fully detached and reparent to PID 1. Eight branches run at once; your terminal stays free.",
  },
] as const;

/* ------------------------------------------------------------------- failures */

export const failures = [
  {
    n: "01",
    src: "internal/registry/registry.go",
    title: "The 24-worktree ceiling",
    naive: "Compute a worktree's ports by multiplying its slot: band + slot × 100.",
    broke:
      "One worktree fanned out across 3000–6900, and at slot 25 it collided with the database band. A hard 24-worktree ceiling, on a machine already running eight.",
    fix: "Each service owns a 100-wide band and the slot is a +1 offset. Bands are reserved globally in ~/.grove — the only thing stopping two repositories both claiming 3100.",
  },
  {
    n: "02",
    src: "internal/provision/env.go",
    title: "The sed that silently stopped matching",
    naive: "sed the main checkout's literal port numbers into each worktree's env files.",
    broke:
      "When the main checkout's ports changed, the patterns quietly stopped matching. No error — every worktree kept pointing at the main backend and the main database.",
    fix: "Env files are computed key=value assignments, never textual substitution. Keys are rewritten in place, preserving order, comments and every key Grove was not asked to touch.",
  },
  {
    n: "03",
    src: "internal/runner/runner.go",
    title: "The vite that survived its own kill",
    naive: "Stop a worktree by killing the pids you recorded when you started it.",
    broke:
      "Dev servers fork children that outlive the pid file. And pids plus current ports only cover what the slot owns now — a vite left on a pre-migration port outlived a full teardown of its own worktree.",
    fix: "down kills in three passes, and the third — anything whose working directory is inside the worktree — is the one everyone forgets. It was proven explicitly: a hand-started server on a port the slot does not own died; a server in the main checkout survived.",
  },
  {
    n: "04",
    src: "internal/dbx/dbx.go",
    title: "Two generators, one container name",
    naive:
      "Two compose generators, both emitting the same container name on the same port — one seeded, one empty.",
    broke: "Whichever ran last won. The convenient command was the one that produced the empty database.",
    fix: "One compose file, and the database is proven non-empty before the command returns. AssertPopulated is a hard error, not a warning.",
  },
  {
    n: "05",
    src: "internal/config/config.go",
    title: "The flag that beats the environment",
    naive: "Configure every service's port through environment variables.",
    broke:
      "Next.js -p overrides PORT. A project wired only through the environment binds whatever its dev script hardcodes — in every worktree, forever.",
    fix: "Ports declare how they bind. via: env or via: flag, per service.",
  },
  {
    n: "06",
    src: "internal/detect/detect.go",
    title: "Six services reading one PORT",
    naive: "Treat each workspace's .env as that service's own env file.",
    broke:
      "A monorepo commonly symlinks every workspace's .env back to one root file. Treated as distinct, six services read the same PORT from that single file, collided, and all moved off their correct bands.",
    fix: "Detection resolves symlinks before treating a file as a service's own env file.",
  },
  {
    n: "07",
    src: "internal/provision/provision.go",
    title: "Copy-on-write is a volume property",
    naive: "Assume cloning from the package store is free because the filesystem supports it.",
    broke:
      "pnpm clones from its store only within one filesystem; across volumes the same install silently becomes full copies. And du cannot see shared blocks — it reports many times the real cost, so the problem is invisible from the obvious tool.",
    fix: "Grove compares the store's device id against the worktree's and downgrades rather than eating the disk. Measure with df, not du.",
  },
  {
    n: "08",
    src: "cmd/delete.go",
    title: "--force by default",
    naive: "Force worktree removal unconditionally, and delete the branch too.",
    broke: "Worktree removal discards uncommitted changes; deleting the branch discards unpushed commits. Both, with no prompt.",
    fix: "delete checks for uncommitted changes and unpushed commits first, refuses, and names the flag that overrides it.",
  },
] as const;

/* ------------------------------------------------------------------- commands */

export const commands = [
  ["init", "detect this repo, write .grove.json, reserve bands"],
  ["create <name> [base]", "new branch + worktree, provisioned and seeded"],
  ["track <branch> [name]", "check out an existing branch, local or remote"],
  ["setup <name>", "re-provision: repair ports and links, convert mode or deps"],
  ["delete <name>", "remove worktree, database, slot and branch"],
  ["db <name> [profile]", "provision its database, verified non-empty"],
  ["up <name> [service...]", "start detached"],
  ["down <name>", "stop — pids, ports, and working directory"],
  ["ps", "what is running"],
  ["logs <name> <service>", "captured output"],
  ["dev [service...]", "run in the foreground, on this worktree's ports"],
  ["list", "worktrees, slots, modes, what is listening"],
  ["ports [-g]", "port table; -g for every project on this machine"],
  ["doctor", "config, registry and machine checks"],
  ["prune", "drop stale worktrees and free their slots"],
] as const;

/* --------------------------------------------------------------------- honest */

export const honest = [
  {
    h: "No tests yet",
    b: "Zero _test.go files against ~6,000 lines. config and detect are pure functions with obvious table tests — that is the highest-value next work, and the most useful thing to contribute.",
  },
  {
    h: "Postgres via Docker only",
    b: "docker-postgres is the one implemented driver. MySQL, SQLite and Mongo are not supported. Migrations are best-effort: a failure warns and continues.",
  },
  {
    h: "Windows is stubbed on purpose",
    b: "Reading another process's working directory has no public Win32 API — it needs NtQueryInformationProcess. WSL2 is the answer today, and the stub documents what a native port would take.",
  },
  {
    h: "Detection proposes, it does not decide",
    b: "It is strongest on JavaScript workspace monorepos. The polyglot path covers Node and Flutter sub-projects tied together by a justfile; anything else means writing .grove.json by hand. It is a contract, not a guess.",
  },
] as const;

export const configExample = `{
  "project": "acme",
  "maxSlots": 99,
  "services": [
    { "name": "api", "kind": "backend", "band": 8200,
      "cmd": "pnpm --filter ./apps/api run dev",
      "port": { "via": "env", "key": "PORT" } },
    { "name": "web", "kind": "frontend", "band": 3100,
      "cmd": "pnpm --filter ./apps/web run dev",
      "port": { "via": "flag", "key": "-p" } }
  ],
  "env": {
    "files": [{
      "path": "apps/api/.env",
      "fullstack": {
        "PORT": "{{port.api}}",
        "DATABASE_URL": "postgresql://acme:acme@localhost:{{port.db}}/{{db.name}}"
      }
    }]
  },
  "db": {
    "driver": "docker-postgres", "band": 5500, "image": "postgres:16",
    "name": "acme_wt{{slot}}", "container": "postgres-acme-wt{{slot}}",
    "verify": { "minTables": 10, "minRows": 1 }
  }
}`;
