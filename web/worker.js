// Runs the Go/WebAssembly engine off the main thread so bot thinking never
// freezes the page. Only the listed engine functions can be called.
'use strict';
importScripts('wasm_exec.js');

const ALLOWED = new Set(['newGame', 'view', 'apply', 'aiStep', 'save', 'load', 'board', 'plants']);
let markReady;
const ready = new Promise((resolve) => { markReady = resolve; });
self.powerGameReady = () => markReady();

const go = new Go();
async function start() {
  let inst;
  try {
    inst = await WebAssembly.instantiateStreaming(fetch('game.wasm'), go.importObject);
  } catch (_) {
    const bytes = await (await fetch('game.wasm')).arrayBuffer();
    inst = await WebAssembly.instantiate(bytes, go.importObject);
  }
  go.run(inst.instance);
}
start().catch((err) => postMessage({ id: 0, out: JSON.stringify({ error: 'engine failed to load: ' + err }) }));

self.onmessage = async (e) => {
  const msg = e.data || {};
  await ready;
  let out;
  if (typeof msg.fn !== 'string' || !ALLOWED.has(msg.fn)) {
    out = '{"error":"unknown call"}';
  } else {
    out = self.PowerGame[msg.fn](typeof msg.arg === 'string' ? msg.arg : '');
  }
  postMessage({ id: msg.id, out });
};
