import React, { useState } from "react";

interface MemoryNode {
  id: string;
  name: string;
  type: "person" | "topic" | "source" | "task" | "preference" | "claim";
  confidence: number;
  freshness: string;
  evidence: number;
  hasContradiction: boolean;
}

const sampleNodes: MemoryNode[] = [
  { id: "1", name: "Weekly team standup preference", type: "preference", confidence: 0.95, freshness: "2 hours ago", evidence: 12, hasContradiction: false },
  { id: "2", name: "Project Alpha deadline", type: "task", confidence: 0.88, freshness: "1 day ago", evidence: 3, hasContradiction: false },
  { id: "3", name: "API pricing changes", type: "claim", confidence: 0.72, freshness: "3 days ago", evidence: 5, hasContradiction: true },
  { id: "4", name: "Sarah Chen", type: "person", confidence: 1.0, freshness: "5 hours ago", evidence: 28, hasContradiction: false },
  { id: "5", name: "Market research methodology", type: "topic", confidence: 0.85, freshness: "1 week ago", evidence: 8, hasContradiction: false },
];

const typeLabels: Record<string, string> = {
  person: "Person",
  topic: "Topic",
  source: "Source",
  task: "Task",
  preference: "Learned preference",
  claim: "Research finding",
};

export default function Memory() {
  const [view, setView] = useState<"list" | "graph">("list");

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-neutral-900">Memory</h1>
          <p className="text-neutral-500 mt-1">Everything the system knows, with evidence links</p>
        </div>
        <div className="flex gap-2">
          <button
            onClick={() => setView("list")}
            className={`px-3 py-1.5 text-sm rounded-lg ${view === "list" ? "bg-neutral-900 text-white" : "bg-neutral-100 text-neutral-600"}`}
          >
            List view
          </button>
          <button
            onClick={() => setView("graph")}
            className={`px-3 py-1.5 text-sm rounded-lg ${view === "graph" ? "bg-neutral-900 text-white" : "bg-neutral-100 text-neutral-600"}`}
          >
            Graph view
          </button>
        </div>
      </div>

      {view === "list" ? (
        <div className="space-y-3">
          {sampleNodes.map((node) => (
            <article key={node.id} className="bg-white rounded-xl border border-neutral-200 p-4">
              <div className="flex items-start justify-between">
                <div>
                  <h3 className="font-medium text-neutral-900">{node.name}</h3>
                  <span className="text-xs text-neutral-500 bg-neutral-100 rounded-full px-2 py-0.5 mt-1 inline-block">
                    {typeLabels[node.type]}
                  </span>
                </div>
                {node.hasContradiction && (
                  <span className="text-xs bg-amber-100 text-amber-800 rounded-full px-2 py-0.5 font-medium">
                    Contradiction detected
                  </span>
                )}
              </div>
              <div className="flex gap-4 mt-3 text-xs text-neutral-500">
                <span>Confidence: {Math.round(node.confidence * 100)}%</span>
                <span>Updated: {node.freshness}</span>
                <span>{node.evidence} evidence links</span>
              </div>
            </article>
          ))}
        </div>
      ) : (
        <div className="bg-white rounded-xl border border-neutral-200 p-12 text-center text-neutral-400">
          <p className="text-lg">Graph visualization</p>
          <p className="text-sm mt-2">Interactive knowledge graph will be rendered here</p>
        </div>
      )}
    </div>
  );
}
