const screens = {
  pick: document.getElementById("screen-pick"),
  run: document.getElementById("screen-run"),
  done: document.getElementById("screen-done"),
};

const titles = {
  pick: { title: "Choose tools", lead: "Check the software this computer needs, then click Install. Required and recommended are labels only." },
  run: { title: "Installing", lead: "Stay on this window until the list finishes. Silent installers do not show Next/Next." },
  done: { title: "Finish", lead: "Review what ran. Unchecked tools were not installed." },
};

function show(name) {
  Object.entries(screens).forEach(([key, el]) => {
    el.classList.toggle("hidden", key !== name);
  });
  ["pick", "run", "done"].forEach((key) => {
    document.getElementById("step-" + key).classList.toggle("active", key === name);
    document.getElementById("step-" + key).classList.toggle("done", order(key) < order(name));
  });
  document.getElementById("title").textContent = titles[name].title;
  document.getElementById("lead").textContent = titles[name].lead;
}

function order(name) {
  return { pick: 0, run: 1, done: 2 }[name] ?? 0;
}

const state = {
  catalog: null,
  queue: [],
  index: 0,
  currentId: "",
  results: {},
  events: null,
};

async function loadCatalog() {
  const errBox = document.getElementById("catalog-error");
  try {
    const res = await fetch("/api/catalog");
    if (!res.ok) {
      throw new Error("Could not load the tool list (" + res.status + ").");
    }
    state.catalog = await res.json();
    errBox.classList.add("hidden");
    if (state.catalog.season) {
      document.getElementById("season").textContent = state.catalog.season + " season";
    }
    const osHint = document.getElementById("os-hint");
    if (osHint && state.catalog.os) {
      osHint.textContent =
        "This computer: " +
        prettyOS(state.catalog.os, state.catalog.arch) +
        ". Nothing installs unless you check it.";
    }
    renderTools();
  } catch (err) {
    errBox.textContent = err.message || "Could not load the tool list. Close this window and open setup again.";
    errBox.classList.remove("hidden");
    document.getElementById("btn-go").disabled = true;
  }
}

document.getElementById("btn-go").addEventListener("click", startInstall);

async function startInstall() {
  const ids = selectedIds();
  if (ids.length === 0) {
    return;
  }
  state.queue = ids;
  state.index = 0;
  state.results = {};
  state.currentId = "";
  document.getElementById("error-box").classList.add("hidden");
  document.getElementById("elevate-banner").classList.add("hidden");
  document.getElementById("bar").classList.remove("error");
  document.getElementById("phase").classList.remove("error");
  buildQueue();
  show("run");
  try {
    await listen();
    const res = await fetch("/api/run", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ toolIds: ids }),
    });
    if (!res.ok) {
      throw new Error("Could not start install (" + res.status + ").");
    }
  } catch (err) {
    showToolError("", err.message || "Could not start install.");
  }
}

function selectedIds() {
  return [...document.querySelectorAll("#tool-list input:checked")].map((el) => el.value);
}

function syncInstallButton() {
  const n = selectedIds().length;
  document.getElementById("btn-go").disabled = n === 0;
  document.getElementById("selected-count").textContent = n === 1 ? "1 selected" : n + " selected";
  document.getElementById("empty-state").classList.toggle("hidden", n > 0);
}

function groupTools(tools) {
  const order = ["required", "recommended", "optional"];
  const buckets = new Map();
  for (const tool of tools) {
    const key = tool.group || "other";
    if (!buckets.has(key)) buckets.set(key, []);
    buckets.get(key).push(tool);
  }
  const keys = [...buckets.keys()].sort((a, b) => {
    const ia = order.indexOf(a);
    const ib = order.indexOf(b);
    return (ia === -1 ? 99 : ia) - (ib === -1 ? 99 : ib);
  });
  return keys.map((key) => [key, buckets.get(key)]);
}

function groupLabel(group) {
  switch (group) {
    case "required":
      return "Core tools";
    case "recommended":
      return "Recommended";
    case "optional":
      return "Optional";
    default:
      return group;
  }
}

function renderTools() {
  const form = document.getElementById("tool-list");
  form.innerHTML = "";
  for (const [group, tools] of groupTools(state.catalog.tools)) {
    const section = document.createElement("section");
    section.className = "tool-group";
    const heading = document.createElement("h3");
    heading.textContent = groupLabel(group);
    section.appendChild(heading);
    for (const tool of tools) {
      section.appendChild(renderTool(tool));
    }
    form.appendChild(section);
  }
  syncInstallButton();
}

function renderTool(tool) {
  const label = document.createElement("label");
  label.className = "tool";
  if (!tool.available) label.classList.add("unavailable");
  if (tool.installed) label.classList.add("installed");

  const box = document.createElement("input");
  box.type = "checkbox";
  box.value = tool.id;
  box.disabled = !tool.available;
  box.addEventListener("change", syncInstallButton);

  const body = document.createElement("span");
  const nameRow = document.createElement("span");
  nameRow.className = "tool-name";
  const name = document.createElement("strong");
  name.textContent = tool.name;
  nameRow.appendChild(name);
  const extra = extraFor(tool);
  if (extra.text) {
    const badge = document.createElement("span");
    badge.className = "badge" + (extra.tone ? " " + extra.tone : "");
    badge.textContent = extra.text;
    nameRow.appendChild(badge);
  }
  const summary = document.createElement("small");
  summary.textContent = tool.reason || tool.summary || "";
  body.appendChild(nameRow);
  body.appendChild(summary);

  label.appendChild(box);
  label.appendChild(body);
  return label;
}

function extraFor(tool) {
  switch (tool.kind) {
    case "windows_only":
      return { text: "Windows only", tone: "warn" };
    case "unavailable":
      return { text: "Not available", tone: "warn" };
    case "vendor_page":
      return { text: "Vendor page", tone: "warn" };
    case "git_clone":
      return { text: "Optional clone" };
    case "download":
      if (tool.installed && tool.current) {
        return { text: "Installed", tone: "ok" };
      }
      if (tool.installed) {
        return { text: "Update", tone: "ok" };
      }
      return { text: tool.version || "" };
    default:
      return { text: "" };
  }
}

function prettyOS(os, arch) {
  const osName = { windows: "Windows", darwin: "macOS", linux: "Linux" }[os] || os;
  const archName = { amd64: "64-bit", arm64: "ARM64", "386": "32-bit" }[arch] || arch;
  return osName + " · " + archName;
}

function listen() {
  if (state.events) {
    state.events.close();
  }
  return new Promise((resolve) => {
    const src = new EventSource("/api/events");
    state.events = src;
    const timer = setTimeout(() => resolve(), 1500);
    src.addEventListener("hello", () => {
      clearTimeout(timer);
      resolve();
    });
    src.onmessage = (ev) => {
      const data = JSON.parse(ev.data);
      onEvent(data);
    };
  });
}

function onEvent(e) {
  if (!e.toolId && (e.phase === "done" || e.phase === "elevate")) {
    if (e.phase === "elevate") {
      document.getElementById("elevate-banner").classList.remove("hidden");
      document.getElementById("phase").textContent = labelPhase(e.phase);
      document.getElementById("tool-name").textContent = "Administrator approval";
      document.getElementById("message").textContent = e.message || "Windows needs one admin yes. After that, this window does the rest.";
      return;
    }
    if (state.events) {
      state.events.close();
      state.events = null;
    }
    renderSummary();
    show("done");
    return;
  }
  if (e.toolId) {
    state.currentId = e.toolId;
    const i = state.queue.indexOf(e.toolId);
    if (i >= 0) state.index = i;
    markQueue(e.toolId, e.phase);
  }
  document.getElementById("elevate-banner").classList.toggle("hidden", e.phase !== "elevate");
  document.getElementById("progress-label").textContent = `Step ${state.index + 1} of ${state.queue.length}`;
  document.getElementById("tool-name").textContent = e.name || toolName(e.toolId);
  document.getElementById("phase").textContent = labelPhase(e.phase);
  document.getElementById("phase").classList.toggle("error", e.phase === "error");
  document.getElementById("message").textContent = e.message || "";
  const pct = e.total ? Math.min(100, Math.round((100 * e.got) / e.total)) : e.phase === "done" ? 100 : e.phase === "download" ? 8 : 40;
  document.getElementById("bar-fill").style.width = pct + "%";
  document.getElementById("bar").setAttribute("aria-valuenow", String(pct));
  document.getElementById("bytes").textContent = e.total ? formatBytes(e.got) + " / " + formatBytes(e.total) : "";

  const open = document.getElementById("btn-open");
  if (e.url) {
    open.href = e.url;
    open.classList.remove("hidden");
  } else {
    open.classList.add("hidden");
  }

  if (e.phase === "error") {
    showToolError(e.toolId, e.message || "This step stopped.");
  } else {
    document.getElementById("error-box").classList.add("hidden");
    document.getElementById("bar").classList.remove("error");
  }

  if (e.phase === "done" || e.phase === "skip" || e.phase === "error") {
    state.results[e.toolId] = e;
  }
}

function showToolError(toolId, message) {
  const box = document.getElementById("error-box");
  box.textContent = message;
  box.classList.remove("hidden");
  document.getElementById("bar").classList.add("error");
  document.getElementById("phase").classList.add("error");
  document.getElementById("phase").textContent = "Stopped";
  if (toolId) markQueue(toolId, "error");
}

function buildQueue() {
  const ol = document.getElementById("run-queue");
  ol.innerHTML = "";
  for (const id of state.queue) {
    const li = document.createElement("li");
    li.id = "q-" + id;
    const mark = document.createElement("span");
    mark.className = "q-mark";
    const name = document.createElement("span");
    name.className = "q-name";
    name.textContent = toolName(id);
    const status = document.createElement("span");
    status.className = "q-status";
    status.textContent = "Waiting";
    li.appendChild(mark);
    li.appendChild(name);
    li.appendChild(status);
    ol.appendChild(li);
  }
}

function markQueue(id, phase) {
  const li = document.getElementById("q-" + id);
  if (!li) return;
  li.classList.remove("current", "ok", "skip", "error");
  const status = li.querySelector(".q-status");
  switch (phase) {
    case "error":
      li.classList.add("error");
      status.textContent = "Stopped";
      break;
    case "skip":
      li.classList.add("skip");
      status.textContent = "Skipped";
      break;
    case "done":
      li.classList.add("ok");
      status.textContent = "Done";
      break;
    default:
      li.classList.add("current");
      status.textContent = labelPhase(phase);
  }
}

function toolName(id) {
  const t = state.catalog?.tools?.find((x) => x.id === id);
  return t ? t.name : id;
}

function labelPhase(phase) {
  switch (phase) {
    case "check":
      return "Checking";
    case "elevate":
      return "Admin once";
    case "download":
      return "Downloading";
    case "install":
      return "Installing";
    case "verify":
      return "Checking it worked";
    case "vendor":
      return "Needs vendor page";
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
  let failed = 0;
  for (const id of state.queue) {
    const ev = state.results[id];
    const li = document.createElement("li");
    const name = toolName(id);
    if (!ev) {
      li.textContent = name + " — no status";
    } else if (ev.phase === "error") {
      failed += 1;
      li.className = "error";
      li.textContent = name + " — stopped. " + (ev.message || "");
    } else if (ev.phase === "skip") {
      li.className = "skip";
      if (ev.url) {
        const body = document.createElement("span");
        body.appendChild(document.createTextNode(name + " — needs vendor page · "));
        const a = document.createElement("a");
        a.href = ev.url;
        a.target = "_blank";
        a.rel = "noreferrer";
        a.textContent = "Open page";
        body.appendChild(a);
        li.appendChild(body);
      } else {
        li.textContent = name + " — skipped";
      }
    } else if (ev.already) {
      li.textContent = name + " — already on this computer";
    } else {
      li.textContent = name + " — finished";
    }
    ul.appendChild(li);
  }
  const kicker = document.getElementById("done-kicker");
  kicker.textContent = failed ? "Finished with " + failed + " stopped " + (failed === 1 ? "step" : "steps") : "Setup finished.";
}

loadCatalog();
