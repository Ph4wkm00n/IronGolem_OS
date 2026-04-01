//! Risk metadata types. Risk information propagates through every action in
//! the system and is used by the policy engine and defense module.

use serde::{Deserialize, Serialize};

/// Risk level classification for actions and events.
#[derive(Debug, Clone, Copy, Default, PartialEq, Eq, PartialOrd, Ord, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum RiskLevel {
    /// No notable risk.
    #[default]
    None,
    /// Low risk; can proceed automatically.
    Low,
    /// Medium risk; may require approval depending on policy.
    Medium,
    /// High risk; typically requires explicit approval.
    High,
    /// Critical risk; requires admin-level approval.
    Critical,
}

/// Risk metadata attached to events, plan nodes, and actions.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RiskMetadata {
    /// Overall risk level.
    pub level: RiskLevel,
    /// Numeric risk score (0.0 to 1.0).
    pub score: f64,
    /// Categories of risk detected.
    pub categories: Vec<RiskCategory>,
    /// Human-readable risk explanation for the UI.
    pub explanation: Option<String>,
}

impl Default for RiskMetadata {
    fn default() -> Self {
        Self {
            level: RiskLevel::None,
            score: 0.0,
            categories: Vec::new(),
            explanation: None,
        }
    }
}

impl RiskMetadata {
    /// Create risk metadata with a specific level.
    pub fn with_level(level: RiskLevel) -> Self {
        let score = match level {
            RiskLevel::None => 0.0,
            RiskLevel::Low => 0.2,
            RiskLevel::Medium => 0.5,
            RiskLevel::High => 0.8,
            RiskLevel::Critical => 1.0,
        };
        Self {
            level,
            score,
            ..Default::default()
        }
    }

    /// Add a risk category.
    pub fn with_category(mut self, category: RiskCategory) -> Self {
        self.categories.push(category);
        self
    }

    /// Add an explanation.
    pub fn with_explanation(mut self, explanation: impl Into<String>) -> Self {
        self.explanation = Some(explanation.into());
        self
    }
}

/// Categories of risk that can be associated with an action.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum RiskCategory {
    /// Action sends data externally.
    DataExfiltration,
    /// Action modifies system configuration.
    ConfigChange,
    /// Action involves privileged operations.
    PrivilegeEscalation,
    /// Action involves financial or billing operations.
    Financial,
    /// Action involves personal or sensitive data.
    PersonalData,
    /// Action could affect other tenants.
    CrossTenant,
    /// Action involves shell or command execution.
    CommandExecution,
    /// Potential prompt injection detected.
    PromptInjection,
    /// SSRF or network access concerns.
    NetworkAccess,
}
