# Plan: playing online with friends

Pass-and-play (several humans sharing one phone) works today. Online play is
designed in but not yet built. The engine was written for it:

| Need | Already in place |
|------|------------------|
| Server is the single source of truth | `engine.Game` is a pure, deterministic state machine; the server would own it |
| Clients can't cheat | `Apply` rejects moves that are out of turn or illegal |
| Hidden info stays hidden | `View()` removes deck order and RNG state |
| Bots can fill empty seats | `ai.Decide` runs server-side, using public info only |
| Resume after disconnect | Full state serialises to JSON and is validated on load |

## Proposed design (standard library only)

1. **Server** – extend `cmd/server` with a small JSON API using `net/http`:
   - `POST /api/rooms` → create a room, returns a room code and a seat token.
   - `POST /api/rooms/{code}/join` → claims a free seat, returns a seat token.
   - `POST /api/rooms/{code}/action` → body is an `engine.Action`; the server
     sets `Action.Player` **from the seat token**, never from the request body.
   - `GET  /api/rooms/{code}/events` → Server-Sent Events stream of
     `View()` updates (SSE is in the standard library's reach; no WebSocket
     dependency needed).
2. **Identity** – seat tokens are 256-bit values from `crypto/rand`, sent as
   `Authorization: Bearer` headers, compared with `crypto/subtle`, and stored
   only as hashes. No accounts or personal data required.
3. **Hardening** – HTTPS only (HSTS), per-IP and per-room rate limits,
   `http.MaxBytesReader` on bodies (actions are tiny), room expiry, a cap on
   rooms per IP, strict CORS (same origin only), and `connect-src` in the CSP
   limited to the game's own origin.
4. **Client** – the existing UI already renders from a `View`; online mode
   swaps the Web Worker calls for `fetch` + `EventSource`.

Room codes should be short but unguessable for joining (e.g. 8 characters
from a 32-symbol alphabet), while seat tokens carry the real authority.
