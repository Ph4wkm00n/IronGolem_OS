// workspace-dashboard.jsx
// Route: /workspace
// Purpose: Landing page for an IronGolem workspace — heartbeat and recent activity.
//
// Per house style, this file is the ENTIRE route. Mock data lives in
// mock-data.jsx (loaded globally as `MOCK`). Reusable patterns
// (HeartbeatStatus, PolicyCard, ResearchCard, RiskBadge, SafetyCard,
// Timeline) are placeholder shapes imported from window — the integrator
// will swap them for the real @irongolem/ui imports.

const { useState, useReducer, useMemo, useEffect } = React;

// ─── State reducer ─────────────────────────────────────────────────────────
// Drives the simulated transitions: approve → moves the event to "taken",
// deny → moves it to "blocked" with cause text.
function eventsReducer(state, action) {
  switch (action.type) {
    case "approve":
      return state.map((e) =>
        e.id === action.id
          ? { ...e, status: "taken", minutesAgo: 0,
              why: "You approved this just now.",
              cause: undefined,
              _justChanged: true }
          : { ...e, _justChanged: false }
      );
    case "deny":
      return state.map((e) =>
        e.id === action.id
          ? { ...e, status: "blocked", minutesAgo: 0,
              cause: "You denied this just now.",
              why: "You held this action. Nothing was sent.",
              _justChanged: true }
          : { ...e, _justChanged: false }
      );
    case "reset":
      return MOCK.EVENTS_RAW.map((e) => ({ ...e }));
    default:
      return state;
  }
}

// ─── Filters ───────────────────────────────────────────────────────────────
const FILTER_CHIPS = [
  { id: "all",       label: "All activity" },
  { id: "attention", label: "Needs attention" },
  { id: "proposed",  label: "Awaiting approval" },
  { id: "blocked",   label: "Blocked" },
  { id: "healed",    label: "Auto-healed" },
];

function applyFilter(events, filterId, statusMeta, dataLimit) {
  let filtered = events;
  if (filterId === "attention") filtered = events.filter((e) => statusMeta[e.status].needsAttn);
  else if (filterId === "proposed") filtered = events.filter((e) => e.status === "proposed");
  else if (filterId === "blocked")  filtered = events.filter((e) => e.status === "blocked");
  else if (filterId === "healed")   filtered = events.filter((e) => e.status === "healed" || e.status === "recovered");
  return filtered.slice(0, dataLimit);
}

// ─── Main route component ──────────────────────────────────────────────────
function WorkspaceDashboard({ tweaks }) {
  const { HeartbeatStatus, PolicyCard, RiskBadge, SafetyCard, ResearchCard } = window.UI;
  const [events, dispatch] = useReducer(eventsReducer, null,
    () => MOCK.EVENTS_RAW.map((e) => ({ ...e })));
  const [filter, setFilter] = useState("all");
  const [drawerEvent, setDrawerEvent] = useState(null);
  const [recentToast, setRecentToast] = useState(null);
  const [policyOpen, setPolicyOpen] = useState(false);

  // Apply tweak: empty state preview overrides everything
  const effectiveEvents = tweaks.showEmpty
    ? []
    : applyFilter(events, filter, MOCK.STATUS_META, tweaks.dataVolume);

  // Compute summary counts from the *current* event set (so transitions tick)
  const counts = useMemo(() => {
    const c = { proposed: 0, blocked: 0, quarantined: 0, healed: 0, taken: 0 };
    events.forEach((e) => { c[e.status] = (c[e.status] || 0) + 1; });
    return c;
  }, [events]);

  const onApprove = (ev) => {
    dispatch({ type: "approve", id: ev.id });
    setRecentToast({ kind: "approved", title: ev.title });
  };
  const onDeny = (ev) => {
    dispatch({ type: "deny", id: ev.id });
    setRecentToast({ kind: "denied", title: ev.title });
  };
  const onOpenDrawer = (ev) => setDrawerEvent(ev);
  const onCloseDrawer = () => setDrawerEvent(null);

  // Toast auto-dismiss
  useEffect(() => {
    if (!recentToast) return undefined;
    const id = setTimeout(() => setRecentToast(null), 3200);
    return () => clearTimeout(id);
  }, [recentToast]);

  const densityClass =
    tweaks.density === "compact"     ? "ig-density-compact"
    : tweaks.density === "comfortable" ? "ig-density-comfortable"
    :                                    "ig-density-cozy";

  return (
    <div className={cls("min-h-screen", densityClass)}>
      <DashHeader workspace={MOCK.WORKSPACE} heartbeat={MOCK.HEARTBEAT}
                  onResetDemo={() => dispatch({ type: "reset" })} />

      <main className="page-container">
        {/* Trust banner — only when trustStyle === "banner" */}
        {tweaks.trustStyle === "banner" && (
          <TrustBanner counts={counts}
                       safety={MOCK.SAFETY}
                       onOpenPolicy={() => setPolicyOpen(true)} />
        )}

        {/* Header band: greeting + heartbeat */}
        <section className="grid grid-cols-1 lg:grid-cols-3 gap-4 mt-4">
          <div className="lg:col-span-2">
            <h1 className="page-title">Good morning, Adam</h1>
            <p className="text-neutral-600 mt-1">
              Here's what your assistant teams did overnight, what's waiting for you,
              and anything they couldn't handle on their own.
            </p>

            {/* Quick stats grid */}
            <div className="mt-5 grid grid-cols-2 md:grid-cols-4 gap-3">
              <Kpi label="Done overnight" tone="safe"        value={counts.taken}
                   icon="Check"
                   hint="Inside the rules. Nothing for you." />
              <Kpi label="Waiting on you" tone="warning"     value={counts.proposed}
                   icon="Bell"
                   hint="Drafted, paused for approval." />
              <Kpi label="Blocked"       tone="blocked"     value={counts.blocked}
                   icon="ShieldOff"
                   hint="Held by a safety rule." />
              <Kpi label="Auto-healed"   tone="recovered"   value={counts.healed}
                   icon="Refresh"
                   hint="Fixed on its own." />
            </div>
          </div>

          <div>
            <HeartbeatStatus heartbeat={MOCK.HEARTBEAT}
                             workspace={MOCK.WORKSPACE}
                             dense={tweaks.density === "compact"}
                             onOpenWhy={() => setPolicyOpen(true)} />
          </div>
        </section>

        {/* Recent activity — primary surface */}
        <section className="mt-8">
          <div className="flex flex-wrap items-end justify-between gap-3">
            <div>
              <h2 className="section-title">Recent activity</h2>
              <p className="text-sm text-neutral-500 mt-1">
                Every blocked or quarantined event includes the cause.
              </p>
            </div>

            <FilterChips active={filter} onChange={setFilter} counts={counts} />
          </div>

          {/* Layout: main timeline column + permissions rail (when chosen) */}
          <div className={cls(
            "mt-4 grid gap-6",
            tweaks.permStyle === "rail" ? "grid-cols-1 lg:grid-cols-[1fr_320px]" : "grid-cols-1",
          )}>
            <div>
              <Timeline
                events={effectiveEvents}
                teams={MOCK.TEAMS}
                statusMeta={MOCK.STATUS_META}
                layoutVariant={tweaks.layoutVariant}
                permStyle={tweaks.permStyle === "rail" ? "hidden" : tweaks.permStyle}
                trustStyle={tweaks.trustStyle === "banner" ? "chip" : tweaks.trustStyle}
                whyStyle={tweaks.whyStyle}
                onApprove={onApprove}
                onDeny={onDeny}
                onOpenDrawer={onOpenDrawer}
              />
            </div>

            {tweaks.permStyle === "rail" && (
              <PermissionsRail events={effectiveEvents} teams={MOCK.TEAMS} />
            )}
          </div>
        </section>

        {/* Bottom row: assistant teams + research findings */}
        <section className="mt-8 grid grid-cols-1 lg:grid-cols-3 gap-4">
          <div className="lg:col-span-2">
            <h2 className="section-title">Assistant teams</h2>
            <p className="text-sm text-neutral-500 mt-1">
              Last 24 hours of reliability per team.
            </p>
            <div className="mt-3 grid grid-cols-1 md:grid-cols-2 gap-3">
              {MOCK.TEAMS.map((t) => (
                <TeamCard key={t.id} team={t}
                          activity={events.filter((e) => e.teamId === t.id).length}
                          history={MOCK.TRUST_HISTORY[t.id]} />
              ))}
            </div>
          </div>

          <div>
            <div className="flex items-end justify-between">
              <div>
                <h2 className="section-title">Research</h2>
                <p className="text-sm text-neutral-500 mt-1">
                  Things you should probably know about today.
                </p>
              </div>
              <a href="#research" className="text-sm text-accent hover:text-accent-solid font-medium">
                All findings
              </a>
            </div>
            <div className="mt-3 space-y-3">
              {MOCK.RESEARCH_FINDINGS.map((f) => (
                <ResearchCard key={f.id} finding={f} />
              ))}
            </div>
          </div>
        </section>

        {/* Safety + Five-layer policy */}
        <section className="mt-8 grid grid-cols-1 lg:grid-cols-3 gap-4 pb-12">
          <div className="lg:col-span-2">
            <SafetyCard safety={MOCK.SAFETY} />
          </div>
          <div>
            <PolicyCard safety={MOCK.SAFETY} onOpenRules={() => setPolicyOpen(true)} />
          </div>
        </section>
      </main>

      {/* Drawer — when whyStyle === "drawer" */}
      {drawerEvent && (
        <WhyDrawer event={drawerEvent}
                   team={MOCK.TEAMS.find((t) => t.id === drawerEvent.teamId)}
                   onClose={onCloseDrawer} />
      )}

      {policyOpen && (
        <SafetyDrawer safety={MOCK.SAFETY} onClose={() => setPolicyOpen(false)} />
      )}

      {/* Toast */}
      {recentToast && <Toast toast={recentToast} />}
    </div>
  );
}

// ─── DashHeader (topbar) ───────────────────────────────────────────────────
function DashHeader({ workspace, heartbeat, onResetDemo }) {
  const isHealthy = heartbeat.status === "healthy";
  return (
    <header className="sticky top-0 z-30 bg-white border-b border-neutral-100"
            style={{ backdropFilter: "saturate(160%) blur(8px)" }}>
      <div className="page-container py-0">
        <div className="flex items-center h-14 gap-4">
          <div className="flex items-center gap-2 shrink-0">
            <span className="h-7 w-7 rounded-md bg-neutral-900 text-white inline-flex items-center justify-center">
              <Icon.Logo size={16} />
            </span>
            <span className="text-sm font-semibold tracking-tight text-neutral-900">
              IronGolem
            </span>
            <span className="text-neutral-300">/</span>
            <span className="text-sm text-neutral-700 inline-flex items-center gap-1.5">
              <span className="h-5 w-5 rounded bg-accent text-accent inline-flex items-center justify-center text-[10px] font-semibold">
                {workspace.initials}
              </span>
              {workspace.name}
            </span>
            <span className="text-[11px] font-mono text-neutral-400 hidden md:inline">{workspace.region}</span>
          </div>

          <nav className="hidden md:flex items-center gap-1 ml-4">
            {[
              ["Workspace", true],
              ["Inbox", false],
              ["Timeline", false],
              ["Teams", false],
              ["Research", false],
              ["Rules", false],
            ].map(([label, active]) => (
              <a key={label} href={`#${label.toLowerCase()}`}
                 className={cls(
                   "px-2.5 py-1.5 rounded-md text-sm font-medium transition-colors",
                   active ? "text-neutral-900 bg-neutral-100" : "text-neutral-600 hover:text-neutral-900 hover:bg-neutral-50",
                 )}>
                {label}
              </a>
            ))}
          </nav>

          <div className="flex-1" />

          <div className="hidden md:flex items-center gap-2 text-xs text-neutral-500">
            <span className="inline-flex items-center gap-1.5">
              <span className={cls(
                "h-1.5 w-1.5 rounded-full",
                isHealthy ? "bg-safe-solid ig-pulse" : "bg-warning-solid",
              )} />
              {isHealthy ? "All systems normal" : "1 degraded"}
            </span>
            <span className="text-neutral-300">·</span>
            <span>last sync {workspace.lastSync}</span>
          </div>

          <button type="button"
                  onClick={onResetDemo}
                  title="Reset mock data"
                  className="inline-flex items-center gap-1.5 text-xs font-medium px-2.5 py-1.5 rounded-md text-neutral-600 hover:text-neutral-900 hover:bg-neutral-50 transition-colors">
            <Icon.Refresh size={12} />
            <span className="hidden sm:inline">Reset</span>
          </button>

          <button type="button"
                  className="inline-flex items-center gap-1.5 text-xs font-medium px-2.5 py-1.5 rounded-md text-neutral-600 hover:text-neutral-900 hover:bg-neutral-50 transition-colors">
            <Icon.Bell size={14} />
          </button>

          <span className="h-7 w-7 rounded-full bg-accent text-accent inline-flex items-center justify-center text-[11px] font-semibold">
            AS
          </span>
        </div>
      </div>
    </header>
  );
}

// ─── Kpi card ─────────────────────────────────────────────────────────────
function Kpi({ label, value, tone, icon, hint }) {
  const I = Icon[icon];
  return (
    <div className={cls("card card-padded relative overflow-hidden")}>
      <div className="flex items-center justify-between">
        <span className="text-[10px] font-medium uppercase tracking-wider text-neutral-500">
          {label}
        </span>
        <span className={cls(`text-${tone}`)}>
          <I size={14} />
        </span>
      </div>
      <div className={cls("mt-2 text-2xl font-semibold tracking-tight tabular-nums", `text-${tone}`)}>
        {value}
      </div>
      <div className="text-xs text-neutral-500 mt-1 leading-snug">{hint}</div>
    </div>
  );
}

// ─── Filter chips ─────────────────────────────────────────────────────────
function FilterChips({ active, onChange, counts }) {
  const countFor = (id) => {
    if (id === "all") return null;
    if (id === "attention") return (counts.proposed || 0) + (counts.blocked || 0) + (counts.quarantined || 0);
    return counts[id] || 0;
  };
  return (
    <div className="inline-flex items-center rounded-lg border border-neutral-200 bg-white p-0.5">
      {FILTER_CHIPS.map((c) => {
        const isActive = c.id === active;
        const n = countFor(c.id);
        return (
          <button key={c.id} type="button"
                  onClick={() => onChange(c.id)}
                  className={cls(
                    "inline-flex items-center gap-1.5 rounded-md text-xs font-medium px-2.5 py-1.5 transition-colors",
                    isActive ? "bg-neutral-100 text-neutral-900" : "text-neutral-600 hover:text-neutral-900",
                  )}>
            {c.label}
            {n != null && (
              <span className={cls(
                "rounded-full font-mono text-[10px] px-1.5 py-0.5",
                isActive ? "bg-white border border-neutral-200 text-neutral-700" : "bg-neutral-100 text-neutral-500",
              )}>{n}</span>
            )}
          </button>
        );
      })}
    </div>
  );
}

// ─── TrustBanner ───────────────────────────────────────────────────────────
function TrustBanner({ counts, safety, onOpenPolicy }) {
  const watching = safety.layers.filter((l) => l.state !== "ok").length;
  return (
    <div className="mt-4 rounded-xl border border-accent bg-accent overflow-hidden">
      <div className="p-4 sm:p-5 flex flex-wrap items-center gap-4">
        <div className="flex items-center gap-3">
          <span className="h-9 w-9 rounded-lg bg-white inline-flex items-center justify-center text-accent shadow-sm">
            <Icon.Shield size={18} />
          </span>
          <div>
            <div className="text-xs font-medium uppercase tracking-wide text-accent">
              Trust posture
            </div>
            <div className="text-sm font-semibold tracking-tight text-neutral-900">
              All 5 safety layers active.{" "}
              {watching > 0
                ? `${watching} layer is watching a high-risk action.`
                : "Everything within charter."}
            </div>
          </div>
        </div>
        <div className="flex-1" />
        <div className="flex items-center gap-3 text-xs text-neutral-600">
          <span><span className="font-semibold text-neutral-900 tabular-nums">{counts.taken}</span> done</span>
          <span className="text-neutral-300">·</span>
          <span><span className="font-semibold text-warning tabular-nums">{counts.proposed}</span> waiting on you</span>
          <span className="text-neutral-300">·</span>
          <span><span className="font-semibold text-blocked tabular-nums">{counts.blocked}</span> blocked</span>
        </div>
        <button type="button"
                onClick={onOpenPolicy}
                className="inline-flex items-center gap-1.5 text-xs font-medium px-3 py-1.5 rounded-md bg-white border border-accent text-accent hover:bg-accent-hover transition-colors">
          Open rules
          <Icon.ArrowRight size={12} />
        </button>
      </div>
    </div>
  );
}

// ─── PermissionsRail ───────────────────────────────────────────────────────
// When permStyle === "rail", a dedicated right-column rail explains
// what permissions are in use right now.
function PermissionsRail({ events, teams }) {
  const grouped = useMemo(() => {
    const m = {};
    events.forEach((e) => {
      if (!m[e.permission]) m[e.permission] = { count: 0, scope: e.permissionScope, teamIds: new Set() };
      m[e.permission].count += 1;
      m[e.permission].teamIds.add(e.teamId);
    });
    return Object.entries(m).sort((a, b) => b[1].count - a[1].count).slice(0, 8);
  }, [events]);

  return (
    <aside className="card card-padded h-fit sticky top-20">
      <div className="flex items-center justify-between">
        <h3 className="section-title">In use right now</h3>
      </div>
      <p className="text-xs text-neutral-500 mt-1">
        Every permission that touched this workspace in the events shown.
      </p>
      <div className="mt-3 space-y-1.5">
        {grouped.length === 0 && (
          <div className="text-xs text-neutral-500 py-4">No permissions in use.</div>
        )}
        {grouped.map(([perm, info]) => (
          <RailPermRow key={perm} perm={perm} info={info} teams={teams} />
        ))}
      </div>
      <div className="mt-4 pt-3 border-t border-neutral-100 text-xs text-neutral-500 leading-relaxed">
        Anything not on this list is not currently allowed.
        <a href="#rules" className="text-accent hover:text-accent-solid font-medium ml-1">
          What else could be added?
        </a>
      </div>
    </aside>
  );
}

function RailPermRow({ perm, info, teams }) {
  const scopeTone = info.scope === "restricted" ? "blocked"
                  : info.scope === "broad"      ? "warning"
                  :                                "neutral";
  return (
    <div className="flex items-center gap-2 rounded-md px-2 py-2 hover:bg-neutral-50 transition-colors">
      <span className={cls(
        "h-6 w-6 rounded-md inline-flex items-center justify-center",
        `bg-${scopeTone}`, `text-${scopeTone}`,
      )}>
        <Icon.Lock size={11} />
      </span>
      <div className="flex-1 min-w-0">
        <div className="font-mono text-[12px] text-neutral-900 truncate">{perm}</div>
        <div className="text-[10px] text-neutral-500">
          {info.scope} · used by {info.teamIds.size} team{info.teamIds.size === 1 ? "" : "s"}
        </div>
      </div>
      <span className="text-xs font-mono text-neutral-400 tabular-nums">{info.count}×</span>
    </div>
  );
}

// ─── TeamCard ──────────────────────────────────────────────────────────────
function TeamCard({ team, activity, history }) {
  const score = useMemo(() => {
    if (!history) return 9;
    return Math.round((history.reduce((s, n) => s + n, 0) / history.length) * 10) / 10;
  }, [history]);
  const scoreTone = score >= 8.9 ? "safe" : score >= 8 ? "warning" : "blocked";

  return (
    <div className="card card-padded">
      <div className="flex items-start justify-between gap-3">
        <div className="flex items-start gap-3 min-w-0">
          <span className={cls(
            "h-8 w-8 rounded-lg shrink-0 inline-flex items-center justify-center font-semibold text-[11px]",
            `bg-${team.color}`, `text-${team.color}`,
          )}>
            {team.name.split(" ").map((w) => w[0]).join("").slice(0, 2)}
          </span>
          <div className="min-w-0">
            <div className="text-sm font-semibold text-neutral-900 tracking-tight">{team.name}</div>
            <div className="text-xs text-neutral-500 mt-0.5 truncate">{team.description}</div>
          </div>
        </div>
        <span className={cls(
          "shrink-0 inline-flex items-center gap-1 rounded-full text-[10px] font-medium px-1.5 py-0.5 border",
          `bg-${scoreTone}`, `text-${scoreTone}`, `border-${scoreTone}`,
        )}>
          {score.toFixed(1)} / 10
        </span>
      </div>

      {/* 24h trust sparkline */}
      <div className="mt-3">
        <div className="flex items-end gap-[2px] h-7">
          {history.map((v, i) => {
            const tone = v >= 9 ? "safe" : v >= 7 ? "warning" : "blocked";
            return (
              <span key={i} className={cls("flex-1 rounded-sm", `bg-${tone}-solid`)}
                    style={{ height: `${(v / 10) * 100}%`, opacity: v >= 9 ? 0.55 : 0.85 }} />
            );
          })}
        </div>
        <div className="flex justify-between text-[10px] text-neutral-400 mt-1 font-mono">
          <span>24h ago</span>
          <span>{activity} events</span>
          <span>now</span>
        </div>
      </div>
    </div>
  );
}

// ─── WhyDrawer ─────────────────────────────────────────────────────────────
function WhyDrawer({ event, team, onClose }) {
  const meta = MOCK.STATUS_META[event.status];
  return (
    <div className="fixed inset-0 z-40">
      <div className="absolute inset-0 ig-drawer-backdrop" onClick={onClose} />
      <aside className="absolute right-0 top-0 bottom-0 w-full sm:w-[440px] bg-white shadow-xl border-l border-neutral-100 overflow-y-auto scrollbar-thin"
             style={{ animation: "slide-in 220ms ease-out" }}>
        <div className="sticky top-0 bg-white border-b border-neutral-100 px-5 py-4 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <StatusMarkSmall status={event.status} />
            <span className={cls(
              "text-xs font-medium uppercase tracking-wide",
              `text-${meta.color}`,
            )}>
              Why this happened
            </span>
          </div>
          <button type="button" onClick={onClose}
                  className="text-neutral-500 hover:text-neutral-900 p-1.5 rounded-md hover:bg-neutral-50">
            <Icon.XCircle size={16} />
          </button>
        </div>

        <div className="p-5 space-y-5">
          <div>
            <h2 className="section-title">{event.title}</h2>
            <div className="mt-2 flex items-center gap-3 text-xs text-neutral-500">
              <TeamPillSmall team={team} />
              <span>·</span>
              <span>{relTimeFull(event.minutesAgo)}</span>
            </div>
          </div>

          {event.cause && (
            <div className={cls(
              "rounded-lg p-4 border",
              event.status === "blocked"     ? "bg-blocked border-blocked text-blocked"
              : event.status === "quarantined" ? "bg-quarantined border-quarantined text-quarantined"
              :                                  "bg-warning border-warning text-warning",
            )}>
              <div className="text-xs font-medium uppercase tracking-wide mb-1">Cause</div>
              <div className="text-sm">{event.cause}</div>
            </div>
          )}

          <DrawerSection label="Reason">
            <p className="text-sm text-neutral-700 leading-relaxed">{event.why}</p>
          </DrawerSection>

          <DrawerSection label="Permission used">
            <div className="rounded-md border border-neutral-200 p-3">
              <div className="font-mono text-sm text-neutral-900">{event.permission}</div>
              <div className="mt-1 flex items-center gap-2 text-xs text-neutral-500">
                <span className="capitalize">{event.permissionScope}</span>
                <span>·</span>
                <RiskBadgeSmall risk={event.risk} />
              </div>
              {event.approvals != null && event.approvals > 0 && (
                <div className="mt-2 text-xs text-neutral-600">
                  Approved <span className="font-medium tabular-nums">{event.approvals.toLocaleString()}×</span> before in this workspace.
                </div>
              )}
              {event.approvals === 0 && (
                <div className="mt-2 text-xs text-warning">
                  First time this exact action has come up.
                </div>
              )}
            </div>
          </DrawerSection>

          {event.target && (
            <DrawerSection label="Target">
              <div className="font-mono text-sm text-neutral-700">{event.target}</div>
            </DrawerSection>
          )}

          <DrawerSection label="What happens next">
            <ul className="text-sm text-neutral-700 space-y-1.5">
              {nextStepsFor(event).map((s, i) => (
                <li key={i} className="flex gap-2">
                  <span className="text-neutral-300 mt-1">·</span>
                  <span>{s}</span>
                </li>
              ))}
            </ul>
          </DrawerSection>
        </div>
      </aside>
      <style>{`@keyframes slide-in { from { transform: translateX(20px); opacity: 0; } to { transform: translateX(0); opacity: 1; } }`}</style>
    </div>
  );
}

function StatusMarkSmall({ status }) {
  const meta = MOCK.STATUS_META[status];
  return <span className={cls("h-2 w-2 rounded-full", `bg-${meta.color}-solid`)} />;
}
function TeamPillSmall({ team }) {
  if (!team) return null;
  return (
    <span className="inline-flex items-center gap-1.5">
      <span className={cls("h-1.5 w-1.5 rounded-full", `bg-${team.color}-solid`)} />
      {team.name}
    </span>
  );
}
function RiskBadgeSmall({ risk }) {
  const m = { low: "safe", medium: "warning", high: "blocked" }[risk] || "warning";
  return (
    <span className={cls(
      "inline-flex items-center gap-1 rounded-full border font-medium text-[10px] px-1.5 py-0.5",
      `bg-${m}`, `text-${m}`, `border-${m}`,
    )}>
      <span className={cls("h-1 w-1 rounded-full", `bg-${m}-solid`)} />
      {risk} risk
    </span>
  );
}
function DrawerSection({ label, children }) {
  return (
    <div>
      <div className="text-xs font-medium uppercase tracking-wide text-neutral-500 mb-2">{label}</div>
      {children}
    </div>
  );
}
function relTimeFull(min) {
  if (min < 1) return "just now";
  if (min < 60) return `${min} minute${min === 1 ? "" : "s"} ago`;
  const h = Math.floor(min / 60);
  if (h < 24) return `${h} hour${h === 1 ? "" : "s"} ago, ${min - h * 60}m ago`;
  return `${Math.floor(h / 24)} day${Math.floor(h / 24) === 1 ? "" : "s"} ago`;
}
function nextStepsFor(ev) {
  switch (ev.status) {
    case "proposed":   return ["Approve to run the action now.", "Deny to keep it from running.", "Edit the draft inline before approving."];
    case "blocked":    return ["The action did not run.", "Adjust the safety rule, or take the action yourself."];
    case "quarantined":return ["The draft is held in your inbox.", "Open it, edit, then send if it looks right."];
    case "taken":      return ["This action ran inside the rules.", "It will appear in the daily summary."];
    case "healed":     return ["Auto-heal ran on its own.", "If this kind of failure repeats, the rule will be reviewed."];
    case "research-update": return ["No action needed — context updated.", "Affected teams adjusted automatically."];
    case "squad-handoff":   return ["Work moved between teams.", "You'll see the next action in the receiving team's events."];
    default: return [];
  }
}

// ─── SafetyDrawer ──────────────────────────────────────────────────────────
function SafetyDrawer({ safety, onClose }) {
  return (
    <div className="fixed inset-0 z-40">
      <div className="absolute inset-0 ig-drawer-backdrop" onClick={onClose} />
      <aside className="absolute right-0 top-0 bottom-0 w-full sm:w-[480px] bg-white shadow-xl border-l border-neutral-100 overflow-y-auto scrollbar-thin">
        <div className="sticky top-0 bg-white border-b border-neutral-100 px-5 py-4 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Icon.Shield size={16} className="text-accent" />
            <span className="text-xs font-medium uppercase tracking-wide text-neutral-500">
              Safety rules
            </span>
          </div>
          <button type="button" onClick={onClose}
                  className="text-neutral-500 hover:text-neutral-900 p-1.5 rounded-md hover:bg-neutral-50">
            <Icon.XCircle size={16} />
          </button>
        </div>
        <div className="p-5">
          <window.UI.PolicyCard safety={safety} />
          <div className="mt-4">
            <window.UI.SafetyCard safety={safety} />
          </div>
        </div>
      </aside>
    </div>
  );
}

// ─── Toast ─────────────────────────────────────────────────────────────────
function Toast({ toast }) {
  const tone = toast.kind === "approved" ? "safe" : "blocked";
  const I = toast.kind === "approved" ? Icon.CheckCircle : Icon.XCircle;
  const verb = toast.kind === "approved" ? "Approved" : "Denied";
  return (
    <div className="fixed bottom-5 left-1/2 -translate-x-1/2 z-50 pointer-events-none"
         style={{ animation: "toast-in 200ms ease-out" }}>
      <div className={cls(
        "pointer-events-auto rounded-lg border shadow-lg bg-white px-4 py-3 flex items-center gap-3 min-w-[280px] max-w-[420px]",
        `border-${tone}`,
      )}>
        <span className={cls("text-", tone)}><I size={18} /></span>
        <div className="flex-1 min-w-0">
          <div className={cls("text-xs font-medium uppercase tracking-wide", `text-${tone}`)}>{verb}</div>
          <div className="text-sm text-neutral-900 truncate">{toast.title}</div>
        </div>
      </div>
      <style>{`@keyframes toast-in { from { transform: translate(-50%, 8px); opacity: 0; } to { transform: translate(-50%, 0); opacity: 1; } }`}</style>
    </div>
  );
}

window.WorkspaceDashboard = WorkspaceDashboard;
