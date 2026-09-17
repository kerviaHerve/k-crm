const titles = { dash: "Tableau de bord", today: "Aujourd'hui", chrono: "Chrono", prospects: "Prospects", clients: "Clients", lost: "Perdus", person: "Fiche", settings: "Réglages" };

let people = [];
let weekLoad = [
  { day: "Lun", n: 0 }, { day: "Mar", n: 0 }, { day: "Mer", n: 0 },
  { day: "Jeu", n: 0 }, { day: "Ven", n: 0 }, { day: "Sam", n: 0 }, { day: "Dim", n: 0 }
];
let currentView = "dash";
let selectedId = "";
let dialogAction = null;
let poleFilter = "all";
let activities = [];

const $ = (id) => document.getElementById(id);
const esc = (s) => String(s ?? "").replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
function ico(name) {
  return `<svg class="icon" aria-hidden="true"><use href="#${name}"/></svg>`;
}

function metaBits(p) {
  return [p.org, p.pole].filter(Boolean).map(esc).join(" · ");
}

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
  const lost = people.filter((p) => p.leadState === "perdu").length;
  document.querySelectorAll("[data-count=today]").forEach((el) => { el.textContent = String(today); });
  document.querySelectorAll("[data-count=prospects]").forEach((el) => { el.textContent = String(prospects); });
  document.querySelectorAll("[data-count=clients]").forEach((el) => { el.textContent = String(clients); });
  document.querySelectorAll("[data-count=lost]").forEach((el) => { el.textContent = String(lost); });
}

function relanceCard(p, withActions) {
  const actions = withActions ? `
    <div class="actions">
      ${p.when === "orphan"
        ? `<button class="btn primary" type="button" data-act="plan" data-id="${p.id}">${ico("Calendar")}Poser une relance</button>`
        : `<button class="btn primary" type="button" data-quick="done" data-days="1" data-id="${p.id}">${ico("Check")}Fait, demain</button>
           <button class="btn" type="button" data-quick="snooze" data-days="3" data-id="${p.id}">${ico("Calendar")}+3 j</button>`}
      ${p.world === "prospect" && p.leadState !== "perdu" ? `<button class="btn" type="button" data-act="validate" data-id="${p.id}">${ico("Check")}Valider</button>` : ""}
    </div>` : `<div class="actions">${whenChip(p.when, p.whenLabel)}</div>`;
  return `
    <article class="relance" data-open="${p.id}" tabindex="0">
      <i class="mark ${markClass(p.when)}" aria-hidden="true"></i>
      <div>
        <div class="who">
          <span class="avatar">${esc(p.initials)}</span>
          <span><b>${esc(p.name)}</b><small>${metaBits(p)}</small></span>
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
        <span><b>${esc(p.name)}</b><small>${metaBits(p)}${p.leadState ? (metaBits(p) ? " · " : "") + esc(p.leadState) : ""}</small></span>
      </div>
      ${whenChip(p.when, p.whenLabel)}
    </button>`;
}

function telHref(phone) {
  const t = String(phone || "").replace(/[^\d+]/g, "");
  return t ? `tel:${t}` : "";
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
      ${p.pole ? `<span class="chip">${esc(p.pole)}</span>` : ""}
      <span class="chip">Lead ${esc(p.leadState)}</span>
      ${p.phone ? `<a class="chip" href="${telHref(p.phone)}"><svg class="icon" aria-hidden="true"><use href="#Phone"/></svg>${esc(p.phone)}</a>` : ""}
      ${p.email ? `<a class="chip" href="mailto:${esc(p.email)}"><svg class="icon" aria-hidden="true"><use href="#Mail"/></svg>${esc(p.email)}</a>` : ""}
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
  document.querySelectorAll("[data-filter-bar]").forEach((el) => {
    if (!activities.length) {
      el.innerHTML = "";
      return;
    }
    el.innerHTML = `
    <div class="filter-row" role="group" aria-label="Filtrer par activité">
      <span class="filter-label">Activité</span>
      <button class="chip-btn" type="button" data-pole="all" aria-pressed="${poleFilter === "all"}">Tous</button>
      ${activities.map((pole) => `<button class="chip-btn" type="button" data-pole="${esc(pole)}" aria-pressed="${poleFilter === pole}">${esc(pole)}</button>`).join("")}
    </div>
    ${poleFilter === "all" ? "" : `<p class="filter-active">Filtre actif: ${esc(poleFilter)}. <button class="btn ghost" type="button" data-pole="all">Retirer</button></p>`}`;
  });
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
  }).join("") || `<div class="empty">${poleFilter === "all" ? "File vide." : "Aucune relance dans cette activité."}</div>`;
  $("todayDetail").innerHTML = detailHtml(person(selectedId));
}

function renderLists() {
  const prospects = byPole(people.filter((p) => p.world === "prospect" && p.leadState !== "perdu"));
  const clients = byPole(people.filter((p) => p.world === "client"));
  const lost = byPole(people.filter((p) => p.leadState === "perdu"));
  $("prospectList").innerHTML = prospects.length ? prospects.map(personRow).join("") : `<div class="empty">Aucun prospect.</div>`;
  $("clientList").innerHTML = clients.length ? clients.map(personRow).join("") : `<div class="empty">Aucun client.</div>`;
  if ($("lostList")) {
    $("lostList").innerHTML = lost.length ? lost.map(personRow).join("") : `<div class="empty">Aucun perdu.</div>`;
  }
}

function renderPerson(id) {
  const p = person(id);
  if (!p) return;
  selectedId = id;
  $("personTitle").textContent = p.name;
  $("personSub").textContent = `${p.world === "client" ? "Client" : "Prospect"}${p.pole ? " · " + p.pole : ""}`;
  $("personScene").innerHTML = detailHtml(p);
  $("personActions").innerHTML = `
    <h2>Gestes</h2>
    <p>${p.world === "prospect" ? "Tant que le lead n'est pas validé, cette fiche reste un prospect." : "Lead déjà validé. Relance optionnelle."}</p>
    <div class="classify field">
      <label for="personPole">Activité</label>
      <select id="personPole" data-classify="${p.id}">${activityOptions(p.pole || "")}</select>
      <div class="classify-new" id="personPoleNewWrap" hidden>
        <label for="personPoleNew">Nouvelle activité</label>
        <input id="personPoleNew" maxlength="40" placeholder="Conseil, atelier…">
        <button class="btn primary" type="button" data-classify-new="${p.id}">Classer</button>
      </div>
    </div>
    <div class="stack">
      ${p.phone ? `<a class="btn" href="${telHref(p.phone)}">${ico("Phone")}Appeler</a>` : ""}
      ${p.email ? `<a class="btn" href="mailto:${esc(p.email)}">${ico("Send")}Écrire</a>` : ""}
      <button class="btn" type="button" data-act="edit" data-id="${p.id}">${ico("Pencil")}Modifier</button>
      <button class="btn" type="button" data-act="note" data-id="${p.id}">${ico("StickyNote")}Note</button>
    </div>
    <details class="fold" open>
      <summary>${ico("Calendar")}Relance</summary>
      <div class="stack">
        ${p.due ? `<a class="btn" href="/ui/api/people/${p.id}/relance.ics">${ico("Calendar")}Agenda</a>` : ""}
        <button class="btn" type="button" data-act="${p.next ? "snooze" : "plan"}" data-id="${p.id}">${ico("Calendar")}${p.next ? "Reporter" : "Poser une relance"}</button>
        ${p.world === "prospect" && p.leadState !== "perdu" ? `<button class="btn" type="button" data-act="wait" data-id="${p.id}">${ico("Clock")}En attente d'eux</button>` : ""}
      </div>
    </details>
    ${p.world === "prospect" && p.leadState !== "perdu" ? `<details class="fold">
      <summary>${ico("Check")}Décision</summary>
      <div class="stack">
        <button class="btn primary" type="button" data-act="validate" data-id="${p.id}">${ico("Check")}Valider le lead</button>
        <button class="btn danger" type="button" data-act="lost" data-id="${p.id}">${ico("Ban")}Marquer perdu</button>
      </div>
    </details>` : ""}`;
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
  if (name === "prospects" || name === "clients" || name === "lost") renderLists();
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
    { label: "Perdus", sub: "vue", run: () => showView("lost") },
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
      .forEach((p) => hits.push({ label: p.name, sub: [p.org, p.pole].filter(Boolean).join(" · "), run: () => showView("person", p.id) }));
  }
  const shown = hits.slice(0, 12);
  $("paletteList").innerHTML = shown.map((h, i) => `
    <button class="palette-item" type="button" data-palette="${i}" ${i === 0 ? "data-first" : ""}>
      <b>${esc(h.label)}</b><small>${esc(h.sub)}</small>
    </button>`).join("") || `<p class="empty">Aucun résultat.</p>`;
  renderPalette._hits = shown;
}

let editId = null;

function activityOptions(selected) {
  const list = activities.slice();
  if (selected && !list.includes(selected)) list.push(selected);
  return [`<option value="">Aucune</option>`]
    .concat(list.map((a) => `<option value="${esc(a)}"${a === selected ? " selected" : ""}>${esc(a)}</option>`))
    .concat(`<option value="__new__">Nouvelle…</option>`)
    .join("");
}

function toggleNewActivityField(on) {
  const wrap = $("cPoleNewWrap");
  if (wrap) wrap.hidden = !on;
  const inp = $("cPoleNew");
  if (!inp) return;
  if (on) inp.focus();
  else inp.value = "";
}

function chosenPole() {
  const sel = $("cPole")?.value || "";
  if (sel === "__new__") return ($("cPoleNew")?.value || "").trim();
  return sel;
}

async function reloadActivities() {
  try {
    const d = await api("GET", "/ui/api/settings");
    activities = d.activities || [];
    if (poleFilter !== "all" && !activities.includes(poleFilter)) poleFilter = "all";
    fillActivitySelect();
    renderActivitySettings();
  } catch (_) {}
}

async function classifyPerson(id, pole) {
  const p = person(id);
  if (!p) return;
  try {
    const updated = await api("POST", `/ui/api/people/${id}`, {
      name: p.name,
      org: p.org,
      pole: pole || "",
      phone: p.phone,
      email: p.email,
      lead: p.lead
    });
    toast(pole ? `${updated.name} · ${pole}` : `${updated.name} sans activité`);
    await reloadActivities();
    await refresh("person", id);
  } catch (e) {
    toast(e.message);
  }
}

function fillActivitySelect(selected) {
  const el = $("cPole");
  if (!el) return;
  el.innerHTML = activityOptions(selected);
  toggleNewActivityField(false);
}

function renderActivitySettings() {
  const el = $("activityList");
  if (!el) return;
  el.innerHTML = activities.length
    ? activities.map((a, i) => `<li><span>${esc(a)}</span><button class="btn ghost" type="button" data-activity-del="${i}">Retirer</button></li>`).join("")
    : `<li class="empty">Aucune. Ajoute les tiennes.</li>`;
}

async function saveActivities(next) {
  const err = $("activityErr");
  if (err) err.textContent = "";
  try {
    const data = await api("POST", "/ui/api/settings/activities", { activities: next });
    activities = data.activities || [];
    if (poleFilter !== "all" && !activities.includes(poleFilter)) poleFilter = "all";
    fillActivitySelect();
    renderActivitySettings();
    showView(currentView);
  } catch (e) {
    if (err) err.textContent = e.message;
  }
}

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
  fillActivitySelect("");
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
  fillActivitySelect(p.pole || "");
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
        pole: chosenPole(),
        phone: $("cPhone").value.trim(),
        email: $("cMail").value.trim(),
        lead: $("cLead").value.trim()
      });
      closeCreate();
      toast(`${updated.name} mis à jour.`);
      await reloadActivities();
      await refresh("person", updated.id);
      return;
    }
    const created = await api("POST", "/ui/api/prospects", {
      name: $("cName").value.trim(),
      org: $("cOrg").value.trim(),
      pole: chosenPole(),
      phone: $("cPhone").value.trim(),
      email: $("cMail").value.trim(),
      lead: $("cLead").value.trim(),
      due: $("cWhen").value,
      why: $("cWhy").value.trim(),
      channel: "tel"
    });
    closeCreate();
    toast(`${created.name} est un prospect.`);
    await reloadActivities();
    await refresh("person", created.id);
  } catch (err) {
    $("createError").textContent = err.message;
  }
}

document.querySelectorAll("[data-theme-option]").forEach((btn) => {
  btn.addEventListener("click", () => setTheme(btn.dataset.themeOption));
});

document.addEventListener("keydown", (event) => {
  if (event.key === "Escape") {
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
  const classifyNew = event.target.closest("[data-classify-new]");
  if (classifyNew) {
    event.stopPropagation();
    const name = ($("personPoleNew")?.value || "").trim();
    if (name) classifyPerson(classifyNew.dataset.classifyNew, name);
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
$("cPole")?.addEventListener("change", () => {
  toggleNewActivityField($("cPole").value === "__new__");
});
document.addEventListener("change", (event) => {
  const sel = event.target.closest("[data-classify]");
  if (!sel) return;
  const wrap = $("personPoleNewWrap");
  if (sel.value === "__new__") {
    if (wrap) wrap.hidden = false;
    $("personPoleNew")?.focus();
    return;
  }
  if (wrap) wrap.hidden = true;
  classifyPerson(sel.dataset.classify, sel.value);
});
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

function setOwner(user) {
  const el = $("ownerName");
  if (el) el.textContent = user || "";
}

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
    setOwner(data.user);
    const fb = $("settingsAvatarFallback");
    if (fb) fb.textContent = (data.user || "?").slice(0, 1).toUpperCase();
    setAvatar(!!data.has_avatar);
    renderKeys(data.keys);
    activities = data.activities || [];
    fillActivitySelect();
    renderActivitySettings();
    renderBackups(data.backups || []);
    renderMailAccounts(data.mail_accounts || []);
  } catch (err) {
    toast(err.message);
  }
}

function showSettingsTab(name) {
  const email = name === "email";
  const paneCompte = $("settingsPaneCompte");
  const paneEmail = $("settingsPaneEmail");
  if (paneCompte) paneCompte.hidden = email;
  if (paneEmail) paneEmail.hidden = !email;
  document.querySelectorAll("[data-settings-tab]").forEach((btn) => {
    btn.setAttribute("aria-selected", String(btn.dataset.settingsTab === (email ? "email" : "compte")));
  });
}

function mailPort(v) {
  const n = parseInt(String(v || "").trim(), 10);
  return Number.isFinite(n) && n > 0 ? n : 0;
}

function fillSmtp(acc) {
  if (!$("smtpForm")) return;
  $("smtpId").value = acc?.id || "";
  $("smtpName").value = acc?.name || "";
  $("smtpHost").value = acc?.host || "";
  $("smtpPort").value = acc?.port || "";
  $("smtpSec").value = acc?.security || "starttls";
  $("smtpUser").value = acc?.username || "";
  $("smtpPass").value = "";
  $("smtpFrom").value = acc?.from || "";
  const out = $("smtpOut");
  if (!out) return;
  if (acc?.last_ok) {
    out.hidden = false;
    out.textContent = "Dernier test OK " + acc.last_ok;
  } else if (acc?.last_error) {
    out.hidden = false;
    out.textContent = "Dernier test : " + acc.last_error;
  } else {
    out.hidden = true;
  }
}

function renderMailAccounts(list) {
  const accounts = list || [];
  fillSmtp(accounts.find((a) => a.kind === "smtp"));
  const imaps = accounts.filter((a) => a.kind === "imap");
  const el = $("imapList");
  if (!el) return;
  el.innerHTML = imaps.map((a) => `
    <div class="key-row">
      <div>
        <strong>${esc(a.name)}</strong>
        <div><code>${esc(a.username)} · ${esc(a.host)}:${a.port} · ${esc(a.folder || "INBOX")}</code></div>
        <div><code>${esc(a.last_ok ? "OK " + a.last_ok : (a.last_error || "pas encore testé"))}</code></div>
      </div>
      <div class="actions">
        <button class="btn ghost" type="button" data-mail-test="${esc(a.id)}">Tester</button>
        <button class="btn ghost" type="button" data-mail-del="${esc(a.id)}">Supprimer</button>
      </div>
    </div>`).join("") || `<p class="empty">Aucune boîte.</p>`;
}

function renderBackups(list) {
  const el = $("backupList");
  if (!el) return;
  el.innerHTML = (list || []).map((b) => `
    <div class="key-row">
      <div>
        <strong>${esc(b.name)}</strong>
        <div><code>${esc(b.at || "")} · ${b.size || 0} o</code></div>
      </div>
      <div class="actions">
        <a class="btn ghost" href="/ui/api/backups/${encodeURIComponent(b.name)}">Télécharger</a>
        <button class="btn ghost" type="button" data-restore="${esc(b.name)}">Restaurer</button>
      </div>
    </div>`).join("") || `<p class="empty">Aucune copie.</p>`;
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
    const qr = $("totpQR");
    const wrap = $("totpQRWrap");
    if (qr && wrap) {
      if (typeof data.qr === "string" && data.qr.startsWith("data:image/png")) {
        qr.src = data.qr;
        wrap.hidden = false;
      } else {
        qr.removeAttribute("src");
        wrap.hidden = true;
      }
    }
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
    const qr = $("totpQR");
    const wrap = $("totpQRWrap");
    if (qr) qr.removeAttribute("src");
    if (wrap) wrap.hidden = true;
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
$("backupNow")?.addEventListener("click", async () => {
  $("backupErr").textContent = "";
  try {
    await api("POST", "/ui/api/backups");
    await loadSettings();
    toast("Sauvegarde créée.");
  } catch (err) {
    $("backupErr").textContent = err.message;
  }
});
$("backupList")?.addEventListener("click", async (event) => {
  const btn = event.target.closest("[data-restore]");
  if (!btn) return;
  if (!confirm("Remplacer le carnet par cette copie au prochain démarrage ?")) return;
  $("backupErr").textContent = "";
  try {
    const out = await api("POST", `/ui/api/backups/${encodeURIComponent(btn.dataset.restore)}/restore`);
    toast(out.restart ? "Restauration posée. Relance en cours." : "Restauration posée. Relance k-crm pour l'appliquer.");
  } catch (err) {
    $("backupErr").textContent = err.message;
  }
});
document.querySelectorAll("[data-settings-tab]").forEach((btn) => {
  btn.addEventListener("click", () => showSettingsTab(btn.dataset.settingsTab));
});
$("smtpForm")?.addEventListener("submit", async (event) => {
  event.preventDefault();
  $("smtpErr").textContent = "";
  const id = $("smtpId").value;
  const body = {
    name: $("smtpName").value.trim(),
    kind: "smtp",
    host: $("smtpHost").value.trim(),
    port: mailPort($("smtpPort").value),
    security: $("smtpSec").value,
    username: $("smtpUser").value.trim(),
    password: $("smtpPass").value,
    from: $("smtpFrom").value.trim()
  };
  try {
    if (id) await api("POST", `/ui/api/mail-accounts/${id}`, body);
    else await api("POST", "/ui/api/mail-accounts", body);
    $("smtpPass").value = "";
    await loadSettings();
    toast(id ? "Compte d'envoi mis à jour." : "Compte d'envoi enregistré.");
  } catch (err) {
    $("smtpErr").textContent = err.message;
  }
});
$("smtpTest")?.addEventListener("click", async () => {
  $("smtpErr").textContent = "";
  const id = $("smtpId").value;
  if (!id) {
    $("smtpErr").textContent = "enregistre le compte avant de tester";
    return;
  }
  try {
    const out = await api("POST", `/ui/api/mail-accounts/${id}/test`);
    await loadSettings();
    toast(out.ok ? "SMTP OK." : ("SMTP : " + (out.error || "échec")));
    if (!out.ok) $("smtpErr").textContent = out.error || "échec";
  } catch (err) {
    $("smtpErr").textContent = err.message;
  }
});
$("imapForm")?.addEventListener("submit", async (event) => {
  event.preventDefault();
  $("imapErr").textContent = "";
  try {
    await api("POST", "/ui/api/mail-accounts", {
      name: $("imapName").value.trim(),
      kind: "imap",
      host: $("imapHost").value.trim(),
      port: mailPort($("imapPort").value),
      security: $("imapSec").value,
      username: $("imapUser").value.trim(),
      password: $("imapPass").value,
      folder: $("imapFolder").value.trim() || "INBOX"
    });
    $("imapForm").reset();
    $("imapSec").value = "tls";
    $("imapFolder").value = "INBOX";
    await loadSettings();
    toast("Boîte ajoutée.");
  } catch (err) {
    $("imapErr").textContent = err.message;
  }
});
$("imapList")?.addEventListener("click", async (event) => {
  const test = event.target.closest("[data-mail-test]");
  const del = event.target.closest("[data-mail-del]");
  $("imapErr").textContent = "";
  try {
    if (test) {
      const out = await api("POST", `/ui/api/mail-accounts/${test.dataset.mailTest}/test`);
      await loadSettings();
      toast(out.ok ? "IMAP OK." : ("IMAP : " + (out.error || "échec")));
      if (!out.ok) $("imapErr").textContent = out.error || "échec";
      return;
    }
    if (del) {
      const r = await fetch(`/ui/api/mail-accounts/${del.dataset.mailDel}`, { method: "DELETE" });
      const out = await r.json().catch(() => ({}));
      if (!r.ok) throw new Error(out.error || "delete");
      await loadSettings();
      toast("Boîte supprimée.");
    }
  } catch (err) {
    $("imapErr").textContent = err.message;
  }
});
$("ingestForm")?.addEventListener("submit", async (event) => {
  event.preventDefault();
  $("ingestErr").textContent = "";
  $("ingestOut").hidden = true;
  try {
    const file = $("ingestFile")?.files?.[0];
    let raw = $("ingestRaw")?.value || "";
    if (file) raw = await file.text();
    if (!String(raw).trim()) throw new Error("colle un message ou choisis un .eml");
    const r = await fetch("/ui/api/ingest", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ raw }),
    });
    const out = await r.json().catch(() => ({}));
    if (!r.ok) throw new Error(out.error || "ingest");
    const line = [out.action, out.reason, out.person_name, out.subject].filter(Boolean).join(" · ");
    $("ingestOut").hidden = false;
    $("ingestOut").textContent = line;
    if (out.action === "filed") {
      await loadState();
      toast("Mail classé sur " + (out.person_name || "la fiche"));
    } else {
      toast("Non classé : " + (out.reason || out.action));
    }
  } catch (err) {
    $("ingestErr").textContent = err.message;
  }
});

setTheme(document.documentElement.dataset.theme);
$("activityForm")?.addEventListener("submit", async (event) => {
  event.preventDefault();
  const name = $("activityName").value.trim();
  if (!name) return;
  await saveActivities(activities.concat(name));
  $("activityName").value = "";
});
$("activityList")?.addEventListener("click", async (event) => {
  const btn = event.target.closest("[data-activity-del]");
  if (!btn) return;
  const i = Number(btn.dataset.activityDel);
  await saveActivities(activities.filter((_, idx) => idx !== i));
});
loadState().then(async () => {
  try {
    const d = await api("GET", "/ui/api/settings");
    activities = d.activities || [];
    setAvatar(!!d.has_avatar);
    setOwner(d.user);
    fillActivitySelect();
    renderActivitySettings();
  } catch (_) {}
  showView("today");
}).catch((err) => toast(err.message));

function groupKey(el) {
  return "kcrm-nav-" + (el.dataset.group || "");
}
function restoreGroups() {
  document.querySelectorAll(".nav-group").forEach((d) => {
    try {
      const v = localStorage.getItem(groupKey(d));
      if (v === "0") d.open = false;
      if (v === "1") d.open = true;
    } catch (_) {}
  });
}
function persistGroups() {
  document.querySelectorAll(".nav-group").forEach((d) => {
    try { localStorage.setItem(groupKey(d), d.open ? "1" : "0"); } catch (_) {}
  });
}
function setRail(on) {
  document.documentElement.dataset.rail = on ? "1" : "0";
  try { localStorage.setItem("kcrm-rail", on ? "1" : "0"); } catch (_) {}
  document.querySelectorAll("[data-rail-toggle]").forEach((btn) => {
    btn.setAttribute("aria-expanded", String(!on));
    btn.setAttribute("aria-label", on ? "Ouvrir le menu" : "Réduire le menu");
    btn.setAttribute("title", on ? "Ouvrir le menu" : "Réduire le menu");
    const use = btn.querySelector("use");
    if (use) use.setAttribute("href", on ? "#PanelLeft" : "#PanelLeftClose");
    const label = btn.querySelector(".nav-label");
    if (label) label.textContent = on ? "Ouvrir" : "Réduire";
  });
  if (on) document.querySelectorAll(".nav-group").forEach((d) => { d.open = true; });
  else restoreGroups();
}
function initRail() {
  let on = false;
  try { on = localStorage.getItem("kcrm-rail") === "1"; } catch (_) {}
  if (!on) restoreGroups();
  setRail(on);
  document.querySelectorAll("[data-rail-toggle]").forEach((btn) => {
    btn.addEventListener("click", (event) => {
      event.preventDefault();
      event.stopPropagation();
      const next = document.documentElement.dataset.rail !== "1";
      if (document.documentElement.dataset.rail !== "1") persistGroups();
      setRail(next);
    });
  });
  document.querySelectorAll(".nav-group").forEach((d) => {
    d.addEventListener("toggle", () => {
      if (document.documentElement.dataset.rail !== "1") persistGroups();
    });
  });
}
initRail();
