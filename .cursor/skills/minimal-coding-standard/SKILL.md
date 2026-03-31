---
name: minimal-coding-standard
description: >-
  Applies a minimal structural coding standard: method length ≤50 lines, file/class
  ≤500 lines, branching limits by language family, and no magic literals in method
  bodies. Use when writing or reviewing code, refactoring, or when the user asks
  for coding standards, style constraints, or “极简编码规范”.
---

# Minimal coding standard

When implementing or reviewing code, enforce the following four rules unless the user overrides them.

## 1. Method length

- No method longer than **50 lines** (from opening `{` / `:` to closing, including blanks inside the method).
- If a method grows past the limit, extract helpers with clear names.

## 2. File and class length

- No source file or single class longer than **500 lines**.
- Split by responsibility (types, modules, partial classes where the language allows) before hitting the limit.

## 3. Branching

**TypeScript, JavaScript, Python**

- Use at most **one nesting level** of **branch** statements (`if` / `else` / `switch` / `match` / ternary). Do not place one branch inside another for business logic—flatten with early returns, guard clauses, or extracted functions. (Loop nesting is not covered by this rule.)

**Java, C#, C++**

- Avoid **switch / case** and **if–else if–else** chains with **three or more** distinct branches for dispatch logic.
- Prefer lookup tables, `Map` / dictionary dispatch, or small design patterns (e.g. strategy, command) instead.

## 4. Magic literals

- **No meaningful** numeric, character, or string literals in **method bodies**.
- Define **named constants**, `enum` members, or `const` / `readonly` / `static final` values at module, type, or configuration level and reference those names.
- **Exempt**: literals only used in **logging**, **assert messages**, or **user-facing documentation strings** inside the method when they are purely explanatory and not business constants.

## Agent behavior

- When editing: satisfy these rules in new code; if existing code violates them, fix only when in scope of the user’s task or when asked to refactor.
- When reviewing: report violations briefly by rule number (1–4) and suggest the smallest change that fixes them.
