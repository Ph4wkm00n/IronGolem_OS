/**
 * RouteStub — placeholder body for v2 routes that don't yet have a Claude
 * Design export. Reuses WorkspaceTopbar so the chrome stays consistent;
 * the body explains where the design will land.
 *
 * Replace each stub with a real Home-style file as Claude Design exports
 * arrive for that route. See docs/design/claude-design-prompts/<route>.md
 * for the pasteable prompt that produces the design.
 */

import React from "react";
import { Link } from "react-router-dom";

import { WorkspaceTopbar } from "./WorkspaceTopbar";

export interface RouteStubProps {
  readonly title: string;
  readonly purpose: string;
  /** Route path (e.g. "/inbox") — referenced in the prompt-file pointer. */
  readonly path: string;
  /** Slug used in the prompt filename (e.g. "inbox" → claude-design-prompts/inbox.md). */
  readonly promptSlug: string;
}

export function RouteStub({ title, purpose, path, promptSlug }: RouteStubProps) {
  return (
    <div className="min-h-screen bg-neutral-50">
      <WorkspaceTopbar />

      <main className="page-container">
        <header className="mb-6">
          <div className="text-xs font-medium uppercase tracking-wide text-neutral-500">
            {path}
          </div>
          <h1 className="page-title mt-1">{title}</h1>
          <p className="text-neutral-600 mt-1 max-w-2xl">{purpose}</p>
        </header>

        <section className="card card-padded max-w-2xl">
          <div className="flex items-start gap-3">
            <span className="h-8 w-8 rounded-lg bg-accent text-accent inline-flex items-center justify-center shrink-0">
              <svg
                xmlns="http://www.w3.org/2000/svg"
                viewBox="0 0 24 24"
                width={18}
                height={18}
                fill="none"
                stroke="currentColor"
                strokeWidth={1.5}
                strokeLinecap="round"
                strokeLinejoin="round"
                aria-hidden
              >
                <path d="M12 3v3M12 18v3M3 12h3M18 12h3M5.6 5.6l2.1 2.1M16.3 16.3l2.1 2.1M5.6 18.4l2.1-2.1M16.3 7.7l2.1-2.1" />
              </svg>
            </span>
            <div className="flex-1 min-w-0">
              <h2 className="section-title">Awaiting Claude Design</h2>
              <p className="mt-2 text-sm text-neutral-700">
                The {title.toLowerCase()} page hasn't been designed in Claude Design yet.
                Until it is, this stub renders with the shared workspace chrome so v2 mode
                stays fully navigable.
              </p>
              <div className="mt-4 rounded-md bg-neutral-50 border border-neutral-100 p-3 text-sm text-neutral-700">
                <div className="text-xs font-medium uppercase tracking-wide text-neutral-500 mb-1">
                  When you're ready to design this route
                </div>
                <p className="mb-2">
                  Open a fresh chat at <span className="font-mono">claude.ai/design</span> and
                  paste the route-specific prompt block from:
                </p>
                <code className="font-mono text-xs block bg-white border border-neutral-200 rounded px-2 py-1">
                  docs/design/claude-design-prompts/{promptSlug}.md
                </code>
                <p className="mt-2 text-xs text-neutral-500">
                  The exported TSX lands in{" "}
                  <code className="font-mono">apps/web/src/_design-inbox/{promptSlug}/</code>,
                  then promotes through the F3-F8 pipeline (audit → translate → register →
                  visual baseline).
                </p>
              </div>
              <div className="mt-4 flex items-center gap-3 text-sm">
                <Link
                  to="/"
                  className="text-accent hover:text-accent-solid font-medium"
                >
                  ← Back to workspace
                </Link>
                <span className="text-neutral-300">·</span>
                <a
                  href={`#${promptSlug}`}
                  className="text-neutral-500 hover:text-neutral-900"
                >
                  See the integration plan
                </a>
              </div>
            </div>
          </div>
        </section>
      </main>
    </div>
  );
}
