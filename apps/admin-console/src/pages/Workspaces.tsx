import React, { useState } from "react";

/* ------------------------------------------------------------------ */
/*  Sample data                                                        */
/* ------------------------------------------------------------------ */

interface Workspace {
  readonly id: string;
  readonly name: string;
  readonly memberCount: number;
  readonly connectorCount: number;
  readonly activeRecipes: number;
  readonly status: "active" | "suspended" | "provisioning";
  readonly dataBoundary: "us" | "eu" | "ap";
  readonly isolated: boolean;
  readonly createdAt: string;
}

const WORKSPACES: Workspace[] = [
  { id: "ws-001", name: "Engineering", memberCount: 18, connectorCount: 8, activeRecipes: 24, status: "active", dataBoundary: "us", isolated: true, createdAt: "2026-01-15" },
  { id: "ws-002", name: "Sales", memberCount: 12, connectorCount: 5, activeRecipes: 15, status: "active", dataBoundary: "us", isolated: true, createdAt: "2026-01-20" },
  { id: "ws-003", name: "HR", memberCount: 6, connectorCount: 3, activeRecipes: 8, status: "active", dataBoundary: "eu", isolated: true, createdAt: "2026-02-01" },
  { id: "ws-004", name: "Finance", memberCount: 4, connectorCount: 4, activeRecipes: 12, status: "active", dataBoundary: "us", isolated: true, createdAt: "2026-02-10" },
  { id: "ws-005", name: "Marketing", memberCount: 7, connectorCount: 6, activeRecipes: 18, status: "active", dataBoundary: "eu", isolated: true, createdAt: "2026-02-15" },
  { id: "ws-006", name: "Support", memberCount: 0, connectorCount: 0, activeRecipes: 0, status: "provisioning", dataBoundary: "ap", isolated: false, createdAt: "2026-03-30" },
];

/* ------------------------------------------------------------------ */
/*  Helpers                                                            */
/* ------------------------------------------------------------------ */

const statusStyles: Record<Workspace["status"], string> = {
  active: "bg-emerald-50 text-emerald-700 border-emerald-200",
  suspended: "bg-red-50 text-red-700 border-red-200",
  provisioning: "bg-amber-50 text-amber-700 border-amber-200",
};

const boundaryLabel: Record<Workspace["dataBoundary"], string> = {
  us: "US",
  eu: "EU",
  ap: "APAC",
};

/* ------------------------------------------------------------------ */
/*  Component                                                          */
/* ------------------------------------------------------------------ */

export function Workspaces() {
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [selectedWorkspace, setSelectedWorkspace] = useState<Workspace | null>(null);

  return (
    <div className="page-container">
      <div className="flex items-center justify-between mb-4">
        <h2 className="page-title">Workspaces</h2>
        <button
          type="button"
          onClick={() => setShowCreateForm(!showCreateForm)}
          className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-md bg-indigo-600 text-white hover:bg-indigo-700 transition-colors"
        >
          <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M12 4.5v15m7.5-7.5h-15" />
          </svg>
          Create Workspace
        </button>
      </div>

      {/* Create workspace form */}
      {showCreateForm && (
        <div className="card-padded mb-4">
          <h3 className="text-sm font-semibold text-neutral-900 mb-3">New Workspace</h3>
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
            <div>
              <label className="block text-xs font-medium text-neutral-600 mb-1">Name</label>
              <input
                type="text"
                placeholder="e.g. Product Team"
                className="w-full px-2.5 py-1.5 text-xs border border-neutral-300 rounded-md focus:outline-none focus:ring-1 focus:ring-indigo-500 focus:border-indigo-500"
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-neutral-600 mb-1">Data Boundary</label>
              <select className="w-full px-2.5 py-1.5 text-xs border border-neutral-300 rounded-md focus:outline-none focus:ring-1 focus:ring-indigo-500 focus:border-indigo-500">
                <option value="us">US</option>
                <option value="eu">EU</option>
                <option value="ap">APAC</option>
              </select>
            </div>
            <div className="flex items-end gap-2">
              <button
                type="button"
                className="px-3 py-1.5 text-xs font-medium rounded-md bg-indigo-600 text-white hover:bg-indigo-700 transition-colors"
              >
                Create
              </button>
              <button
                type="button"
                onClick={() => setShowCreateForm(false)}
                className="px-3 py-1.5 text-xs font-medium rounded-md border border-neutral-300 text-neutral-600 hover:bg-neutral-50 transition-colors"
              >
                Cancel
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Workspace table */}
      <div className="card overflow-hidden">
        <table className="w-full text-xs">
          <thead>
            <tr className="text-left text-neutral-500 bg-neutral-50 border-b border-neutral-200">
              <th className="px-3 py-2 font-medium">Name</th>
              <th className="px-3 py-2 font-medium">Members</th>
              <th className="px-3 py-2 font-medium">Connectors</th>
              <th className="px-3 py-2 font-medium">Recipes</th>
              <th className="px-3 py-2 font-medium">Boundary</th>
              <th className="px-3 py-2 font-medium">Isolation</th>
              <th className="px-3 py-2 font-medium">Status</th>
              <th className="px-3 py-2 font-medium">Created</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-neutral-100">
            {WORKSPACES.map((ws) => (
              <tr
                key={ws.id}
                className="hover:bg-neutral-50 cursor-pointer transition-colors"
                onClick={() => setSelectedWorkspace(selectedWorkspace?.id === ws.id ? null : ws)}
              >
                <td className="px-3 py-2 font-medium text-neutral-900">{ws.name}</td>
                <td className="px-3 py-2 text-neutral-600">{ws.memberCount}</td>
                <td className="px-3 py-2 text-neutral-600">{ws.connectorCount}</td>
                <td className="px-3 py-2 text-neutral-600">{ws.activeRecipes}</td>
                <td className="px-3 py-2">
                  <span className="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-semibold bg-neutral-100 text-neutral-600">
                    {boundaryLabel[ws.dataBoundary]}
                  </span>
                </td>
                <td className="px-3 py-2">
                  {ws.isolated ? (
                    <span className="inline-flex items-center gap-1 text-emerald-600">
                      <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                        <path strokeLinecap="round" strokeLinejoin="round" d="M16.5 10.5V6.75a4.5 4.5 0 10-9 0v3.75m-.75 11.25h10.5a2.25 2.25 0 002.25-2.25v-6.75a2.25 2.25 0 00-2.25-2.25H6.75a2.25 2.25 0 00-2.25 2.25v6.75a2.25 2.25 0 002.25 2.25z" />
                      </svg>
                      Isolated
                    </span>
                  ) : (
                    <span className="text-neutral-400">Pending</span>
                  )}
                </td>
                <td className="px-3 py-2">
                  <span className={`inline-flex items-center px-1.5 py-0.5 rounded border text-[10px] font-semibold capitalize ${statusStyles[ws.status]}`}>
                    {ws.status}
                  </span>
                </td>
                <td className="px-3 py-2 text-neutral-500">{ws.createdAt}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Workspace detail panel */}
      {selectedWorkspace && (
        <div className="card-padded mt-3">
          <div className="flex items-center justify-between mb-3">
            <h3 className="text-sm font-semibold text-neutral-900">
              {selectedWorkspace.name} — Detail
            </h3>
            <button
              type="button"
              onClick={() => setSelectedWorkspace(null)}
              className="text-neutral-400 hover:text-neutral-600"
            >
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 text-xs">
            <div>
              <p className="text-neutral-500 mb-0.5">Data Boundary</p>
              <p className="font-medium text-neutral-800">{boundaryLabel[selectedWorkspace.dataBoundary]} Region</p>
            </div>
            <div>
              <p className="text-neutral-500 mb-0.5">Isolation Status</p>
              <p className="font-medium text-neutral-800">{selectedWorkspace.isolated ? "Fully Isolated" : "Provisioning"}</p>
            </div>
            <div>
              <p className="text-neutral-500 mb-0.5">Active Recipes</p>
              <p className="font-medium text-neutral-800">{selectedWorkspace.activeRecipes}</p>
            </div>
            <div>
              <p className="text-neutral-500 mb-0.5">Created</p>
              <p className="font-medium text-neutral-800">{selectedWorkspace.createdAt}</p>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
