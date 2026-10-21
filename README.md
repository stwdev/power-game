# ⚡ Power Game

A mobile game about building an electricity network: auction power plants,
buy coal, oil, garbage and uranium, connect cities and power them for cash.
It's an unofficial fan game based on the mechanics of the classic power-grid board
game, with its own original map (six fictional regions, 42 cities) and art.

- **Plays on Android** as an installable, offline-capable web app (PWA).
- **2–6 players:** any mix of humans (pass-and-play on one phone) and bots.
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

The app has to be served over HTTPS to install. The easiest way is free GitHub Pages:

1. In the GitHub repo go to **Settings → Pages** and set **Source** to
   **GitHub Actions**.
2. Merge to `main`. The workflow tests, builds and publishes the game to
   `https://<you>.github.io/<repo>/`.
3. Open that link in **Chrome on Android**, tap **⋮ → Add to Home screen /
   Install app**. It then runs full screen and works offline.

You can also try it locally: `./build.sh && go run ./cmd/server` and open
<http://127.0.0.1:8080>. To test on your phone over home Wi-Fi, use
`go run ./cmd/server -addr 0.0.0.0:8080` and browse to your computer's IP
(it will play, but Android only installs it from HTTPS).

> Want a real `.apk` for the Play Store later? The PWA can be wrapped as a
> Trusted Web Activity with Google's Bubblewrap tool without code changes.

## How to play (short version)

Each round has four phases:

1. **Auction** – in turn order, pick a plant from the top row and bid at
   least its number; others raise or pass. Everyone buys exactly one plant in
   round 1; after that buying is optional (max 3 plants, 4 with 2 players).
2. **Buy fuel** – last player first. Plants store twice what they burn;
   prices rise as the market empties.
3. **Build** – last player first. First house is $10; later houses cost the
   house ($10/$15/$20) plus the cheapest road from your network.
4. **Bureaucracy** – plants burn fuel to power your cities, you get paid,
   and the markets refill.

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
