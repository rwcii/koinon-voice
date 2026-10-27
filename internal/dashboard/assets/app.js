// Reads the board again when a host's state changes, and keeps any note being written.
(function () {
  const board = document.getElementById("board");
  if (!board || !window.EventSource) return;
  let pending = false;
  async function reload() {
    const active = document.activeElement;
    if (active && active.tagName === "TEXTAREA" && board.contains(active)) {
      pending = true; // wait until the note loses focus
      return;
    }
    pending = false;
    const notes = {};
    board.querySelectorAll("textarea[data-key]").forEach((t) => { if (t.value) notes[t.dataset.key] = t.value; });
    const r = await fetch("/board", { credentials: "same-origin" });
    if (!r.ok) return;
    board.innerHTML = await r.text(); // HTML from the server's escaping templates
    board.querySelectorAll("textarea[data-key]").forEach((t) => { if (notes[t.dataset.key]) t.value = notes[t.dataset.key]; });
  }
  board.addEventListener("focusout", () => { if (pending) setTimeout(reload, 0); });
  new EventSource("/events").addEventListener("change", reload);
})();
