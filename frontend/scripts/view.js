import { fetchJSON, lineHasIssue } from "./common.js";

const $ = (id) => document.getElementById(id);

/** Resolve log file name from ?name=, #name=, or sessionStorage (set when clicking from index). */
function resolveFileName() {
  const q = new URLSearchParams(window.location.search);
  let name = q.get("name");
  if (name) return name.trim();

  const rawHash = window.location.hash.replace(/^#/, "");
  if (rawHash) {
    const hq = new URLSearchParams(rawHash);
    name = hq.get("name");
    if (name) return name.trim();
  }

  try {
    const s = sessionStorage.getItem("log-viewer-file");
    if (s) return s.trim();
  } catch {
    /* ignore */
  }
  return "";
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
    if (lineHasIssue(row.text)) span.classList.add("log-line--issue");
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

async function loadContent() {
  const name = resolveFileName();
  if (!name) {
    setError(new Error("缺少文件参数：请从首页选择日志"));
    return;
  }
  $("file-title").textContent = name;
  document.title = `${name} · Log Viewer`;
  const tail = Math.min(5000, Math.max(1, $("tail-input").valueAsNumber || 400));
  $("status").textContent = "加载中…";
  try {
    const q = new URLSearchParams({ name, tail: String(tail) });
    const data = await fetchJSON(`/api/logs/content?${q}`);
    renderLines(data);
    try {
      sessionStorage.setItem("log-viewer-file", name);
    } catch {
      /* ignore */
    }
  } catch (e) {
    setError(e);
  }
}

function init() {
  const name = resolveFileName();
  const back = $("back-link");
  back.href = "index.html";

  if (!name) {
    setError(new Error("缺少文件参数：请从首页选择日志"));
    return;
  }

  $("tail-input").addEventListener("change", () => loadContent());
  $("refresh-btn").addEventListener("click", () => loadContent());
  loadContent();
}

init();
