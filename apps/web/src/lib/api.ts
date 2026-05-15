/**
 * API client for communicating with the Go backend gateway.
 *
 * All requests go through the gateway service which handles
 * authentication, tenant routing, and rate limiting.
 */

import type {
  Plan,
  Recipe,
  Squad,
  ApprovalRequest,
  ResearchTopic,
  MemoryEntry,
  Event,
  PolicyEvaluation,
  HeartbeatStatus,
} from "@irongolem/schema";

/* ------------------------------------------------------------------ */
/*  Configuration                                                      */
/* ------------------------------------------------------------------ */

const DEFAULT_BASE_URL = "/api/v1";

interface ApiConfig {
  baseUrl: string;
  /** Auth token injected on every request. */
  token: string | null;
  /** Workspace ID for multi-tenant routing. */
  workspaceId: string | null;
}

const config: ApiConfig = {
  baseUrl: DEFAULT_BASE_URL,
  token: null,
  workspaceId: null,
};

/** Initialise the API client. Call once at app boot. */
export function configure(opts: Partial<ApiConfig>): void {
  Object.assign(config, opts);
}

/* ------------------------------------------------------------------ */
/*  HTTP helpers                                                       */
/* ------------------------------------------------------------------ */

class ApiError extends Error {
  constructor(
    public readonly status: number,
    public readonly statusText: string,
    public readonly body: unknown,
  ) {
    super(`API ${status}: ${statusText}`);
    this.name = "ApiError";
  }
}

async function request<T>(
  method: string,
  path: string,
  body?: unknown,
): Promise<T> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    Accept: "application/json",
  };

  if (config.token) {
    headers["Authorization"] = `Bearer ${config.token}`;
  }
  if (config.workspaceId) {
    headers["X-Workspace-Id"] = config.workspaceId;
  }

  const res = await fetch(`${config.baseUrl}${path}`, {
    method,
    headers,
    body: body ? JSON.stringify(body) : undefined,
  });

  if (!res.ok) {
    const errorBody = await res.json().catch(() => null);
    throw new ApiError(res.status, res.statusText, errorBody);
  }

  // 204 No Content
  if (res.status === 204) return undefined as T;

  return res.json() as Promise<T>;
}

function get<T>(path: string): Promise<T> {
  return request<T>("GET", path);
}

function post<T>(path: string, body?: unknown): Promise<T> {
  return request<T>("POST", path, body);
}

function put<T>(path: string, body?: unknown): Promise<T> {
  return request<T>("PUT", path, body);
}

function del<T = void>(path: string): Promise<T> {
  return request<T>("DELETE", path);
}

/* ------------------------------------------------------------------ */
/*  Paginated response                                                 */
/* ------------------------------------------------------------------ */

export interface PaginatedResponse<T> {
  readonly items: readonly T[];
  readonly total: number;
  readonly page: number;
  readonly pageSize: number;
}

interface PaginationParams {
  page?: number;
  pageSize?: number;
}

function paginationQuery(p?: PaginationParams): string {
  if (!p) return "";
  const params = new URLSearchParams();
  if (p.page !== undefined) params.set("page", String(p.page));
  if (p.pageSize !== undefined) params.set("page_size", String(p.pageSize));
  const qs = params.toString();
  return qs ? `?${qs}` : "";
}

/* ------------------------------------------------------------------ */
/*  Domain endpoints                                                   */
/* ------------------------------------------------------------------ */

/** Health / heartbeat */
export const health = {
  getStatus(): Promise<{ status: HeartbeatStatus; message: string; uptimeSeconds: number }> {
    return get("/health/status");
  },

  getTimeline(params?: PaginationParams): Promise<PaginatedResponse<Event>> {
    return get(`/health/timeline${paginationQuery(params)}`);
  },
};

/** Plans */
export const plans = {
  list(params?: PaginationParams): Promise<PaginatedResponse<Plan>> {
    return get(`/plans${paginationQuery(params)}`);
  },

  get(id: string): Promise<Plan> {
    return get(`/plans/${id}`);
  },

  pause(id: string): Promise<Plan> {
    return post(`/plans/${id}/pause`);
  },

  resume(id: string): Promise<Plan> {
    return post(`/plans/${id}/resume`);
  },

  rollback(id: string): Promise<Plan> {
    return post(`/plans/${id}/rollback`);
  },
};

/** Recipes */
export const recipes = {
  list(params?: PaginationParams): Promise<PaginatedResponse<Recipe>> {
    return get(`/recipes${paginationQuery(params)}`);
  },

  get(id: string): Promise<Recipe> {
    return get(`/recipes/${id}`);
  },

  activate(id: string): Promise<Recipe> {
    return post(`/recipes/${id}/activate`);
  },

  deactivate(id: string): Promise<Recipe> {
    return post(`/recipes/${id}/deactivate`);
  },
};

/** Squads */
export const squads = {
  list(): Promise<readonly Squad[]> {
    return get("/squads");
  },

  get(id: string): Promise<Squad> {
    return get(`/squads/${id}`);
  },
};

/** Approvals */
export const approvals = {
  listPending(params?: PaginationParams): Promise<PaginatedResponse<ApprovalRequest>> {
    return get(`/approvals/pending${paginationQuery(params)}`);
  },

  approve(id: string): Promise<ApprovalRequest> {
    return post(`/approvals/${id}/approve`);
  },

  deny(id: string, reason?: string): Promise<ApprovalRequest> {
    return post(`/approvals/${id}/deny`, reason ? { reason } : undefined);
  },
};

/** Research */
export const research = {
  listTopics(params?: PaginationParams): Promise<PaginatedResponse<ResearchTopic>> {
    return get(`/research/topics${paginationQuery(params)}`);
  },

  getTopic(id: string): Promise<ResearchTopic> {
    return get(`/research/topics/${id}`);
  },

  refresh(topicId: string): Promise<void> {
    return post(`/research/topics/${topicId}/refresh`);
  },
};

/** Memory */
export const memory = {
  list(params?: PaginationParams): Promise<PaginatedResponse<MemoryEntry>> {
    return get(`/memory${paginationQuery(params)}`);
  },

  get(id: string): Promise<MemoryEntry> {
    return get(`/memory/${id}`);
  },

  getConnections(id: string): Promise<readonly MemoryEntry[]> {
    return get(`/memory/${id}/connections`);
  },
};

/** Security / policy */
export const security = {
  getBlockedActions(params?: PaginationParams): Promise<PaginatedResponse<Event>> {
    return get(`/security/blocked${paginationQuery(params)}`);
  },

  getQuarantinedItems(params?: PaginationParams): Promise<PaginatedResponse<Event>> {
    return get(`/security/quarantined${paginationQuery(params)}`);
  },

  getPolicyCoverage(): Promise<PolicyEvaluation[]> {
    return get("/security/policy-coverage");
  },
};

/** Events */
export const events = {
  list(params?: PaginationParams & { kind?: string }): Promise<PaginatedResponse<Event>> {
    const base = paginationQuery(params);
    const sep = base ? "&" : "?";
    const kindParam = params?.kind ? `${sep}kind=${params.kind}` : "";
    return get(`/events${base}${kindParam}`);
  },

  get(id: string): Promise<Event> {
    return get(`/events/${id}`);
  },
};

/* ------------------------------------------------------------------ */
/*  v2 namespace — mock/real toggle for the redesigned UI              */
/* ------------------------------------------------------------------ */

import * as homeMock from "../_mocks/home";
import * as inboxMock from "../_mocks/inbox";
import * as recipesMock from "../_mocks/recipes";
import * as researchMock from "../_mocks/research";
import * as memoryMock from "../_mocks/memory";
import * as healthMock from "../_mocks/health";
import * as securityMock from "../_mocks/security";
import * as settingsMock from "../_mocks/settings";

export type {
  EventItem,
  EventStatus,
  Team,
  ResearchFinding,
  HeartbeatState,
  WorkspaceInfo,
  SafetyShape as HomeSafetyShape,
  SafetyLayer,
} from "../_mocks/home";
export type {
  Item as InboxItem,
  Source as InboxSource,
  Risk as InboxRisk,
  Status as InboxStatus,
  Draft as InboxDraft,
  EmailDraft,
  CalendarDraft,
  WebhookDraft,
  TelegramDraft,
  SafetyShape as InboxSafetyShape,
  AuditStep,
} from "../_mocks/inbox";
export type {
  Recipe,
  Category as RecipeCategory,
  RecipeStatus,
  Risk as RecipeRisk,
  PermScope,
  Permission as RecipePermission,
  SafetyShape as RecipeSafetyShape,
  PolicyLayer as RecipePolicyLayer,
  RunOutcome,
  RunEvent,
} from "../_mocks/recipes";
export type {
  Finding as ResearchFinding2,
  Topic as ResearchTopic2,
  Impact as ResearchImpact,
  Action as ResearchAction,
  SourceKind as ResearchSourceKind,
  Agreement as ResearchAgreement,
  SourceSnippet,
  ClassifierTrace,
} from "../_mocks/research";
export type {
  MemoryItem,
  Subject as MemorySubject,
  Evidence as MemoryEvidence,
} from "../_mocks/memory";
export type {
  HealthComponent,
  CanonicalState as HealthState,
  ComponentCategory,
  HealStory,
  HealEvent,
  PredictiveWarning,
} from "../_mocks/health";
export type {
  Layer as SecurityLayer,
  LayerId,
  LayerState,
  Scope as SecurityScope,
  AuditEntry as SecurityAuditEntry,
  AuditKind as SecurityAuditKind,
  Policy as SecurityPolicy,
  PolicyState,
  PolicyHistoryEntry,
} from "../_mocks/security";
export type {
  Connector,
  ConnectorState,
  ConnectorScope,
  DeploymentMode,
  ModeCard,
  RecipeRequest,
  RequestStatus,
  Session as SettingsSession,
  Operator as SettingsOperator,
  Workspace as SettingsWorkspace,
} from "../_mocks/settings";

/**
 * Decide mock vs real for one v2 route.
 *
 * Resolution order (first match wins):
 *   1. `VITE_API_MODE_<route>=real|mock` — per-route override.
 *   2. `VITE_API_MODE=real|mock` — workspace-wide override.
 *   3. Default: mock.
 *
 * Per-route overrides exist because the v0.1 backend lands endpoint-by-endpoint
 * (Inbox first, then Health, then Home). We don't want flipping Inbox to real
 * to also flip Recipes/Research/Memory — those stay on mocks until v0.2.
 */
type V2Route = "home" | "inbox" | "recipes" | "research" | "memory" | "health" | "security" | "settings";

function isRealForRoute(route: V2Route): boolean {
  const env = import.meta.env as Record<string, string | undefined>;
  const perRoute = env[`VITE_API_MODE_${route.toUpperCase()}`];
  if (perRoute === "real") return true;
  if (perRoute === "mock") return false;
  return env.VITE_API_MODE === "real";
}

export const v2 = {
  home: {
    getMock: () => ({
      workspace: homeMock.mockWorkspace,
      heartbeat: homeMock.mockHeartbeat,
      teams: homeMock.mockTeams,
      trustHistory: homeMock.mockTrustHistory,
      safety: homeMock.mockSafety,
      researchFindings: homeMock.mockResearchFindings,
      events: homeMock.mockInitialEvents,
    }),
    /**
     * v0.2 Step 6 wired the live `/api/v1/home` endpoint
     * (services/gateway/internal/handler/home.go). When
     * `VITE_API_MODE_HOME=real` the call goes to the gateway; otherwise
     * the mock object resolves synchronously through the Promise.
     */
    async load() {
      if (!isRealForRoute("home")) return v2.home.getMock();
      return get<ReturnType<typeof v2.home.getMock>>("/home");
    },
  },
  inbox: {
    getMock: () => inboxMock.mockInboxItems,
    /**
     * v0.2 Step 3 wired the live `/api/v1/inbox` endpoint on the gateway
     * (see services/gateway/internal/handler/inbox.go). When
     * `VITE_API_MODE_INBOX=real` the call goes to the gateway; otherwise
     * the mock array is returned synchronously through the Promise.
     *
     * The gateway shape is `{ items, total, page, page_size }` to mirror
     * the existing `/events` pagination contract; we unwrap `items` here
     * so the page-level code stays oblivious to the envelope.
     */
    async list() {
      if (!isRealForRoute("inbox")) return v2.inbox.getMock();
      const envelope = await get<{ items: typeof inboxMock.mockInboxItems }>("/inbox");
      return envelope.items;
    },
  },
  recipes: {
    getMock: () => ({
      recipes: recipesMock.mockRecipes,
      fiveLayers: recipesMock.mockFiveLayers,
    }),
    async list() {
      if (!isRealForRoute("recipes")) return v2.recipes.getMock();
      return get<ReturnType<typeof v2.recipes.getMock>>("/v2/recipes");
    },
  },
  research: {
    getMock: () => ({
      findings: researchMock.mockFindings,
      quietlyArchivedToday: researchMock.mockQuietlyArchivedToday,
      sourcesMonitored: researchMock.mockSourcesMonitored,
    }),
    async load() {
      if (!isRealForRoute("research")) return v2.research.getMock();
      return get<ReturnType<typeof v2.research.getMock>>("/v2/research");
    },
  },
  memory: {
    getMock: () => memoryMock.mockMemory,
    async list() {
      if (!isRealForRoute("memory")) return v2.memory.getMock();
      return get<typeof memoryMock.mockMemory>("/v2/memory");
    },
  },
  health: {
    getMock: () => ({
      components: healthMock.mockComponents,
      healEvents: healthMock.mockHealEvents,
      predictive: healthMock.mockPredictive,
    }),
    /**
     * v0.2 Step 6 wired `/api/v1/health/status` to return components
     * (probed from gateway + db + connectors) plus empty healEvents +
     * predictive until the self-heal log lands in v0.3+.
     */
    async load() {
      if (!isRealForRoute("health")) return v2.health.getMock();
      return get<ReturnType<typeof v2.health.getMock>>("/health/status");
    },
  },
  security: {
    getMock: () => ({
      layers: securityMock.mockLayers,
      policies: securityMock.mockPoliciesInitial,
      audit: securityMock.mockAudit,
    }),
    async load() {
      if (!isRealForRoute("security")) return v2.security.getMock();
      return get<ReturnType<typeof v2.security.getMock>>("/v2/security");
    },
  },
  settings: {
    getMock: () => ({
      operator: settingsMock.mockOperator,
      workspace: settingsMock.mockWorkspace,
      sessions: settingsMock.mockSessions,
      connectors: settingsMock.mockConnectors,
      modes: settingsMock.mockModes,
      recipeRequests: settingsMock.mockRecipeRequests,
    }),
    async load() {
      if (!isRealForRoute("settings")) return v2.settings.getMock();
      return get<ReturnType<typeof v2.settings.getMock>>("/v2/settings");
    },
  },
} as const;

/** Aggregate API namespace. */
export const api = {
  configure,
  health,
  plans,
  recipes,
  squads,
  approvals,
  research,
  memory,
  security,
  events,
  v2,
} as const;

export default api;
