export const THEME_STORAGE_KEY = "log-viewer-theme";

export const THEMES = Object.freeze(["dark", "light", "blue"]);

const apiBase = () => {
  const m = document.querySelector('meta[name="api-base"]');
  return (m && m.getAttribute("content")) || "http://127.0.0.1:8080";
};

export function getTheme() {
  try {
    const t = localStorage.getItem(THEME_STORAGE_KEY);
    if (THEMES.includes(t)) return t;
  } catch {
    /* ignore */
  }
  return "dark";
}

export function applyTheme(theme) {
  const t = THEMES.includes(theme) ? theme : "dark";
  document.documentElement.setAttribute("data-theme", t);
  document.documentElement.style.colorScheme = t === "light" ? "light" : "dark";
}

export function setTheme(theme) {
  if (!THEMES.includes(theme)) return;
  try {
    localStorage.setItem(THEME_STORAGE_KEY, theme);
  } catch {
    /* ignore */
  }
  applyTheme(theme);
}

export function setupTheme() {
  applyTheme(getTheme());
  const root = document.querySelector(".theme-switch");
  if (!root) return;

  const buttons = root.querySelectorAll("[data-theme-pick]");

  function syncPressed() {
    const current = getTheme();
    for (const btn of buttons) {
      const pick = btn.getAttribute("data-theme-pick");
      const on = pick === current;
      btn.setAttribute("aria-pressed", on ? "true" : "false");
      btn.classList.toggle("is-active", on);
    }
  }

  for (const btn of buttons) {
    btn.addEventListener("click", () => {
      const pick = btn.getAttribute("data-theme-pick");
      if (THEMES.includes(pick)) {
        setTheme(pick);
        syncPressed();
      }
    });
  }
  syncPressed();
}

export async function fetchJSON(path) {
  const res = await fetch(`${apiBase()}${path}`);
  if (!res.ok) {
    const t = await res.text();
    throw new Error(t || res.statusText);
  }
  return res.json();
}

export { apiBase };

/** Heuristic: line looks like an error / failure in common log formats (EN + CN). */
export function lineHasIssue(text) {
  if (!text || !text.trim()) return false;
  return (
    /\b(ERROR|FATAL|CRITICAL|PANIC|Exception|Traceback|FAIL(?:URE)?|segfault|SIG[A-Z]+)\b/i.test(
      text
    ) ||
    /(错误|异常|失败|崩溃|致命)/.test(text) ||
    /\bWARN(ING)?\b.*\b(fail|error|invalid|denied|refused)\b/i.test(text)
  );
}

export function contentHasIssue(lines) {
  if (!lines || !lines.length) return false;
  return lines.some((row) => lineHasIssue(row.text));
}
