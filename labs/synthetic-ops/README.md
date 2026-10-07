# synthetic-ops

> **WARNING: DELIBERATELY VULNERABLE SOFTWARE. CI AND LAB USE ONLY.**
>
> This service contains planted SQL injection, stored and reflected XSS,
> IDOR, open redirect, path traversal, SSRF, missing authentication, a
> default credential and information disclosure. It exists so `assay` has a
> target with complete, machine-readable ground truth.
>
> - Never expose it to a network you do not fully control. Bind to loopback.
> - Never deploy it, package it, or reuse its handlers anywhere else.
> - Never point it at real data. It seeds its own throwaway SQLite database.
> - `/api/fetch` refuses non-loopback hosts unless started with
>   `-unsafe-ssrf`; CI must never pass that flag.

Lab marker (used by the path-traversal checker as a benign target file):
`SYNTH-LAB-README-MARKER-0001`

## Running

```sh
cd labs/synthetic-ops
go run ./cmd/synthetic-ops -addr 127.0.0.1:9000
```

Flags: `-addr` (default `:9000`), `-db` (default `:memory:`), `-public`
(default `./public`), `-unsafe-ssrf` (default off).

## Accounts

| role  | username | password       | note                        |
|-------|----------|----------------|-----------------------------|
| admin | admin    | `admin123`     | planted default credential  |
| user  | alice    | `Wonderland-1` |                             |
| user  | bob      | `Builder-22`   |                             |

## Endpoints and planted weaknesses

Every entry below is listed in `ground_truth.json` with its CWE, severity
and a `source_ref` of the form `handlers/<file>.go:<line>` pointing at a
`// VULN:` comment. `handlers_test.go` verifies both the behavior and that
every `source_ref` still points at a `// VULN:` line.

| method | path                | weakness                     |
|--------|---------------------|------------------------------|
| POST   | `/api/login`        | default credential; no rate limit |
| GET    | `/api/users/{id}`   | IDOR (no ownership check)    |
| GET    | `/api/search?q=`    | SQL injection                |
| POST   | `/api/profile`      | stored XSS (`bio`, rendered at `/profile/{id}`) |
| GET    | `/search?term=`     | reflected XSS                |
| GET    | `/redirect?to=`     | open redirect                |
| GET    | `/files?name=`      | path traversal under `./public` |
| GET    | `/api/admin/stats`  | missing authentication       |
| GET    | `/api/fetch?url=`   | SSRF (loopback only by default) |
| GET    | `/debug/config`     | information disclosure (marker `SYNTH-SECRET-MARKER-0001`) |
| GET    | `/backup.sql`       | sensitive file (marker `SYNTH-BACKUP-MARKER-0001`) |
| GET    | `/api/boom`         | verbose error page with stack trace |
| *      | everywhere          | no security headers; CORS `*` with credentials |

## Fixes

`fixes/<id>.diff` holds a unified diff per planted weakness. `assay
remediate` attaches them to findings against this lab and `remediate.Verify`
applies one to a temporary copy, rebuilds, and re-runs the checker.

This module is separate from the root module so its SQLite dependency never
enters the harness binary.
