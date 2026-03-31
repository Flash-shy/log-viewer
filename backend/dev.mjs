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

const apiKey = (process.env.LOG_VIEWER_API_KEY || "").trim();
const defaultOrigins = ["http://127.0.0.1:5173", "http://localhost:5173"];
const corsOrigins = parseCorsOrigins(process.env.LOG_VIEWER_CORS_ORIGINS);

function parseCorsOrigins(raw) {
  if (!raw || !String(raw).trim()) return defaultOrigins;
  const s = String(raw).trim();
  if (s === "*") return ["*"];
  return s
    .split(",")
    .map((x) => x.trim())
    .filter(Boolean);
}

function cors(req, res) {
  const o = req.headers.origin;
  if (corsOrigins.length === 1 && corsOrigins[0] === "*") {
    res.setHeader("Access-Control-Allow-Origin", "*");
  } else if (o && corsOrigins.includes(o)) {
    res.setHeader("Access-Control-Allow-Origin", o);
  }
  res.setHeader("Access-Control-Allow-Methods", "GET, OPTIONS");
  res.setHeader(
    "Access-Control-Allow-Headers",
    "Content-Type, Authorization, X-API-Key"
  );
}

function unauthorized(req) {
  if (!apiKey) return false;
  const auth = req.headers.authorization;
  const bearer =
    auth && auth.startsWith("Bearer ")
      ? auth.slice("Bearer ".length).trim()
      : "";
  if (bearer === apiKey) return false;
  if (req.headers["x-api-key"] === apiKey) return false;
  return true;
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
    const st = await fs.lstat(path.join(logsDir, e.name));
    if (st.isSymbolicLink()) continue;
    if (!st.isFile()) continue;
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
  const lst = await fs.lstat(p);
  if (lst.isSymbolicLink()) throw new Error("invalid");
  if (!lst.isFile()) throw new Error("notfound");
  if (lst.size > 10 << 20) throw new Error("toolarge");
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
  let slice = lines.slice(start);
  let lineStart = start;
  let truncated = false;
  const maxReturn = 5000;
  if (slice.length > maxReturn) {
    slice = slice.slice(-maxReturn);
    lineStart = total - slice.length;
    truncated = true;
  }
  const out = slice.map((text, i) => ({ no: lineStart + i + 1, text }));
  return { file: name, totalLines: total, lines: out, truncated };
}

const server = http.createServer(async (req, res) => {
  cors(req, res);
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
    const publicPaths = new Set(["/api/health", "/openapi.yaml", "/api/docs"]);
    if (unauthorized(req) && !publicPaths.has(u.pathname) && req.method !== "OPTIONS") {
      res.statusCode = 401;
      res.setHeader("Content-Type", "text/plain; charset=utf-8");
      res.end("unauthorized");
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
      const rawTail = u.searchParams.get("tail");
      let tail = 400;
      if (rawTail !== null && rawTail !== "") {
        const n = Number(rawTail);
        if (!Number.isFinite(n) || n < 1) {
          res.statusCode = 400;
          res.end("bad request");
          return;
        }
        if (n > 500_000) {
          res.statusCode = 400;
          res.end("tail too large");
          return;
        }
        tail = Math.floor(n);
      }
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
        if (e.message === "toolarge") {
          res.statusCode = 413;
          res.end("payload too large");
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
    res.end("internal server error");
  }
});

server.listen(port, "127.0.0.1", () => {
  console.log(`dev.mjs logs: ${logsDir}`);
  console.log(`listening on http://127.0.0.1:${port}`);
});
