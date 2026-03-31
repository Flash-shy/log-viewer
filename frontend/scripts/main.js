const apiBase = () => {
  const m = document.querySelector('meta[name="api-base"]');
  return (m && m.getAttribute("content")) || "http://127.0.0.1:8080";
};

const $ = (id) => document.getElementById(id);

async function fetchJSON(path) {
  const res = await fetch(`${apiBase()}${path}`);
  if (!res.ok) {
    const t = await res.text();
    throw new Error(t || res.statusText);
  }
  return res.json();
}

function renderLines(data) {
  const pre = $("log-view");
  const lines = data.lines || [];
  if (lines.length === 0) {
    pre.textContent = data.totalLines === 0 ? "(空文件)" : "(无行返回)";
    return;
  }
  pre.replaceChildren();
  const frag = document.createDocumentFragment();
  for (const row of lines) {
    const span = document.createElement("span");
    span.className = "log-line";
    const no = document.createElement("span");
    no.className = "line-no";
    no.textContent = String(row.no);
    span.append(no);
    span.append(document.createTextNode(row.text));
    frag.append(span);
    frag.append(document.createTextNode("\n"));
  }
  pre.append(frag);
  const tail = $("tail-input").valueAsNumber || 400;
  let msg = `共 ${data.totalLines} 行`;
  if (data.truncated) {
    msg += ` · 仅显示其中一段（末尾 ${tail} 行或分页上限）`;
  }
  $("status").textContent = msg;
  $("status").classList.remove("error");
}

function setError(err) {
  $("status").textContent = err.message || String(err);
  $("status").classList.add("error");
  $("log-view").textContent = "";
}

async function loadFileList() {
  const data = await fetchJSON("/api/logs");
  const sel = $("file-select");
  sel.innerHTML = "";
  const files = data.files || [];
  if (files.length === 0) {
    const opt = document.createElement("option");
    opt.value = "";
    opt.textContent = "(无文件)";
    sel.append(opt);
    return;
  }
  for (const f of files) {
    const opt = document.createElement("option");
    opt.value = f.name;
    opt.textContent = `${f.name} (${f.size} B)`;
    sel.append(opt);
  }
}

async function loadContent() {
  const name = $("file-select").value;
  if (!name) {
    setError(new Error("没有可显示的日志文件"));
    return;
  }
  const tail = Math.min(5000, Math.max(1, $("tail-input").valueAsNumber || 400));
  $("status").textContent = "加载中…";
  try {
    const q = new URLSearchParams({ name, tail: String(tail) });
    const data = await fetchJSON(`/api/logs/content?${q}`);
    renderLines(data);
  } catch (e) {
    setError(e);
  }
}

async function init() {
  try {
    await loadFileList();
    await loadContent();
  } catch (e) {
    setError(e);
  }

  $("file-select").addEventListener("change", () => loadContent());
  $("refresh-btn").addEventListener("click", () => loadContent());
}

init();
