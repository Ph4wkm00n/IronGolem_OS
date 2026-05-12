#!/usr/bin/env bun
/**
 * Dump the canonical `:root { ... }` block from @irongolem/design-tokens.
 *
 * The output is a regenerable companion to the hand-written `:root` block
 * in `apps/web/src/styles/globals.css`. Today the two are kept in sync by
 * hand; this script is the audit + future automation seam.
 *
 *   bun run scripts/dump-tokens-css.ts
 *   bun run scripts/dump-tokens-css.ts > apps/web/src/styles/tokens.generated.css
 *
 * When the team is ready to remove the hand-written block, switch
 * globals.css to `@import "./tokens.generated.css";` and wire this script
 * into the build (e.g. a `pretypecheck` step or a Makefile target).
 */

import { buildRootCssVariables } from "../packages/design-tokens/src/tailwind-bridge.ts";

const body = buildRootCssVariables();
const out = `:root {\n${body}\n}\n`;
process.stdout.write(out);
