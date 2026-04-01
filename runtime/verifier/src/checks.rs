//! Built-in verification checks.

use async_trait::async_trait;
use irongolem_core::Result;

use crate::contract::{CheckResult, VerificationResult, Verifier};

/// Verifier that checks for empty or null outputs.
pub struct NonEmptyVerifier;

#[async_trait]
impl Verifier for NonEmptyVerifier {
    async fn verify(&self, output: &serde_json::Value) -> Result<VerificationResult> {
        let is_empty = output.is_null()
            || output.as_str().is_some_and(|s| s.is_empty())
            || output.as_array().is_some_and(|a| a.is_empty())
            || output.as_object().is_some_and(|o| o.is_empty());

        Ok(VerificationResult {
            passed: !is_empty,
            checks: vec![CheckResult {
                name: "non_empty".to_string(),
                passed: !is_empty,
                message: if is_empty {
                    "Output is empty or null".to_string()
                } else {
                    "Output is non-empty".to_string()
                },
            }],
            confidence: if is_empty { 0.0 } else { 1.0 },
            suggestions: if is_empty {
                vec!["Agent should produce non-empty output".to_string()]
            } else {
                Vec::new()
            },
        })
    }

    fn name(&self) -> &str {
        "non_empty"
    }
}

/// Verifier that checks JSON output conforms to an expected schema shape.
pub struct SchemaVerifier {
    pub required_fields: Vec<String>,
}

#[async_trait]
impl Verifier for SchemaVerifier {
    async fn verify(&self, output: &serde_json::Value) -> Result<VerificationResult> {
        let obj = output.as_object();
        let mut checks = Vec::new();
        let mut all_passed = true;

        for field in &self.required_fields {
            let present = obj.map(|o| o.contains_key(field)).unwrap_or(false);
            if !present {
                all_passed = false;
            }
            checks.push(CheckResult {
                name: format!("has_{field}"),
                passed: present,
                message: if present {
                    format!("Field '{field}' is present")
                } else {
                    format!("Required field '{field}' is missing")
                },
            });
        }

        Ok(VerificationResult {
            passed: all_passed,
            checks,
            confidence: if all_passed { 1.0 } else { 0.5 },
            suggestions: if all_passed {
                Vec::new()
            } else {
                vec!["Ensure all required fields are present in output".to_string()]
            },
        })
    }

    fn name(&self) -> &str {
        "schema"
    }
}
