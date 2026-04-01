//! Verifier contract trait and result types.

use async_trait::async_trait;
use irongolem_core::Result;
use serde::{Deserialize, Serialize};

/// The result of a verification check.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct VerificationResult {
    /// Whether the output passed verification.
    pub passed: bool,
    /// Individual check results.
    pub checks: Vec<CheckResult>,
    /// Overall confidence score (0.0 to 1.0).
    pub confidence: f64,
    /// Suggestions for improvement if verification failed.
    pub suggestions: Vec<String>,
}

/// Result of a single verification check.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CheckResult {
    /// Name of the check.
    pub name: String,
    /// Whether this check passed.
    pub passed: bool,
    /// Description of the check result.
    pub message: String,
}

/// Trait for verification implementations.
#[async_trait]
pub trait Verifier: Send + Sync {
    /// Verify an output against quality gates.
    async fn verify(&self, output: &serde_json::Value) -> Result<VerificationResult>;

    /// Name of this verifier for logging and tracing.
    fn name(&self) -> &str;
}
