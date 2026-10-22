//go:build js && wasm

// Command wasm exposes the game engine to the browser. Every call takes and
// returns JSON strings; inputs are size-limited and validated by the engine,
// and panics are caught so bad input cannot crash the page.
package main

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"syscall/js"

	"powergame/ai"
	"powergame/engine"
)

const maxInput = 1 << 20

var game *engine.Game

func main() {
	api := map[string]any{
		"newGame": wrap(newGame),
		"view":    wrap(func(string) (any, error) { return view() }),
		"apply":   wrap(apply),
		"aiStep":  wrap(func(string) (any, error) { return aiStep() }),
		"save":    wrap(save),
		"load":    wrap(load),
		"board":   wrap(func(string) (any, error) { return board() }),
		"maps":    wrap(func(string) (any, error) { return engine.Maps(), nil }),
		"preview": wrap(preview),
		"restock": wrap(func(string) (any, error) { return restockDefaults(), nil }),
		"plants":  wrap(func(string) (any, error) { return engine.AllPlants(), nil }),
	}
	js.Global().Set("PowerGame", js.ValueOf(api))
	js.Global().Call("powerGameReady")
	select {}
}

type result struct {
	OK    any    `json:"ok,omitempty"`
	Error string `json:"error,omitempty"`
}

func wrap(fn func(string) (any, error)) js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) (out any) {
		defer func() {
			if r := recover(); r != nil {
				out = encode(result{Error: fmt.Sprint("internal error: ", r)})
			}
		}()
		arg := ""
		if len(args) > 0 && args[0].Type() == js.TypeString {
			arg = args[0].String()
		}
		if len(arg) > maxInput {
			return encode(result{Error: "input too large"})
		}
		v, err := fn(arg)
		if err != nil {
			return encode(result{Error: err.Error()})
		}
		return encode(result{OK: v})
	})
}

func encode(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return `{"error":"encode failed"}`
	}
	return string(b)
}

func view() (any, error) {
	if game == nil {
		return nil, errors.New("no game")
	}
	return game.View(), nil
}

// board is the current game's map.
func board() (any, error) {
	if game == nil {
		return nil, errors.New("no game")
	}
	return game.Board(), nil
}

// preview builds a map for the setup screen. A random map with no code
// gets a fresh seed; a code replays a shared map.
func preview(arg string) (any, error) {
	var req struct {
		ID   string `json:"id"`
		Code string `json:"code"`
	}
	if err := json.Unmarshal([]byte(arg), &req); err != nil {
		return nil, errors.New("bad map request")
	}
	spec := engine.MapSpec{ID: req.ID}
	if req.ID == "random" {
		if req.Code != "" {
			seed, err := engine.ParseMapCode(req.Code)
			if err != nil {
				return nil, err
			}
			spec.Seed = seed
		} else {
			seed, err := randomSeed()
			if err != nil {
				return nil, err
			}
			spec.Seed = seed % engine.MaxMapSeed
		}
	}
	return engine.BuildBoard(spec)
}

// restockDefaults is the standard fuel restock table for each table size.
func restockDefaults() map[int]engine.RestockTable {
	out := map[int]engine.RestockTable{}
	for n := engine.MinPlayers; n <= engine.MaxPlayers; n++ {
		out[n] = engine.DefaultRestock(n)
	}
	return out
}

func randomSeed() (uint64, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(b[:]), nil
}

func newGame(arg string) (any, error) {
	var cfg engine.Config
	if err := json.Unmarshal([]byte(arg), &cfg); err != nil {
		return nil, errors.New("bad config")
	}
	seed, err := randomSeed()
	if err != nil {
		return nil, err
	}
	cfg.Seed = seed
	g, err := engine.NewGame(cfg)
	if err != nil {
		return nil, err
	}
	game = g
	return view()
}

// apply performs a move for a human seat on this device.
func apply(arg string) (any, error) {
	if game == nil {
		return nil, errors.New("no game")
	}
	var a engine.Action
	if err := json.Unmarshal([]byte(arg), &a); err != nil {
		return nil, errors.New("bad action")
	}
	cur := game.CurrentPlayer()
	if cur < 0 || game.Players[cur].AI != engine.Human {
		return nil, errors.New("not a human turn")
	}
	if err := game.Apply(a); err != nil {
		return nil, err
	}
	return view()
}

// aiStep plays one move if a bot is to act.
func aiStep() (any, error) {
	if game == nil {
		return nil, errors.New("no game")
	}
	cur := game.CurrentPlayer()
	if cur < 0 || game.Players[cur].AI == engine.Human {
		return nil, errors.New("not a bot turn")
	}
	if err := game.Apply(ai.Decide(game)); err != nil {
		return nil, err
	}
	return view()
}

func save(string) (any, error) {
	if game == nil {
		return nil, errors.New("no game")
	}
	b, err := json.Marshal(game)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func load(arg string) (any, error) {
	g, err := engine.Load([]byte(arg))
	if err != nil {
		return nil, err
	}
	game = g
	return view()
}
