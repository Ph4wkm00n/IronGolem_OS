// timeline.jsx — Placeholder for @irongolem/ui Timeline.
// Primary explanation surface. Renders the full event vocabulary:
// taken / proposed / blocked / healed / quarantined / research-update / squad-handoff
//
// Variants exposed via props (driven by Tweaks):
//   - layoutVariant: "list" | "grouped" | "rail"
//   - permStyle:     "inline" | "popover" | "hidden"  (the dedicated rail
//                     is rendered by the parent route, not this component)
//   - trustStyle:    "chip" | "banner" | "inline"
//   - whyStyle:      "link" | "expand" | "drawer"
//   - density:       "compact" | "cozy" | "comfortable"

const { useState, useMemo } = React;

function Timeline({
  events, teams, statusMeta,
  layoutVariant = "list",
  permStyle = "inline",
  trustStyle = "chip",
  whyStyle = "link",
  onApprove, onDeny, onOpenDrawer,
}) {
  if (events.length === 0) return <TimelineEmpty />;

  if (layoutVariant === "grouped") {
    const groups = groupByStatus(events, statusMeta);
    return (
      <div className="space-y-6">
        {groups.map((g) => (
          <div key={g.name}>
            <GroupHeader name={g.name} count={g.items.length} tone={g.tone} />
            <div className="card overflow-hidden divide-y divide-neutral-100">
              {g.items.map((ev) => (
                <EventRow key={ev.id} event={ev} team={teams.find((t) => t.id === ev.teamId)}
                          statusMeta={statusMeta} permStyle={permStyle}
                          trustStyle={trustStyle} whyStyle={whyStyle}
                          onApprove={onApprove} onDeny={onDeny}
                          onOpenDrawer={onOpenDrawer} />
              ))}
            </div>
          </div>
        ))}
      </div>
    );
  }

  if (layoutVariant === "rail") {
    // Vertical timeline w/ continuous left rail, time stamps offset
    return (
      <div className="relative pl-8">
        <div className="absolute left-3 top-2 bottom-2 w-px bg-neutral-200" />
        <div className="space-y-1">
          {events.map((ev) => (
            <RailEntry key={ev.id} event={ev} team={teams.find((t) => t.id === ev.teamId)}
                       statusMeta={statusMeta} permStyle={permStyle}
                       trustStyle={trustStyle} whyStyle={whyStyle}
                       onApprove={onApprove} onDeny={onDeny}
                       onOpenDrawer={onOpenDrawer} />
          ))}
        </div>
      </div>
    );
  }

  // Default: flat list
  return (
    <div className="card overflow-hidden divide-y divide-neutral-100">
      {events.map((ev) => (
        <EventRow key={ev.id} event={ev} team={teams.find((t) => t.id === ev.teamId)}
                  statusMeta={statusMeta} permStyle={permStyle}
                  trustStyle={trustStyle} whyStyle={whyStyle}
                  onApprove={onApprove} onDeny={onDeny}
                  onOpenDrawer={onOpenDrawer} />
      ))}
    </div>
  );
}

function GroupHeader({ name, count, tone }) {
  return (
    <div className="flex items-center gap-2 mb-2 px-1">
      <span className={`h-1.5 w-1.5 rounded-full bg-${tone}-solid`} />
      <h3 className="text-xs font-medium uppercase tracking-wide text-neutral-500">
        {name}
      </h3>
      <span className="text-xs text-neutral-400 font-mono">{count}</span>
    </div>
  );
}

function groupByStatus(events, statusMeta) {
  const buckets = {};
  events.forEach((e) => {
    const g = statusMeta[e.status].group;
    (buckets[g] = buckets[g] || []).push(e);
  });
  const order = ["Needs attention", "Auto-healed", "Recent activity"];
  return order
    .filter((n) => buckets[n])
    .map((name) => {
      const items = buckets[name];
      const tone = name === "Needs attention" ? "warning"
                 : name === "Auto-healed"   ? "recovered"
                 :                            "neutral";
      return { name, items, tone };
    });
}

// Status mark — colored dot + icon for each event status
function StatusMark({ status, statusMeta, size = 32 }) {
  const meta = statusMeta[status];
  const iconMap = {
    proposed: "Bell", blocked: "ShieldOff", quarantined: "Lock",
    taken: "Check", healed: "Refresh",
    "research-update": "Sparkles", "squad-handoff": "ArrowRight",
  };
  const I = Icon[iconMap[status]] || Icon.Dot;
  return (
    <span className={cls(
      "inline-flex items-center justify-center rounded-full shrink-0",
      `bg-${meta.color}`, `text-${meta.color}`,
    )} style={{ width: size, height: size }}>
      <I size={Math.round(size * 0.5)} />
    </span>
  );
}

function StatusChip({ status, statusMeta }) {
  const meta = statusMeta[status];
  return (
    <span className={cls(
      "inline-flex items-center gap-1 rounded-full border text-[10px] font-medium uppercase tracking-wide px-1.5 py-0.5",
      `bg-${meta.color}`, `text-${meta.color}`, `border-${meta.color}`,
    )}>
      {meta.label}
    </span>
  );
}

function TeamPill({ team }) {
  if (!team) return null;
  return (
    <span className="inline-flex items-center gap-1.5 text-xs text-neutral-600">
      <span className={cls(
        "h-1.5 w-1.5 rounded-full",
        `bg-${team.color}-solid`,
      )} />
      {team.name}
    </span>
  );
}

function relTime(min) {
  if (min < 1) return "just now";
  if (min < 60) return `${min}m ago`;
  const h = Math.floor(min / 60);
  if (h < 24) return `${h}h ago`;
  return `${Math.floor(h / 24)}d ago`;
}

// ─── EventRow ──────────────────────────────────────────────────────────────
// The primary unit. Used in flat-list AND grouped layouts.
function EventRow({
  event, team, statusMeta, permStyle, trustStyle, whyStyle,
  onApprove, onDeny, onOpenDrawer,
}) {
  const [expanded, setExpanded] = useState(false);
  const meta = statusMeta[event.status];
  const isProposed = event.status === "proposed";
  const isBlockedOrQuarantined = event.status === "blocked" || event.status === "quarantined";
  const onWhy = () => {
    if (whyStyle === "expand") setExpanded((v) => !v);
    else onOpenDrawer(event); // "link" and "drawer" both open the detail drawer
  };

  return (
    <div className="ig-event flex gap-3 hover:bg-neutral-50 transition-colors group"
         data-status={event.status}>
      <StatusMark status={event.status} statusMeta={statusMeta} size={28} />

      <div className="flex-1 min-w-0">
        {/* Title row */}
        <div className="flex items-start gap-2">
          <div className="flex-1 min-w-0">
            <div className="ig-event-title font-medium text-neutral-900 leading-snug">
              {event.title}
            </div>

            {/* Cause line — required for blocked / quarantined */}
            {isBlockedOrQuarantined && event.cause && (
              <div className={cls(
                "ig-event-meta mt-1 inline-flex items-center gap-1.5 rounded-md px-2 py-1",
                event.status === "blocked"
                  ? "bg-blocked text-blocked"
                  : "bg-quarantined text-quarantined",
              )}>
                <Icon.AlertTriangle size={12} />
                <span>{event.cause}</span>
              </div>
            )}

            {/* Meta row */}
            <div className="ig-event-meta mt-1.5 flex flex-wrap items-center gap-x-3 gap-y-1 text-neutral-500">
              <TeamPill team={team} />
              {trustStyle === "chip" && <StatusChip status={event.status} statusMeta={statusMeta} />}
              <RiskBadge risk={event.risk} />
              {permStyle === "inline" && (
                <PermissionBadge permission={event.permission}
                                 scope={event.permissionScope}
                                 trustStyle="chip" />
              )}
              {permStyle === "popover" && (
                <PermissionHoverTrigger permission={event.permission}
                                        scope={event.permissionScope}
                                        approvals={event.approvals} />
              )}
              <span className="inline-flex items-center gap-1">
                <Icon.Clock size={11} />
                {relTime(event.minutesAgo)}
              </span>
              {event.target && (
                <span className="font-mono text-[11px] text-neutral-400 truncate max-w-[260px]">
                  → {event.target}
                </span>
              )}
            </div>

            {/* Trust inline */}
            {trustStyle === "inline" && (
              <div className="ig-event-meta mt-1.5 text-neutral-500">
                {event.approvals != null && event.approvals > 0 ? (
                  <>You've approved this kind of action <span className="text-neutral-700 font-medium">{event.approvals.toLocaleString()}× before</span>.</>
                ) : (
                  <>First time this exact action has come up.</>
                )}
              </div>
            )}

            {/* Why — inline-expand variant */}
            {whyStyle === "expand" && expanded && (
              <div className="mt-2 rounded-md bg-neutral-50 border border-neutral-200 p-3 text-sm text-neutral-700">
                <div className="text-xs font-medium uppercase tracking-wide text-neutral-500 mb-1">
                  Why this happened
                </div>
                {event.why}
                {event.approvals != null && (
                  <div className="mt-2 text-xs text-neutral-500">
                    Past approvals for this kind of action: <span className="font-mono">{event.approvals.toLocaleString()}</span>
                  </div>
                )}
              </div>
            )}
          </div>

          {/* Right gutter — Why link + approve/deny */}
          <div className="flex flex-col items-end gap-2 shrink-0">
            {whyStyle !== "expand" && (
              <button
                type="button"
                onClick={onWhy}
                className="text-xs text-accent hover:text-accent-solid font-medium opacity-0 group-hover:opacity-100 focus-visible:opacity-100 transition-opacity inline-flex items-center gap-1"
              >
                Why?
                <Icon.ArrowRight size={11} />
              </button>
            )}
            {whyStyle === "expand" && (
              <button
                type="button"
                onClick={onWhy}
                className="text-xs text-accent hover:text-accent-solid font-medium opacity-0 group-hover:opacity-100 focus-visible:opacity-100 transition-opacity inline-flex items-center gap-1"
              >
                {expanded ? "Hide why" : "Why?"}
                {expanded ? <Icon.ChevronUp size={11} /> : <Icon.ChevronDown size={11} />}
              </button>
            )}
            {isProposed && (
              <div className="flex items-center gap-1.5">
                <button
                  type="button"
                  onClick={() => onDeny(event)}
                  className="text-xs font-medium px-2.5 py-1.5 rounded-md border border-neutral-200 text-neutral-700 hover:bg-neutral-50 transition-colors"
                >
                  Deny
                </button>
                <button
                  type="button"
                  onClick={() => onApprove(event)}
                  className="text-xs font-medium px-2.5 py-1.5 rounded-md bg-accent-solid text-white hover:bg-accent-solid-hover transition-colors inline-flex items-center gap-1"
                >
                  <Icon.Check size={12} />
                  Approve
                </button>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}

// ─── RailEntry ─────────────────────────────────────────────────────────────
// Timeline-rail layout — vertical line with status nodes
function RailEntry(props) {
  const { event, statusMeta } = props;
  const meta = statusMeta[event.status];
  return (
    <div className="relative pb-3">
      {/* node */}
      <span className={cls(
        "absolute -left-[18px] top-3 inline-flex items-center justify-center rounded-full bg-white",
        "border-2", `border-${meta.color}-solid`,
      )} style={{ width: 14, height: 14 }}>
        <span className={cls("rounded-full", `bg-${meta.color}-solid`)} style={{ width: 6, height: 6 }} />
      </span>
      <div className="card-padded">
        <EventRow {...props} />
      </div>
    </div>
  );
}

// ─── PermissionHoverTrigger ────────────────────────────────────────────────
function PermissionHoverTrigger({ permission, scope, approvals }) {
  const [open, setOpen] = useState(false);
  return (
    <span className="relative inline-flex"
          onMouseEnter={() => setOpen(true)}
          onMouseLeave={() => setOpen(false)}
          onFocus={() => setOpen(true)}
          onBlur={() => setOpen(false)}>
      <button type="button"
              className="inline-flex items-center gap-1 rounded-md border border-neutral-200 bg-white px-1.5 py-0.5 text-[11px] font-medium text-neutral-600 hover:bg-neutral-50 transition-colors">
        <Icon.Lock size={11} />
        <span className="font-mono lowercase">{permission}</span>
      </button>
      {open && (
        <div className="absolute left-0 top-full mt-1 z-20 w-64 rounded-lg border border-neutral-200 bg-white shadow-lg p-3">
          <div className="text-xs font-medium uppercase tracking-wide text-neutral-500 mb-1">
            Permission used
          </div>
          <div className="text-sm font-mono text-neutral-900">{permission}</div>
          <div className="mt-2 text-xs text-neutral-600">
            Scope: <span className="font-medium">{scope}</span>
          </div>
          {approvals != null && (
            <div className="mt-1 text-xs text-neutral-500">
              Approved {approvals.toLocaleString()}× before in this workspace.
            </div>
          )}
        </div>
      )}
    </span>
  );
}

// ─── Empty state ───────────────────────────────────────────────────────────
function TimelineEmpty() {
  return (
    <div className="card card-padded">
      <div className="flex flex-col items-center text-center py-10 max-w-md mx-auto">
        <div className="h-12 w-12 rounded-full bg-safe inline-flex items-center justify-center mb-4">
          <Icon.CheckCircle size={24} className="text-safe" />
        </div>
        <h3 className="section-title">Nothing needs your attention</h3>
        <p className="text-sm text-neutral-500 mt-2">
          Your assistant teams are working inside their charters. New activity will show up here as it happens.
        </p>
        <div className="mt-4 inline-flex items-center gap-1.5 text-xs text-neutral-400">
          <span className="h-1.5 w-1.5 rounded-full bg-safe-solid ig-pulse" />
          Heartbeat green for {17} days
        </div>
      </div>
    </div>
  );
}

window.Timeline = Timeline;
