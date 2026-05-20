/**
 * Provider profile types — mirrors `runtime/runtimed/src/provider.rs`
 * `ProviderProfile` exactly.
 *
 * v0.3 Step 3 of `Plans/modular-puzzling-blum.md`. The settings UI uses
 * this shape to render "currently active" alongside available providers
 * by calling `GET /api/v1/providers`. Field names + casing match the
 * serde wire format on the Rust side; any drift breaks the round-trip.
 *
 * Secrets are NEVER part of this shape. `apiKeyEnv` names the env var
 * that holds the secret; the secret value never leaves the runtime
 * process.
 */

/** Authentication mode declared by a `ProviderProfile`. */
export type AuthType = "api_key" | "oauth" | "bedrock" | "none";

/** Stable wire identifier — matches the Rust serde lowercase form. */
export type ProviderName = "mock" | "anthropic" | "openai" | (string & {});

/**
 * Declarative provider metadata.
 *
 * The `default_temperature` semantics: `null` means "do not send the
 * field" — some providers refuse the parameter (Kimi/Moonshot) and
 * others apply their own default when omitted. A numeric value means
 * "send this on every request". Adopted from hermes-agent's
 * `OMIT_TEMPERATURE` sentinel.
 */
export interface ProviderProfile {
  /** Stable wire identifier — matches `ProviderKind` serialized form. */
  readonly name: ProviderName;
  /** Human-readable label for the settings UI. */
  readonly display_name: string;
  /** How the provider authenticates. v0.3 wires only `api_key`. */
  readonly auth_type: AuthType;
  /** Inference base URL. Empty string for `mock`. */
  readonly base_url: string;
  /** Optional explicit models endpoint. When unset, callers fall back
   *  to `${base_url}/models`. */
  readonly models_url: string | null;
  /** Static, non-sensitive headers sent on every request. */
  readonly default_headers: Record<string, string>;
  /** Default temperature; `null` means "do not send the field". */
  readonly fixed_temperature: number | null;
  /** Default `max_tokens` value. */
  readonly default_max_tokens: number;
  /** Curated model list shown when the live models endpoint fails. */
  readonly fallback_models: readonly string[];
  /** Env var name the API key is read from. Empty for `mock`. */
  readonly api_key_env: string;
}

/** Wire shape of `GET /api/v1/providers`. */
export interface ProvidersResponse {
  /** `name` field of the active provider. */
  readonly active: ProviderName;
  /** Every known profile — active included. Stable order. */
  readonly profiles: readonly ProviderProfile[];
}

/** Convenience: locate the active profile in a `ProvidersResponse`.
 *  Returns `undefined` when the active name is missing from the list —
 *  treat that as a transient error and refetch. */
export function activeProfile(
  resp: ProvidersResponse,
): ProviderProfile | undefined {
  return resp.profiles.find((p) => p.name === resp.active);
}
