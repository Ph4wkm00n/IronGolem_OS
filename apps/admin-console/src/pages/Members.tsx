import React, { useState } from "react";

/* ------------------------------------------------------------------ */
/*  Sample data                                                        */
/* ------------------------------------------------------------------ */

type MemberRole = "owner" | "admin" | "member" | "viewer";

interface Member {
  readonly id: string;
  readonly name: string;
  readonly email: string;
  readonly role: MemberRole;
  readonly workspace: string;
  readonly lastActive: string;
  readonly status: "active" | "invited" | "disabled";
}

const MEMBERS: Member[] = [
  { id: "m-001", name: "Alice Chen", email: "alice@irongolem.local", role: "owner", workspace: "Engineering", lastActive: "2026-04-01T14:22:00Z", status: "active" },
  { id: "m-002", name: "Bob Martinez", email: "bob@irongolem.local", role: "admin", workspace: "Engineering", lastActive: "2026-04-01T13:45:00Z", status: "active" },
  { id: "m-003", name: "Carol Kim", email: "carol@irongolem.local", role: "admin", workspace: "Sales", lastActive: "2026-04-01T12:30:00Z", status: "active" },
  { id: "m-004", name: "David Patel", email: "david@irongolem.local", role: "member", workspace: "HR", lastActive: "2026-04-01T10:15:00Z", status: "active" },
  { id: "m-005", name: "Eva Johansson", email: "eva@irongolem.local", role: "member", workspace: "Finance", lastActive: "2026-03-31T18:00:00Z", status: "active" },
  { id: "m-006", name: "Frank Wu", email: "frank@irongolem.local", role: "viewer", workspace: "Marketing", lastActive: "2026-03-30T09:45:00Z", status: "active" },
  { id: "m-007", name: "Grace Liu", email: "grace@irongolem.local", role: "member", workspace: "Engineering", lastActive: "Never", status: "invited" },
  { id: "m-008", name: "Henry Okafor", email: "henry@irongolem.local", role: "viewer", workspace: "Sales", lastActive: "2026-03-15T14:00:00Z", status: "disabled" },
];

/* ------------------------------------------------------------------ */
/*  Helpers                                                            */
/* ------------------------------------------------------------------ */

const roleStyles: Record<MemberRole, string> = {
  owner: "bg-indigo-50 text-indigo-700 border-indigo-200",
  admin: "bg-violet-50 text-violet-700 border-violet-200",
  member: "bg-neutral-100 text-neutral-700 border-neutral-200",
  viewer: "bg-neutral-50 text-neutral-500 border-neutral-200",
};

const statusStyles: Record<Member["status"], string> = {
  active: "text-emerald-600",
  invited: "text-amber-600",
  disabled: "text-neutral-400",
};

function formatLastActive(value: string): string {
  if (value === "Never") return "Never";
  const d = new Date(value);
  const now = new Date("2026-04-01T15:00:00Z");
  const diffH = Math.round((now.getTime() - d.getTime()) / 3600000);
  if (diffH < 1) return "Just now";
  if (diffH < 24) return `${diffH}h ago`;
  const diffD = Math.round(diffH / 24);
  return `${diffD}d ago`;
}

/* ------------------------------------------------------------------ */
/*  Component                                                          */
/* ------------------------------------------------------------------ */

export function Members() {
  const [showInviteForm, setShowInviteForm] = useState(false);

  return (
    <div className="page-container">
      <div className="flex items-center justify-between mb-4">
        <h2 className="page-title">Members</h2>
        <button
          type="button"
          onClick={() => setShowInviteForm(!showInviteForm)}
          className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-md bg-indigo-600 text-white hover:bg-indigo-700 transition-colors"
        >
          <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M19 7.5v3m0 0v3m0-3h3m-3 0h-3m-2.25-4.125a3.375 3.375 0 11-6.75 0 3.375 3.375 0 016.75 0zM4 19.235v-.11a6.375 6.375 0 0112.75 0v.109A12.318 12.318 0 0110.374 21c-2.331 0-4.512-.645-6.374-1.766z" />
          </svg>
          Invite Member
        </button>
      </div>

      {/* Invite form */}
      {showInviteForm && (
        <div className="card-padded mb-4">
          <h3 className="text-sm font-semibold text-neutral-900 mb-3">Invite Member</h3>
          <div className="grid grid-cols-1 sm:grid-cols-4 gap-3">
            <div>
              <label className="block text-xs font-medium text-neutral-600 mb-1">Email</label>
              <input
                type="email"
                placeholder="user@example.com"
                className="w-full px-2.5 py-1.5 text-xs border border-neutral-300 rounded-md focus:outline-none focus:ring-1 focus:ring-indigo-500 focus:border-indigo-500"
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-neutral-600 mb-1">Role</label>
              <select className="w-full px-2.5 py-1.5 text-xs border border-neutral-300 rounded-md focus:outline-none focus:ring-1 focus:ring-indigo-500 focus:border-indigo-500">
                <option value="viewer">Viewer</option>
                <option value="member">Member</option>
                <option value="admin">Admin</option>
              </select>
            </div>
            <div>
              <label className="block text-xs font-medium text-neutral-600 mb-1">Workspace</label>
              <select className="w-full px-2.5 py-1.5 text-xs border border-neutral-300 rounded-md focus:outline-none focus:ring-1 focus:ring-indigo-500 focus:border-indigo-500">
                <option>Engineering</option>
                <option>Sales</option>
                <option>HR</option>
                <option>Finance</option>
                <option>Marketing</option>
              </select>
            </div>
            <div className="flex items-end gap-2">
              <button
                type="button"
                className="px-3 py-1.5 text-xs font-medium rounded-md bg-indigo-600 text-white hover:bg-indigo-700 transition-colors"
              >
                Send Invite
              </button>
              <button
                type="button"
                onClick={() => setShowInviteForm(false)}
                className="px-3 py-1.5 text-xs font-medium rounded-md border border-neutral-300 text-neutral-600 hover:bg-neutral-50 transition-colors"
              >
                Cancel
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Member table */}
      <div className="card overflow-hidden">
        <table className="w-full text-xs">
          <thead>
            <tr className="text-left text-neutral-500 bg-neutral-50 border-b border-neutral-200">
              <th className="px-3 py-2 font-medium">Name</th>
              <th className="px-3 py-2 font-medium">Email</th>
              <th className="px-3 py-2 font-medium">Role</th>
              <th className="px-3 py-2 font-medium">Workspace</th>
              <th className="px-3 py-2 font-medium">Last Active</th>
              <th className="px-3 py-2 font-medium">Status</th>
              <th className="px-3 py-2 font-medium">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-neutral-100">
            {MEMBERS.map((member) => (
              <tr key={member.id} className="hover:bg-neutral-50 transition-colors">
                <td className="px-3 py-2 font-medium text-neutral-900">{member.name}</td>
                <td className="px-3 py-2 text-neutral-600">{member.email}</td>
                <td className="px-3 py-2">
                  <span className={`inline-flex items-center px-1.5 py-0.5 rounded border text-[10px] font-semibold capitalize ${roleStyles[member.role]}`}>
                    {member.role}
                  </span>
                </td>
                <td className="px-3 py-2 text-neutral-600">{member.workspace}</td>
                <td className="px-3 py-2 text-neutral-500">{formatLastActive(member.lastActive)}</td>
                <td className="px-3 py-2">
                  <span className={`text-[10px] font-semibold capitalize ${statusStyles[member.status]}`}>
                    {member.status}
                  </span>
                </td>
                <td className="px-3 py-2">
                  <select
                    defaultValue={member.role}
                    className="text-[10px] border border-neutral-200 rounded px-1 py-0.5 text-neutral-600 focus:outline-none focus:ring-1 focus:ring-indigo-500"
                  >
                    <option value="owner">Owner</option>
                    <option value="admin">Admin</option>
                    <option value="member">Member</option>
                    <option value="viewer">Viewer</option>
                  </select>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
