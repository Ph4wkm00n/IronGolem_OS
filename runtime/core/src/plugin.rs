//! Plugin permission vocabulary.
//!
//! v0.4 adoption wave — Rust mirror of `packages/schema/src/plugins.ts`.
//! Locks in the closed resource/action vocabulary a plugin manifest may
//! request before the plugin loader lands, so install-time review can
//! diff an enumerable permission surface instead of free text. The two
//! lists MUST stay label-identical with the TS side; both carry a sync
//! test (the hook-decision pattern from v0.3 Step 2).

use serde::{Deserialize, Serialize};

/// Every resource a plugin permission may name. Wire format is the
/// lowercase string, identical to the TS union.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum PluginPermissionResource {
    Connectors,
    Events,
    Llm,
    Storage,
    Network,
    Commitments,
    Audit,
    Ui,
}

impl PluginPermissionResource {
    /// All resources, in the same order as
    /// `PLUGIN_PERMISSION_RESOURCES` in schema/src/plugins.ts.
    pub const ALL: [PluginPermissionResource; 8] = [
        PluginPermissionResource::Connectors,
        PluginPermissionResource::Events,
        PluginPermissionResource::Llm,
        PluginPermissionResource::Storage,
        PluginPermissionResource::Network,
        PluginPermissionResource::Commitments,
        PluginPermissionResource::Audit,
        PluginPermissionResource::Ui,
    ];
}

/// Every action a plugin permission may request on a resource.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum PluginPermissionAction {
    Read,
    Write,
    Execute,
    Subscribe,
}

impl PluginPermissionAction {
    /// All actions, same order as `PLUGIN_PERMISSION_ACTIONS` in TS.
    pub const ALL: [PluginPermissionAction; 4] = [
        PluginPermissionAction::Read,
        PluginPermissionAction::Write,
        PluginPermissionAction::Execute,
        PluginPermissionAction::Subscribe,
    ];
}

/// One declared permission: a resource plus the actions needed on it.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct PluginPermissionDeclaration {
    pub resource: PluginPermissionResource,
    pub actions: Vec<PluginPermissionAction>,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn resource_wire_format_matches_schema_ts() {
        // These strings are duplicated in packages/schema/src/plugins.ts
        // (PLUGIN_PERMISSION_RESOURCES) — if either side drifts, manifest
        // validation disagrees between domains.
        let expected = [
            "connectors",
            "events",
            "llm",
            "storage",
            "network",
            "commitments",
            "audit",
            "ui",
        ];
        for (resource, want) in PluginPermissionResource::ALL.iter().zip(expected) {
            let got = serde_json::to_string(resource).expect("serialize resource");
            assert_eq!(got, format!("\"{want}\""));
        }
    }

    #[test]
    fn action_wire_format_matches_schema_ts() {
        let expected = ["read", "write", "execute", "subscribe"];
        for (action, want) in PluginPermissionAction::ALL.iter().zip(expected) {
            let got = serde_json::to_string(action).expect("serialize action");
            assert_eq!(got, format!("\"{want}\""));
        }
    }

    #[test]
    fn unknown_resource_rejected_on_deserialize() {
        let err = serde_json::from_str::<PluginPermissionDeclaration>(
            r#"{"resource":"filesystem","actions":["read"]}"#,
        );
        assert!(err.is_err(), "undeclared resource must not deserialize");

        let err = serde_json::from_str::<PluginPermissionDeclaration>(
            r#"{"resource":"llm","actions":["mine_bitcoin"]}"#,
        );
        assert!(err.is_err(), "undeclared action must not deserialize");
    }

    #[test]
    fn declaration_round_trips() {
        let decl = PluginPermissionDeclaration {
            resource: PluginPermissionResource::Llm,
            actions: vec![PluginPermissionAction::Execute],
        };
        let line = serde_json::to_string(&decl).expect("serialize");
        assert_eq!(line, r#"{"resource":"llm","actions":["execute"]}"#);
        let back: PluginPermissionDeclaration = serde_json::from_str(&line).expect("deserialize");
        assert_eq!(back, decl);
    }
}
