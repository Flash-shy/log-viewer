#!/usr/bin/env node
/**
 * Optional dev server when Go is unavailable. Same routes as cmd/server (subset).
 * Usage: LOG_VIEWER_LOG_DIR=/path/to/logs node dev.mjs
 */
import http from "node:http";
import fs from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const port = Number(process.env.PORT) || 8080;
const logsDir =
  process.env.LOG_VIEWER_LOG_DIR ||
  path.resolve(__dirname, "../logs");

function cors(res) {
  res.setHeader("Access-Control-Allow-Origin", "*");
  res.setHeader("Access-Control-Allow-Methods", "GET, OPTIONS");
}

function safeName(name) {
  if (!name || name !== path.basename(name)) return false;
  return /^[a-zA-Z0-9._-]+$/.test(name);
}

async function listFiles() {
  const ents = await fs.readdir(logsDir, { withFileTypes: true });
  const files = [];
  for (const e of ents) {
    if (e.isDirectory()) continue;
    if (!safeName(e.name)) continue;
    const st = await fs.stat(path.join(logsDir, e.name));
    files.push({ name: e.name, size: st.size });
  }
  files.sort((a, b) => a.name.localeCompare(b.name));
  return { files };
}

async function readContent(name, tail) {
  if (!safeName(name)) throw new Error("invalid");
  const p = path.join(logsDir, name);
  const rel = path.relative(path.resolve(logsDir), path.resolve(p));
  if (rel.startsWith("..") || path.isAbsolute(rel)) throw new Error("invalid");
  let buf;
  try {
    buf = await fs.readFile(p, "utf8");
  } catch (e) {
    if (e && e.code === "ENOENT") throw new Error("notfound");
    throw e;
  }
  let lines = buf.split(/\r?\n/);
  if (lines.length && lines[lines.length - 1] === "") lines.pop();
  const total = lines.length;
  const t = Math.min(tail, total);
  const start = total - t;
  const slice = lines.slice(start);
  const out = slice.map((text, i) => ({ no: start + i + 1, text }));
  return { file: name, totalLines: total, lines: out, truncated: false };
}

const server = http.createServer(async (req, res) => {
  cors(res);
  if (req.method === "OPTIONS") {
    res.writeHead(204);
    res.end();
    return;
  }
  const u = new URL(req.url || "/", `http://127.0.0.1`);
  try {
    if (u.pathname === "/api/health" && req.method === "GET") {
      res.setHeader("Content-Type", "application/json");
      res.end(JSON.stringify({ ok: true }));
      return;
    }
    if (u.pathname === "/api/logs" && req.method === "GET") {
      const data = await listFiles();
      res.setHeader("Content-Type", "application/json");
      res.end(JSON.stringify(data));
      return;
    }
    if (u.pathname === "/api/logs/content" && req.method === "GET") {
      const name = u.searchParams.get("name") || "";
      const tail = Math.min(5000, Math.max(1, Number(u.searchParams.get("tail")) || 400));
      try {
        const data = await readContent(name, tail);
        res.setHeader("Content-Type", "application/json");
        res.end(JSON.stringify(data));
      } catch (e) {
        if (e.message === "notfound") {
          res.statusCode = 404;
          res.end("not found");
          return;
        }
        if (e.message === "invalid") {
          res.statusCode = 400;
          res.end("invalid name");
          return;
        }
        throw e;
      }
      return;
    }
    if (u.pathname === "/openapi.yaml" && req.method === "GET") {
      const specPath = path.join(__dirname, "cmd/server/openapi/openapi.yaml");
      const body = await fs.readFile(specPath, "utf8");
      res.setHeader("Content-Type", "application/yaml; charset=utf-8");
      res.end(body);
      return;
    }
    if (u.pathname === "/api/docs" && req.method === "GET") {
      const docPath = path.join(__dirname, "cmd/server/openapi/docs.html");
      const body = await fs.readFile(docPath, "utf8");
      res.setHeader("Content-Type", "text/html; charset=utf-8");
      res.end(body);
      return;
    }
    res.statusCode = 404;
    res.end();
  } catch (e) {
    res.statusCode = e.message === "invalid" ? 400 : 500;
    res.setHeader("Content-Type", "text/plain; charset=utf-8");
    res.end(String(e));
  }
});

server.listen(port, "127.0.0.1", () => {
  console.log(`dev.mjs logs: ${logsDir}`);
  console.log(`listening on http://127.0.0.1:${port}`);
});
