import React, { useEffect, useState } from "react";
import { SafetyCard, RiskBadge } from "@irongolem/ui";
import type { Recipe } from "@irongolem/schema";
import api from "../lib/api";

/* ------------------------------------------------------------------ */
/*  State                                                              */
/* ------------------------------------------------------------------ */

interface RecipesState {
  recipes: readonly Recipe[];
  loading: boolean;
  selectedId: string | null;
}

/* ------------------------------------------------------------------ */
/*  Page                                                               */
/* ------------------------------------------------------------------ */

export function Recipes() {
  const [state, setState] = useState<RecipesState>({
    recipes: [],
    loading: true,
    selectedId: null,
  });

  useEffect(() => {
    let cancelled = false;

    async function load() {
      try {
        const res = await api.recipes.list({ pageSize: 50 });
        if (!cancelled) {
          setState((prev) => ({ ...prev, loading: false, recipes: res.items }));
        }
      } catch {
        if (!cancelled) setState((prev) => ({ ...prev, loading: false }));
      }
    }

    load();
    return () => { cancelled = true; };
  }, []);

  const selectedRecipe = state.recipes.find((r) => r.id === state.selectedId);

  return (
    <div className="page-container">
      <h1 className="page-title mb-6">Recipes</h1>

      {state.loading ? (
        <div className="flex items-center justify-center py-20">
          <div className="h-8 w-8 animate-spin rounded-full border-4 border-neutral-200 border-t-indigo-600" />
        </div>
      ) : state.recipes.length === 0 ? (
        <div className="card-padded text-center py-12">
          <p className="text-neutral-500">No recipes available yet.</p>
          <p className="text-sm text-neutral-400 mt-1">
            Recipes are automation templates that tell your assistants what to do and when.
          </p>
        </div>
      ) : (
        <div className="grid gap-6 lg:grid-cols-3">
          {/* Recipe gallery */}
          <div className="lg:col-span-2">
            <div className="grid gap-4 sm:grid-cols-2">
              {state.recipes.map((recipe) => (
                <RecipeCard
                  key={recipe.id}
                  recipe={recipe}
                  isSelected={recipe.id === state.selectedId}
                  onSelect={() =>
                    setState((prev) => ({
                      ...prev,
                      selectedId: prev.selectedId === recipe.id ? null : recipe.id,
                    }))
                  }
                />
              ))}
            </div>
          </div>

          {/* Safety detail panel */}
          <div className="lg:col-span-1">
            {selectedRecipe ? (
              <div className="sticky top-6 space-y-4">
                <SafetyCard
                  title={`${selectedRecipe.title} — Safety summary`}
                  canAccess={selectedRecipe.canAccess as string[]}
                  cannotAccess={selectedRecipe.cannotAccess as string[]}
                  needsApprovalFor={selectedRecipe.needsApprovalFor as string[]}
                  stopsIf={selectedRecipe.stopsIf as string[]}
                  riskLevel={selectedRecipe.riskLevel}
                />

                <div className="flex gap-2">
                  {selectedRecipe.isActive ? (
                    <button
                      type="button"
                      className="flex-1 rounded-lg bg-neutral-100 px-4 py-2 text-sm font-medium text-neutral-700 hover:bg-neutral-200 transition-colors"
                      onClick={() => handleDeactivate(selectedRecipe.id)}
                    >
                      Deactivate
                    </button>
                  ) : (
                    <button
                      type="button"
                      className="flex-1 rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-500 transition-colors"
                      onClick={() => handleActivate(selectedRecipe.id)}
                    >
                      Activate
                    </button>
                  )}
                </div>
              </div>
            ) : (
              <div className="card-padded text-center text-sm text-neutral-500">
                Select a recipe to see its safety summary.
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );

  async function handleActivate(id: string) {
    try {
      const updated = await api.recipes.activate(id);
      setState((prev) => ({
        ...prev,
        recipes: prev.recipes.map((r) => (r.id === id ? updated : r)),
      }));
    } catch {
      // Error handling would use a toast notification in production
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
      // Error handling would use a toast notification in production
    }
  }
}

/* ------------------------------------------------------------------ */
/*  Recipe card                                                        */
/* ------------------------------------------------------------------ */

function RecipeCard({
  recipe,
  isSelected,
  onSelect,
}: {
  recipe: Recipe;
  isSelected: boolean;
  onSelect: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onSelect}
      className={`w-full text-left rounded-xl border p-4 transition-all ${
        isSelected
          ? "border-indigo-300 bg-indigo-50/50 ring-1 ring-indigo-200"
          : "border-neutral-200 bg-white hover:border-neutral-300 hover:shadow-sm"
      }`}
    >
      <div className="flex items-start justify-between gap-2">
        <h3 className="text-sm font-semibold text-neutral-900">{recipe.title}</h3>
        <RiskBadge level={recipe.riskLevel} size="sm" />
      </div>

      <p className="mt-1.5 text-sm text-neutral-600 line-clamp-2">
        {recipe.description}
      </p>

      <div className="mt-3 flex items-center gap-2">
        <span className="text-xs text-neutral-400">{recipe.category}</span>
        {recipe.isActive && (
          <span className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-xs font-medium bg-emerald-100 text-emerald-700">
            Active
          </span>
        )}
      </div>
    </button>
  );
}
