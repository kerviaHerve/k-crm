const themeNames = { carbon: "Carbon", atelier: "Atelier", studio: "Studio", mineral: "Minérale", sand: "Sable" };
const sizeNames = { s: "Compacte", l: "Standard", xl: "Confortable" };
const titles = { dash: "Tableau de bord", today: "Aujourd'hui", chrono: "Chrono", prospects: "Prospects", clients: "Clients", person: "Fiche", settings: "Réglages" };
const POLES = ["Kervia", "Exonik", "EmoSana", "OP3", "perso"];

let people = [];
let weekLoad = [
  { day: "Lun", n: 0 }, { day: "Mar", n: 0 }, { day: "Mer", n: 0 },
  { day: "Jeu", n: 0 }, { day: "Ven", n: 0 }, { day: "Sam", n: 0 }, { day: "Dim", n: 0 }
];
let currentView = "dash";
let selectedId = "";
let dialogAction = null;
let poleFilter = "all";

const $ = (id) => document.getElementById(id);
const esc = (s) => String(s ?? "").replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));

function isoShift(days) {
  const d = new Date();
  d.setDate(d.getDate() + days);
  return d.toISOString().slice(0, 10);
}

function setTheme(value) {
  const theme = value === "light" ? "light" : "dark";
  document.documentElement.dataset.theme = theme;
  try { localStorage.setItem("kcrm-alt-theme", theme); } catch (_) {}
  document.querySelectorAll("[data-theme-option]").forEach((btn) => {
    btn.setAttribute("aria-pressed", String(btn.dataset.themeOption === theme));
  });
  const status = $("themeStatus");
  if (status) status.textContent = theme === "light" ? "Papier" : "Sombre";
  const live = $("interfaceAnnouncement");
  if (live) live.textContent = theme === "light" ? "Ambiance papier" : "Ambiance sombre";
}

function setSize(value, focusButton = false) {
  const sizes = ["s", "l", "xl"];
  if (!sizes.includes(value)) value = "l";
  document.documentElement.dataset.kerviaSize = value;
  try { localStorage.setItem("kervia-ui-size", value); } catch (_) {}
  document.querySelectorAll("[data-size]").forEach((btn) => {
    btn.setAttribute("aria-pressed", String(btn.dataset.size === value));
  });
  const status = $("sizeStatus");
  if (status) status.textContent = `${sizeNames[value]} sélectionnée`;
  if (focusButton) document.querySelector(`[data-size="${value}"]`)?.focus();
}

function toast(text) {
  const el = $("toast");
  el.textContent = text;
  el.hidden = false;
  clearTimeout(toast._t);
  toast._t = setTimeout(() => { el.hidden = true; }, 2400);
}

function person(id) { return people.find((p) => p.id === id); }
function leadStateOf(p) { return p.lead_state || p.leadState || ""; }
function channelIcon(channel) {
  if (channel === "mail") return "Mail";
  if (channel === "rdv") return "Calendar";
  return "Phone";
}
function worldChip(world) {
  return `<span class="chip">${world === "client" ? "Client" : "Prospect"}</span>`;
}
function whenChip(when, label) {
  return `<span class="chip">${esc(label || when)}</span>`;
}
function markClass(when) {
  if (when === "overdue") return "late";
  if (when === "orphan") return "orphan";
  if (when === "today") return "today";
  return "ok";
}

async function loadState() {
  const r = await fetch("/ui/api/state");
  if (!r.ok) throw new Error("state");
  const data = await r.json();
  people = (data.people || []).map((p) => {
    p.leadState = leadStateOf(p);
    p.timeline = p.timeline || [];
    p.initials = p.initials || "?";
    return p;
  });
  if (Array.isArray(data.week_load) && data.week_load.length) weekLoad = data.week_load;
}

async function api(method, url, body) {
  const r = await fetch(url, {
    method,
    headers: body ? { "Content-Type": "application/json" } : undefined,
    body: body ? JSON.stringify(body) : undefined
  });
  const data = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(data.error || `http ${r.status}`);
  return data;
}

function counts() {
  const today = people.filter((p) => ["overdue", "today", "orphan"].includes(p.when)).length;
  const prospects = people.filter((p) => p.world === "prospect" && p.leadState !== "perdu").length;
  const clients = people.filter((p) => p.world === "client").length;
  document.querySelectorAll("[data-count=today]").forEach((el) => { el.textContent = String(today); });
  document.querySelectorAll("[data-count=prospects]").forEach((el) => { el.textContent = String(prospects); });
  document.querySelectorAll("[data-count=clients]").forEach((el) => { el.textContent = String(clients); });
}

function relanceCard(p, withActions) {
  const actions = withActions ? `
    <div class="actions">
      ${p.when === "orphan"
        ? `<button class="btn primary" type="button" data-act="plan" data-id="${p.id}">Poser une relance</button>`
        : `<button class="btn primary" type="button" data-quick="done" data-days="1" data-id="${p.id}">Fait, demain</button>
           <button class="btn" type="button" data-quick="snooze" data-days="3" data-id="${p.id}">+3 j</button>`}
      ${p.world === "prospect" && p.leadState !== "perdu" ? `<button class="btn" type="button" data-act="validate" data-id="${p.id}">Valider</button>` : ""}
    </div>` : `<div class="actions">${whenChip(p.when, p.whenLabel)}</div>`;
  return `
    <article class="relance" data-open="${p.id}" tabindex="0">
      <i class="mark ${markClass(p.when)}" aria-hidden="true"></i>
      <div>
        <div class="who">
          <span class="avatar">${esc(p.initials)}</span>
          <span><b>${esc(p.name)}</b><small>${esc(p.org)} · ${esc(p.pole)}</small></span>
        </div>
        <div class="meta">
          ${worldChip(p.world)}
          ${whenChip(p.when, p.whenLabel)}
          <span class="chip"><svg class="icon" aria-hidden="true"><use href="#${channelIcon(p.channel)}"/></svg>${esc(p.why || "Pas de suite")}</span>
          ${p.phone ? `<span class="chip"><svg class="icon" aria-hidden="true"><use href="#Phone"/></svg>${esc(p.phone)}</span>` : ""}
        </div>
      </div>
      ${actions}
    </article>`;
}

function personRow(p) {
  return `
    <button class="person-row" type="button" data-open="${p.id}">
      <i class="mark ${markClass(p.when)}" aria-hidden="true"></i>
      <div class="who">
        <span class="avatar">${esc(p.initials)}</span>
        <span><b>${esc(p.name)}</b><small>${esc(p.org)} · ${esc(p.pole)} · ${esc(p.leadState)}</small></span>
      </div>
      ${whenChip(p.when, p.whenLabel)}
    </button>`;
}

function detailHtml(p) {
  if (!p) return `<h2>Fiche</h2><p class="empty">Choisis une relance pour voir la personne.</p>`;
  const timeline = (p.timeline || []).map((e) => `
    <div class="event">
      <i class="event-icon"><svg class="icon"><use href="#Check"/></svg></i>
      <div><b>${esc(e.title)}</b><p>${esc(e.body)}</p><time>${esc(e.t)}</time></div>
    </div>`).join("");
  return `
    <h2>${esc(p.name)}</h2>
    <p class="scene-name">${esc(p.org)}</p>
    <div class="meta">
      ${worldChip(p.world)}
      <span class="chip">${esc(p.pole)}</span>
      <span class="chip">Lead ${esc(p.leadState)}</span>
      ${p.phone ? `<span class="chip"><svg class="icon" aria-hidden="true"><use href="#Phone"/></svg>${esc(p.phone)}</span>` : ""}
      ${p.email ? `<span class="chip"><svg class="icon" aria-hidden="true"><use href="#Mail"/></svg>${esc(p.email)}</span>` : ""}
    </div>
    ${p.lead ? `<p>${esc(p.lead)}</p>` : ""}
    <div class="next-action">
      <strong>${esc(p.next || "Aucune prochaine action")}</strong>
      <span>${p.world === "prospect" && !p.next && p.leadState !== "perdu" ? "Prospect cassé: relance, perdu, ou valider le lead." : esc(p.whenLabel)}</span>
    </div>
    <div class="timeline">${timeline}</div>`;
}

function byPole(list) {
  if (poleFilter === "all") return list;
  return list.filter((p) => p.pole === poleFilter);
}

function renderFilters() {
  const html = `
    <div class="filter-row" role="group" aria-label="Filtrer par pôle">
      <span class="filter-label">Pôle</span>
      <button class="chip-btn" type="button" data-pole="all" aria-pressed="${poleFilter === "all"}">Tous</button>
      ${POLES.map((pole) => `<button class="chip-btn" type="button" data-pole="${pole}" aria-pressed="${poleFilter === pole}">${pole}</button>`).join("")}
    </div>
    ${poleFilter === "all" ? "" : `<p class="filter-active">Filtre actif: ${poleFilter}. <button class="btn ghost" type="button" data-pole="all">Retirer</button></p>`}`;
  document.querySelectorAll("[data-filter-bar]").forEach((el) => { el.innerHTML = html; });
}

function renderDash() {
  const pool = byPole(people);
  const overdue = pool.filter((p) => p.when === "overdue");
  const today = pool.filter((p) => p.when === "today");
  const orphans = pool.filter((p) => p.when === "orphan");
  const clients = pool.filter((p) => p.world === "client");
  const prospects = pool.filter((p) => p.world === "prospect" && p.leadState !== "perdu");
  const kpis = [
    { value: overdue.length, label: "En retard", meaning: "À rattraper avant le reste.", tone: "late" },
    { value: today.length, label: "Aujourd'hui", meaning: "Relances dues ce jour.", tone: "today" },
    { value: orphans.length, label: "Prospects sans suite", meaning: "Fiches cassées. Relance, perdu, ou valider.", tone: "orphan" },
    { value: clients.length, label: "Clients", meaning: `${prospects.length} prospects ouverts. Conversion seulement par lead validé.`, tone: "ok" }
  ];
  $("dashKpis").innerHTML = kpis.map((k) => `
    <article class="kpi">
      <i class="mark ${k.tone}" aria-hidden="true"></i>
      <strong>${k.value}</strong>
      <b>${k.label}</b>
      <small>${k.meaning}</small>
    </article>`).join("");
  const max = Math.max(...weekLoad.map((d) => d.n), 1);
  $("dashChart").innerHTML = weekLoad.map((d) => `
    <div class="bar-col">
      <span class="bar" style="height:${Math.round((d.n / max) * 100)}%"></span>
      <b>${d.n}</b>
      <small>${esc(d.day)}</small>
    </div>`).join("");
  $("dashChartTable").querySelector("tbody").innerHTML = weekLoad.map((d) => `<tr><td>${esc(d.day)}</td><td>${d.n}</td></tr>`).join("");
  const alerts = [...overdue, ...orphans];
  $("dashAlerts").innerHTML = alerts.length
    ? alerts.map((p) => personRow(p)).join("")
    : `<div class="empty">Aucune exception.</div>`;
  const now = pool.filter((p) => ["overdue", "orphan", "today"].includes(p.when));
  $("dashNow").innerHTML = now.map((p) => relanceCard(p, true)).join("") || `<div class="empty">Rien à traiter.</div>`;
}

function renderToday() {
  const order = { overdue: 0, orphan: 1, today: 2, week: 3, none: 4 };
  const items = byPole(people.filter((p) => p.when !== "none" && p.leadState !== "perdu"))
    .sort((a, b) => order[a.when] - order[b.when]);
  const groups = [
    ["En retard", items.filter((p) => p.when === "overdue")],
    ["Sans relance", items.filter((p) => p.when === "orphan")],
    ["Aujourd'hui", items.filter((p) => p.when === "today")],
    ["Cette semaine", items.filter((p) => p.when === "week")]
  ];
  $("todayList").innerHTML = groups.map(([label, list]) => {
    if (!list.length) return "";
    return `<div class="section-label">${label}</div>${list.map((p) => relanceCard(p, true)).join("")}`;
  }).join("") || `<div class="empty">${poleFilter === "all" ? "File vide." : "Aucune relance dans ce pôle."}</div>`;
  $("todayDetail").innerHTML = detailHtml(person(selectedId));
}

function renderLists() {
  const prospects = byPole(people.filter((p) => p.world === "prospect"));
  const clients = byPole(people.filter((p) => p.world === "client"));
  $("prospectList").innerHTML = prospects.length ? prospects.map(personRow).join("") : `<div class="empty">Aucun prospect.</div>`;
  $("clientList").innerHTML = clients.length ? clients.map(personRow).join("") : `<div class="empty">Aucun client.</div>`;
}

function renderPerson(id) {
  const p = person(id);
  if (!p) return;
  selectedId = id;
  $("personTitle").textContent = p.name;
  $("personSub").textContent = `${p.world === "client" ? "Client" : "Prospect"} · ${p.pole}`;
  $("personScene").innerHTML = detailHtml(p);
  $("personActions").innerHTML = `
    <h2>Gestes</h2>
    <p>${p.world === "prospect" ? "Tant que le lead n'est pas validé, cette fiche reste un prospect." : "Lead déjà validé. Relance optionnelle."}</p>
    <div class="stack">
      ${p.world === "prospect" && p.leadState !== "perdu" ? `<button class="btn primary" type="button" data-act="validate" data-id="${p.id}">Valider le lead</button>` : ""}
      <button class="btn" type="button" data-act="edit" data-id="${p.id}">Modifier la fiche</button>
      <button class="btn" type="button" data-act="note" data-id="${p.id}">Ajouter une note</button>
      <button class="btn" type="button" data-act="${p.next ? "snooze" : "plan"}" data-id="${p.id}">${p.next ? "Reporter" : "Poser une relance"}</button>
      ${p.world === "prospect" && p.leadState !== "perdu" ? `<button class="btn" type="button" data-act="wait" data-id="${p.id}">En attente d'eux</button>` : ""}
      ${p.world === "prospect" && p.leadState !== "perdu" ? `<button class="btn danger" type="button" data-act="lost" data-id="${p.id}">Marquer perdu</button>` : ""}
    </div>`;
}

function renderChrono() {
  const today = new Date().toISOString().slice(0, 10);
  const pool = byPole(people.filter((p) => p.when !== "none" && p.leadState !== "perdu"));
  const undated = pool.filter((p) => !p.due);
  const dated = pool.filter((p) => p.due).sort((a, b) => a.due.localeCompare(b.due));
  const groups = [];
  if (undated.length) groups.push({ key: "none", title: "Sans date", tag: "À poser", items: undated });
  const byDay = new Map();
  dated.forEach((p) => {
    if (!byDay.has(p.due)) byDay.set(p.due, []);
    byDay.get(p.due).push(p);
  });
  for (const [iso, items] of byDay) {
    const d = new Date(`${iso}T12:00:00`);
    const title = d.toLocaleDateString("fr-CH", { weekday: "long", day: "numeric", month: "long" });
    let tag = "À venir";
    if (iso < today) tag = "En retard";
    else if (iso === today) tag = "Aujourd'hui";
    groups.push({ key: iso, title, tag, items });
  }
  $("chronoList").innerHTML = groups.length
    ? `<ol class="tl">${groups.map((g) => `
      <li class="tl-flag">
        <span class="tl-dot tl-dot-flag" aria-hidden="true"></span>
        <div class="tl-flag-copy"><small>${esc(g.tag)}</small><h2>${esc(g.title)}</h2></div>
      </li>
      ${g.items.map((p) => `
      <li class="tl-node">
        <span class="tl-dot ${markClass(p.when)}" aria-hidden="true"></span>
        <div class="tl-card">${relanceCard(p, true)}</div>
      </li>`).join("")}`).join("")}</ol>`
    : `<div class="empty">Aucune relance dans le temps.</div>`;
}

function showView(name, id) {
  currentView = name;
  if (id) selectedId = id;
  document.querySelectorAll(".view").forEach((el) => { el.hidden = el.id !== `view-${name}`; });
  document.querySelectorAll("[data-nav]").forEach((btn) => {
    btn.setAttribute("aria-current", btn.dataset.nav === name ? "page" : "false");
  });
  $("pageTitle").textContent = titles[name] || "K-CRM";
  if (name === "dash") renderDash();
  if (name === "today") renderToday();
  if (name === "chrono") renderChrono();
  if (name === "prospects" || name === "clients") renderLists();
  if (name === "person") renderPerson(selectedId);
  if (name === "settings") loadSettings();
  renderFilters();
  counts();
}

async function refresh(view, id) {
  await loadState();
  showView(view || currentView, id || selectedId);
}

function openDialog(action, id) {
  if (action === "edit") { openEdit(id); return; }
  const p = person(id);
  dialogAction = { action, id };
  $("dialog").hidden = false;
  const needDate = action !== "validate" && action !== "lost" && action !== "note";
  $("dialogFieldWrap").hidden = !needDate;
  $("dialogWhyWrap").hidden = !needDate;
  $("dialogNoteWrap").hidden = action !== "note";
  $("dialogField").required = needDate;
  $("dialogNote").required = action === "note";
  $("dialogNote").value = "";
  $("dialogWhy").value = p?.why || "";
  if (needDate) $("dialogField").value = isoShift(action === "done" ? 1 : 3);
  if (action === "validate") {
    $("dialogTitle").textContent = "Valider le lead";
    $("dialogCopy").textContent = `${p.name} devient client. Ce n'est pas un simple champ.`;
    $("dialogOk").textContent = "Valider";
  } else if (action === "lost") {
    $("dialogTitle").textContent = "Marquer perdu";
    $("dialogCopy").textContent = `${p.name} sort de la chasse. Plus d'obligation de relance.`;
    $("dialogOk").textContent = "Marquer perdu";
  } else if (action === "note") {
    $("dialogTitle").textContent = "Note sur le fil";
    $("dialogCopy").textContent = "Une note libre. Ça n'est pas une relance.";
    $("dialogOk").textContent = "Ajouter";
  } else if (action === "wait") {
    $("dialogTitle").textContent = "En attente d'eux";
    $("dialogCopy").textContent = "On attend leur retour. Une date de relance reste obligatoire.";
    $("dialogWhy").value = p?.why || "En attente d'eux";
    $("dialogOk").textContent = "Poser la date";
  } else if (action === "done") {
    $("dialogTitle").textContent = "Relance faite";
    $("dialogCopy").textContent = "Prospect: une relance faite exige la suivante.";
    $("dialogOk").textContent = "Poser la suite";
  } else {
    $("dialogTitle").textContent = action === "plan" ? "Poser une relance" : "Reporter";
    $("dialogCopy").textContent = "Une date est obligatoire. « Plus tard » n'existe pas.";
    $("dialogOk").textContent = "Enregistrer";
  }
}

function closeDialog() {
  $("dialog").hidden = true;
  dialogAction = null;
}

async function applyDialog() {
  const { action, id } = dialogAction;
  try {
    if (action === "validate") await api("POST", `/ui/api/people/${id}/validate`);
    else if (action === "lost") await api("POST", `/ui/api/people/${id}/lost`, { why: "Marqué perdu" });
    else if (action === "note") {
      const body = $("dialogNote").value.trim();
      if (!body) { $("dialogNote").focus(); return false; }
      await api("POST", `/ui/api/people/${id}/notes`, { body });
    } else {
      const due = $("dialogField").value;
      const why = $("dialogWhy").value.trim();
      if (!due || !why) { $("dialogField").focus(); return false; }
      const mode = action === "done" ? "complete" : (action === "wait" ? "wait" : "plan");
      await api("POST", `/ui/api/people/${id}/relance`, { mode, due, why, channel: "tel" });
    }
    toast("Enregistré.");
    closeDialog();
    await refresh(action === "validate" ? "clients" : action === "lost" ? "prospects" : currentView, id);
  } catch (err) {
    toast(err.message);
  }
  return true;
}

async function quick(id, kind, days) {
  const p = person(id);
  if (!p) return;
  const due = isoShift(days);
  const why = p.why || "suite";
  try {
    if (kind === "done") await api("POST", `/ui/api/people/${id}/relance`, { mode: "complete", due, why, channel: p.channel || "tel" });
    else await api("POST", `/ui/api/people/${id}/relance`, { mode: "plan", due, why, channel: p.channel || "tel" });
    toast(kind === "done" ? `Fait, suite le ${due}` : `Reporté au ${due}`);
    await refresh(currentView, id);
  } catch (err) {
    toast(err.message);
  }
}

function typingInField() {
  const t = document.activeElement;
  return t && (t.tagName === "INPUT" || t.tagName === "TEXTAREA" || t.tagName === "SELECT");
}

function openPalette() {
  $("palette").hidden = false;
  $("paletteInput").value = "";
  renderPalette();
  $("paletteInput").focus();
}

function closePalette() { $("palette").hidden = true; }

async function renderPalette() {
  const q = ($("paletteInput").value || "").trim();
  const verbs = [
    { label: "Nouveau prospect", sub: "N", run: () => openCreate() },
    { label: "Exporter CSV", sub: "sauvegarde", run: () => { closePalette(); window.location.href = "/ui/api/export.csv"; } },
    { label: "Importer CSV", sub: "prospects", run: () => { closePalette(); $("importFile").click(); } },
    { label: "Tableau de bord", sub: "vue", run: () => showView("dash") },
    { label: "Aujourd'hui", sub: "vue", run: () => showView("today") },
    { label: "Chrono", sub: "vue", run: () => showView("chrono") },
    { label: "Prospects", sub: "vue", run: () => showView("prospects") },
    { label: "Clients", sub: "vue", run: () => showView("clients") },
    { label: "Réglages", sub: "vue", run: () => showView("settings") }
  ];
  const qn = q.toLowerCase();
  const hits = verbs.filter((v) => !qn || v.label.toLowerCase().includes(qn));
  if (q.length >= 2) {
    try {
      const r = await fetch(`/ui/api/search?q=${encodeURIComponent(q)}`);
      const data = await r.json();
      (data.hits || []).slice(0, 8).forEach((h) => {
        hits.push({
          label: h.name,
          sub: `${h.match || "fiche"}${h.snippet ? " · " + h.snippet : ""}`,
          run: () => showView("person", h.id)
        });
      });
    } catch (_) {}
  } else if (q) {
    people.filter((p) => `${p.name} ${p.org} ${p.pole}`.toLowerCase().includes(qn))
      .slice(0, 8)
      .forEach((p) => hits.push({ label: p.name, sub: `${p.org} · ${p.pole}`, run: () => showView("person", p.id) }));
  }
  const shown = hits.slice(0, 12);
  $("paletteList").innerHTML = shown.map((h, i) => `
    <button class="palette-item" type="button" data-palette="${i}" ${i === 0 ? "data-first" : ""}>
      <b>${esc(h.label)}</b><small>${esc(h.sub)}</small>
    </button>`).join("") || `<p class="empty">Aucun résultat.</p>`;
  renderPalette._hits = shown;
}

let editId = null;

function openCreate() {
  editId = null;
  closePalette();
  $("create").hidden = false;
  $("createError").textContent = "";
  $("createForm").reset();
  $("createForm").querySelector("h2").textContent = "Nouveau prospect";
  $("cWhen").required = true;
  $("cWhy").required = true;
  $("cLead").required = true;
  $("cWhen").closest(".field").hidden = false;
  $("cWhy").closest(".field").hidden = false;
  $("cWhen").value = isoShift(1);
  $("cName").focus();
}

function openEdit(id) {
  const p = person(id);
  if (!p) return;
  editId = id;
  closePalette();
  $("create").hidden = false;
  $("createError").textContent = "";
  $("createForm").querySelector("h2").textContent = "Modifier la fiche";
  $("cName").value = p.name || "";
  $("cOrg").value = p.org || "";
  $("cPole").value = p.pole || "Kervia";
  $("cPhone").value = p.phone || "";
  $("cMail").value = p.email || "";
  $("cLead").value = p.lead || "";
  $("cWhen").required = false;
  $("cWhy").required = false;
  $("cLead").required = false;
  $("cWhen").closest(".field").hidden = true;
  $("cWhy").closest(".field").hidden = true;
  $("cName").focus();
}

function closeCreate() { $("create").hidden = true; editId = null; }

async function createProspect(event) {
  event.preventDefault();
  try {
    if (editId) {
      const updated = await api("POST", `/ui/api/people/${editId}`, {
        name: $("cName").value.trim(),
        org: $("cOrg").value.trim(),
        pole: $("cPole").value,
        phone: $("cPhone").value.trim(),
        email: $("cMail").value.trim(),
        lead: $("cLead").value.trim()
      });
      closeCreate();
      toast(`${updated.name} mis à jour.`);
      await refresh("person", updated.id);
      return;
    }
    const created = await api("POST", "/ui/api/prospects", {
      name: $("cName").value.trim(),
      org: $("cOrg").value.trim(),
      pole: $("cPole").value,
      phone: $("cPhone").value.trim(),
      email: $("cMail").value.trim(),
      lead: $("cLead").value.trim(),
      due: $("cWhen").value,
      why: $("cWhy").value.trim(),
      channel: "tel"
    });
    closeCreate();
    toast(`${created.name} est un prospect.`);
    await refresh("person", created.id);
  } catch (err) {
    $("createError").textContent = err.message;
  }
}

document.querySelectorAll("[data-theme-option]").forEach((btn) => {
  btn.addEventListener("click", () => setTheme(btn.dataset.themeOption));
});

const trigger = $("preferencesTrigger");
const preferences = $("preferences");
if (trigger) {
  trigger.addEventListener("click", () => showView("settings"));
}

document.addEventListener("keydown", (event) => {
  if (event.key === "Escape") {
    if (preferences && !preferences.hidden) { preferences.hidden = true; trigger?.setAttribute("aria-expanded", "false"); trigger?.focus(); }
    if (!$("create").hidden) closeCreate();
    if (!$("dialog").hidden) closeDialog();
    if (!$("palette").hidden) closePalette();
  }
  if (typingInField()) {
    if (event.key === "Enter" && !$("palette").hidden) {
      event.preventDefault();
      renderPalette._hits?.[0]?.run();
      closePalette();
    }
    return;
  }
  if (event.key === "/" ) { event.preventDefault(); openPalette(); }
  if (event.key === "n" || event.key === "N") { event.preventDefault(); openCreate(); }
  if (event.key === "1") {
    const first = people.find((p) => p.when === "overdue" || p.when === "today");
    if (first) quick(first.id, "done", 1);
  }
});
$("paletteInput").addEventListener("input", () => {
  clearTimeout(renderPalette._t);
  renderPalette._t = setTimeout(renderPalette, 120);
});
$("paletteList").addEventListener("click", (event) => {
  const btn = event.target.closest("[data-palette]");
  if (!btn) return;
  renderPalette._hits?.[Number(btn.dataset.palette)]?.run();
  closePalette();
});

document.addEventListener("pointerdown", (event) => {
  if (preferences && !preferences.hidden && !preferences.contains(event.target) && !trigger.contains(event.target)) {
    preferences.hidden = true;
    trigger.setAttribute("aria-expanded", "false");
  }
});

document.addEventListener("click", (event) => {
  const pole = event.target.closest("[data-pole]");
  if (pole && pole.dataset.pole) {
    poleFilter = pole.dataset.pole;
    showView(currentView === "person" ? "prospects" : currentView);
    return;
  }
  const nav = event.target.closest("[data-nav]");
  if (nav && nav.dataset.nav) { showView(nav.dataset.nav); return; }
  const quickBtn = event.target.closest("[data-quick]");
  if (quickBtn) {
    event.stopPropagation();
    quick(quickBtn.dataset.id, quickBtn.dataset.quick, Number(quickBtn.dataset.days));
    return;
  }
  const act = event.target.closest("[data-act]");
  if (act) { event.stopPropagation(); openDialog(act.dataset.act, act.dataset.id); return; }
  const open = event.target.closest("[data-open]");
  if (open) showView("person", open.dataset.open);
});

$("searchBtn").addEventListener("click", openPalette);
$("dialogCancel").addEventListener("click", closeDialog);
$("dialog").addEventListener("click", (event) => { if (event.target === $("dialog")) closeDialog(); });
$("palette").addEventListener("click", (event) => { if (event.target === $("palette")) closePalette(); });
$("dialogForm").addEventListener("submit", (event) => { event.preventDefault(); applyDialog(); });
$("newPersonBtn").addEventListener("click", openCreate);
$("newPersonBtn2").addEventListener("click", openCreate);
$("createCancel").addEventListener("click", closeCreate);
$("create").addEventListener("click", (event) => { if (event.target === $("create")) closeCreate(); });
$("createForm").addEventListener("submit", createProspect);
$("importFile").addEventListener("change", async (event) => {
  const file = event.target.files && event.target.files[0];
  event.target.value = "";
  if (!file) return;
  const data = new FormData();
  data.append("file", file);
  try {
    const r = await fetch("/ui/api/import.csv", { method: "POST", body: data });
    const out = await r.json();
    if (!r.ok) throw new Error(out.error || "import");
    toast(`${out.created || 0} prospects créés, ${out.skipped || 0} ignorés.`);
    await refresh("today");
  } catch (err) {
    toast(err.message);
  }
});

function setAvatar(has) {
  ["ownerAvatar", "settingsAvatar"].forEach((id) => {
    const img = $(id);
    if (!img) return;
    if (has) {
      img.src = `/ui/api/avatar?t=${Date.now()}`;
      img.hidden = false;
    } else {
      img.hidden = true;
    }
  });
  const fb = $("settingsAvatarFallback");
  if (fb) fb.hidden = !!has;
}

function renderKeys(keys) {
  const list = $("keyList");
  if (!list) return;
  list.innerHTML = (keys || []).map((k) => `
    <div class="key-row">
      <div>
        <strong>${esc(k.name)}</strong>
        <div><code>${esc(k.prefix)}…</code></div>
      </div>
      <button class="btn ghost" type="button" data-revoke="${esc(k.id)}">Révoquer</button>
    </div>`).join("") || `<p class="empty">Aucune clé.</p>`;
}

async function loadSettings() {
  try {
    const data = await api("GET", "/ui/api/settings");
    $("settingsUser").textContent = data.user || "—";
    const fb = $("settingsAvatarFallback");
    if (fb) fb.textContent = (data.user || "?").slice(0, 1).toUpperCase();
    setAvatar(!!data.has_avatar);
    renderKeys(data.keys);
  } catch (err) {
    toast(err.message);
  }
}

$("logoutBtn")?.addEventListener("click", async () => {
  try { await api("POST", "/logout"); } catch (_) {}
  location.href = "/login";
});
$("avatarPick")?.addEventListener("click", () => $("avatarFile").click());
$("avatarFile")?.addEventListener("change", async (event) => {
  const file = event.target.files && event.target.files[0];
  event.target.value = "";
  if (!file) return;
  const data = new FormData();
  data.append("file", file);
  $("avatarErr").textContent = "";
  try {
    const r = await fetch("/ui/api/avatar", { method: "POST", body: data });
    const out = await r.json();
    if (!r.ok) throw new Error(out.error || "avatar");
    setAvatar(true);
    toast("Avatar enregistré.");
  } catch (err) {
    $("avatarErr").textContent = err.message;
  }
});
$("passwordForm")?.addEventListener("submit", async (event) => {
  event.preventDefault();
  $("pwErr").textContent = "";
  try {
    await api("POST", "/ui/api/settings/password", {
      current: $("pwCurrent").value,
      next: $("pwNext").value,
      code: $("pwCode").value.trim()
    });
    $("passwordForm").reset();
    toast("Mot de passe changé.");
  } catch (err) {
    $("pwErr").textContent = err.message;
  }
});
$("totpStartForm")?.addEventListener("submit", async (event) => {
  event.preventDefault();
  $("totpErr").textContent = "";
  try {
    const data = await api("POST", "/ui/api/settings/totp/start", {
      password: $("totpPass").value,
      code: $("totpOld").value.trim()
    });
    $("totpSecret").textContent = data.secret;
    $("totpConfirmForm").hidden = false;
    $("totpNew").focus();
  } catch (err) {
    $("totpErr").textContent = err.message;
  }
});
$("totpConfirmForm")?.addEventListener("submit", async (event) => {
  event.preventDefault();
  $("totpErr").textContent = "";
  try {
    await api("POST", "/ui/api/settings/totp/confirm", { code: $("totpNew").value.trim() });
    $("totpStartForm").reset();
    $("totpConfirmForm").reset();
    $("totpConfirmForm").hidden = true;
    toast("Nouveau 2FA actif.");
  } catch (err) {
    $("totpErr").textContent = err.message;
  }
});
$("keyForm")?.addEventListener("submit", async (event) => {
  event.preventDefault();
  $("keyErr").textContent = "";
  try {
    const out = await api("POST", "/ui/api/keys", { name: $("keyName").value.trim() });
    $("keyName").value = "";
    $("keyOnce").hidden = false;
    $("keyOnce").textContent = out.token;
    await loadSettings();
    toast("Clé créée. Copie-la maintenant.");
  } catch (err) {
    $("keyErr").textContent = err.message;
  }
});
$("keyList")?.addEventListener("click", async (event) => {
  const btn = event.target.closest("[data-revoke]");
  if (!btn) return;
  $("keyErr").textContent = "";
  try {
    const r = await fetch(`/ui/api/keys/${btn.dataset.revoke}`, { method: "DELETE" });
    const out = await r.json().catch(() => ({}));
    if (!r.ok) throw new Error(out.error || "revoke");
    $("keyOnce").hidden = true;
    await loadSettings();
    toast("Clé révoquée.");
  } catch (err) {
    $("keyErr").textContent = err.message;
  }
});

setTheme(document.documentElement.dataset.theme);
loadState().then(() => {
  fetch("/ui/api/settings").then((r) => r.json()).then((d) => setAvatar(!!d.has_avatar)).catch(() => {});
  showView("today");
}).catch((err) => toast(err.message));
