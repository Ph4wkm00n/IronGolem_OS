// Research.tsx — IronGolem OS
// Route: /research
// One-file route per house style. Mock data at the top, then the route.
// Anything tagged TODO(integrator) is a placeholder the integrator will
// swap for the real @irongolem/ui import or live API call.
//
// React 19, TS strict, Tailwind utility classes only. Semantic palette
// (bg-safe / bg-warning / bg-blocked / bg-accent / bg-neutral / text-*)
// is provided by globals.css and behaves correctly in light + dark themes.
//
// Mandatory patterns wired in:
//   1. Visible Trust — confidence pill + bar render BEFORE the title is read.
//   2. Suppress-on-OK — only meaningful, novel findings surface in the grid.
//      ("Quietly archived" counter shown in header for transparency.)
//   3. Explainable Autonomy — "Why this finding?" opens a drawer with
//      sources, snippet excerpts, and the classifier that flagged it.
//   4. Contradiction-aware — when sources disagree, surface a warning chip
//      and pin the conflicting source to the top of the drawer evidence.

import * as React from "react";
const { useState, useMemo, useEffect } = React;

// ───────────────────────────────────────────────────────────────────────────
//  Types
// ───────────────────────────────────────────────────────────────────────────

type Topic = "pricing" | "api" | "supplier" | "industry" | "internal";

type Impact = "low" | "medium" | "high";

type Action =
  | "apply-finding"
  | "mark-reviewed"
  | "discuss-standup";

type SourceKind = "release-notes" | "index" | "alert" | "paper" | "digest" | "filing" | "blog";

type SourceSnippet = {
  id: string;
  name: string;            // human-readable source name
  url: string;             // canonical URL (never opened, displayed only)
  kind: SourceKind;
  publishedAt: string;     // "2h ago" / "yesterday"
  snippet: string;         // raw excerpt from the source
  agreement: "agrees" | "conflicts" | "neutral";
};

type ClassifierTrace = {
  name: string;            // "PriceMoveDetector v3"
  rule: string;            // rule that flagged it
  confidence: number;      // 0..1
};

type Finding = {
  id: string;
  title: string;           // plain language, 1-2 lines
  summary: string;         // 2-3 sentences
  topic: Topic;
  impact: Impact;
  confidence: number;      // 0..100
  freshness: string;       // "2h ago", "yesterday"
  freshnessMinutes: number; // for sorting
  primarySource: string;   // shown at card footer in font-mono
  sources: SourceSnippet[];
  contradictionCount: number; // count of sources marked "conflicts"
  classifier: ClassifierTrace;
  suggestedAction: Action;
  featured?: boolean;      // one finding gets the "Top impact" treatment
};

// ───────────────────────────────────────────────────────────────────────────
//  Mock findings — 14 items, mixed across topics / confidence / impact.
//  Three carry contradictions. One is featured.
//  TODO(integrator): replace with `useResearchQuery()`.
// ───────────────────────────────────────────────────────────────────────────

const MOCK_FINDINGS: Finding[] = [
  {
    id: "f01",
    title: "Carbon credit spot price up 11% overnight, broadest move in three months",
    summary:
      "Two of three approved price sources show carbon spot at $89.40 this morning, an 11% jump on the three-month range. The move is concentrated in N-American voluntary credits; compliance markets are flat. Your Q3 Riverbend purchase draft references the old price.",
    topic: "pricing",
    impact: "high",
    confidence: 92,
    freshness: "2h ago",
    freshnessMinutes: 120,
    primarySource: "bloomberg.terminal/cc.spot",
    sources: [
      { id: "s01a", name: "Bloomberg carbon spot",  url: "bloomberg.terminal/cc.spot", kind: "index",
        publishedAt: "2h ago",  snippet: "Voluntary carbon spot closes $89.40, +11.2% on 90-day MA. North-American basket leads gains; compliance basket -0.3%.", agreement: "agrees" },
      { id: "s01b", name: "S&P Platts CCM",          url: "spglobal.com/ccm/daily",     kind: "index",
        publishedAt: "3h ago",  snippet: "CCM index +10.8%, with NA-VCS posting strongest single-session gain since February.", agreement: "agrees" },
      { id: "s01c", name: "Reuters commodities desk",  url: "reuters.com/markets/commodities", kind: "release-notes",
        publishedAt: "4h ago",  snippet: "Bid-ask spread widened materially overnight; one regional desk reports paper at $84.10 — below other prints.", agreement: "conflicts" },
    ],
    contradictionCount: 1,
    classifier: {
      name: "PriceMoveDetector v3",
      rule: "Move ≥5% on ≥2 approved sources within 6h window",
      confidence: 0.92,
    },
    suggestedAction: "apply-finding",
    featured: true,
  },
  {
    id: "f02",
    title: "Stripe deprecating legacy webhook payload format on Aug 1",
    summary:
      "Stripe published a deprecation notice for the v1 webhook payload format used by your dispute-evidence pipeline. The v2 format adds a signature header and renames three fields. Your `acknowledge Stripe disputes` recipe will need its handler updated before August.",
    topic: "api",
    impact: "high",
    confidence: 96,
    freshness: "5h ago",
    freshnessMinutes: 300,
    primarySource: "stripe.com/changelog/2026-05",
    sources: [
      { id: "s02a", name: "Stripe changelog", url: "stripe.com/changelog/2026-05", kind: "release-notes",
        publishedAt: "5h ago", snippet: "v1 webhook payload is deprecated effective Aug 1, 2026. Endpoints not migrated will continue to receive payloads but headers will be downgraded.", agreement: "agrees" },
      { id: "s02b", name: "Stripe developer mailing list", url: "groups.stripe.com/devs", kind: "alert",
        publishedAt: "5h ago", snippet: "Direct notice to integrators using v1 payloads on the dispute.* and charge.dispute.* topics. Migration guide linked.", agreement: "agrees" },
    ],
    contradictionCount: 0,
    classifier: {
      name: "ChangelogWatcher v2",
      rule: "Deprecation keyword + endpoint in active recipe permission list",
      confidence: 0.96,
    },
    suggestedAction: "apply-finding",
  },
  {
    id: "f03",
    title: "Yates Holdings filed an updated W-9; bank routing has changed",
    summary:
      "Yates Holdings, a vendor on your standing-purchase list, filed a new W-9 with a different routing number than the one on record. The change is not yet reflected in your vendor table. Any auto-approved standing-order PO would still use the old routing.",
    topic: "supplier",
    impact: "high",
    confidence: 88,
    freshness: "yesterday",
    freshnessMinutes: 60 * 26,
    primarySource: "irs.gov/filings/yates-holdings",
    sources: [
      { id: "s03a", name: "IRS public filings", url: "irs.gov/filings/yates-holdings", kind: "filing",
        publishedAt: "yesterday", snippet: "W-9 amendment filed 2026-05-10. Address unchanged; bank routing updated.", agreement: "agrees" },
      { id: "s03b", name: "Yates AP team email", url: "ops@yatesholdings.com", kind: "alert",
        publishedAt: "yesterday", snippet: "FYI — please update payment details. New routing/account on the attached letter.", agreement: "agrees" },
      { id: "s03c", name: "Workspace vendor table", url: "internal://vendors/yates", kind: "internal" as SourceKind,
        publishedAt: "12d ago", snippet: "Routing on file: 121-000-358 (Bank of the West). Last verified 12 days ago.", agreement: "conflicts" },
    ],
    contradictionCount: 1,
    classifier: {
      name: "VendorBankChange v1",
      rule: "External W-9 routing differs from internal vendor table",
      confidence: 0.88,
    },
    suggestedAction: "apply-finding",
  },
  {
    id: "f04",
    title: "OpenAI lowered pricing on the model your drafting pipeline uses",
    summary:
      "The model your drafting recipes route through dropped 35% in input price and 20% in output price, effective immediately. No quality or rate-limit changes were announced. Your monthly drafting spend would drop roughly $84/month at current volume.",
    topic: "pricing",
    impact: "medium",
    confidence: 94,
    freshness: "8h ago",
    freshnessMinutes: 480,
    primarySource: "openai.com/pricing",
    sources: [
      { id: "s04a", name: "OpenAI pricing page", url: "openai.com/pricing", kind: "release-notes",
        publishedAt: "8h ago", snippet: "Input pricing reduced from $0.20 → $0.13 per 1M tokens. Output pricing reduced from $0.80 → $0.64 per 1M tokens.", agreement: "agrees" },
      { id: "s04b", name: "OpenAI blog", url: "openai.com/blog/pricing-may-2026", kind: "blog",
        publishedAt: "8h ago", snippet: "Effective immediately across all standard tiers. Enterprise contracts unaffected.", agreement: "agrees" },
    ],
    contradictionCount: 0,
    classifier: {
      name: "PricingWatch v4",
      rule: "Vendor in active recipe list has pricing delta ≥10%",
      confidence: 0.94,
    },
    suggestedAction: "mark-reviewed",
  },
  {
    id: "f05",
    title: "Pump-supplier Halford issued a recall on lot 24-118 maintenance pumps",
    summary:
      "Halford notified affected accounts that pumps shipped under lot 24-118 may have a sealing defect. Your maintenance PO from April referenced lot 24-118. The recall offers replacement at no cost; nothing on your side is overdue yet.",
    topic: "supplier",
    impact: "high",
    confidence: 90,
    freshness: "yesterday",
    freshnessMinutes: 60 * 30,
    primarySource: "halford.io/recall/2024-118",
    sources: [
      { id: "s05a", name: "Halford recall notice", url: "halford.io/recall/2024-118", kind: "alert",
        publishedAt: "yesterday", snippet: "Lot 24-118 may exhibit premature seal failure under continuous duty. Replacement available through standard RMA.", agreement: "agrees" },
      { id: "s05b", name: "Workspace PO history", url: "internal://purchasing/po", kind: "internal" as SourceKind,
        publishedAt: "1mo ago", snippet: "PO-24-099 references lot 24-118, pump replacement, qty 2.", agreement: "agrees" },
    ],
    contradictionCount: 0,
    classifier: {
      name: "RecallWatcher v2",
      rule: "Supplier recall lot ID matches internal PO record",
      confidence: 0.90,
    },
    suggestedAction: "apply-finding",
  },
  {
    id: "f06",
    title: "Lithium spot off 4% on stronger inventory data — directional, not material",
    summary:
      "Three approved price sources show lithium carbonate spot down 4% on the week, attributed to looser Q2 inventory reports out of Chile. The move is below your 5% material-change threshold; logged here as directional context only.",
    topic: "pricing",
    impact: "low",
    confidence: 79,
    freshness: "5h ago",
    freshnessMinutes: 300,
    primarySource: "spglobal.com/lithium",
    sources: [
      { id: "s06a", name: "S&P Platts lithium daily", url: "spglobal.com/lithium",  kind: "index",
        publishedAt: "5h ago",  snippet: "Spot lithium carbonate -3.9% on the week.", agreement: "agrees" },
      { id: "s06b", name: "Bloomberg metals", url: "bloomberg.terminal/metals.li", kind: "index",
        publishedAt: "6h ago", snippet: "Lithium carbonate prints -4.1%, in line with broader battery-metals weakness.", agreement: "agrees" },
      { id: "s06c", name: "Fastmarkets", url: "fastmarkets.com/lithium",  kind: "index",
        publishedAt: "7h ago", snippet: "Index move -4.0%, attributed to looser Chile Q2 inventories.", agreement: "agrees" },
    ],
    contradictionCount: 0,
    classifier: {
      name: "PriceMoveDetector v3",
      rule: "Move <5% on watched index — directional log only",
      confidence: 0.79,
    },
    suggestedAction: "mark-reviewed",
  },
  {
    id: "f07",
    title: "EU AI Act enforcement guidance for SMEs published — actionable in Q4",
    summary:
      "The EU Commission released SME-specific enforcement guidance for the AI Act. Operator-in-the-loop systems (like this workspace) are explicitly carved out of the high-risk classification. Your existing safety posture appears compliant.",
    topic: "industry",
    impact: "medium",
    confidence: 86,
    freshness: "yesterday",
    freshnessMinutes: 60 * 22,
    primarySource: "ec.europa.eu/ai-act/sme",
    sources: [
      { id: "s07a", name: "EU Commission release", url: "ec.europa.eu/ai-act/sme", kind: "release-notes",
        publishedAt: "yesterday", snippet: "Operator-in-the-loop systems with auditable safety layers are not classified as high-risk under Annex III.", agreement: "agrees" },
      { id: "s07b", name: "Hartlaw client memo", url: "hartlaw.com/memos/2026-05-eu-ai-sme", kind: "digest",
        publishedAt: "12h ago", snippet: "Workspaces matching the operator-loop pattern (drafts surface for approval) likely qualify for the SME carve-out.", agreement: "agrees" },
    ],
    contradictionCount: 0,
    classifier: {
      name: "RegulatoryDigest v2",
      rule: "Jurisdiction match + SME + AI Act keyword cluster",
      confidence: 0.86,
    },
    suggestedAction: "mark-reviewed",
  },
  {
    id: "f08",
    title: "Telegram desktop client introduced a breaking change in chat-bot auth",
    summary:
      "Telegram pushed a desktop client change on May 9 that breaks the long-lived bot session your ops channel notifier uses. Re-auth is required; messages are queueing locally and will retry once auth is refreshed.",
    topic: "api",
    impact: "medium",
    confidence: 81,
    freshness: "yesterday",
    freshnessMinutes: 60 * 18,
    primarySource: "telegram.org/changelog/desktop",
    sources: [
      { id: "s08a", name: "Telegram changelog", url: "telegram.org/changelog/desktop", kind: "release-notes",
        publishedAt: "yesterday", snippet: "Desktop client 5.1.4 changes long-lived bot session handling; sessions must be re-authorized.", agreement: "agrees" },
      { id: "s08b", name: "Workspace ops bot logs", url: "internal://logs/ops-bot",  kind: "internal" as SourceKind,
        publishedAt: "yesterday", snippet: "Auth failures observed since May 9 14:02 PT; 4 queued messages pending retry.", agreement: "agrees" },
      { id: "s08c", name: "Independent dev forum", url: "telegram-dev.forum/sessions", kind: "blog",
        publishedAt: "12h ago", snippet: "Multiple operators reporting auth failures; Telegram acknowledged in support thread.", agreement: "neutral" },
    ],
    contradictionCount: 0,
    classifier: {
      name: "ChangelogWatcher v2",
      rule: "Vendor changelog mentions endpoint used by active recipe",
      confidence: 0.81,
    },
    suggestedAction: "discuss-standup",
  },
  {
    id: "f09",
    title: "Riverbend filed a procurement RFP with stated scope similar to your existing contract",
    summary:
      "Public procurement records show Riverbend opened an RFP for FY27 sourcing with line items overlapping your current MSA scope. This could indicate competitive re-bid; it could also be routine. Your renewal window opens in 11 weeks.",
    topic: "industry",
    impact: "high",
    confidence: 73,
    freshness: "10h ago",
    freshnessMinutes: 600,
    primarySource: "sam.gov/rfps/riverbend-fy27",
    sources: [
      { id: "s09a", name: "SAM.gov RFP filing", url: "sam.gov/rfps/riverbend-fy27", kind: "filing",
        publishedAt: "10h ago", snippet: "RFP-FY27-118 opened. Scope: dock-side uniforms, lot tracking, weekly cadence. Award date Aug 14.", agreement: "agrees" },
      { id: "s09b", name: "Riverbend procurement page", url: "riverbend.co/procurement", kind: "release-notes",
        publishedAt: "12h ago", snippet: "Routine FY27 procurement cycle; no incumbent prejudice. Incumbents are encouraged to respond.", agreement: "neutral" },
      { id: "s09c", name: "Industry analyst note", url: "industrybeat.com/riverbend-rfp", kind: "blog",
        publishedAt: "8h ago", snippet: "Some uncertainty whether this is a true re-bid or routine compliance. Incumbent advantage looks intact.", agreement: "neutral" },
    ],
    contradictionCount: 0,
    classifier: {
      name: "RFPWatcher v1",
      rule: "Customer name + scope keyword overlap with existing MSA",
      confidence: 0.73,
    },
    suggestedAction: "discuss-standup",
  },
  {
    id: "f10",
    title: "Bloomberg and Reuters disagree on natural gas Henry Hub print",
    summary:
      "Two of your three approved energy-pricing sources reported diverging prints for Henry Hub close: Bloomberg $2.84, Reuters $2.71. S&P Platts is offline for maintenance. Material to your facility utility forecast if confirmed.",
    topic: "pricing",
    impact: "medium",
    confidence: 64,
    freshness: "4h ago",
    freshnessMinutes: 240,
    primarySource: "bloomberg.terminal/ng.hh",
    sources: [
      { id: "s10a", name: "Bloomberg natural gas", url: "bloomberg.terminal/ng.hh", kind: "index",
        publishedAt: "4h ago", snippet: "Henry Hub close: $2.84/MMBtu, +1.1% on session.", agreement: "agrees" },
      { id: "s10b", name: "Reuters energy desk", url: "reuters.com/markets/energy", kind: "release-notes",
        publishedAt: "5h ago", snippet: "Henry Hub close prints $2.71/MMBtu, -3.5% on session.", agreement: "conflicts" },
      { id: "s10c", name: "S&P Platts (offline)", url: "spglobal.com/platts/ng",   kind: "index",
        publishedAt: "—", snippet: "Source offline for scheduled maintenance.", agreement: "neutral" },
    ],
    contradictionCount: 1,
    classifier: {
      name: "PriceMoveDetector v3",
      rule: "Source disagreement >5% between approved sources",
      confidence: 0.64,
    },
    suggestedAction: "discuss-standup",
  },
  {
    id: "f11",
    title: "New paper: operator-in-the-loop AI systems show 4× lower error rates",
    summary:
      "A NeurIPS preprint compared operator-in-the-loop systems to fully autonomous agents across 12 production deployments. Loop systems showed 4× lower irrecoverable error rates with only marginal latency cost. Aligns with this workspace's safety posture.",
    topic: "industry",
    impact: "low",
    confidence: 82,
    freshness: "3d ago",
    freshnessMinutes: 60 * 24 * 3,
    primarySource: "arxiv.org/abs/2604.0118",
    sources: [
      { id: "s11a", name: "ArXiv preprint", url: "arxiv.org/abs/2604.0118", kind: "paper",
        publishedAt: "3d ago", snippet: "Operator-loop systems exhibit 4.1× lower irrecoverable error rate (n=412 incidents across 12 deployments).", agreement: "agrees" },
      { id: "s11b", name: "PaperDigest weekly", url: "paperdigest.com/2026-w19", kind: "digest",
        publishedAt: "2d ago", snippet: "Featured paper of the week; methodology is sound but sample size is modest.", agreement: "neutral" },
    ],
    contradictionCount: 0,
    classifier: {
      name: "PaperWatch v1",
      rule: "Paper matches workspace topic cluster + ≥1 trusted digest highlight",
      confidence: 0.82,
    },
    suggestedAction: "mark-reviewed",
  },
  {
    id: "f12",
    title: "Slack will charge for active webhook senders starting Q3",
    summary:
      "Slack's Q3 pricing update introduces a per-message fee for high-volume webhook senders. Your ops-bot sits at 12% of the proposed threshold today; expansion to a second channel could put it over. No immediate cost impact this quarter.",
    topic: "pricing",
    impact: "low",
    confidence: 71,
    freshness: "yesterday",
    freshnessMinutes: 60 * 28,
    primarySource: "slack.com/pricing-update-q3",
    sources: [
      { id: "s12a", name: "Slack pricing page",     url: "slack.com/pricing-update-q3",  kind: "release-notes",
        publishedAt: "yesterday", snippet: "Webhook senders above 10k messages/month will incur a $0.0008/message fee starting Q3.", agreement: "agrees" },
      { id: "s12b", name: "Workspace usage report", url: "internal://usage/ops-bot",     kind: "internal" as SourceKind,
        publishedAt: "yesterday", snippet: "Ops-bot 30-day average: 1,212 messages/month — about 12% of the new threshold.", agreement: "agrees" },
    ],
    contradictionCount: 0,
    classifier: {
      name: "PricingWatch v4",
      rule: "Vendor pricing change + workspace usage above 10%",
      confidence: 0.71,
    },
    suggestedAction: "mark-reviewed",
  },
  {
    id: "f13",
    title: "Two analysts disagree on Trent & Co liquidity outlook",
    summary:
      "Trent & Co, a current customer with outstanding wire instructions, drew divergent analyst notes today. Moody's flagged a covenant-watch event; Fitch issued an affirm with stable outlook. Worth surfacing — your AP team has open exposure.",
    topic: "supplier",
    impact: "medium",
    confidence: 68,
    freshness: "6h ago",
    freshnessMinutes: 360,
    primarySource: "moodys.com/notes/trentco-2026",
    sources: [
      { id: "s13a", name: "Moody's analyst note", url: "moodys.com/notes/trentco-2026", kind: "alert",
        publishedAt: "6h ago", snippet: "Covenant-watch event triggered by Q1 leverage drift. Watchlist with negative implication.", agreement: "agrees" },
      { id: "s13b", name: "Fitch analyst note", url: "fitchratings.com/notes/trentco", kind: "alert",
        publishedAt: "7h ago", snippet: "Affirms BBB- with stable outlook. Covenant headroom remains adequate per Q1 disclosures.", agreement: "conflicts" },
    ],
    contradictionCount: 1,
    classifier: {
      name: "CounterpartyMonitor v2",
      rule: "Two trusted analysts diverge on the same counterparty within 24h",
      confidence: 0.68,
    },
    suggestedAction: "discuss-standup",
  },
  {
    id: "f14",
    title: "Internal: heartbeat dropped one system for 18 minutes last night",
    summary:
      "Workspace heartbeat showed 18/19 systems green between 02:14 and 02:32 PT last night. The degraded system was the cold-storage roller; logs indicate a transient S3 throttle. Self-healed without operator intervention.",
    topic: "internal",
    impact: "low",
    confidence: 97,
    freshness: "10h ago",
    freshnessMinutes: 600,
    primarySource: "internal://heartbeat/log",
    sources: [
      { id: "s14a", name: "Heartbeat history", url: "internal://heartbeat/log", kind: "internal" as SourceKind,
        publishedAt: "10h ago", snippet: "System 'cold-storage roller' degraded 02:14–02:32 PT. Recovered without operator action.", agreement: "agrees" },
      { id: "s14b", name: "Cloud provider status", url: "status.cloud-provider.com",  kind: "alert",
        publishedAt: "10h ago", snippet: "S3 partial throttling observed in us-west-2 between 09:00 and 09:35 UTC.", agreement: "agrees" },
    ],
    contradictionCount: 0,
    classifier: {
      name: "InternalHealthDigest v1",
      rule: "Heartbeat dip with external corroboration",
      confidence: 0.97,
    },
    suggestedAction: "mark-reviewed",
  },
];

// 1,284 quietly-archived items don't show up here, but transparency about
// that is part of the contract — display the count in the header.
const QUIETLY_ARCHIVED_TODAY = 1284;
const SOURCES_MONITORED = 47;

// ───────────────────────────────────────────────────────────────────────────
//  Static maps
// ───────────────────────────────────────────────────────────────────────────

const TOPIC_META: Record<Topic, { label: string; tone: "accent" | "recovered" | "warning" | "quarantined" | "neutral" }> = {
  pricing:  { label: "Pricing",       tone: "accent"      },
  api:      { label: "API changes",   tone: "recovered"   },
  supplier: { label: "Supplier risk", tone: "warning"     },
  industry: { label: "Industry",      tone: "quarantined" },
  internal: { label: "Internal",      tone: "neutral"     },
};

const IMPACT_META: Record<Impact, { label: string }> = {
  low:    { label: "Low impact"    },
  medium: { label: "Medium impact" },
  high:   { label: "High impact"   },
};

const ACTION_META: Record<Action, { label: string }> = {
  "apply-finding":   { label: "Apply finding"     },
  "mark-reviewed":   { label: "Mark reviewed"     },
  "discuss-standup": { label: "Discuss in standup" },
};

// ───────────────────────────────────────────────────────────────────────────
//  Inline icons (Heroicons-style, stroke 1.5).
//  TODO(integrator): replace with `@irongolem/ui/icons`.
// ───────────────────────────────────────────────────────────────────────────

const Svg = ({ d, size = 16, className = "", vb = "0 0 24 24" }:
  { d: React.ReactNode; size?: number; className?: string; vb?: string }) => (
  <svg viewBox={vb} width={size} height={size} fill="none" stroke="currentColor"
       strokeWidth={1.5} strokeLinecap="round" strokeLinejoin="round"
       className={className} aria-hidden="true">{d}</svg>
);

type IconProps = { size?: number; className?: string };
const ICON = {
  Clock:    (p: IconProps) => <Svg {...p} d={<><circle cx="12" cy="12" r="9" /><path d="M12 7v5l3 2" /></>} />,
  AlertTriangle: (p: IconProps) => <Svg {...p} d={<><path d="M12 4 2.5 20h19L12 4Z" /><path d="M12 10v4" /><circle cx="12" cy="17.5" r=".5" fill="currentColor" stroke="none" /></>} />,
  Sparkles: (p: IconProps) => <Svg {...p} d={<><path d="M12 3v3M12 18v3M3 12h3M18 12h3M5.6 5.6l2.1 2.1M16.3 16.3l2.1 2.1M5.6 18.4l2.1-2.1M16.3 7.7l2.1-2.1" /></>} />,
  Search:   (p: IconProps) => <Svg {...p} d={<><circle cx="11" cy="11" r="6" /><path d="m20 20-4.3-4.3" /></>} />,
  Check:    (p: IconProps) => <Svg {...p} d={<path d="m5 12 5 5L20 7" />} />,
  X:        (p: IconProps) => <Svg {...p} d={<><path d="M6 6l12 12" /><path d="M18 6 6 18" /></>} />,
  ArrowRight: (p: IconProps) => <Svg {...p} d={<><path d="M5 12h14" /><path d="m13 6 6 6-6 6" /></>} />,
  ArrowLeft:  (p: IconProps) => <Svg {...p} d={<><path d="M19 12H5" /><path d="m11 6-6 6 6 6" /></>} />,
  Link:     (p: IconProps) => <Svg {...p} d={<><path d="M10 14a4 4 0 0 0 5.7 0l3-3a4 4 0 0 0-5.7-5.7l-1 1" /><path d="M14 10a4 4 0 0 0-5.7 0l-3 3a4 4 0 0 0 5.7 5.7l1-1" /></>} />,
  File:     (p: IconProps) => <Svg {...p} d={<><path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8l-5-5Z" /><path d="M14 3v5h5" /></>} />,
  Activity: (p: IconProps) => <Svg {...p} d={<path d="M3 12h4l3-7 4 14 3-7h4" />} />,
  ChevronDown: (p: IconProps) => <Svg {...p} d={<path d="m6 9 6 6 6-6" />} />,
  Eye:      (p: IconProps) => <Svg {...p} d={<><path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12Z" /><circle cx="12" cy="12" r="3" /></>} />,
  Pin:      (p: IconProps) => <Svg {...p} d={<><path d="M12 17v5" /><path d="M9 3h6l-1 6h2l1 4H7l1-4h2L9 3Z" /></>} />,
  Bolt:     (p: IconProps) => <Svg {...p} d={<path d="M13 2 4 14h7l-1 8 9-12h-7l1-8Z" />} />,
  Cpu:      (p: IconProps) => <Svg {...p} d={<><rect x="5" y="5" width="14" height="14" rx="2" /><rect x="9" y="9" width="6" height="6" rx="1" /><path d="M9 2v3M15 2v3M9 19v3M15 19v3M2 9h3M2 15h3M19 9h3M19 15h3" /></>} />,
  Layers:   (p: IconProps) => <Svg {...p} d={<><path d="m12 3 9 5-9 5-9-5 9-5Z" /><path d="m3 13 9 5 9-5" /><path d="m3 18 9 5 9-5" /></>} />,
};

const KIND_META: Record<SourceKind, { label: string; IconCmp: React.ComponentType<IconProps> }> = {
  "release-notes": { label: "Release notes", IconCmp: ICON.File   },
  index:           { label: "Index",         IconCmp: ICON.Activity },
  alert:           { label: "Alert",         IconCmp: ICON.AlertTriangle },
  paper:           { label: "Paper",         IconCmp: ICON.File   },
  digest:          { label: "Digest",        IconCmp: ICON.Sparkles },
  filing:          { label: "Filing",        IconCmp: ICON.File   },
  blog:            { label: "Blog",          IconCmp: ICON.Link   },
};

// ───────────────────────────────────────────────────────────────────────────
//  Helpers
// ───────────────────────────────────────────────────────────────────────────

const cx = (...c: (string | false | null | undefined)[]) => c.filter(Boolean).join(" ");

function confidenceTone(c: number): "safe" | "warning" | "blocked" {
  if (c >= 85) return "safe";
  if (c >= 70) return "warning";
  return "blocked";
}

function impactRank(i: Impact): number {
  return ({ low: 1, medium: 2, high: 3 } as const)[i];
}

// ───────────────────────────────────────────────────────────────────────────
//  Placeholder versions of @irongolem/ui patterns.
//  TODO(integrator): swap for real imports.
// ───────────────────────────────────────────────────────────────────────────

function ConfidencePill({ value, size = "sm" }: { value: number; size?: "sm" | "md" }) {
  const tone = confidenceTone(value);
  const sizeCx = size === "sm" ? "text-[10.5px] px-1.5 py-0.5" : "text-[12px] px-2 py-0.5";
  return (
    <span className={cx(
      "inline-flex items-center gap-1.5 rounded-full border font-medium tabular-nums",
      sizeCx, `bg-${tone}`, `text-${tone}`, `border-${tone}`,
    )}>
      <span className={cx("h-1.5 w-1.5 rounded-full", `bg-${tone}-solid`)} />
      {value}% confidence
    </span>
  );
}

function ConfidenceBar({ value, size = "sm" }: { value: number; size?: "sm" | "md" }) {
  const tone = confidenceTone(value);
  return (
    <div className={cx(
      "w-full rounded-full overflow-hidden",
      size === "sm" ? "h-1" : "h-1.5",
      "bg-neutral-100",
    )} aria-hidden="true">
      <div
        className={cx("h-full rounded-full transition-all duration-300", `bg-${tone}-solid`)}
        style={{ width: `${Math.max(2, Math.min(100, value))}%` }}
      />
    </div>
  );
}

function TopicChip({ topic, size = "sm" }: { topic: Topic; size?: "sm" | "md" }) {
  const m = TOPIC_META[topic];
  const sizeCx = size === "sm" ? "text-[10.5px] px-1.5 py-0.5" : "text-[11px] px-2 py-0.5";
  return (
    <span className={cx(
      "inline-flex items-center gap-1 rounded-full border font-medium",
      sizeCx, `bg-${m.tone}`, `text-${m.tone}`, `border-${m.tone}`,
    )}>
      {m.label}
    </span>
  );
}

function ContradictionChip({ count }: { count: number }) {
  if (count === 0) return null;
  return (
    <span className="inline-flex items-center gap-1 rounded-md bg-warning border border-warning text-warning text-[10.5px] font-medium px-1.5 py-0.5">
      <ICON.AlertTriangle size={11} />
      {count} conflicting source{count === 1 ? "" : "s"}
    </span>
  );
}

function FreshnessChip({ value }: { value: string }) {
  return (
    <span className="inline-flex items-center gap-1 text-[11px] text-neutral-500">
      <ICON.Clock size={11} />
      {value}
    </span>
  );
}

function ImpactDot({ impact }: { impact: Impact }) {
  const tone = impact === "high" ? "blocked" : impact === "medium" ? "warning" : "neutral";
  return (
    <span className="inline-flex items-center gap-1 text-[10.5px] text-neutral-500">
      <span className={cx("h-1.5 w-1.5 rounded-full", `bg-${tone}-solid`)} />
      {IMPACT_META[impact].label}
    </span>
  );
}

// ── ResearchCard (default + featured layouts) ──────────────────────────
function ResearchCard({
  finding, onWhy, onAct, featured = false,
}: {
  finding: Finding;
  onWhy: () => void;
  onAct: () => void;
  featured?: boolean;
}) {
  const action = ACTION_META[finding.suggestedAction].label;
  const conflict = finding.contradictionCount > 0;
  return (
    <article className={cx(
      "group card flex flex-col h-full overflow-hidden transition-shadow",
      featured ? "ring-1 ring-accent-border" : "hover:shadow-elevated",
    )}>
      {featured && (
        <div className="bg-accent-solid text-white px-4 py-1.5 flex items-center gap-1.5 text-[11px] font-medium uppercase tracking-wide">
          <ICON.Sparkles size={12} /> Top impact today
        </div>
      )}

      <div className={cx("px-4 pt-4 pb-3 flex flex-col gap-3", featured && "px-6 pt-5")}>
        {/* Visible Trust — confidence appears BEFORE the title */}
        <div className="flex items-center justify-between gap-2 flex-wrap">
          <ConfidencePill value={finding.confidence} size={featured ? "md" : "sm"} />
          <div className="flex items-center gap-2">
            <ImpactDot impact={finding.impact} />
            <FreshnessChip value={finding.freshness} />
          </div>
        </div>

        {/* Title + bar */}
        <div>
          <h3 className={cx(
            "font-semibold tracking-tight text-neutral-900 leading-snug",
            featured ? "text-[19px]" : "text-[15px]",
          )}>
            {finding.title}
          </h3>
          <div className="mt-2"><ConfidenceBar value={finding.confidence} size={featured ? "md" : "sm"} /></div>
        </div>

        {/* Summary */}
        <p className={cx(
          "text-neutral-600 leading-relaxed",
          featured ? "text-[13.5px]" : "text-[12.5px]",
        )}>
          {finding.summary}
        </p>

        {/* Topic + contradiction */}
        <div className="flex flex-wrap items-center gap-1.5">
          <TopicChip topic={finding.topic} size={featured ? "md" : "sm"} />
          {finding.sources.length > 1 && (
            <span className="inline-flex items-center gap-1 rounded-full bg-neutral-100 px-1.5 py-0.5 text-[10.5px] font-medium text-neutral-600">
              <ICON.Layers size={10} /> {finding.sources.length} sources
            </span>
          )}
          <ContradictionChip count={finding.contradictionCount} />
        </div>
      </div>

      {/* Sources foot — font-mono primary source attribution */}
      <div className="px-4 py-2 border-t border-neutral-100 bg-neutral-50/50 flex items-center justify-between gap-2 text-[11px]">
        <span className="inline-flex items-center gap-1 text-neutral-500 min-w-0">
          <ICON.Link size={11} className="shrink-0" />
          <span className="font-mono truncate" title={finding.primarySource}>
            {finding.primarySource}
          </span>
        </span>
        <button type="button" onClick={onWhy}
                className="shrink-0 inline-flex items-center gap-0.5 text-accent hover:text-accent-solid font-medium">
          Why this finding?
          <ICON.ArrowRight size={10} />
        </button>
      </div>

      {/* Action row */}
      <footer className="border-t border-neutral-100 px-3 py-2 flex items-center justify-between bg-white">
        <span className={cx(
          "inline-flex items-center gap-1 text-[10.5px]",
          conflict ? "text-warning" : "text-neutral-400",
        )}>
          {conflict
            ? <><ICON.AlertTriangle size={11} /> Surfaced with conflict</>
            : <><ICON.Cpu size={11} /> Surfaced by {finding.classifier.name}</>}
        </span>
        <button type="button" onClick={onAct}
                className="inline-flex items-center gap-1.5 rounded-md bg-accent-solid text-white hover:bg-accent-solid-hover px-2.5 py-1.5 text-[12.5px] font-medium transition-colors">
          <ICON.Check size={12} /> {action}
        </button>
      </footer>
    </article>
  );
}

// ── Source row (drawer body) ───────────────────────────────────────────
function SourceRow({ source }: { source: SourceSnippet }) {
  const k = KIND_META[source.kind];
  const KIcn = k.IconCmp;
  const agreementTone =
    source.agreement === "conflicts" ? "warning" :
    source.agreement === "agrees"    ? "safe"    : "neutral";
  return (
    <li className={cx(
      "px-3 py-3 flex gap-3 border-l-2",
      source.agreement === "conflicts" ? "border-l-warning-solid bg-warning/40" : "border-l-transparent",
    )}>
      <span className={cx(
        "shrink-0 h-7 w-7 rounded-md inline-flex items-center justify-center",
        `bg-${agreementTone}`, `text-${agreementTone}`,
      )}>
        <KIcn size={13} />
      </span>
      <div className="flex-1 min-w-0">
        <div className="flex items-baseline gap-2 flex-wrap">
          <span className="text-[12.5px] font-semibold text-neutral-900 truncate">{source.name}</span>
          <span className={cx(
            "text-[10px] font-medium uppercase tracking-wide rounded-full border px-1.5 py-0.5",
            `bg-${agreementTone}`, `text-${agreementTone}`, `border-${agreementTone}`,
          )}>
            {source.agreement}
          </span>
          <span className="text-[10.5px] text-neutral-400">· {k.label}</span>
          <span className="text-[10.5px] text-neutral-400 ml-auto font-mono tabular-nums">{source.publishedAt}</span>
        </div>
        <div className="text-[11.5px] font-mono text-neutral-500 truncate mt-0.5" title={source.url}>{source.url}</div>
        <blockquote className="mt-2 rounded-md bg-neutral-50 border border-neutral-100 px-3 py-2 text-[12.5px] text-neutral-700 leading-relaxed">
          "{source.snippet}"
        </blockquote>
      </div>
    </li>
  );
}

function ClassifierTraceCard({ trace }: { trace: ClassifierTrace }) {
  const tone = confidenceTone(Math.round(trace.confidence * 100));
  return (
    <div className="rounded-xl border border-neutral-200 bg-white p-4">
      <div className="flex items-center justify-between mb-3">
        <div>
          <div className="text-[11px] font-medium uppercase tracking-wide text-neutral-500">Why this surfaced</div>
          <div className="text-[14px] font-semibold tracking-tight text-neutral-900 mt-0.5">{trace.name}</div>
        </div>
        <ConfidencePill value={Math.round(trace.confidence * 100)} size="md" />
      </div>
      <div className="rounded-md bg-neutral-50 border border-neutral-100 px-3 py-2">
        <div className="text-[10.5px] font-medium uppercase tracking-wide text-neutral-400">Rule</div>
        <div className="font-mono text-[12.5px] text-neutral-700 mt-0.5 leading-snug">{trace.rule}</div>
      </div>
      <div className={cx(
        "mt-3 text-[11.5px] flex items-center gap-1.5",
        `text-${tone}`,
      )}>
        <ICON.Bolt size={12} />
        Classifier confidence: <span className="font-medium tabular-nums">{Math.round(trace.confidence * 100)}%</span>
      </div>
    </div>
  );
}

function DetailDrawer({ finding, onClose }: { finding: Finding; onClose: () => void }) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => { if (e.key === "Escape") onClose(); };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  // Pin conflicting sources to the top — contradiction-aware ordering.
  const ordered = useMemo(() => {
    const conflicts = finding.sources.filter((s) => s.agreement === "conflicts");
    const rest = finding.sources.filter((s) => s.agreement !== "conflicts");
    return [...conflicts, ...rest];
  }, [finding.sources]);

  return (
    <div className="fixed inset-0 z-40 flex justify-end" role="dialog" aria-modal="true">
      <div className="ig-drawer-backdrop absolute inset-0" onClick={onClose} />
      <aside className="relative h-full w-full max-w-[680px] bg-white shadow-xl border-l border-neutral-200 flex flex-col"
             style={{ boxShadow: "var(--shadow-elevated)" }}>
        {/* Sticky top bar */}
        <header className="sticky top-0 z-10 bg-white/90 backdrop-blur border-b border-neutral-100 px-5 py-3 flex items-center gap-2">
          <button type="button" onClick={onClose}
                  className="inline-flex items-center gap-1 text-[12px] font-medium text-neutral-700 hover:text-neutral-900 px-2 py-1 rounded-md hover:bg-neutral-50">
            <ICON.ArrowLeft size={13} /> Back
          </button>
          <span className="text-[11px] font-mono text-neutral-400">{finding.id.toUpperCase()}</span>
          <span className="text-neutral-300">·</span>
          <TopicChip topic={finding.topic} />
          <div className="ml-auto flex items-center gap-2">
            <ConfidencePill value={finding.confidence} />
            <button type="button" onClick={onClose}
                    className="h-7 w-7 inline-flex items-center justify-center rounded-md text-neutral-500 hover:text-neutral-900 hover:bg-neutral-50">
              <ICON.X size={14} />
            </button>
          </div>
        </header>

        <div className="flex-1 overflow-y-auto scrollbar-thin">
          <div className="px-6 py-5">
            <h1 className="text-[20px] font-semibold tracking-tight text-neutral-900 leading-tight">
              {finding.title}
            </h1>
            <div className="mt-3"><ConfidenceBar value={finding.confidence} size="md" /></div>

            <div className="mt-3 flex flex-wrap items-center gap-x-3 gap-y-1.5">
              <ImpactDot impact={finding.impact} />
              <FreshnessChip value={finding.freshness} />
              <span className="text-neutral-300">·</span>
              <span className="inline-flex items-center gap-1 text-[11.5px] text-neutral-500">
                <ICON.Layers size={11} /> {finding.sources.length} source{finding.sources.length === 1 ? "" : "s"}
              </span>
              <ContradictionChip count={finding.contradictionCount} />
            </div>

            <p className="mt-4 text-[13.5px] text-neutral-700 leading-relaxed">
              {finding.summary}
            </p>

            {/* Contradiction callout when applicable */}
            {finding.contradictionCount > 0 && (
              <div className="mt-4 rounded-xl border border-warning bg-warning p-4">
                <div className="flex items-start gap-3">
                  <span className="h-7 w-7 shrink-0 rounded-md bg-warning-solid text-white inline-flex items-center justify-center">
                    <ICON.AlertTriangle size={14} />
                  </span>
                  <div className="min-w-0">
                    <div className="text-[11px] font-medium uppercase tracking-wide text-warning">Sources disagree</div>
                    <div className="mt-1 text-[13px] text-warning leading-relaxed">
                      {finding.contradictionCount} of {finding.sources.length} source{finding.sources.length === 1 ? "" : "s"} contradict the headline finding.
                      Conflicting source{finding.contradictionCount === 1 ? " is" : "s are"} pinned to the top of the evidence below.
                    </div>
                  </div>
                </div>
              </div>
            )}

            {/* Sources */}
            <section className="mt-5">
              <div className="flex items-baseline justify-between mb-2">
                <h2 className="text-[11px] font-medium uppercase tracking-wide text-neutral-500">Evidence</h2>
                <span className="text-[10.5px] text-neutral-400 font-mono tabular-nums">
                  {finding.sources.length} source{finding.sources.length === 1 ? "" : "s"}
                </span>
              </div>
              <ul className="divide-y divide-neutral-100 rounded-xl border border-neutral-200 bg-white overflow-hidden">
                {ordered.map((s) => <SourceRow key={s.id} source={s} />)}
              </ul>
            </section>

            {/* Classifier trace */}
            <section className="mt-5 pb-6">
              <h2 className="text-[11px] font-medium uppercase tracking-wide text-neutral-500 mb-2">
                Classifier
              </h2>
              <ClassifierTraceCard trace={finding.classifier} />
            </section>
          </div>
        </div>

        {/* Sticky footer — action options */}
        <footer className="sticky bottom-0 bg-white/95 backdrop-blur border-t border-neutral-100 px-5 py-3 flex items-center justify-between gap-2">
          <span className="text-[11px] text-neutral-500">
            Suggested: <span className="text-neutral-800 font-medium">{ACTION_META[finding.suggestedAction].label}</span>
          </span>
          <div className="flex items-center gap-2">
            <button type="button"
                    className="inline-flex items-center gap-1.5 rounded-md border border-neutral-200 bg-white hover:bg-neutral-50 text-neutral-700 px-3 py-1.5 text-[12.5px] font-medium transition-colors">
              <ICON.Eye size={12} /> Mark reviewed
            </button>
            <button type="button"
                    className="inline-flex items-center gap-1.5 rounded-md bg-accent-solid text-white hover:bg-accent-solid-hover px-3 py-1.5 text-[12.5px] font-medium transition-colors">
              <ICON.Check size={12} /> {ACTION_META[finding.suggestedAction].label}
            </button>
          </div>
        </footer>
      </aside>
    </div>
  );
}

// ───────────────────────────────────────────────────────────────────────────
//  Filter / sort controls
// ───────────────────────────────────────────────────────────────────────────

type SortKey = "recent" | "impact" | "confidence";
type TopicFilter = "all" | Topic;

function SortPicker({ value, onChange }: { value: SortKey; onChange: (v: SortKey) => void }) {
  const opts: { id: SortKey; label: string }[] = [
    { id: "recent",     label: "Most recent"        },
    { id: "impact",     label: "Highest impact"     },
    { id: "confidence", label: "Highest confidence" },
  ];
  return (
    <div className="inline-flex items-center rounded-lg border border-neutral-200 bg-white p-0.5">
      {opts.map((o) => (
        <button
          key={o.id}
          type="button"
          onClick={() => onChange(o.id)}
          className={cx(
            "px-2.5 py-1 rounded-md text-[12px] font-medium transition-colors",
            value === o.id
              ? "bg-neutral-900 text-white"
              : "text-neutral-600 hover:text-neutral-900",
          )}
        >
          {o.label}
        </button>
      ))}
    </div>
  );
}

function TopicChips({ value, counts, onChange }: {
  value: TopicFilter;
  counts: Record<TopicFilter, number>;
  onChange: (v: TopicFilter) => void;
}) {
  const all: { id: TopicFilter; label: string }[] = [
    { id: "all", label: "All" },
    ...(Object.keys(TOPIC_META) as Topic[]).map((t) => ({ id: t, label: TOPIC_META[t].label })),
  ];
  return (
    <div className="flex flex-wrap items-center gap-1.5">
      {all.map((t) => {
        const isActive = value === t.id;
        const n = counts[t.id] ?? 0;
        return (
          <button
            key={t.id}
            type="button"
            onClick={() => onChange(t.id)}
            className={cx(
              "inline-flex items-center gap-1.5 rounded-full text-[12px] font-medium px-2.5 py-1 border transition-colors",
              isActive
                ? "bg-neutral-900 text-white border-neutral-900"
                : "bg-white text-neutral-700 border-neutral-200 hover:bg-neutral-50",
            )}
          >
            {t.label}
            <span className={cx(
              "rounded-full font-mono tabular-nums text-[10px] px-1.5 py-px",
              isActive ? "bg-white/20 text-white" : "bg-neutral-100 text-neutral-500",
            )}>
              {n}
            </span>
          </button>
        );
      })}
    </div>
  );
}

function HideLowToggle({ value, onChange }: { value: boolean; onChange: (v: boolean) => void }) {
  return (
    <label className="inline-flex items-center gap-2 text-[12px] text-neutral-600 cursor-pointer select-none">
      <span className={cx(
        "relative inline-flex h-4 w-7 items-center rounded-full transition-colors",
        value ? "bg-accent-solid" : "bg-neutral-200",
      )}>
        <span className={cx(
          "absolute h-3 w-3 bg-white rounded-full shadow transition-all",
          value ? "left-[14px]" : "left-[2px]",
        )} />
      </span>
      <input type="checkbox" checked={value} onChange={(e) => onChange(e.target.checked)} className="sr-only" />
      Hide low-confidence (&lt;70%)
    </label>
  );
}

// ───────────────────────────────────────────────────────────────────────────
//  Empty state
// ───────────────────────────────────────────────────────────────────────────

function EmptyState() {
  return (
    <div className="card card-padded">
      <div className="flex flex-col items-center text-center py-12 max-w-md mx-auto">
        <div className="h-12 w-12 rounded-full bg-safe inline-flex items-center justify-center mb-4 text-safe">
          <ICON.Sparkles size={22} />
        </div>
        <h3 className="section-title">No new findings</h3>
        <p className="text-[13px] text-neutral-500 mt-2 leading-relaxed">
          The research team is monitoring <span className="text-neutral-700 font-medium tabular-nums">{SOURCES_MONITORED} sources</span>.
          You'll see findings here whenever something materially new comes up.
        </p>
        <div className="mt-4 inline-flex items-center gap-1.5 text-[11px] text-neutral-400">
          <span className="h-1.5 w-1.5 rounded-full bg-safe-solid ig-pulse" />
          Last source check: 2 minutes ago
        </div>
      </div>
    </div>
  );
}

// ───────────────────────────────────────────────────────────────────────────
//  Header
// ───────────────────────────────────────────────────────────────────────────

function ResearchHeader({ surfaced, archived }: { surfaced: number; archived: number }) {
  return (
    <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
      <div>
        <div className="flex items-center gap-2">
          <h1 className="page-title">Research</h1>
          <span className="inline-flex items-center gap-1.5 rounded-full bg-accent px-2 py-0.5 text-[11px] text-accent font-medium">
            <ICON.Sparkles size={11} />
            {surfaced} new findings
          </span>
        </div>
        <p className="mt-1 text-[13.5px] text-neutral-500 max-w-2xl leading-relaxed">
          What the research team found in external sources that matters to this workspace.
          {" "}
          <span className="text-neutral-700">{archived.toLocaleString()}</span>
          {" "}other items checked today were quietly archived because nothing changed.
        </p>
      </div>
      <div className="inline-flex items-center gap-1.5 text-[11.5px] text-neutral-500">
        <ICON.Activity size={12} className="text-safe" />
        Watching <span className="text-neutral-700 font-medium tabular-nums">{SOURCES_MONITORED}</span> sources
      </div>
    </div>
  );
}

// ───────────────────────────────────────────────────────────────────────────
//  Filter / sort bar
// ───────────────────────────────────────────────────────────────────────────

function FilterBar({
  sort, topic, hideLow, counts,
  onSort, onTopic, onHideLow,
}: {
  sort: SortKey;
  topic: TopicFilter;
  hideLow: boolean;
  counts: Record<TopicFilter, number>;
  onSort: (s: SortKey) => void;
  onTopic: (t: TopicFilter) => void;
  onHideLow: (v: boolean) => void;
}) {
  return (
    <div className="card card-padded flex flex-col gap-3">
      <div className="flex items-center justify-between gap-3 flex-wrap">
        <div className="flex items-center gap-2">
          <span className="text-[11px] font-medium uppercase tracking-wide text-neutral-500">Sort by</span>
          <SortPicker value={sort} onChange={onSort} />
        </div>
        <HideLowToggle value={hideLow} onChange={onHideLow} />
      </div>
      <div>
        <div className="text-[11px] font-medium uppercase tracking-wide text-neutral-500 mb-2">Topic</div>
        <TopicChips value={topic} counts={counts} onChange={onTopic} />
      </div>
    </div>
  );
}

// ───────────────────────────────────────────────────────────────────────────
//  Research — the route
// ───────────────────────────────────────────────────────────────────────────

export function Research(): JSX.Element {
  const [findings] = useState<Finding[]>(MOCK_FINDINGS);
  const [sort, setSort]       = useState<SortKey>("recent");
  const [topic, setTopic]     = useState<TopicFilter>("all");
  const [hideLow, setHideLow] = useState(false);
  const [openId, setOpenId]   = useState<string | null>(null);
  const [toast, setToast]     = useState<string | null>(null);

  // Lock body scroll while a drawer is open
  useEffect(() => {
    document.documentElement.style.overflow = openId ? "hidden" : "";
    return () => { document.documentElement.style.overflow = ""; };
  }, [openId]);

  // Auto-dismiss the toast
  useEffect(() => {
    if (!toast) return;
    const id = window.setTimeout(() => setToast(null), 2200);
    return () => window.clearTimeout(id);
  }, [toast]);

  const counts: Record<TopicFilter, number> = useMemo(() => {
    const out: Record<TopicFilter, number> = {
      all: findings.length,
      pricing: 0, api: 0, supplier: 0, industry: 0, internal: 0,
    };
    for (const f of findings) out[f.topic]++;
    return out;
  }, [findings]);

  // Featured finding: highest impact, then freshest. Suppress-on-OK still
  // applies — only items already in `findings` are considered.
  const featured = useMemo(() => {
    const sorted = [...findings].sort((a, b) => {
      const i = impactRank(b.impact) - impactRank(a.impact);
      if (i !== 0) return i;
      return a.freshnessMinutes - b.freshnessMinutes;
    });
    return sorted[0] ?? null;
  }, [findings]);

  const rest = useMemo(() => {
    const out = findings.filter((f) => f.id !== featured?.id);
    if (topic !== "all") return out.filter((f) => f.topic === topic);
    return out;
  }, [findings, featured, topic]);

  const filtered = useMemo(() => {
    let xs = rest;
    if (hideLow) xs = xs.filter((f) => f.confidence >= 70);

    const sorted = [...xs];
    sorted.sort((a, b) => {
      if (sort === "recent")     return a.freshnessMinutes - b.freshnessMinutes;
      if (sort === "impact")     return impactRank(b.impact) - impactRank(a.impact);
      if (sort === "confidence") return b.confidence - a.confidence;
      return 0;
    });
    return sorted;
  }, [rest, sort, hideLow]);

  // Featured visibility: only when "all" topic AND it survives hide-low.
  const showFeatured = featured && topic === "all" && (!hideLow || featured.confidence >= 70);
  const totalShown = filtered.length + (showFeatured ? 1 : 0);

  const opened = useMemo(() => findings.find((f) => f.id === openId) || null, [findings, openId]);

  const handleAct = (f: Finding) => {
    setToast(`${ACTION_META[f.suggestedAction].label} · ${f.title.slice(0, 56)}${f.title.length > 56 ? "…" : ""}`);
  };

  return (
    <main className="min-h-screen bg-app">
      <div className="page-container max-w-[78rem]">
        <ResearchHeader surfaced={findings.length} archived={QUIETLY_ARCHIVED_TODAY} />

        <div className="mt-5">
          <FilterBar
            sort={sort} topic={topic} hideLow={hideLow}
            counts={counts}
            onSort={setSort} onTopic={setTopic} onHideLow={setHideLow}
          />
        </div>

        <div className="mt-5">
          {totalShown === 0 ? (
            <EmptyState />
          ) : (
            <>
              {showFeatured && featured && (
                <div className="mb-4">
                  <ResearchCard
                    finding={featured}
                    featured
                    onWhy={() => setOpenId(featured.id)}
                    onAct={() => handleAct(featured)}
                  />
                </div>
              )}
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                {filtered.map((f) => (
                  <ResearchCard
                    key={f.id}
                    finding={f}
                    onWhy={() => setOpenId(f.id)}
                    onAct={() => handleAct(f)}
                  />
                ))}
              </div>
            </>
          )}
        </div>

        {/* Footer hint — gentle pointer to the explainability surface */}
        <footer className="mt-10 mb-4 flex flex-wrap items-center justify-between gap-3 text-[12px] text-neutral-500">
          <div className="inline-flex items-center gap-1.5">
            <ICON.Eye size={13} className="text-accent" />
            Every finding can be traced back to its sources and the classifier that flagged it.
          </div>
          <a href="#sources" className="text-accent hover:text-accent-solid font-medium">
            Manage sources →
          </a>
        </footer>
      </div>

      {/* Drawer */}
      {opened && (
        <DetailDrawer finding={opened} onClose={() => setOpenId(null)} />
      )}

      {/* Toast */}
      {toast && (
        <div className="fixed bottom-5 left-1/2 -translate-x-1/2 z-50">
          <div className="rounded-lg bg-neutral-900 text-white shadow-lg px-3.5 py-2 text-[12.5px] font-medium inline-flex items-center gap-2">
            <ICON.Check size={13} className="text-safe-solid" />
            {toast}
          </div>
        </div>
      )}
    </main>
  );
}

;(window as unknown as { Research: typeof Research }).Research = Research;
