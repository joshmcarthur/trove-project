import { h, render } from "./preact.module.js";
import htm from "./htm.module.js";

const html = htm.bind(h);
const VERIFIER_KEY = "trove.wall.pkce";
const ITEMS_URL = "/wall/api/items";
const BLOBS_URL = "/wall/api/blobs/";
const config = {
  clientId: document.body.dataset.clientId,
  authorizeUrl: document.body.dataset.authorizeUrl,
  tokenUrl: document.body.dataset.tokenUrl,
  redirectUri: new URL(".", window.location.href).href,
};

let accessToken = null;
let refreshToken = null;
let items = [];
let selectedId = null;
let status = "Signing in…";
let error = "";
const blobUrls = new Map();

function b64url(bytes) {
  let binary = "";
  bytes.forEach((byte) => {
    binary += String.fromCharCode(byte);
  });
  return btoa(binary).replaceAll("+", "-").replaceAll("/", "_").replace(/=+$/u, "");
}

function randomVerifier() {
  return b64url(crypto.getRandomValues(new Uint8Array(32)));
}

async function challengeS256(verifier) {
  const digest = await crypto.subtle.digest(
    "SHA-256",
    new TextEncoder().encode(verifier),
  );
  return b64url(new Uint8Array(digest));
}

function paint() {
  render(html`<${App} />`, document.getElementById("wall"));
}

function visibleValues(values) {
  return Object.entries(values).filter(([key]) => !key.startsWith("$") && key !== "title");
}

function displayValue(value) {
  if (value == null) {
    return "";
  }
  if (typeof value === "string" || typeof value === "number") {
    return String(value);
  }
  return JSON.stringify(value);
}

function formatWhen(iso) {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) {
    return iso;
  }
  return date.toLocaleString(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  });
}

function provenanceNote(row) {
  const parts = [];
  if (row.model) {
    parts.push(row.model);
  }
  if (typeof row.prompt_hash === "string" && row.prompt_hash) {
    parts.push(row.prompt_hash.slice(0, 12));
  }
  if (row.cost != null) {
    parts.push(`cost ${row.cost}`);
  }
  return parts.join(" · ");
}

function selectedItem() {
  return items.find((item) => item.id === selectedId) || items[0] || null;
}

function photoSha(item) {
  if (item.type === "capture.image") {
    const sha = item.values.$blob;
    return typeof sha === "string" && sha ? sha : null;
  }
  if (item.type === "capture.link") {
    const preview = (item.links || []).find((link) => {
      const sha = link.values && link.values.$blob;
      return (
        link.rel === "derived_from" &&
        link.type === "capture.image" &&
        typeof sha === "string" &&
        sha
      );
    });
    return preview ? preview.values.$blob : null;
  }
  return null;
}

function derivedRows(item) {
  return (item.links || []).filter(
    (link) => link.rel === "derived_from" && link.type !== "colour",
  );
}

async function authorizedGet(url, retried = false) {
  const response = await fetch(url, {
    headers: { Authorization: `Bearer ${accessToken}` },
    credentials: "omit",
  });
  if (response.status === 401 && !retried && (await refreshAccess())) {
    return authorizedGet(url, true);
  }
  return response;
}

async function startLogin() {
  const verifier = randomVerifier();
  sessionStorage.setItem(VERIFIER_KEY, verifier);
  const challenge = await challengeS256(verifier);
  const url = new URL(config.authorizeUrl, window.location.origin);
  url.searchParams.set("response_type", "code");
  url.searchParams.set("client_id", config.clientId);
  url.searchParams.set("redirect_uri", config.redirectUri);
  url.searchParams.set("scope", "read:all");
  url.searchParams.set("code_challenge", challenge);
  url.searchParams.set("code_challenge_method", "S256");
  window.location.assign(url.toString());
}

async function exchangeCode(code) {
  const verifier = sessionStorage.getItem(VERIFIER_KEY);
  sessionStorage.removeItem(VERIFIER_KEY);
  const body = new URLSearchParams({
    grant_type: "authorization_code",
    code,
    redirect_uri: config.redirectUri,
    client_id: config.clientId,
    code_verifier: verifier || "",
  });
  const response = await fetch(config.tokenUrl, {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    credentials: "omit",
    body,
  });
  if (!response.ok) {
    throw new Error("Sign-in failed.");
  }
  const payload = await response.json();
  accessToken = payload.access_token;
  refreshToken = payload.refresh_token || null;
  history.replaceState({}, "", config.redirectUri);
}

async function refreshAccess() {
  if (!refreshToken) {
    return false;
  }
  const body = new URLSearchParams({
    grant_type: "refresh_token",
    refresh_token: refreshToken,
    client_id: config.clientId,
  });
  const response = await fetch(config.tokenUrl, {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    credentials: "omit",
    body,
  });
  if (!response.ok) {
    accessToken = null;
    refreshToken = null;
    return false;
  }
  const payload = await response.json();
  accessToken = payload.access_token;
  if (payload.refresh_token) {
    refreshToken = payload.refresh_token;
  }
  return true;
}

async function loadItems() {
  const response = await authorizedGet(ITEMS_URL);
  if (!response.ok) {
    throw new Error("Could not load these items.");
  }
  const payload = await response.json();
  items = payload.items || [];
  if (!items.some((item) => item.id === selectedId)) {
    selectedId = items[0] ? items[0].id : null;
  }
  status = items.length ? `${items.length} on the wall` : "Nothing on the wall yet.";
}

async function loadBlobUrls() {
  const shas = [
    ...new Set(items.map(photoSha).filter((sha) => sha && !blobUrls.has(sha))),
  ];
  await Promise.all(
    shas.map(async (sha) => {
      const response = await authorizedGet(BLOBS_URL + encodeURIComponent(sha));
      if (!response.ok) {
        blobUrls.set(sha, null);
        return;
      }
      blobUrls.set(sha, URL.createObjectURL(await response.blob()));
    }),
  );
}

function Chip({ hex }) {
  return html`<span class="chip" style=${{ background: hex }} title=${hex}></span>`;
}

function DerivedList({ rows }) {
  if (!rows.length) {
    return null;
  }
  return html`
    <div class="derived">
      <h3>Derived</h3>
      ${rows.map(
        (link) => html`
          <div key=${link.id} class="derived-item">
            <p class="type">${link.type}</p>
            <dl class="kv">
              ${visibleValues(link.values).map(
                ([key, value]) => html`
                  <div key=${key}>
                    <dt>${key}</dt>
                    <dd>${displayValue(value)}</dd>
                  </div>
                `,
              )}
            </dl>
          </div>
        `,
      )}
    </div>
  `;
}

function Tile({ item, title }) {
  const sha = photoSha(item);
  const url = sha ? blobUrls.get(sha) : null;
  if (typeof url === "string") {
    return html`<img class="tile" src=${url} alt=${title} />`;
  }
  return html`<div class="tile tile-empty" aria-hidden="true"></div>`;
}

function ItemCard({ item, current }) {
  const title = item.values.title || item.type;
  const extras = visibleValues(item.values).slice(0, 4);
  const derived = current ? derivedRows(item) : [];
  return html`
    <button
      type="button"
      class="card"
      aria-current=${current ? "true" : undefined}
      onClick=${() => {
        selectedId = item.id;
        paint();
      }}
    >
      <${Tile} item=${item} title=${title} />
      <div class="card-copy">
        <p class="type">${item.type}</p>
        <h2 class="title">${title}</h2>
        <dl class="kv">
          ${extras.map(
            ([key, value]) => html`
              <div key=${key}>
                <dt>${key}</dt>
                <dd>${displayValue(value)}</dd>
              </div>
            `,
          )}
        </dl>
        ${item.colours.length
          ? html`<div class="chips">${item.colours.map((hex) => html`<${Chip} hex=${hex} />`)}</div>`
          : null}
        <${DerivedList} rows=${derived} />
      </div>
    </button>
  `;
}

function Provenance({ item }) {
  if (!item) {
    return html`<aside class="panel"><p class="empty">Select an item to see who set it.</p></aside>`;
  }
  const rows = item.provenance || [];
  return html`
    <aside class="panel">
      <h2>Provenance</h2>
      ${rows.length
        ? html`
            <ol class="timeline">
              ${rows.map((row) => {
                const note = provenanceNote(row);
                return html`
                  <li key=${row.kind + row.agent + row.when}>
                    <span class="when">${formatWhen(row.when)}</span>
                    <div>
                      <div class="kind">${row.kind}</div>
                      <div class="agent">${row.agent}</div>
                      ${note ? html`<div class="agent">${note}</div>` : null}
                    </div>
                  </li>
                `;
              })}
            </ol>
          `
        : html`<p class="empty">No history for this item.</p>`}
    </aside>
  `;
}

function App() {
  const current = selectedItem();
  return html`
    <header class="masthead">
      <h1>Trove</h1>
      <p class="status">${error || status}</p>
    </header>
    ${error
      ? html`<p class="error">${error}</p>`
      : html`
          <div class="layout">
            <section class="grid" aria-label="Items">
              ${items.map(
                (item) =>
                  html`<${ItemCard} item=${item} current=${current && current.id === item.id} />`,
              )}
            </section>
            <${Provenance} item=${current} />
          </div>
        `}
  `;
}

async function boot() {
  paint();
  try {
    const params = new URLSearchParams(window.location.search);
    const code = params.get("code");
    if (code) {
      status = "Finishing sign-in…";
      paint();
      await exchangeCode(code);
    } else if (!accessToken) {
      await startLogin();
      return;
    }
    await loadItems();
    paint();
    await loadBlobUrls();
  } catch (err) {
    error = err.message || "Could not open the wall.";
    status = "";
  }
  paint();
}

boot();
