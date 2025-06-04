# Demo Service

A tiny Go HTTP service with **synthetic failures** that make it easy to demonstrate problem detection. 

---

## Features & Endpoints

| Path       | Purpose                                                            | Typical Error Scenarios                         |
| ---------- | ------------------------------------------------------------------ | ----------------------------------------------- |
| `/`        | Health check / welcome JSON                                        | —                                               |
| `/panic`   | Launches a goroutine that panics; recovered so the server lives    | `level=error msg="recovered goroutine panic" …` |
| `/slow`    | Sleeps 6 s; if the client aborts early we log context cancellation | `level=error msg="context canceled" …`          |
| `/migrate` | Runs an **intentionally broken** SQL migration                     | `level=error msg="migration failed" …`          |
| `/health`  | Lightweight liveness probe, no extra logging                       | —                                               |

---

## Quick Start

```bash
git clone <repo‑url>
cd demo‑service

go run .
```

Server starts on **`:8080`**:

```
level=info msg="starting server" addr=:8080
```

---

## Playing with the Failures

Below are example requests and the *exact* log lines you should expect so you can wire them into your detection rules.

> The `duration=…` value will vary, so you can replace it with `.*` in regexes.

### `/` – baseline request

```bash
curl -s http://localhost:8080/ | jq
```

Logs:

```
level=info method=GET path=/ status=200 duration=…
```

---

### `/panic` – goroutine panic

```bash
curl -s http://localhost:8080/panic | jq
```

Logs:

```
level=info method=GET path=/panic status=200 duration=…
level=error msg="recovered goroutine panic" panic=intentional panic inside goroutine for demo purposes
```

---

### `/slow` – client abort

Trigger with a 1‑second timeout so the client drops before the 6‑second sleep finishes:

```bash
curl --max-time 1 http://localhost:8080/slow || true
```

Logs:

```
level=info method=GET path=/slow status=200 duration=…   # emitted after handler returns (if it returns)
level=error msg="context canceled" path=/slow err=context canceled
```

If you let it run the full 6 s instead, you’ll just see a normal `status=200` line.

---

### `/migrate` – SQL migration failure

```bash
curl -v http://localhost:8080/migrate
```

Logs:

```
level=info method=GET path=/migrate status=500 duration=…
level=error msg="migration failed" err=alter table: no such table: imaginary
```

---

### `/health` – liveness probe

```bash
curl -s http://localhost:8080/health
```

No extra logs (handler bypasses the logging middleware).

---

## Customising

- Add new problem cases by creating a handler that logs at `level=error`.
- Switch log format or destination by editing `log.SetFlags` / `log.SetOutput` in `main.go`.

Happy hunting! 🚀