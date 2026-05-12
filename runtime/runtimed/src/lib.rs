//! Library surface for the `runtimed` binary. Exposing internals as a library
//! lets the integration tests drive the executor and the NDJSON loop directly
//! without spawning a child process.

pub mod executor;
pub mod loop_io;
pub mod provider;

pub use executor::RealStepExecutor;
pub use loop_io::{ProcessResult, process_request};
pub use provider::{LlmProvider, MockProvider, ProviderConfig, build_provider};
