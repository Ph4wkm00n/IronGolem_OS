// ui-primitives.jsx — placeholder versions of the @irongolem/ui patterns.
// The integrator will swap each one for the real import. Each component
// below is annotated with the component name it stands in for.

const cls = (...c) => c.filter(Boolean).join(" ");

// ─── HeartbeatStatus ─────────────────────────────────────────────────────
// Placeholder for @irongolem/ui HeartbeatStatus.
// Health-status card. Top-line workspace pulse.
function HeartbeatStatus({ heartbeat, workspace, dense, onOpenWhy }) {
  const isHealthy = heartbeat.status === "healthy";
  return (
    <div className={cls("card card-padded", dense && "p-3")}>
      <div className="flex items-center justify-between gap-3">
        <div className="flex items-center gap-3">
          <span className="relative inline-flex">
            <span className={cls(
              "h-2.5 w-2.5 rounded-full",
              isHealthy ? "bg-safe-solid ig-pulse" : "ig-pulse-warn",
            )} />
          </span>
          <div>
            <div className="text-xs font-medium uppercase tracking-wide text-neutral-500">
              Heartbeat
            </div>
            <div className="text-base font-semibold tracking-tight text-neutral-900 mt-0.5">
              {isHealthy ? "Everything is running normally" : "One system is recovering"}
            </div>
          </div>
        </div>
        <button
          type="button"
          onClick={onOpenWhy}
          className="text-xs text-accent hover:text-accent-solid font-medium px-2 py-1 rounded hover:bg-accent-hover transition-colors"
        >
          Why?
        </button>
      </div>

      <div className="mt-4 grid grid-cols-3 gap-3">
        <Stat
          label="Systems"
          value={`${heartbeat.systemsGreen}/${heartbeat.systemsTotal}`}
          tone={heartbeat.systemsGreen === heartbeat.systemsTotal ? "safe" : "warning"}
          hint="green / total"
        />
        <Stat
          label="Uptime"
          value={`${workspace.uptimeStreak}`}
          tone="neutral"
          hint={`${workspace.uptimeHours}h streak`}
        />
        <Stat
          label="Last sync"
          value={workspace.lastSync.replace(" ago", "")}
          tone="neutral"
          hint="ago"
        />
      </div>

      {!isHealthy && heartbeat.oneDegraded && (
        <div className="mt-3 rounded-md bg-warning border border-warning p-3 text-sm text-warning">
          <div className="font-medium">{heartbeat.oneDegraded.name}</div>
          <div className="text-warning opacity-90 mt-0.5">{heartbeat.oneDegraded.reason}</div>
        </div>
      )}
    </div>
  );
}

function Stat({ label, value, hint, tone = "neutral" }) {
  const toneText = {
    safe: "text-safe", warning: "text-warning", blocked: "text-blocked",
    recovered: "text-recovered", quarantined: "text-quarantined",
    accent: "text-accent", neutral: "text-neutral-900",
  }[tone];
  return (
    <div>
      <div className="text-[10px] font-medium uppercase tracking-wider text-neutral-500">{label}</div>
      <div className={cls("mt-1 text-lg font-semibold tracking-tight tabular-nums", toneText)}>
        {value}
      </div>
      {hint && <div className="text-[11px] text-neutral-400 mt-0.5">{hint}</div>}
    </div>
  );
}

// ─── RiskBadge ───────────────────────────────────────────────────────────
// Placeholder for @irongolem/ui RiskBadge.
function RiskBadge({ risk, size = "sm" }) {
  const map = {
    low:    { tone: "safe",    label: "low risk" },
    medium: { tone: "warning", label: "med risk" },
    high:   { tone: "blocked", label: "high risk" },
  };
  const m = map[risk] || map.medium;
  return (
    <span className={cls(
      "inline-flex items-center gap-1 rounded-full border font-medium",
      size === "sm" ? "text-[10px] px-1.5 py-0.5" : "text-xs px-2 py-0.5",
      `bg-${m.tone}`, `text-${m.tone}`, `border-${m.tone}`,
    )}>
      <span className={cls("h-1.5 w-1.5 rounded-full", `bg-${m.tone}-solid`)} />
      {m.label}
    </span>
  );
}

// ─── PermissionBadge ─────────────────────────────────────────────────────
// Visible Trust pattern. Renders the permission an action used / would use.
function PermissionBadge({ permission, scope = "scoped", trustStyle = "chip", approvals }) {
  const scopeMap = {
    scoped:     { tone: "neutral",   icon: "Lock",  label: "Scoped" },
    broad:      { tone: "warning",   icon: "Eye",   label: "Broad" },
    restricted: { tone: "blocked",   icon: "ShieldOff", label: "Restricted" },
  };
  const s = scopeMap[scope] || scopeMap.scoped;
  const I = Icon[s.icon];

  if (trustStyle === "inline") {
    return (
      <span className="text-xs text-neutral-500">
        Permission needed:{" "}
        <span className={cls("font-medium", `text-${s.tone}`)}>{permission}</span>
        {approvals != null && approvals > 0 && (
          <span className="text-neutral-400"> · approved {approvals.toLocaleString()}× before</span>
        )}
      </span>
    );
  }
  return (
    <span className={cls(
      "inline-flex items-center gap-1 rounded-md border px-1.5 py-0.5 text-[11px] font-medium",
      `bg-${s.tone}`, `text-${s.tone}`, `border-${s.tone}`,
    )}>
      <I size={11} />
      <span className="font-mono lowercase">{permission}</span>
    </span>
  );
}

// ─── PolicyCard ──────────────────────────────────────────────────────────
// Placeholder for @irongolem/ui PolicyCard — five-layer permission explainer.
function PolicyCard({ safety, compact, onOpenRules }) {
  return (
    <div className="card card-padded">
      <div className="flex items-center justify-between">
        <div>
          <div className="text-xs font-medium uppercase tracking-wide text-neutral-500">
            Safety rules
          </div>
          <h2 className="section-title mt-0.5">Five layers, all active</h2>
        </div>
        <button type="button" onClick={onOpenRules}
                className="text-sm text-accent hover:text-accent-solid font-medium">
          Open rules
        </button>
      </div>

      <ol className="mt-4 space-y-2">
        {safety.layers.map((layer) => {
          const isWatching = layer.state === "watching";
          return (
            <li key={layer.id}
                className={cls(
                  "flex items-center gap-3 rounded-lg border px-3 py-2",
                  isWatching
                    ? "bg-warning border-warning"
                    : "border-neutral-100 bg-white",
                )}>
              <span className={cls(
                "h-6 w-6 rounded-full inline-flex items-center justify-center text-[11px] font-semibold",
                isWatching ? "bg-warning-solid text-white" : "bg-safe-solid text-white",
              )}>
                {layer.id}
              </span>
              <div className="flex-1 min-w-0">
                <div className="text-sm font-medium text-neutral-900">{layer.name}</div>
                {!compact && (
                  <div className="text-xs text-neutral-500 truncate">{layer.note}</div>
                )}
              </div>
              {isWatching
                ? <Icon.AlertTriangle className="text-warning" size={16} />
                : <Icon.CheckCircle  className="text-safe" size={16} />}
            </li>
          );
        })}
      </ol>
    </div>
  );
}

// ─── SafetyCard ──────────────────────────────────────────────────────────
// Placeholder for @irongolem/ui SafetyCard — can / cannot / needs approval / stops if
function SafetyCard({ safety }) {
  const Section = ({ label, items, tone, icon }) => {
    const I = Icon[icon];
    return (
      <div>
        <div className="flex items-center gap-1.5 mb-2">
          <span className={cls("text-", tone)}><I size={14} /></span>
          <span className={cls(
            "text-xs font-medium uppercase tracking-wide",
            `text-${tone}`,
          )}>{label}</span>
        </div>
        <ul className="space-y-1">
          {items.map((it, i) => (
            <li key={i} className="text-sm text-neutral-700 flex gap-2">
              <span className={cls("text-", tone, "mt-1.5 h-1 w-1 rounded-full shrink-0", `bg-${tone}-solid`)} />
              <span>{it}</span>
            </li>
          ))}
        </ul>
      </div>
    );
  };

  return (
    <div className="card card-padded">
      <div className="flex items-center justify-between mb-4">
        <h2 className="section-title">What this workspace can do today</h2>
        <span className="inline-flex items-center gap-1.5 text-xs text-safe font-medium">
          <Icon.Shield size={14} />
          Posture: {safety.posture}
        </span>
      </div>
      <div className="grid grid-cols-1 md:grid-cols-2 gap-x-6 gap-y-5">
        <Section label="Can"           items={safety.can}           tone="safe"        icon="Check" />
        <Section label="Needs approval" items={safety.needsApproval} tone="warning"     icon="Bell" />
        <Section label="Cannot"        items={safety.cannot}        tone="blocked"     icon="Slash" />
        <Section label="Stops if"      items={safety.stopsIf}       tone="quarantined" icon="Pause" />
      </div>
    </div>
  );
}

// ─── ResearchCard ────────────────────────────────────────────────────────
// Placeholder for @irongolem/ui ResearchCard — research finding with
// confidence + freshness.
function ResearchCard({ finding }) {
  const confPct = Math.round(finding.confidence * 100);
  const confTone = confPct >= 85 ? "safe" : confPct >= 70 ? "warning" : "blocked";
  return (
    <article className="card card-padded h-full flex flex-col">
      <div className="flex items-start justify-between gap-3">
        <h3 className="text-base font-semibold tracking-tight text-neutral-900 leading-snug">
          {finding.title}
        </h3>
        <span className={cls(
          "shrink-0 inline-flex items-center gap-1 rounded-full border text-[10px] font-medium px-1.5 py-0.5",
          `bg-${confTone}`, `text-${confTone}`, `border-${confTone}`,
        )}>
          {confPct}%
        </span>
      </div>
      <p className="text-sm text-neutral-700 mt-2 flex-1">{finding.summary}</p>
      <div className="mt-3 flex items-center justify-between text-xs text-neutral-500">
        <span className="font-mono">{finding.source}</span>
        <span className="inline-flex items-center gap-1">
          <Icon.Clock size={12} /> {finding.freshness}
        </span>
      </div>
    </article>
  );
}

window.UI = { HeartbeatStatus, RiskBadge, PermissionBadge, PolicyCard, SafetyCard, ResearchCard };
