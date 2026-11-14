# Platform services

Owner: Kaiyrbekuly Aitbek.

One folder per Go service (`services/auth`, `services/users`, ...), each with its own `go.mod`, `Dockerfile` and README. CI builds and tests every module found here. See `docs/03_architecture/architecture.md` for the list of services and their responsibilities.

Current modules (stage 2): `platform/` (shared kit, no binary), `auth/` (authentication, accounts, groups — the minimal `users` responsibilities are folded in for M4), `quizzes/`. Services import the kit with `replace github.com/Newterios/Examory/services/platform => ../platform`; Docker builds use `services/` as the build context (`docker build -f quizzes/Dockerfile services`). Developer entry points are in the root `Makefile`.
