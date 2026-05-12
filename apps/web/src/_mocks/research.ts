// route: /research — typed mock data for the Research page.
// Consumed via `api.v2.research.getMock()`; never imported by pages directly.
// TODO: align with @irongolem/schema once ResearchTopic/Finding models stabilise.

export type Topic = "pricing" | "api" | "supplier" | "industry" | "internal";
export type Impact = "low" | "medium" | "high";
export type Action = "apply-finding" | "mark-reviewed" | "discuss-standup";
export type SourceKind = "release-notes" | "index" | "alert" | "paper" | "digest" | "filing" | "blog";
export type Agreement = "agrees" | "conflicts" | "neutral";

export interface SourceSnippet {
  readonly id: string;
  readonly name: string;
  readonly url: string;
  readonly kind: SourceKind;
  readonly publishedAt: string;
  readonly snippet: string;
  readonly agreement: Agreement;
}

export interface ClassifierTrace {
  readonly name: string;
  readonly rule: string;
  readonly confidence: number;
}

export interface Finding {
  readonly id: string;
  readonly title: string;
  readonly summary: string;
  readonly topic: Topic;
  readonly impact: Impact;
  readonly confidence: number;
  readonly freshness: string;
  readonly freshnessMinutes: number;
  readonly primarySource: string;
  readonly sources: readonly SourceSnippet[];
  readonly contradictionCount: number;
  readonly classifier: ClassifierTrace;
  readonly suggestedAction: Action;
  readonly featured?: boolean;
}

export const mockQuietlyArchivedToday = 1284;
export const mockSourcesMonitored = 47;

export const mockFindings: readonly Finding[] = [
  {
    id: "f01",
    title: "Carbon credit spot price up 11% overnight, broadest move in three months",
    summary:
      "Two of three approved price sources show carbon spot at $89.40 this morning, an 11% jump on the three-month range. The move is concentrated in N-American voluntary credits; compliance markets are flat. Your Q3 Riverbend purchase draft references the old price.",
    topic: "pricing", impact: "high", confidence: 92, freshness: "2h ago", freshnessMinutes: 120,
    primarySource: "bloomberg.terminal/cc.spot",
    sources: [
      { id: "s01a", name: "Bloomberg carbon spot", url: "bloomberg.terminal/cc.spot", kind: "index", publishedAt: "2h ago", snippet: "Voluntary carbon spot closes $89.40, +11.2% on 90-day MA. North-American basket leads gains; compliance basket -0.3%.", agreement: "agrees" },
      { id: "s01b", name: "S&P Platts CCM", url: "spglobal.com/ccm/daily", kind: "index", publishedAt: "3h ago", snippet: "CCM index +10.8%, with NA-VCS posting strongest single-session gain since February.", agreement: "agrees" },
      { id: "s01c", name: "Reuters commodities desk", url: "reuters.com/markets/commodities", kind: "release-notes", publishedAt: "4h ago", snippet: "Bid-ask spread widened materially overnight; one regional desk reports paper at $84.10 — below other prints.", agreement: "conflicts" },
    ],
    contradictionCount: 1,
    classifier: { name: "PriceMoveDetector v3", rule: "Move ≥5% on ≥2 approved sources within 6h window", confidence: 0.92 },
    suggestedAction: "apply-finding", featured: true,
  },
  {
    id: "f02",
    title: "Stripe deprecating legacy webhook payload format on Aug 1",
    summary:
      "Stripe published a deprecation notice for the v1 webhook payload format used by your dispute-evidence pipeline. The v2 format adds a signature header and renames three fields. Your `acknowledge Stripe disputes` recipe will need its handler updated before August.",
    topic: "api", impact: "high", confidence: 96, freshness: "5h ago", freshnessMinutes: 300,
    primarySource: "stripe.com/changelog/2026-05",
    sources: [
      { id: "s02a", name: "Stripe changelog", url: "stripe.com/changelog/2026-05", kind: "release-notes", publishedAt: "5h ago", snippet: "v1 webhook payload is deprecated effective Aug 1, 2026. Endpoints not migrated will continue to receive payloads but headers will be downgraded.", agreement: "agrees" },
      { id: "s02b", name: "Stripe developer mailing list", url: "groups.stripe.com/devs", kind: "alert", publishedAt: "5h ago", snippet: "Direct notice to integrators using v1 payloads on the dispute.* and charge.dispute.* topics. Migration guide linked.", agreement: "agrees" },
    ],
    contradictionCount: 0,
    classifier: { name: "ChangelogWatcher v2", rule: "Deprecation keyword + endpoint in active recipe permission list", confidence: 0.96 },
    suggestedAction: "apply-finding",
  },
  {
    id: "f03",
    title: "Yates Holdings filed an updated W-9; bank routing has changed",
    summary:
      "Yates Holdings, a vendor on your standing-purchase list, filed a new W-9 with a different routing number than the one on record. The change is not yet reflected in your vendor table. Any auto-approved standing-order PO would still use the old routing.",
    topic: "supplier", impact: "high", confidence: 88, freshness: "yesterday", freshnessMinutes: 60 * 26,
    primarySource: "irs.gov/filings/yates-holdings",
    sources: [
      { id: "s03a", name: "IRS public filings", url: "irs.gov/filings/yates-holdings", kind: "filing", publishedAt: "yesterday", snippet: "W-9 amendment filed 2026-05-10. Address unchanged; bank routing updated.", agreement: "agrees" },
      { id: "s03b", name: "Yates AP team email", url: "ops@yatesholdings.com", kind: "alert", publishedAt: "yesterday", snippet: "FYI — please update payment details. New routing/account on the attached letter.", agreement: "agrees" },
      { id: "s03c", name: "Workspace vendor table", url: "internal://vendors/yates", kind: "filing", publishedAt: "12d ago", snippet: "Routing on file: 121-000-358 (Bank of the West). Last verified 12 days ago.", agreement: "conflicts" },
    ],
    contradictionCount: 1,
    classifier: { name: "VendorBankChange v1", rule: "External W-9 routing differs from internal vendor table", confidence: 0.88 },
    suggestedAction: "apply-finding",
  },
  {
    id: "f04",
    title: "OpenAI lowered pricing on the model your drafting pipeline uses",
    summary:
      "The model your drafting recipes route through dropped 35% in input price and 20% in output price, effective immediately. No quality or rate-limit changes were announced. Your monthly drafting spend would drop roughly $84/month at current volume.",
    topic: "pricing", impact: "medium", confidence: 94, freshness: "8h ago", freshnessMinutes: 480,
    primarySource: "openai.com/pricing",
    sources: [
      { id: "s04a", name: "OpenAI pricing page", url: "openai.com/pricing", kind: "release-notes", publishedAt: "8h ago", snippet: "Input pricing reduced from $0.20 → $0.13 per 1M tokens. Output pricing reduced from $0.80 → $0.64 per 1M tokens.", agreement: "agrees" },
      { id: "s04b", name: "OpenAI blog", url: "openai.com/blog/pricing-may-2026", kind: "blog", publishedAt: "8h ago", snippet: "Effective immediately across all standard tiers. Enterprise contracts unaffected.", agreement: "agrees" },
    ],
    contradictionCount: 0,
    classifier: { name: "PricingWatch v4", rule: "Vendor in active recipe list has pricing delta ≥10%", confidence: 0.94 },
    suggestedAction: "mark-reviewed",
  },
  {
    id: "f05",
    title: "Pump-supplier Halford issued a recall on lot 24-118 maintenance pumps",
    summary:
      "Halford notified affected accounts that pumps shipped under lot 24-118 may have a sealing defect. Your maintenance PO from April referenced lot 24-118. The recall offers replacement at no cost; nothing on your side is overdue yet.",
    topic: "supplier", impact: "high", confidence: 90, freshness: "yesterday", freshnessMinutes: 60 * 30,
    primarySource: "halford.io/recall/2024-118",
    sources: [
      { id: "s05a", name: "Halford recall notice", url: "halford.io/recall/2024-118", kind: "alert", publishedAt: "yesterday", snippet: "Lot 24-118 may exhibit premature seal failure under continuous duty. Replacement available through standard RMA.", agreement: "agrees" },
      { id: "s05b", name: "Workspace PO history", url: "internal://purchasing/po", kind: "filing", publishedAt: "1mo ago", snippet: "PO-24-099 references lot 24-118, pump replacement, qty 2.", agreement: "agrees" },
    ],
    contradictionCount: 0,
    classifier: { name: "RecallWatcher v2", rule: "Supplier recall lot ID matches internal PO record", confidence: 0.90 },
    suggestedAction: "apply-finding",
  },
  {
    id: "f06",
    title: "Lithium spot off 4% on stronger inventory data — directional, not material",
    summary:
      "Three approved price sources show lithium carbonate spot down 4% on the week, attributed to looser Q2 inventory reports out of Chile. The move is below your 5% material-change threshold; logged here as directional context only.",
    topic: "pricing", impact: "low", confidence: 79, freshness: "5h ago", freshnessMinutes: 300,
    primarySource: "spglobal.com/lithium",
    sources: [
      { id: "s06a", name: "S&P Platts lithium daily", url: "spglobal.com/lithium", kind: "index", publishedAt: "5h ago", snippet: "Spot lithium carbonate -3.9% on the week.", agreement: "agrees" },
      { id: "s06b", name: "Bloomberg metals", url: "bloomberg.terminal/metals.li", kind: "index", publishedAt: "6h ago", snippet: "Lithium carbonate prints -4.1%, in line with broader battery-metals weakness.", agreement: "agrees" },
      { id: "s06c", name: "Fastmarkets", url: "fastmarkets.com/lithium", kind: "index", publishedAt: "7h ago", snippet: "Index move -4.0%, attributed to looser Chile Q2 inventories.", agreement: "agrees" },
    ],
    contradictionCount: 0,
    classifier: { name: "PriceMoveDetector v3", rule: "Move <5% on watched index — directional log only", confidence: 0.79 },
    suggestedAction: "mark-reviewed",
  },
  {
    id: "f07",
    title: "EU AI Act enforcement guidance for SMEs published — actionable in Q4",
    summary:
      "The EU Commission released SME-specific enforcement guidance for the AI Act. Operator-in-the-loop systems (like this workspace) are explicitly carved out of the high-risk classification. Your existing safety posture appears compliant.",
    topic: "industry", impact: "medium", confidence: 86, freshness: "yesterday", freshnessMinutes: 60 * 22,
    primarySource: "ec.europa.eu/ai-act/sme",
    sources: [
      { id: "s07a", name: "EU Commission release", url: "ec.europa.eu/ai-act/sme", kind: "release-notes", publishedAt: "yesterday", snippet: "Operator-in-the-loop systems with auditable safety layers are not classified as high-risk under Annex III.", agreement: "agrees" },
      { id: "s07b", name: "Hartlaw client memo", url: "hartlaw.com/memos/2026-05-eu-ai-sme", kind: "digest", publishedAt: "12h ago", snippet: "Workspaces matching the operator-loop pattern (drafts surface for approval) likely qualify for the SME carve-out.", agreement: "agrees" },
    ],
    contradictionCount: 0,
    classifier: { name: "RegulatoryDigest v2", rule: "Jurisdiction match + SME + AI Act keyword cluster", confidence: 0.86 },
    suggestedAction: "mark-reviewed",
  },
  {
    id: "f08",
    title: "Telegram desktop client introduced a breaking change in chat-bot auth",
    summary:
      "Telegram pushed a desktop client change on May 9 that breaks the long-lived bot session your ops channel notifier uses. Re-auth is required; messages are queueing locally and will retry once auth is refreshed.",
    topic: "api", impact: "medium", confidence: 81, freshness: "yesterday", freshnessMinutes: 60 * 18,
    primarySource: "telegram.org/changelog/desktop",
    sources: [
      { id: "s08a", name: "Telegram changelog", url: "telegram.org/changelog/desktop", kind: "release-notes", publishedAt: "yesterday", snippet: "Desktop client 5.1.4 changes long-lived bot session handling; sessions must be re-authorized.", agreement: "agrees" },
      { id: "s08b", name: "Workspace ops bot logs", url: "internal://logs/ops-bot", kind: "filing", publishedAt: "yesterday", snippet: "Auth failures observed since May 9 14:02 PT; 4 queued messages pending retry.", agreement: "agrees" },
      { id: "s08c", name: "Independent dev forum", url: "telegram-dev.forum/sessions", kind: "blog", publishedAt: "12h ago", snippet: "Multiple operators reporting auth failures; Telegram acknowledged in support thread.", agreement: "neutral" },
    ],
    contradictionCount: 0,
    classifier: { name: "ChangelogWatcher v2", rule: "Vendor changelog mentions endpoint used by active recipe", confidence: 0.81 },
    suggestedAction: "discuss-standup",
  },
  {
    id: "f09",
    title: "Riverbend filed a procurement RFP with stated scope similar to your existing contract",
    summary:
      "Public procurement records show Riverbend opened an RFP for FY27 sourcing with line items overlapping your current MSA scope. This could indicate competitive re-bid; it could also be routine. Your renewal window opens in 11 weeks.",
    topic: "industry", impact: "high", confidence: 73, freshness: "10h ago", freshnessMinutes: 600,
    primarySource: "sam.gov/rfps/riverbend-fy27",
    sources: [
      { id: "s09a", name: "SAM.gov RFP filing", url: "sam.gov/rfps/riverbend-fy27", kind: "filing", publishedAt: "10h ago", snippet: "RFP-FY27-118 opened. Scope: dock-side uniforms, lot tracking, weekly cadence. Award date Aug 14.", agreement: "agrees" },
      { id: "s09b", name: "Riverbend procurement page", url: "riverbend.co/procurement", kind: "release-notes", publishedAt: "12h ago", snippet: "Routine FY27 procurement cycle; no incumbent prejudice. Incumbents are encouraged to respond.", agreement: "neutral" },
      { id: "s09c", name: "Industry analyst note", url: "industrybeat.com/riverbend-rfp", kind: "blog", publishedAt: "8h ago", snippet: "Some uncertainty whether this is a true re-bid or routine compliance. Incumbent advantage looks intact.", agreement: "neutral" },
    ],
    contradictionCount: 0,
    classifier: { name: "RFPWatcher v1", rule: "Customer name + scope keyword overlap with existing MSA", confidence: 0.73 },
    suggestedAction: "discuss-standup",
  },
  {
    id: "f10",
    title: "Bloomberg and Reuters disagree on natural gas Henry Hub print",
    summary:
      "Two of your three approved energy-pricing sources reported diverging prints for Henry Hub close: Bloomberg $2.84, Reuters $2.71. S&P Platts is offline for maintenance. Material to your facility utility forecast if confirmed.",
    topic: "pricing", impact: "medium", confidence: 64, freshness: "4h ago", freshnessMinutes: 240,
    primarySource: "bloomberg.terminal/ng.hh",
    sources: [
      { id: "s10a", name: "Bloomberg natural gas", url: "bloomberg.terminal/ng.hh", kind: "index", publishedAt: "4h ago", snippet: "Henry Hub close: $2.84/MMBtu, +1.1% on session.", agreement: "agrees" },
      { id: "s10b", name: "Reuters energy desk", url: "reuters.com/markets/energy", kind: "release-notes", publishedAt: "5h ago", snippet: "Henry Hub close prints $2.71/MMBtu, -3.5% on session.", agreement: "conflicts" },
      { id: "s10c", name: "S&P Platts (offline)", url: "spglobal.com/platts/ng", kind: "index", publishedAt: "—", snippet: "Source offline for scheduled maintenance.", agreement: "neutral" },
    ],
    contradictionCount: 1,
    classifier: { name: "PriceMoveDetector v3", rule: "Source disagreement >5% between approved sources", confidence: 0.64 },
    suggestedAction: "discuss-standup",
  },
  {
    id: "f11",
    title: "New paper: operator-in-the-loop AI systems show 4× lower error rates",
    summary:
      "A NeurIPS preprint compared operator-in-the-loop systems to fully autonomous agents across 12 production deployments. Loop systems showed 4× lower irrecoverable error rates with only marginal latency cost. Aligns with this workspace's safety posture.",
    topic: "industry", impact: "low", confidence: 82, freshness: "3d ago", freshnessMinutes: 60 * 24 * 3,
    primarySource: "arxiv.org/abs/2604.0118",
    sources: [
      { id: "s11a", name: "ArXiv preprint", url: "arxiv.org/abs/2604.0118", kind: "paper", publishedAt: "3d ago", snippet: "Operator-loop systems exhibit 4.1× lower irrecoverable error rate (n=412 incidents across 12 deployments).", agreement: "agrees" },
      { id: "s11b", name: "PaperDigest weekly", url: "paperdigest.com/2026-w19", kind: "digest", publishedAt: "2d ago", snippet: "Featured paper of the week; methodology is sound but sample size is modest.", agreement: "neutral" },
    ],
    contradictionCount: 0,
    classifier: { name: "PaperWatch v1", rule: "Paper matches workspace topic cluster + ≥1 trusted digest highlight", confidence: 0.82 },
    suggestedAction: "mark-reviewed",
  },
  {
    id: "f12",
    title: "Slack will charge for active webhook senders starting Q3",
    summary:
      "Slack's Q3 pricing update introduces a per-message fee for high-volume webhook senders. Your ops-bot sits at 12% of the proposed threshold today; expansion to a second channel could put it over. No immediate cost impact this quarter.",
    topic: "pricing", impact: "low", confidence: 71, freshness: "yesterday", freshnessMinutes: 60 * 28,
    primarySource: "slack.com/pricing-update-q3",
    sources: [
      { id: "s12a", name: "Slack pricing page", url: "slack.com/pricing-update-q3", kind: "release-notes", publishedAt: "yesterday", snippet: "Webhook senders above 10k messages/month will incur a $0.0008/message fee starting Q3.", agreement: "agrees" },
      { id: "s12b", name: "Workspace usage report", url: "internal://usage/ops-bot", kind: "filing", publishedAt: "yesterday", snippet: "Ops-bot 30-day average: 1,212 messages/month — about 12% of the new threshold.", agreement: "agrees" },
    ],
    contradictionCount: 0,
    classifier: { name: "PricingWatch v4", rule: "Vendor pricing change + workspace usage above 10%", confidence: 0.71 },
    suggestedAction: "mark-reviewed",
  },
  {
    id: "f13",
    title: "Two analysts disagree on Trent & Co liquidity outlook",
    summary:
      "Trent & Co, a current customer with outstanding wire instructions, drew divergent analyst notes today. Moody's flagged a covenant-watch event; Fitch issued an affirm with stable outlook. Worth surfacing — your AP team has open exposure.",
    topic: "supplier", impact: "medium", confidence: 68, freshness: "6h ago", freshnessMinutes: 360,
    primarySource: "moodys.com/notes/trentco-2026",
    sources: [
      { id: "s13a", name: "Moody's analyst note", url: "moodys.com/notes/trentco-2026", kind: "alert", publishedAt: "6h ago", snippet: "Covenant-watch event triggered by Q1 leverage drift. Watchlist with negative implication.", agreement: "agrees" },
      { id: "s13b", name: "Fitch analyst note", url: "fitchratings.com/notes/trentco", kind: "alert", publishedAt: "7h ago", snippet: "Affirms BBB- with stable outlook. Covenant headroom remains adequate per Q1 disclosures.", agreement: "conflicts" },
    ],
    contradictionCount: 1,
    classifier: { name: "CounterpartyMonitor v2", rule: "Two trusted analysts diverge on the same counterparty within 24h", confidence: 0.68 },
    suggestedAction: "discuss-standup",
  },
  {
    id: "f14",
    title: "Internal: heartbeat dropped one system for 18 minutes last night",
    summary:
      "Workspace heartbeat showed 18/19 systems green between 02:14 and 02:32 PT last night. The degraded system was the cold-storage roller; logs indicate a transient S3 throttle. Self-healed without operator intervention.",
    topic: "internal", impact: "low", confidence: 97, freshness: "10h ago", freshnessMinutes: 600,
    primarySource: "internal://heartbeat/log",
    sources: [
      { id: "s14a", name: "Heartbeat history", url: "internal://heartbeat/log", kind: "filing", publishedAt: "10h ago", snippet: "System 'cold-storage roller' degraded 02:14–02:32 PT. Recovered without operator action.", agreement: "agrees" },
      { id: "s14b", name: "Cloud provider status", url: "status.cloud-provider.com", kind: "alert", publishedAt: "10h ago", snippet: "S3 partial throttling observed in us-west-2 between 09:00 and 09:35 UTC.", agreement: "agrees" },
    ],
    contradictionCount: 0,
    classifier: { name: "InternalHealthDigest v1", rule: "Heartbeat dip with external corroboration", confidence: 0.97 },
    suggestedAction: "mark-reviewed",
  },
];
