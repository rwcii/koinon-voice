// Runs assets/app.js against a stand-in DOM, fetch and EventSource, and prints one JSON
// result per case. script_test.go checks them.
const fs = require("node:fs");
const vm = require("node:vm");
const source = fs.readFileSync(process.argv[2], "utf8");

function setup() {
  const listeners = {};
  const textarea = { tagName: "TEXTAREA", dataset: { key: "local/p-1" }, value: "" };
  const board = {
    html: "initial",
    contains: (x) => x === textarea,
    querySelectorAll: () => [textarea],
    addEventListener: (n, f) => { listeners[n] = f; },
    set innerHTML(v) { this.html = v; textarea.value = ""; }, // a swap makes a new, empty textarea
    get innerHTML() { return this.html; },
  };
  const doc = { activeElement: null, getElementById: () => board };
  const requests = [];
  let es;
  class EventSource { constructor() { es = this; this.events = {}; } addEventListener(n, f) { this.events[n] = f; } }
  const ctx = { document: doc, window: { EventSource }, EventSource, setTimeout,
    fetch: () => new Promise((resolve) => requests.push(resolve)) };
  vm.runInNewContext(source, ctx);
  const answer = (i, html) => requests[i]({ ok: true, text: async () => html });
  return { textarea, board, doc, requests, listeners, answer, get es() { return es; } };
}
const tick = () => new Promise((r) => setTimeout(r, 0));

(async () => {
  const out = {};

  // A note typed in a focused textarea while a read is in flight is not erased; the swap
  // waits for focusout and keeps the note.
  let x = setup();
  let p = x.es.events.change();
  x.textarea.value = "typed during the read";
  x.doc.activeElement = x.textarea;
  x.answer(0, "revision 2");
  await p;
  out.focusedNote = x.textarea.value;
  out.focusedBoard = x.board.html;
  x.doc.activeElement = null;
  x.listeners.focusout();
  await tick();
  x.answer(1, "revision 2");
  await tick(); await tick();
  out.afterFocusoutBoard = x.board.html;
  out.afterFocusoutNote = x.textarea.value;

  // A note typed without focus during the read is kept across the swap.
  x = setup();
  p = x.es.events.change();
  x.textarea.value = "unfocused note";
  x.answer(0, "revision 2");
  await p;
  out.unfocusedNote = x.textarea.value;

  // Changes during a read cause one more read; the last board shown is the newest.
  x = setup();
  p = x.es.events.change();
  x.es.events.change();
  x.es.events.change();
  x.answer(0, "revision 2");
  await tick(); await tick();
  out.boardBeforeSecond = x.board.html;
  x.answer(1, "revision 3");
  await p;
  out.coalescedBoard = x.board.html;
  out.coalescedReads = x.requests.length;

  // The stream's open event reads the board.
  x = setup();
  p = x.es.events.open();
  out.readsOnOpen = x.requests.length;
  x.answer(0, "after open");
  await p;
  out.openBoard = x.board.html;

  console.log(JSON.stringify(out));
})();
