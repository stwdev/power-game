# ⚡ Power Game

A mobile game about building an electricity network: auction power plants,
buy coal, oil, garbage and uranium, connect cities and power them for cash.
It's an unofficial fan game based on the mechanics of the classic power-grid board
game, with its own original map (six fictional regions, 42 cities) and art.

- **Plays on Android** as an installable, offline-capable web app (PWA).
- **2–6 players:** any mix of humans (pass-and-play on one phone) and bots.
- **Changeable board:** play the Classic map, the Lakeside ring or the
  Ridge zig-zag, or generate a **random map** every game. Each random map
  has a short code (like `K3F9-2QXA`) you can share to replay it. You can
  also let the game pick the regions in play, or choose them yourself on
  the setup screen.
- **Three AI levels**
  - **Easy** – plays loosely and makes mistakes. Good for learning.
  - **Medium** – solid heuristics: values plants by income and fuel cost,
    buys the fuel it needs and grows to match its capacity.
  - **Hard** – looks ahead. For each key decision (which plant to auction,
    whether to raise a bid, how many cities to build) it simulates the rest
    of the game many times and picks the move that wins most often. In
    testing it wins about 85–90% of games against Easy + Medium.
- **Written in Go with zero third-party dependencies.** The UI is
  plain HTML/CSS/JS with no frameworks. See [SECURITY.md](SECURITY.md).
- **Online play with friends** is planned; the engine is built for it.
  See [docs/MULTIPLAYER.md](docs/MULTIPLAYER.md).

## Install on your Android phone

### Option A: Android app (APK) with Obtainium

Each GitHub release has a signed APK attached. In
[Obtainium](https://github.com/ImranR98/Obtainium) tap **Add App** and paste
`https://github.com/stwdev/power-game`. It installs the latest release and
picks up updates automatically. Or just download the `.apk` from the
Releases page and open it.

The app plays fully offline and requests **no Android permissions** (not
even internet access). The release notes list the signing certificate's
SHA-256 fingerprint so you can check it.

**Publishing a release (maintainers).** Do this once:
1. Run `go run ./cmd/genkey ~/power-game-key` on a computer you trust (needs Go).
   **Back up that folder.** Android only accepts updates signed with the same key.
2. In GitHub go to **Settings → Secrets and variables → Actions** and add
   `ANDROID_SIGNING_KEY` (the contents of `signing-key.b64`) and
   `ANDROID_SIGNING_CERT` (the contents of `signing-cert.pem`).

Then, for each release: **Actions → Release APK → Run workflow**, enter a
version such as `1.0.0` (or push a tag `v1.0.0`). Use a higher version every
time. The APK is built with the plain Android SDK tools (no Gradle) by
`android/build.sh`.

### Option B: web app (PWA)

The web app has to be served over HTTPS to install. The easiest way is free GitHub Pages:

1. In **Settings → General**, make sure the **default branch** is `main`.
   GitHub Pages only deploys from the default branch.
2. In **Settings → Pages**, set **Source** to **GitHub Actions**.
3. Push to `main`. The workflow tests, builds and publishes the game to
   `https://<you>.github.io/<repo>/`.
4. Open that link in **Chrome on Android** and tap **⋮ → Install app**.

You can also try it locally: run `./build.sh && go run ./cmd/server` and open
<http://127.0.0.1:8080>.

## How to play (short version)

Each round has four phases:

1. **Auction** – in turn order, pick a plant from the top row and bid at
   least its number; others raise or pass. Everyone buys exactly one plant in
   round 1; after that buying is optional (max 3 plants, 4 with 2 players).
2. **Buy fuel** – last player first. Plants store twice what they burn;
   prices rise as the market empties.
3. **Build** – last player first. First house is $10; later houses cost the
   house ($10/$15/$20) plus the cheapest road from your network.
4. **Power** – in turn order, choose which plants to run (or none, to
   save fuel); hybrid plants let you pick the coal/oil mix. You're paid for
   each city powered, then the markets refill.

Step 2 opens a second house per city; step 3 (a card in the deck) a third and
puts every plant on sale. When someone reaches the end city count, the player
powering the most cities that round wins (ties go to cash).

## Project layout

```
engine/      rules: state machine, map, plants, markets, validation
ai/          Easy/Medium heuristics, Hard Monte Carlo search
cmd/wasm/    WebAssembly bridge used by the browser
cmd/server/  hardened static file server (stdlib only)
cmd/icongen/ draws the PNG app icons
cmd/genkey/  creates the Android signing key
android/     Android wrapper app (one Java file) and its build script
web/         HTML/CSS/JS front end, service worker, manifest
build.sh     builds everything into dist/
```

## Development

Needs Go 1.24+ only.

```sh
go test ./...                     # rules, AI strength and server tests
go test -short ./...              # skips the slower Hard-AI tests
H2H=1 go test ./ai -run H2H -v    # AI strength benchmark
./build.sh                        # builds dist/
go run ./cmd/server               # serves dist/ on 127.0.0.1:8080
```
