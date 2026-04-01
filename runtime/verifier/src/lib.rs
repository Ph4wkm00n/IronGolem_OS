//! # IronGolem Verifier
//!
//! Output quality gates. Verifiers check agent outputs before they reach
//! users, catching hallucinations, policy violations, and format errors.

pub mod checks;
pub mod contract;

pub use contract::{VerificationResult, Verifier};
