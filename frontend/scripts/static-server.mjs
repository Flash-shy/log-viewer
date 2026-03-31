#!/usr/bin/env node
/**
 * Minimal static file server (no npx/serve). Serves the frontend directory with
 * correct MIME types for ES modules.
 */
import http from "node:http";
import fs from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(__dirname, "..");
const port = Number(process.env.PORT) || 5173;
const host = process.env.HOST || "127.0.0.1";

const mime = {
  ".html": "text/html; charset=utf-8",
  ".js": "text/javascript; charset=utf-8",
  ".mjs": "text/javascript; charset=utf-8",
  ".css": "text/css; charset=utf-8",
  ".json": "application/json",
  ".svg": "image/svg+xml",
  ".ico": "image/x-icon",
  ".png": "image/png",
  ".webp": "image/webp",
};

function safePath(urlPathname) {
  const decoded = decodeURIComponent(urlPathname.split("?")[0] || "/");
  const noLead = decoded.replace(/^\/+/, "") || ".";
  const rel =
    decoded === "/" || noLead === ""
      ? "index.html"
      : path.normalize(noLead).replace(/^(\.\.(\/|\\|$))+/, "");
  if (rel.startsWith("..") || path.isAbsolute(rel)) {
    return null;
  }
  const abs = path.resolve(root, rel);
  const rootResolved = path.resolve(root);
  if (!abs.startsWith(rootResolved + path.sep) && abs !== rootResolved) {
    return null;
  }
  return abs;
}

/** If path has no extension and the file is missing, try path + ".html" (same as serve cleanUrls). */
async function resolveExistingFile(abs) {
  let st = await fs.stat(abs).catch(() => null);
  if (st?.isFile()) {
    return { filePath: abs, st };
  }
  if (!path.extname(abs)) {
    const html = `${abs}.html`;
    st = await fs.stat(html).catch(() => null);
    if (st?.isFile()) {
      return { filePath: html, st };
    }
  }
  return null;
}

const server = http.createServer(async (req, res) => {
  try {
    const url = new URL(req.url || "/", `http://${host}`);
    const abs = safePath(url.pathname);
    if (!abs) {
      res.writeHead(403);
      res.end("forbidden");
      return;
    }
    const resolved = await resolveExistingFile(abs);
    if (!resolved) {
      res.writeHead(404);
      res.end("not found");
      return;
    }
    const { filePath } = resolved;
    const data = await fs.readFile(filePath);
    const ext = path.extname(filePath).toLowerCase();
    res.setHeader("Content-Type", mime[ext] || "application/octet-stream");
    res.writeHead(200);
    res.end(data);
  } catch (e) {
    res.writeHead(500);
    res.setHeader("Content-Type", "text/plain; charset=utf-8");
    res.end(String(e));
  }
});

server.listen(port, host, () => {
  console.error(`Log Viewer frontend: http://${host}:${port}/`);
});
