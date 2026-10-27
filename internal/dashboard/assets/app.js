// Reads the board again when a host's state changes, and keeps any note being written.
// One read runs at a time; changes during a read cause one more read, so the newest board
// is the one shown. The stream's open event, also after a reconnect, reads the board too,
// so a change while no stream was open is not missed.
(function () {
  const board = document.getElementById("board");
  if (!board || !window.EventSource) return;
  let running = false;
  let again = false;
  let deferred = false;
  function editing() {
    const active = document.activeElement;
    return active && active.tagName === "TEXTAREA" && board.contains(active);
  }
  async function reload() {
    if (running) {
      again = true;
      return;
    }
    running = true;
    try {
      do {
        again = false;
        const r = await fetch("/board", { credentials: "same-origin" });
        if (!r.ok) break;
        const html = await r.text();
        if (again) continue; // a newer change came during the read: read again
        if (editing()) {
          deferred = true; // swap when the note loses focus
          break;
        }
        const notes = {};
        board.querySelectorAll("textarea[data-key]").forEach((t) => { if (t.value) notes[t.dataset.key] = t.value; });
        board.innerHTML = html; // HTML from the server's escaping templates
        board.querySelectorAll("textarea[data-key]").forEach((t) => { if (notes[t.dataset.key]) t.value = notes[t.dataset.key]; });
      } while (again);
    } finally {
      running = false;
    }
  }
  board.addEventListener("focusout", () => {
    if (deferred) {
      deferred = false;
      setTimeout(reload, 0);
    }
  });
  const events = new EventSource("/events");
  events.addEventListener("open", reload);
  events.addEventListener("change", reload);
})();
