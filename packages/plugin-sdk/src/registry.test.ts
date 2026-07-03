import { describe, expect, test } from "bun:test";
import {
  AllowAllPolicyChecker,
  GrantedPermissionsPolicyChecker,
  PluginRegistry,
  PluginValidationError,
} from "./registry";
import type { Plugin, PluginManifest, PluginPermission } from "./types";

function manifestWith(permissions: PluginPermission[]): PluginManifest {
  return {
    id: "com.example.test",
    name: "Test Plugin",
    version: "1.0.0",
    description: "test",
    author: "tests",
    entryPoint: "index.ts",
    capabilities: ["tool"],
    permissions,
  };
}

function pluginStub(m: PluginManifest): Plugin {
  return {
    id: m.id,
    name: m.name,
    version: m.version,
    description: m.description,
    author: m.author,
    capabilities: m.capabilities,
    hooks: [],
  };
}

describe("manifest permission vocabulary", () => {
  test("permissions from the closed vocabulary validate", () => {
    const registry = new PluginRegistry(new AllowAllPolicyChecker());
    const m = manifestWith([{ resource: "llm", actions: ["execute"] }]);
    expect(registry.register(m, pluginStub(m)).state).toBe("validated");
  });

  test("unknown resource is rejected at validation", () => {
    const registry = new PluginRegistry(new AllowAllPolicyChecker());
    const m = manifestWith([{ resource: "filesystem", actions: ["read"] }]);
    expect(() => registry.register(m, pluginStub(m))).toThrow(PluginValidationError);
    expect(() => registry.register(m, pluginStub(m))).toThrow(/unknown resource "filesystem"/);
  });

  test("unknown action is rejected at validation", () => {
    const registry = new PluginRegistry(new AllowAllPolicyChecker());
    const m = manifestWith([{ resource: "llm", actions: ["mine_bitcoin"] }]);
    expect(() => registry.register(m, pluginStub(m))).toThrow(/unknown action "mine_bitcoin"/);
  });

  test("empty actions list is rejected", () => {
    const registry = new PluginRegistry(new AllowAllPolicyChecker());
    const m = manifestWith([{ resource: "events", actions: [] }]);
    expect(() => registry.register(m, pluginStub(m))).toThrow(/at least one action/);
  });

  test("duplicate resource declaration is rejected", () => {
    const registry = new PluginRegistry(new AllowAllPolicyChecker());
    const m = manifestWith([
      { resource: "events", actions: ["read"] },
      { resource: "events", actions: ["subscribe"] },
    ]);
    expect(() => registry.register(m, pluginStub(m))).toThrow(/duplicate resource/);
  });
});

describe("deny-by-default policy", () => {
  test("default registry denies any permission-requesting plugin", () => {
    const registry = new PluginRegistry(); // no checker: deny-by-default
    const m = manifestWith([{ resource: "storage", actions: ["read"] }]);
    expect(() => registry.register(m, pluginStub(m))).toThrow(
      /not allowed by workspace policy/,
    );
  });

  test("default registry accepts a zero-permission plugin", () => {
    const registry = new PluginRegistry();
    const m = manifestWith([]);
    expect(registry.register(m, pluginStub(m)).state).toBe("validated");
  });

  test("granted resource+action passes, missing action fails", () => {
    const granted = new GrantedPermissionsPolicyChecker([
      { resource: "storage", actions: ["read"] },
    ]);
    expect(granted.arePermissionsAllowed([{ resource: "storage", actions: ["read"] }])).toBe(
      true,
    );
    expect(
      granted.arePermissionsAllowed([{ resource: "storage", actions: ["read", "write"] }]),
    ).toBe(false);
    expect(granted.arePermissionsAllowed([{ resource: "network", actions: ["read"] }])).toBe(
      false,
    );
  });
});
