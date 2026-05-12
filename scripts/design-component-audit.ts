#!/usr/bin/env bun
/**
 * Component-dedup + translation audit for Claude Design exports.
 *
 * Step F3 of `Plans/integrate-claude-design.md`. Scans the staged design
 * source at `apps/web/src/_design-inbox/<route>/project/` and emits an
 * `AUDIT.md` next to it. The integrator reads the report before promoting
 * the design to `apps/web/src/pages/v2/<Route>.tsx`.
 *
 * Eight rule categories, derived from the patterns I caught manually across
 * the eight initial route ports:
 *
 *   1. tailwind-dynamic     — `${tone}` template strings in className.
 *                             Tailwind's JIT can't see these; must lift to
 *                             a static TONE classmap.
 *   2. ui-substitution      — local `function RiskBadge` (etc.) that
 *                             duplicates a @irongolem/ui export.
 *   3. preview-shim         — `(window as ...).<Route> = <Route>` at bottom
 *                             of file; drop in production.
 *   4. react-namespace      — `import * as React from "react"`; convert to
 *                             named imports per project style.
 *   5. jsx-element-return   — `: JSX.Element` return type; TS 6 deprecates
 *                             the global JSX namespace.
 *   6. bg-app               — `bg-app` class doesn't exist in the bridge;
 *                             swap for `bg-neutral-50`.
 *   7. raw-hex-tailwind     — `bg-[#abc123]` arbitrary-color classes; prefer
 *                             semantic tokens.
 *   8. inline-keyframes     — `<style>@keyframes</style>` blocks; globals.css
 *                             already ships ig-pulse / ig-toast-in / ig-slide-in.
 *
 * Plus one informational category:
 *
 *   - candidate-ui          — top-level `function <PascalCase>()` definitions
 *                             that DON'T match existing @irongolem/ui exports.
 *                             Flagged as possible graduation targets.
 *
 * Usage:
 *   bun run scripts/design-component-audit.ts <route>
 *
 * <route> is a directory name under apps/web/src/_design-inbox/, e.g.
 * `settings`, `dashboard`, `inbox`. The matching design source file is
 * auto-detected by basename match against the route slug (`Settings.tsx`,
 * `workspace-dashboard.jsx`, etc.).
 */

import { readdir, readFile, stat, writeFile } from "node:fs/promises";
import { basename, dirname, join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const ROOT = resolve(__dirname, "..");
const INBOX = join(ROOT, "apps/web/src/_design-inbox");

const UI_COMPONENT_NAMES = [
  "HeartbeatStatus",
  "PolicyCard",
  "ResearchCard",
  "RiskBadge",
  "SafetyCard",
  "Timeline",
] as const;

type Severity = "blocker" | "should-fix" | "info";
type Rule =
  | "tailwind-dynamic"
  | "ui-substitution"
  | "preview-shim"
  | "react-namespace"
  | "jsx-element-return"
  | "bg-app"
  | "raw-hex-tailwind"
  | "inline-keyframes"
  | "candidate-ui";

interface Finding {
  readonly rule: Rule;
  readonly severity: Severity;
  readonly file: string;        // relative to INBOX
  readonly line: number;
  readonly excerpt: string;     // ≤ 120 chars
  readonly recommendation: string;
  readonly extra?: string;
}

const SEVERITY_ORDER: Record<Severity, number> = { blocker: 0, "should-fix": 1, info: 2 };

const RULE_META: Record<Rule, { title: string; description: string }> = {
  "tailwind-dynamic": {
    title: "Dynamic Tailwind tone interpolation",
    description:
      "Template-string class names with `${tone}` are invisible to Tailwind's JIT compiler. The resulting class won't ship in the CSS bundle and the element will render unstyled. Lift the tone lookup into a static `TONE` classmap (see `pages/v2/Home.tsx` for the canonical pattern).",
  },
  "ui-substitution": {
    title: "Local reimplementation of an @irongolem/ui component",
    description:
      "This design file defines a function with the same name as a component already exported from `@irongolem/ui`. Replace the local definition with `import { X } from \"@irongolem/ui\"` unless the local divergence is intentional.",
  },
  "preview-shim": {
    title: "Browser-preview shim",
    description:
      "Bottom-of-file `(window as ...).<Route> = <Route>` exists only so Claude Design's HTML preview can mount the route. Production builds should drop this entirely.",
  },
  "react-namespace": {
    title: "Namespace React import",
    description:
      "Project style is `import React, { useState } from \"react\"`. The namespace form makes named hooks awkward and is inconsistent with the rest of v2.",
  },
  "jsx-element-return": {
    title: "`JSX.Element` return-type annotation",
    description:
      "TypeScript 6 deprecates the global `JSX` namespace. Remove the annotation entirely — TS infers the right return type from the JSX.",
  },
  "bg-app": {
    title: "`bg-app` Tailwind class",
    description:
      "The design's `bg-app` CSS variable isn't wired in this project. Use `bg-neutral-50` instead.",
  },
  "raw-hex-tailwind": {
    title: "Arbitrary-color Tailwind class",
    description:
      "Hex values in `bg-[#...]` / `text-[#...]` classes bypass the design tokens. Prefer the semantic palette (`bg-safe`, `text-warning`, etc.) so dark-mode / theme swaps work later.",
  },
  "inline-keyframes": {
    title: "Inline `<style>` keyframes",
    description:
      "`globals.css` already ships `ig-pulse`, `ig-toast-in`, `ig-slide-in`. Reuse the existing keyframes rather than re-declaring them inline.",
  },
  "candidate-ui": {
    title: "Possible @irongolem/ui graduation candidate",
    description:
      "Top-level component functions that don't match an existing @irongolem/ui export. Most are page-local helpers and stay inline; flag for review only if you see the same shape appear in a second route — that's the point to graduate.",
  },
};

// ─────────────────────────────────────────────────────────────────────────────
// Rule runners — each takes raw file text + filename, returns findings.
// All rules are regex-based: high recall, low precision, human-reviewed.
// ─────────────────────────────────────────────────────────────────────────────

interface RuleContext {
  readonly file: string;       // relative path
  readonly source: string;
  readonly fileBasename: string; // without extension, normalized
}

function findingsFor(ctx: RuleContext): Finding[] {
  return [
    ...checkTailwindDynamic(ctx),
    ...checkUiSubstitution(ctx),
    ...checkPreviewShim(ctx),
    ...checkReactNamespace(ctx),
    ...checkJsxElementReturn(ctx),
    ...checkBgApp(ctx),
    ...checkRawHexTailwind(ctx),
    ...checkInlineKeyframes(ctx),
    ...checkCandidateUi(ctx),
  ];
}

function* lines(source: string): Generator<{ lineNo: number; text: string }> {
  const arr = source.split("\n");
  for (let i = 0; i < arr.length; i += 1) {
    yield { lineNo: i + 1, text: arr[i] ?? "" };
  }
}

function excerpt(text: string, max = 120): string {
  const t = text.trim();
  if (t.length <= max) return t;
  return t.slice(0, max - 1) + "…";
}

function checkTailwindDynamic(ctx: RuleContext): Finding[] {
  // Match `${...}` inside any backtick string that mentions a Tailwind prefix.
  // Two passes to avoid false-positives from non-className templates.
  const out: Finding[] = [];
  const prefixes = "(?:bg|text|border|ring|hover:bg|hover:text|hover:border)";
  // Find any backtick segment with `bg-${`, `text-${`, etc.
  const tplRe = new RegExp(`\\b${prefixes}-\\$\\{[^}]+\\}`, "g");
  for (const { lineNo, text } of lines(ctx.source)) {
    let m: RegExpExecArray | null;
    while ((m = tplRe.exec(text)) !== null) {
      out.push({
        rule: "tailwind-dynamic",
        severity: "blocker",
        file: ctx.file,
        line: lineNo,
        excerpt: excerpt(m[0]),
        recommendation: "Add a `TONE` classmap (see `apps/web/src/pages/v2/Home.tsx` lines ~80-110) and reference `TONE[name].bg` / `.text` / `.border` instead.",
      });
    }
  }
  return out;
}

function checkUiSubstitution(ctx: RuleContext): Finding[] {
  const out: Finding[] = [];
  const re = new RegExp(`\\bfunction\\s+(${UI_COMPONENT_NAMES.join("|")})\\s*\\(`, "g");
  for (const { lineNo, text } of lines(ctx.source)) {
    let m: RegExpExecArray | null;
    while ((m = re.exec(text)) !== null) {
      const name = m[1]!;
      out.push({
        rule: "ui-substitution",
        severity: "should-fix",
        file: ctx.file,
        line: lineNo,
        excerpt: excerpt(text),
        recommendation: `Replace with \`import { ${name} } from "@irongolem/ui"\` unless this local version intentionally diverges. The integrator should reconcile prop shapes before deleting the local def.`,
      });
    }
  }
  return out;
}

function checkPreviewShim(ctx: RuleContext): Finding[] {
  const out: Finding[] = [];
  // Two forms seen across bundles:
  //   1. `(window as unknown as { X: typeof X }).X = X` — newer TSX bundles
  //   2. `window.X = X;` — older JSX bundles (no type cast)
  const cast = /\(\s*window\s+as\s+[^)]+\)\.\w+\s*=/;
  const plain = /^\s*window\.[A-Z]\w*\s*=\s*[A-Z]\w*\s*;?\s*$/;
  for (const { lineNo, text } of lines(ctx.source)) {
    if (cast.test(text) || plain.test(text)) {
      out.push({
        rule: "preview-shim",
        severity: "blocker",
        file: ctx.file,
        line: lineNo,
        excerpt: excerpt(text),
        recommendation: "Delete this line in the production port. The shim exists only so the design's preview HTML can mount the route without a bundler.",
      });
    }
  }
  return out;
}

function checkReactNamespace(ctx: RuleContext): Finding[] {
  const out: Finding[] = [];
  const re = /^\s*import\s+\*\s+as\s+React\s+from\s+["']react["']/;
  for (const { lineNo, text } of lines(ctx.source)) {
    if (re.test(text)) {
      out.push({
        rule: "react-namespace",
        severity: "should-fix",
        file: ctx.file,
        line: lineNo,
        excerpt: excerpt(text),
        recommendation: 'Switch to `import React, { useState, useMemo, useEffect } from "react"` and update any `React.useState` / `React.useMemo` calls in the file to use the named hooks directly.',
      });
    }
  }
  return out;
}

function checkJsxElementReturn(ctx: RuleContext): Finding[] {
  const out: Finding[] = [];
  const re = /\):\s*JSX\.Element\b/;
  for (const { lineNo, text } of lines(ctx.source)) {
    if (re.test(text)) {
      out.push({
        rule: "jsx-element-return",
        severity: "blocker",
        file: ctx.file,
        line: lineNo,
        excerpt: excerpt(text),
        recommendation: "Remove the `: JSX.Element` annotation — TS infers the right return type from the function body. (TS 6+ retires the global JSX namespace.)",
      });
    }
  }
  return out;
}

function checkBgApp(ctx: RuleContext): Finding[] {
  const out: Finding[] = [];
  const re = /\bbg-app\b/;
  for (const { lineNo, text } of lines(ctx.source)) {
    if (re.test(text)) {
      out.push({
        rule: "bg-app",
        severity: "blocker",
        file: ctx.file,
        line: lineNo,
        excerpt: excerpt(text),
        recommendation: "Replace `bg-app` with `bg-neutral-50`. The design's `--bg-app` CSS variable isn't wired in this project — `bg-neutral-50` is the production app-shell surface.",
      });
    }
  }
  return out;
}

function checkRawHexTailwind(ctx: RuleContext): Finding[] {
  const out: Finding[] = [];
  const re = /\b(bg|text|border|fill|stroke|ring)-\[#[0-9a-fA-F]{3,8}\]/g;
  for (const { lineNo, text } of lines(ctx.source)) {
    let m: RegExpExecArray | null;
    while ((m = re.exec(text)) !== null) {
      out.push({
        rule: "raw-hex-tailwind",
        severity: "info",
        file: ctx.file,
        line: lineNo,
        excerpt: excerpt(m[0]),
        recommendation: "Use a semantic-palette class from the @irongolem/design-tokens bridge (`bg-safe`, `text-warning`, etc.) instead of an arbitrary hex. Theme swaps depend on this.",
      });
    }
  }
  return out;
}

function checkInlineKeyframes(ctx: RuleContext): Finding[] {
  const out: Finding[] = [];
  const re = /@keyframes\s+\w+/g;
  for (const { lineNo, text } of lines(ctx.source)) {
    let m: RegExpExecArray | null;
    while ((m = re.exec(text)) !== null) {
      out.push({
        rule: "inline-keyframes",
        severity: "should-fix",
        file: ctx.file,
        line: lineNo,
        excerpt: excerpt(m[0]),
        recommendation: "If this keyframe matches one already in `apps/web/src/styles/globals.css` (`ig-pulse`, `ig-toast-in`, `ig-slide-in`), drop the inline `<style>` block and apply the existing class. Otherwise consider promoting the keyframe to globals.css.",
      });
    }
  }
  return out;
}

function checkCandidateUi(ctx: RuleContext): Finding[] {
  const out: Finding[] = [];
  // Top-level function declarations starting with a capital letter.
  const re = /^\s*(?:export\s+)?function\s+([A-Z][A-Za-z0-9]+)\s*\(/;
  const existing = new Set<string>(UI_COMPONENT_NAMES);
  // Heuristic exclusion list — these patterns are route-local helpers, not
  // graduation candidates.
  const suffixDeny = /(Section|Header|Footer|Row|Item|List|Pill|Chip|Toggle|Drawer|Modal|Panel|Card|Empty|Trail|Mark)$/;
  for (const { lineNo, text } of lines(ctx.source)) {
    const m = re.exec(text);
    if (!m) continue;
    const name = m[1]!;
    if (existing.has(name)) continue;                       // already in @irongolem/ui
    if (suffixDeny.test(name)) continue;                     // route-local helper shape
    if (name.length < 4) continue;                           // too generic
    if (normalize(name) === ctx.fileBasename) continue;      // the route's main export
    out.push({
      rule: "candidate-ui",
      severity: "info",
      file: ctx.file,
      line: lineNo,
      excerpt: excerpt(text),
      recommendation: `Inline component \`${name}\`. If the same shape appears in a second route, consider graduating it to \`packages/ui/src/components/${name}.tsx\`.`,
    });
  }
  return out;
}

// ─────────────────────────────────────────────────────────────────────────────
// File discovery
// ─────────────────────────────────────────────────────────────────────────────

function normalize(s: string): string {
  return s.toLowerCase().replace(/[-_\s.]/g, "");
}

async function discoverSourceFiles(routeDir: string, routeSlug: string): Promise<string[]> {
  const projectDir = join(routeDir, "project");
  let entries: string[] = [];
  try {
    entries = await readdir(projectDir);
  } catch {
    return [];
  }
  const slug = normalize(routeSlug);
  const matches: string[] = [];
  for (const entry of entries) {
    if (!/\.(tsx|jsx)$/.test(entry)) continue;
    const base = entry.replace(/\.(tsx|jsx)$/, "");
    if (normalize(base).includes(slug)) {
      matches.push(join(projectDir, entry));
    }
  }
  return matches.sort();
}

// ─────────────────────────────────────────────────────────────────────────────
// Report rendering
// ─────────────────────────────────────────────────────────────────────────────

function escapeCell(s: string): string {
  return s.replace(/\|/g, "\\|").replace(/`/g, "\\`");
}

function renderReport(routeSlug: string, files: readonly string[], findings: readonly Finding[]): string {
  const sorted = [...findings].sort((a, b) => {
    const s = SEVERITY_ORDER[a.severity] - SEVERITY_ORDER[b.severity];
    if (s !== 0) return s;
    if (a.rule !== b.rule) return a.rule.localeCompare(b.rule);
    if (a.file !== b.file) return a.file.localeCompare(b.file);
    return a.line - b.line;
  });

  const byRule = new Map<Rule, Finding[]>();
  for (const f of sorted) {
    const list = byRule.get(f.rule);
    if (list) list.push(f);
    else byRule.set(f.rule, [f]);
  }

  const blockers = sorted.filter((f) => f.severity === "blocker").length;
  const shouldFix = sorted.filter((f) => f.severity === "should-fix").length;
  const info = sorted.filter((f) => f.severity === "info").length;

  const lines: string[] = [];
  lines.push(`# Audit — ${routeSlug}`);
  lines.push("");
  lines.push(`> Generated by \`scripts/design-component-audit.ts\` on ${new Date().toISOString().slice(0, 10)}.`);
  lines.push("");
  lines.push("## Summary");
  lines.push("");
  lines.push(`- **${blockers}** blocker${blockers === 1 ? "" : "s"} — must address before promotion`);
  lines.push(`- **${shouldFix}** should-fix item${shouldFix === 1 ? "" : "s"} — recommended swaps`);
  lines.push(`- **${info}** info item${info === 1 ? "" : "s"} — review and graduate if you see a pattern`);
  lines.push("");
  if (files.length === 0) {
    lines.push("No source files matched the route slug. Did you mistype the route name, or is the design bundle missing the matching `.tsx` / `.jsx`?");
    lines.push("");
    return lines.join("\n");
  }
  lines.push(`**Files scanned (${files.length}):**`);
  lines.push("");
  for (const f of files) lines.push(`- \`${f}\``);
  lines.push("");

  const ruleOrder: Rule[] = [
    "tailwind-dynamic",
    "preview-shim",
    "jsx-element-return",
    "bg-app",
    "ui-substitution",
    "react-namespace",
    "inline-keyframes",
    "raw-hex-tailwind",
    "candidate-ui",
  ];

  for (const rule of ruleOrder) {
    const items = byRule.get(rule);
    if (!items || items.length === 0) continue;
    const meta = RULE_META[rule];
    const sev = items[0]!.severity;
    const sevLabel = sev === "blocker" ? "🛑 BLOCKER" : sev === "should-fix" ? "⚠️  SHOULD-FIX" : "ℹ️  INFO";
    lines.push(`## ${sevLabel} · ${meta.title} (${items.length})`);
    lines.push("");
    lines.push(meta.description);
    lines.push("");
    lines.push("| File | Line | Excerpt |");
    lines.push("|---|---:|---|");
    for (const f of items) {
      lines.push(`| \`${escapeCell(f.file)}\` | ${f.line} | \`${escapeCell(f.excerpt)}\` |`);
    }
    lines.push("");
    // Recommendation block (per rule, since recommendations don't vary by instance for most rules)
    const recos = new Set(items.map((i) => i.recommendation));
    lines.push("**Recommendation:**");
    lines.push("");
    for (const r of recos) lines.push(`- ${r}`);
    lines.push("");
  }

  lines.push("---");
  lines.push("");
  lines.push("## How to use this report");
  lines.push("");
  lines.push("1. Read top-down — blockers first, then should-fix, then info.");
  lines.push("2. Accept or reject each finding consciously. Some `should-fix` items may be intentional (e.g. a route's local `SafetyCard` could be intentionally divergent from the @irongolem/ui shape; the audit can't know).");
  lines.push("3. Apply the accepted changes when translating to `apps/web/src/pages/v2/<Route>.tsx`.");
  lines.push("4. Capture an Interceptor visual baseline once the production port is live.");
  lines.push("");
  lines.push("This file is regenerated every run — feel free to delete it once the route is ported.");
  lines.push("");

  return lines.join("\n");
}

// ─────────────────────────────────────────────────────────────────────────────
// Main
// ─────────────────────────────────────────────────────────────────────────────

async function main() {
  const routeArg = process.argv[2];
  if (!routeArg) {
    console.error("usage: bun run scripts/design-component-audit.ts <route>");
    console.error("       where <route> is a directory under apps/web/src/_design-inbox/");
    process.exit(1);
  }

  const routeDir = join(INBOX, routeArg);
  try {
    const st = await stat(routeDir);
    if (!st.isDirectory()) throw new Error("not a directory");
  } catch {
    console.error(`error: \`${routeDir}\` does not exist or is not a directory`);
    process.exit(1);
  }

  const sourceFiles = await discoverSourceFiles(routeDir, routeArg);
  const findings: Finding[] = [];
  for (const abs of sourceFiles) {
    const source = await readFile(abs, "utf8");
    const rel = relative(INBOX, abs);
    const fileBasename = normalize(basename(abs).replace(/\.(tsx|jsx)$/, ""));
    findings.push(...findingsFor({ file: rel, source, fileBasename }));
  }

  const report = renderReport(routeArg, sourceFiles.map((f) => relative(INBOX, f)), findings);
  const outPath = join(routeDir, "AUDIT.md");
  await writeFile(outPath, report);

  const blockers = findings.filter((f) => f.severity === "blocker").length;
  const shouldFix = findings.filter((f) => f.severity === "should-fix").length;
  const info = findings.filter((f) => f.severity === "info").length;
  console.log(
    `${routeArg}: ${blockers} blocker · ${shouldFix} should-fix · ${info} info → ${relative(ROOT, outPath)}`,
  );
}

main().catch((err: unknown) => {
  console.error(err);
  process.exit(1);
});
