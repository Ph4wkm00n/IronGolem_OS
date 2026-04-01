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
} as const;

export default api;
