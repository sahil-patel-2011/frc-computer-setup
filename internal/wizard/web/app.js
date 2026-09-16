const screens = {
  welcome: document.getElementById("screen-welcome"),
  pick: document.getElementById("screen-pick"),
  run: document.getElementById("screen-run"),
  done: document.getElementById("screen-done"),
};

function show(name) {
  Object.entries(screens).forEach(([key, el]) => {
    el.classList.toggle("hidden", key !== name);
  });
}

const state = {
  catalog: null,
  queue: [],
  index: 0,
  currentId: "",
  results: {},
};

document.getElementById("btn-begin").addEventListener("click", async () => {
  const res = await fetch("/api/catalog");
  state.catalog = await res.json();
  const osHint = document.getElementById("os-hint");
  if (osHint && state.catalog.os) {
    osHint.textContent = "This computer: " + state.catalog.os + "/" + state.catalog.arch + ". Stay here until it says done.";
  }
  renderTools();
  show("pick");
});

document.getElementById("btn-go").addEventListener("click", async () => {
  const ids = [...document.querySelectorAll("#tool-list input:checked")].map((el) => el.value);
  if (ids.length === 0) {
    return;
  }
  state.queue = ids;
  state.index = 0;
  show("run");
  listen();
  await fetch("/api/run", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ toolIds: ids }),
  });
});

document.getElementById("btn-continue").addEventListener("click", () => ack("installed"));
document.getElementById("btn-skip").addEventListener("click", () => ack("skip"));

function renderTools() {
  const form = document.getElementById("tool-list");
  form.innerHTML = "";
  for (const tool of state.catalog.tools) {
    const label = document.createElement("label");
    label.className = "tool";
    const extra = extraFor(tool);
    const disabled = tool.available ? "" : "disabled";
    const checked = tool.selected && tool.available ? "checked" : "";
    if (!tool.available) {
      label.classList.add("unavailable");
    }
    label.innerHTML = `
      <input type="checkbox" value="${tool.id}" ${checked} ${disabled} />
      <span>
        <strong>${tool.name}</strong>
        <span class="badge">${tool.group}${extra ? " · " + extra : ""}</span>
        <small>${tool.reason || tool.summary}</small>
      </span>`;
    form.appendChild(label);
  }
}

function extraFor(tool) {
  switch (tool.kind) {
    case "windows_only":
      return "windows only";
    case "unavailable":
      return "no installer here";
    case "vendor_page":
      return "vendor page";
    case "download":
      return tool.version || "";
    default:
      return tool.kind || "";
  }
}

function listen() {
  const src = new EventSource("/api/events");
  src.onmessage = (ev) => {
    const data = JSON.parse(ev.data);
    onEvent(data);
  };
}

function onEvent(e) {
  if (!e.toolId && e.phase === "done") {
    renderSummary();
    show("done");
    return;
  }
  if (e.toolId) {
    state.currentId = e.toolId;
    const i = state.queue.indexOf(e.toolId);
    if (i >= 0) state.index = i;
  }
  document.getElementById("progress-label").textContent = `Step ${state.index + 1} of ${state.queue.length}`;
  document.getElementById("tool-name").textContent = e.name || toolName(e.toolId);
  document.getElementById("phase").textContent = labelPhase(e.phase);
  document.getElementById("message").textContent = e.message || "";
  const pct = e.total ? Math.min(100, Math.round((100 * e.got) / e.total)) : e.phase === "done" ? 100 : e.phase === "download" ? 8 : 40;
  document.getElementById("bar-fill").style.width = pct + "%";
  document.getElementById("bytes").textContent = e.total ? formatBytes(e.got) + " / " + formatBytes(e.total) : "";

  const need = !!e.needAck;
  document.getElementById("btn-continue").classList.toggle("hidden", !need);
  document.getElementById("btn-skip").classList.toggle("hidden", !need);
  const open = document.getElementById("btn-open");
  if (e.url) {
    open.href = e.url;
    open.classList.remove("hidden");
  } else {
    open.classList.add("hidden");
  }

  if (e.phase === "done" || e.phase === "skip" || e.phase === "error") {
    state.results[e.toolId] = e;
  }
}

function ack(action) {
  fetch("/api/ack", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ toolId: state.currentId, action }),
  });
  document.getElementById("btn-continue").classList.add("hidden");
  document.getElementById("btn-skip").classList.add("hidden");
}

function toolName(id) {
  const t = state.catalog?.tools?.find((x) => x.id === id);
  return t ? t.name : id;
}

function labelPhase(phase) {
  switch (phase) {
    case "check":
      return "Checking";
    case "download":
      return "Downloading";
    case "install":
      return "Installing";
    case "verify":
      return "Checking it worked";
    case "vendor":
      return "Vendor page";
    case "done":
      return "Done";
    case "skip":
      return "Skipped";
    case "error":
      return "Stopped";
    default:
      return phase || "";
  }
}

function formatBytes(n) {
  if (!n) return "0 B";
  const units = ["B", "KB", "MB", "GB"];
  let i = 0;
  let v = n;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i += 1;
  }
  return v.toFixed(i === 0 ? 0 : 1) + " " + units[i];
}

function renderSummary() {
  const ul = document.getElementById("summary");
  ul.innerHTML = "";
  for (const id of state.queue) {
    const ev = state.results[id];
    const li = document.createElement("li");
    const name = toolName(id);
    if (!ev) {
      li.textContent = name + " — no status";
    } else if (ev.phase === "error") {
      li.textContent = name + " — stopped. " + (ev.message || "");
    } else if (ev.phase === "skip") {
      li.textContent = name + " — skipped";
    } else if (ev.already) {
      li.textContent = name + " — already on this laptop";
    } else {
      li.textContent = name + " — finished";
    }
    ul.appendChild(li);
  }
}
