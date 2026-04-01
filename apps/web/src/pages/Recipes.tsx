import React, { useEffect, useState } from "react";
import { SafetyCard, RiskBadge } from "@irongolem/ui";
import type { Recipe } from "@irongolem/schema";
import api from "../lib/api";

/* ------------------------------------------------------------------ */
/*  Placeholder data (used when the API is unavailable)                */
/* ------------------------------------------------------------------ */

const SAMPLE_RECIPES: readonly Recipe[] = [
  {
    id: "recipe-email-triage",
    title: "Email Triage",
    description:
      "Automatically sorts incoming emails by priority, drafts quick replies for routine messages, and flags anything that needs your personal attention.",
    category: "Inbox",
    riskLevel: "low",
    isActive: false,
    canAccess: ["Read emails", "Draft replies", "Apply labels"],
    cannotAccess: ["Send emails without approval", "Delete emails"],
    needsApprovalFor: ["Sending any reply", "Archiving threads"],
    stopsIf: ["More than 50 emails processed in one hour"],
    createdAt: "2026-03-15T10:00:00Z",
    updatedAt: "2026-03-28T14:30:00Z",
  },
  {
    id: "recipe-calendar-manager",
    title: "Calendar Manager",
    description:
      "Keeps your calendar organized by detecting conflicts, suggesting optimal meeting times, and sending reminders before important events.",
    category: "Scheduling",
    riskLevel: "medium",
    isActive: true,
    canAccess: ["Read calendar events", "Suggest reschedules"],
    cannotAccess: ["Delete events", "Invite external participants"],
    needsApprovalFor: ["Moving any meeting", "Declining invitations"],
    stopsIf: ["More than 10 calendar changes per day"],
    createdAt: "2026-03-10T08:00:00Z",
    updatedAt: "2026-03-29T09:15:00Z",
  },
  {
    id: "recipe-research-monitor",
    title: "Research Monitor",
    description:
      "Tracks topics you care about across the web, summarizes new findings, and alerts you when something important changes.",
    category: "Research",
    riskLevel: "low",
    isActive: false,
    canAccess: ["Search the web", "Read bookmarked sources", "Create summaries"],
    cannotAccess: ["Post content online", "Subscribe to paid services"],
    needsApprovalFor: ["Adding new tracked topics"],
    stopsIf: ["Confidence drops below 40%", "Sources contradict each other"],
    createdAt: "2026-03-20T12:00:00Z",
    updatedAt: "2026-03-30T16:45:00Z",
  },
];

/* ------------------------------------------------------------------ */
/*  State                                                              */
/* ------------------------------------------------------------------ */

interface RecipesState {
  recipes: readonly Recipe[];
  expandedId: string | null;
  loading: boolean;
}

/* ------------------------------------------------------------------ */
/*  Page                                                               */
/* ------------------------------------------------------------------ */

export function Recipes() {
  const [state, setState] = useState<RecipesState>({
    recipes: [],
    expandedId: null,
    loading: true,
  });

  useEffect(() => {
    let cancelled = false;

    async function load() {
      try {
        const res = await api.recipes.list({ pageSize: 50 });
        if (!cancelled) {
          setState({
            recipes: res.items.length > 0 ? res.items : SAMPLE_RECIPES,
            expandedId: null,
            loading: false,
          });
        }
      } catch {
        if (!cancelled) {
          setState({ recipes: SAMPLE_RECIPES, expandedId: null, loading: false });
        }
      }
    }

    load();
    return () => {
      cancelled = true;
    };
  }, []);

  function toggleSafety(id: string) {
    setState((prev) => ({
      ...prev,
      expandedId: prev.expandedId === id ? null : id,
    }));
  }

  async function handleActivate(id: string) {
    try {
      const updated = await api.recipes.activate(id);
      setState((prev) => ({
        ...prev,
        recipes: prev.recipes.map((r) => (r.id === id ? updated : r)),
      }));
    } catch {
      // Silently fail in demo mode
    }
  }

  async function handleDeactivate(id: string) {
    try {
      const updated = await api.recipes.deactivate(id);
      setState((prev) => ({
        ...prev,
        recipes: prev.recipes.map((r) => (r.id === id ? updated : r)),
      }));
    } catch {
      // Silently fail in demo mode
    }
  }

  return (
    <div className="page-container">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="page-title">Recipes</h1>
          <p className="mt-1 text-sm text-neutral-500">
            Automation templates that handle tasks for you, with safety rules built in.
          </p>
        </div>
      </div>

      {state.loading ? (
        <div className="flex items-center justify-center py-20">
          <div className="h-8 w-8 animate-spin rounded-full border-4 border-neutral-200 border-t-indigo-600" />
        </div>
      ) : (
        <div className="grid gap-6 sm:grid-cols-2 lg:grid-cols-3">
          {state.recipes.map((recipe) => (
            <RecipeCard
              key={recipe.id}
              recipe={recipe}
              expanded={state.expandedId === recipe.id}
              onToggleSafety={() => toggleSafety(recipe.id)}
              onActivate={() => handleActivate(recipe.id)}
              onDeactivate={() => handleDeactivate(recipe.id)}
            />
          ))}
        </div>
      )}
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Recipe card                                                        */
/* ------------------------------------------------------------------ */

interface RecipeCardProps {
  recipe: Recipe;
  expanded: boolean;
  onToggleSafety: () => void;
  onActivate: () => void;
  onDeactivate: () => void;
}

function RecipeCard({
  recipe,
  expanded,
  onToggleSafety,
  onActivate,
  onDeactivate,
}: RecipeCardProps) {
  return (
    <article
      className="flex flex-col rounded-xl border border-neutral-200 bg-white shadow-sm overflow-hidden hover:shadow-md transition-shadow"
      aria-label={`Recipe: ${recipe.title}`}
    >
      <div className="flex-1 p-4">
        {/* Header */}
        <div className="flex items-start justify-between gap-2 mb-2">
          <div>
            <h3 className="text-base font-semibold text-neutral-900">
              {recipe.title}
            </h3>
            <p className="text-xs text-neutral-500 mt-0.5">{recipe.category}</p>
          </div>
          <RiskBadge level={recipe.riskLevel} size="sm" />
        </div>

        {/* Description */}
        <p className="text-sm text-neutral-600 leading-relaxed">
          {recipe.description}
        </p>

        {/* Safety summary toggle */}
        <button
          type="button"
          onClick={onToggleSafety}
          className="mt-3 text-xs font-medium text-indigo-600 hover:text-indigo-500 flex items-center gap-1"
        >
          <svg
            className={`w-3 h-3 transition-transform ${expanded ? "rotate-90" : ""}`}
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            strokeWidth={2}
          >
            <path strokeLinecap="round" strokeLinejoin="round" d="M9 5l7 7-7 7" />
          </svg>
          {expanded ? "Hide safety details" : "View safety details"}
        </button>

        {/* Expanded safety card */}
        {expanded && (
          <div className="mt-3">
            <SafetyCard
              canAccess={[...recipe.canAccess]}
              cannotAccess={[...recipe.cannotAccess]}
              needsApprovalFor={[...recipe.needsApprovalFor]}
              stopsIf={[...recipe.stopsIf]}
              riskLevel={recipe.riskLevel}
            />
          </div>
        )}
      </div>

      {/* Footer */}
      <div className="border-t border-neutral-100 px-4 py-3 flex items-center justify-between">
        {recipe.isActive ? (
          <>
            <span className="flex items-center gap-1.5 text-xs font-medium text-emerald-700">
              <span className="h-2 w-2 rounded-full bg-emerald-500" />
              Active
            </span>
            <button
              type="button"
              onClick={onDeactivate}
              className="rounded-lg border border-neutral-200 px-3 py-1.5 text-xs font-medium text-neutral-700 hover:bg-neutral-50 transition-colors"
            >
              Deactivate
            </button>
          </>
        ) : (
          <>
            <span className="flex items-center gap-1.5 text-xs font-medium text-neutral-400">
              <span className="h-2 w-2 rounded-full bg-neutral-300" />
              Inactive
            </span>
            <button
              type="button"
              onClick={onActivate}
              className="rounded-lg bg-indigo-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-indigo-500 transition-colors"
            >
              Activate
            </button>
          </>
        )}
      </div>
    </article>
  );
}
