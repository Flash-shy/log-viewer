import { fetchJSON, contentHasIssue } from "./common.js";

const $ = (id) => document.getElementById(id);

function formatSize(n) {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / (1024 * 1024)).toFixed(1)} MB`;
}

async function detectIssue(name) {
  const q = new URLSearchParams({ name, tail: "200" });
  try {
    const data = await fetchJSON(`/api/logs/content?${q}`);
    return contentHasIssue(data.lines || []);
  } catch {
    return false;
  }
}

function setStatus(msg, isError) {
  const el = $("status");
  el.textContent = msg;
  el.classList.toggle("error", !!isError);
}

function renderList(files, issueMap) {
  const ul = $("log-list");
  ul.innerHTML = "";
  if (files.length === 0) {
    const li = document.createElement("li");
    li.className = "log-list__empty";
    li.textContent = "目录下没有日志文件";
    ul.append(li);
    return;
  }
  for (const f of files) {
    const bad = issueMap.get(f.name);
    const li = document.createElement("li");
    li.className = "log-item" + (bad ? " log-item--issue" : "");

    const a = document.createElement("a");
    a.className = "log-item__link";
    // Hash survives more reliably than ?query for file:// and some static servers; query still supported in view.js
    a.href = `view.html#name=${encodeURIComponent(f.name)}`;
    a.addEventListener("click", () => {
      try {
        sessionStorage.setItem("log-viewer-file", f.name);
      } catch {
        /* ignore */
      }
    });

    const name = document.createElement("span");
    name.className = "log-item__name";
    name.textContent = f.name;
    if (bad) name.title = "末尾约 200 行内检测到错误或告警类内容";

    const meta = document.createElement("span");
    meta.className = "log-item__meta";
    meta.textContent = formatSize(f.size);

    if (bad) {
      const mark = document.createElement("span");
      mark.className = "log-item__mark";
      mark.setAttribute("aria-hidden", "true");
      mark.textContent = "🚨";
      a.append(mark, name, meta);
    } else {
      a.append(name, meta);
    }
    li.append(a);
    ul.append(li);
  }
}

async function init() {
  setStatus("加载列表…", false);
  try {
    const data = await fetchJSON("/api/logs");
    const files = data.files || [];
    const issueMap = new Map();

    if (files.length > 0) {
      setStatus("正在分析日志末尾是否有异常…", false);
      const results = await Promise.all(
        files.map(async (f) => {
          const bad = await detectIssue(f.name);
          return [f.name, bad];
        })
      );
      for (const [name, bad] of results) issueMap.set(name, bad);
    }

    renderList(files, issueMap);
    setStatus(
      files.length === 0
        ? ""
        : `共 ${files.length} 个文件 · 标记 🚨 表示末尾约 200 行内疑似存在问题`,
      false
    );
  } catch (e) {
    setStatus(e.message || String(e), true);
    $("log-list").innerHTML = "";
  }
}

init();
