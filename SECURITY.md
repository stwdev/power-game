# Security

Security was a design requirement from the start. This is what the game does
and does not do.

## Supply chain
- **Zero third-party code.** The engine, AI, WebAssembly bridge, server and icon
  generator use only the Go standard library (`go.mod` has no `require`s). The
  UI is plain JavaScript with no npm packages, frameworks or CDNs. The only
  non-project file shipped is `wasm_exec.js`, copied from your own Go install.
- CI uses only GitHub's official actions with a read-only default token
  (`persist-credentials: false`). For maximum assurance, pin them to commit SHAs.

## In the browser
- **Strict Content Security Policy:** `default-src 'none'`; scripts only from
  the same origin (plus `wasm-unsafe-eval`, which is required to run
  WebAssembly — it does not allow JavaScript `eval`). No inline scripts or
  styles, no third-party connections, `base-uri`/`form-action` locked.
- **No HTML injection:** the UI never uses `innerHTML`; all text (including
  player names) is inserted with `textContent`. Names are also stripped of
  control/bidi characters and limited to 16 characters by the engine.
- **No network use during play, no accounts, no analytics, no ads, no
  permissions** (camera, location, etc. are disabled via `Permissions-Policy`).
- The engine runs in a Web Worker that only accepts an allow-list of calls.
- Saved games live in your browser's `localStorage`. They are treated as
  untrusted on load: size-limited, parsed and fully validated (ownership,
  resource totals, turn order, plant uniqueness...) before use.
- Every Go entry point recovers from panics and limits input size.

## Game integrity (ready for online play)
- All moves go through `engine.Game.Apply`, which checks it is that player's
  turn and that the move is legal; illegal moves never change state.
- `Game.View()` strips hidden information (the deck order and the random
  generator state) — this is what a server would send to clients.
- AI opponents only use public information; a test proves their decisions do
  not depend on the hidden deck order.
- Randomness for new games comes from `crypto/rand`.

## Server (`cmd/server`)
- Serves static files only: GET/HEAD, no directory listings, no dotfiles,
  path traversal blocked by `io/fs`.
- Sends CSP, `nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy`,
  COOP/CORP, `Permissions-Policy` and HSTS (when TLS is on).
- Read/write/idle timeouts and a header-size cap; binds to `127.0.0.1` unless
  you choose otherwise.

## Android app (APK)
- **Requests no permissions**, including no `INTERNET` permission. The game is
  bundled in the APK and served to the WebView from a private origin.
  Requests to any other address are refused, and the app never navigates
  anywhere else.
- The WebView has file and content access disabled, cannot open new windows,
  and serves the same Content Security Policy as the web version.
  `allowBackup` is off.
- Built from source in CI with the Android SDK's own tools (aapt2, javac, d8,
  zipalign, apksigner). No Gradle, plugins or libraries are downloaded.
- Signed with a key that only exists in your GitHub secrets. In CI it is
  written to a private temp file and deleted straight after signing. Each
  release publishes the certificate fingerprint and an APK SHA-256 checksum.
- The release workflow passes user input through environment variables (not
  inline into scripts), checks the version format strictly, and is the only
  job with write access to the repository.

## Reporting
Please open a private security advisory on the GitHub repository.
