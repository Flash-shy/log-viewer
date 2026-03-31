const apiBase = () => {
  const m = document.querySelector('meta[name="api-base"]');
  return (m && m.getAttribute("content")) || "http://127.0.0.1:8080";
};

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
