# System design — Telegram Image Converter Bot (Go)

Below is a complete project template (Markdown) you can drop into VSCode for an implementer to follow. It describes architecture, domain model, flows, constraints, operational concerns (rate-limiting, file-size limit, concurrency), component responsibilities, sqlite schema, and testing checklist. **No runnable agent code is included** — only design, interfaces, and clear instructions for implementation in Go using `go-telegram-bot-api`, `godotenv`, and SQLite.

> Filename suggestion: `PROJECT.md`

---

# Image Converter Bot — Project Template

## Overview
Small Telegram bot written in Go.  
User flow (high level):
1. User registers once (username + phone via Telegram contact). Store `username`, `phone`, `createdAt`, `updatedAt`, `imageConvertedCounter`.
2. User uploads an image.
3. Bot replies with conversion options (inline keyboard).
4. User selects conversion option.
5. Bot streams conversion result back to user (no files stored on disk).
6. Log only filename and errors (minimal logging).
7. Nothing else is persisted.

Goals / non-goals:
- Goal: clear Clean-Architecture project with safe concurrent processing and no persistent files.
- Non-goal: analytics, metrics, long-term storage of images.

---

## Architecture (Clean + DDD)

Layers:
- **Domain** — Entities, value objects, domain interfaces (e.g., `UserRepository`, `Converter` interface).
- **Usecases / Application** — Orchestrates flows: `RegisterUser`, `HandleImageUpload`, `ConvertImage`, `RateLimitCheck`.
- **Delivery / Adapters** — Telegram adapter: receives updates, sends responses. HTTP adapter (optional for health checks).
- **Infrastructure** — Implementations: SQLite repo, converter implementations (strategy implementations), rate limiter store (in-memory), file streaming helpers, env loader, minimal logger.
- **Cmd / Main** — Bootstrapping, DI (wire or manual), dotenv loading, graceful shutdown.

Component responsibilities:
- **Bot (delivery)**: Listen to updates, parse commands/contacts, present inline keyboards, forward image data as `io.Reader` to usecases, stream upload of converted result to Telegram using Bot API.
- **Usecase**:
  - Validate user registration.
  - Check rate limiter & file-size.
  - Call converter strategy with `io.Reader` and options.
  - Increment `imageConvertedCounter`.
  - Return resulting `io.Reader` (stream) to Bot layer for sending.
- **Converter strategy**: Implement `Convert(ctx, in io.Reader, option) (io.Reader, error)` semantics using streaming (no temporary file).
- **Repo (sqlite)**: Save user and counters, minimal schema.
- **Rate limiter**: Token bucket per Telegram user id implemented in-memory with persistence optional (redis) — but for this project keep it in-memory to satisfy constraints.

---

## Data model (sqlite)

Table: `users`

```sql
CREATE TABLE IF NOT EXISTS users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  telegram_id INTEGER NOT NULL UNIQUE,
  username TEXT,
  phone TEXT,
  created_at DATETIME NOT NULL DEFAULT (datetime('now')),
  updated_at DATETIME NOT NULL DEFAULT (datetime('now')),
  image_converted_counter INTEGER NOT NULL DEFAULT 0
);
```

Fields to store (explicitly required):
- `telegram_id` (unique)
- `username`
- `phone`
- `created_at`
- `updated_at`
- `image_converted_counter`

Migrations: one SQL file with the `CREATE TABLE` above.

Notes:
- Only these fields are stored. No file references, no temporary paths.

---

## Configuration (.env)
List of required config keys (example names):
- `TELEGRAM_TOKEN`
- `ENV` (dev|prod)
- `SQLITE_PATH` (e.g., `./data/bot.db`)
- `MAX_FILE_BYTES` (e.g., `10_485_760` for 10MB)
- `RATE_LIMIT_PER_MIN` (e.g., `10`) — conversions per minute per user
- `CONCURRENCY_LIMIT` (e.g., `4`) — global concurrent conversions
- `LOG_LEVEL` (`info` or `error`)

---

## Supported conversion options (initial)
- `PNG → JPG` (with quality option)
- `JPG → PNG`
- `Any → WEBP` (lossy/lossless toggle)
- `Resize (width x height)` (preserve aspect default)
- `Grayscale`
- `Rotate (90/180/270)`
- `Compress / Re-encode` (reduce size with quality parameter)

Design note: implement each as a **Strategy** that conforms to a single converter interface. Each strategy must accept options and work with `io.Reader`/`io.Writer` streams.

---

## Converter Strategy (design)
- **Interface (conceptual)**: `type Converter interface { Convert(ctx context.Context, in io.Reader, opts ConvertOptions) (io.Reader, error) }`
- Implementation requirements:
  - Use streaming (pipes) to avoid full-buffering into memory and avoid on-disk temp files.
  - Use `io.Pipe()` to connect a decoder->transformer->encoder pipeline.
  - If using external CLI tools (e.g., `ffmpeg`), use `os/exec` with `StdinPipe`/`StdoutPipe` and stream through pipes.
  - Limit per-conversion memory usage by streaming and by bounding readers (e.g., `io.LimitReader` using `MAX_FILE_BYTES`).
  - Ensure converters honor context cancellation.

Concurrency-safety:
- Each conversion uses its own `io.Pipe`; no shared mutable buffers.
- Global concurrency semaphore to limit simultaneous conversions (`CONCURRENCY_LIMIT`).
- Per-user rate limiter separate from concurrency control.

Fallbacks:
- If an image format cannot be decoded by the chosen library, respond with a clear user-friendly message.

---

## No-temp-file strategy (details)
- Use streaming path:
  1. Telegram sends file; bot fetches file bytes as an `io.Reader` (Telegram API supports file download).
  2. Immediately wrap it with `io.LimitReader` to enforce size limit.
  3. Pass `io.Reader` into converter strategy.
  4. Converter does decode->transform->encode while streaming into an `io.PipeWriter`; `io.PipeReader` returned to bot for sending back.
  5. Bot sends the `io.Reader` content as file upload (use multipart streaming upload supported by Telegram API).
- If external binary is used, connect its stdin/stdout to pipes and stream.

Memory safety:
- Enforce `MAX_FILE_BYTES` to avoid huge memory usage.
- If using in-memory image libraries that decode whole image into memory, choose libraries optimized for streaming or accept memory usage tradeoff and set low `MAX_FILE_BYTES`.

When temporary files are unavoidable (e.g., library forces it), ensure:
- Use unique temp files under OS temp dir using `ioutil.TempFile` with `O_RDWR|O_EXCL`.
- Immediately delete temp file after use: `defer os.Remove(tmp)`.
- Protect temp files with unique names and per-conversion lifetime — avoid reuse across goroutines.

---

## Rate limiting
Policy:
- Per-user: `RATE_LIMIT_PER_MIN` conversions per minute.
- Global: optional global rate cap to protect resources.

Implementation suggestions:
- Use token bucket per `telegram_id` in memory:
  - Store `tokens`, `lastRefill` for each user.
  - On request: refill based on elapsed time, then consume token if available.
  - If no token, reply with friendly "Rate limit exceeded. Try again in Xs."

Edge cases:
- Restart loses in-memory buckets; acceptable for small bot. If persistence needed, use Redis later.

Throttling UX:
- When user is rate-limited, include Retry-After seconds in the message.
- Consider a lower soft-limit for unregistered users (but we require registration so a simple per-user bucket is enough).

---

## File-size limit and validation
- Enforce `MAX_FILE_BYTES` at the moment of download: wrap Telegram file response reader with `io.LimitReader`.
- Reject files larger than limit with message: "File too large. Max: X MB."
- Also check declared file size from Telegram metadata to pre-reject before download.

---

## Concurrency control
- Global semaphore (`CONCURRENCY_LIMIT`) to cap simultaneous conversions.
- Acquire semaphore before starting conversion; release after upload is finished or on error.
- If semaphore full, reply "Server is busy, please try again later" or queue (prefer reject for simplicity).

---

## Telegram UX & Flows

### Registration
- `/start` triggers registration check.
- Ask for contact (Telegram supports sending a contact). Use `request_contact` button to get phone.
- Save `telegram_id`, `username`, `phone`, timestamps. If phone not provided, accept optional (but requirement says save phone).
- Once registered, send short instructions.

### Upload & Convert
1. User sends image (photo or document).
2. Bot inspects message:
   - If user not registered: prompt to register first.
   - Validate file size.
3. Bot replies with inline keyboard with listed conversion options (and a cancel button).
4. User clicks an option, bot acknowledges ("processing...").
5. Bot runs rate limiter and concurrency checks.
6. On success: stream converted result back to user as document (so the file keeps extension).
7. On failure: send error message.

Important UI behaviors:
- Keep messages minimal.
- Provide progress only as simple messages (no metrics) — e.g., "Processing, please wait..."
- After success increment `image_converted_counter`.

---

## Error handling & logging
- Minimal logging:
  - On each upload: log `telegram_id`, original file name (if available), conversion option, error (if any). Keep log minimal.
  - Do NOT log image bytes.
- Store logs to stdout/stderr.
- Sensitive errors should be returned to user in friendly text; keep internal error details in logs only.
- Graceful cancel: if user cancels, try to cancel conversion via context.

---

## Security considerations
- Validate and sanitize any options passed from inline buttons.
- Enforce `MAX_FILE_BYTES`.
- Validate file type by decoding header rather than trusting filename.
- Prevent command injection if using `os/exec`.
- Run external binaries with limited privileges and sandbox if possible.
- Rate limit to protect from abuse.

---

## High-level sequence diagram (text)

```
User -> Bot (Telegram): /start or sends contact -> Bot: Register in sqlite
User -> Bot: Sends image
Bot -> Usecase: validate user, check file size
Usecase -> RateLimiter: allow?
RateLimiter -> Usecase: allowed or denied
Usecase -> Bot: show conversion options (inline keyboard)
User -> Bot: selects option
Bot -> Usecase: Acquire global semaphore; start conversion
Usecase -> ConverterStrategy: Convert(ctx, io.Reader, options) -> returns io.Reader (stream)
Usecase -> Bot: Stream result to Telegram upload endpoint
Bot -> Telegram: sendDocument (stream)
Usecase -> Repo: increment imageConvertedCounter
Usecase -> Release semaphore -> Done
```

---

## Folder structure (suggested)
```
/cmd/bot/main.go
/internal/
  domain/
    user.go
    converter.go (interface & types for ConvertOptions)
  usecase/
    register_user.go
    handle_upload.go
    convert_image.go
  delivery/
    telegram/
      handler.go
      keyboard.go
  infra/
    sqlite/
      user_repo.go
    converter/
      png_to_jpg.go
      webp.go
      resize.go
    throttler/
      token_bucket.go
    memsem/
      semaphore.go
  config/
    env.go
  logger/
    logger.go (minimal)
  migrations/
    001_create_users.sql
```

---

## Implementation checklist (developer)
- [ ] Project skeleton and `go.mod`.
- [ ] `.env` loading (godotenv).
- [ ] SQLite setup + migration runner (on startup).
- [ ] Domain types & repository interface.
- [ ] Telegram adapter: update polling, file download, file upload streaming, inline keyboards, contact request.
- [ ] Converter strategy interface + at least 3 implementations (PNG→JPG, Any→WEBP, Resize).
- [ ] Streaming pipeline using `io.Pipe`; no temp files by default.
- [ ] Rate limiter token-bucket per user.
- [ ] Global concurrency semaphore.
- [ ] Minimal logger for filename and errors.
- [ ] Unit tests for converters (use in-memory readers).
- [ ] Integration tests for end-to-end flow (mock Telegram or use a test bot).
- [ ] Deployment note: ensure environment variables set.

---

## Testing & QA
- Unit tests:
  - Converter: pass sample images via `io.Reader`, assert output format and non-empty bytes.
  - Rate limiter: token consumption behavior.
  - Repository: SQLite insert/read and counter increment.
- Integration tests:
  - Simulate upload -> choose -> conversion -> download.
  - High concurrency test: spawn N concurrent conversion requests; assert no temp-file collision and server maintains `CONCURRENCY_LIMIT`.
- Edge cases:
  - Oversized file rejected.
  - Corrupt image handling.
  - Rapid repeated requests => rate-limited.
  - Cancelled context mid-conversion.

---

## Deployment notes
- Single binary built with go build.
- Environment: supply `.env` or environment variables.
- Concurrency settings tuned by `CONCURRENCY_LIMIT` and `RATE_LIMIT_PER_MIN`.
- For production, consider moving rate limiter to Redis for cross-instance consistence.

---

## Minimal logging policy (what to log)
- On successful conversion: `timestamp, telegram_id, filename (if present), conversion_option`.
- On error: `timestamp, telegram_id, error_message`.
- Never log: raw image bytes, file contents, or phone numbers in logs (phone stored in DB only).

---

## Example messages (user-facing)
- On successful registration: `Registration complete. Send an image to convert.`
- On too-large file: `File too large — maximum allowed is X MB.`
- On rate limit exceeded: `Rate limit exceeded. Try again in Y seconds.`
- On busy server: `Server is busy processing other requests. Try again shortly.`

---

## Migration SQL file (single)
File: `migrations/001_create_users.sql`
> See schema under **Data model** section.

---

## Extensions & future improvements (not required now)
- Persist rate limiter to Redis for multi-instance.
- Add logout/delete account flow.
- Add user quota by plan (free/pro).
- Replace in-memory converter with libvips via `bimg` for performance.
- Support background queue with durable worker for very large jobs.

---

## Acceptance criteria (definition of done)
- A registered user can upload an image and receive converted result.
- No image files are persisted to disk in normal flow.
- The bot saves only the user row with required fields and increments `image_converted_counter`.
- Enforced file-size and per-user rate-limiting.
- Concurrency limited by `CONCURRENCY_LIMIT`.
- Logging limited to filename and errors.
- Project follows Clean Architecture layout described above.
- Tests for converter logic and rate limiter present.

---

## Implementation hints (practical)
- For streaming upload to Telegram, use multipart writer with `io.Copy` from the conversion pipe reader into the request body.
- To decode/encode images in pure Go, use `image`, `image/jpeg`, `image/png`, and `golang.org/x/image/webp` or third-party libs. Beware that some pure-Go decoders load full image into memory.
- For large/fast processing, consider libvips bindings (`bimg`) but watch cgo complexity and temp-file behavior.
- Use contexts to cancel conversion on timeout or user cancellation.
- Wrap readers with `io.LimitReader` to protect memory.

---

## Questions to resolve during implementation (pick early)
- Max file size (MB) — choose exact limit.
- Allowed image formats (decide whether to permit animated GIFs / multi-frame images).
- Whether to accept videos (out of scope now).
- Where the bot will run (single instance or horizontal scale). If scaling, plan for a distributed rate limiter (Redis).

---

## Contact & quick-start
1. Clone repo.
2. Create `.env` with required keys.
3. `go build ./cmd/bot` then run binary.
4. Use Telegram bot token from BotFather and test flow.

---
