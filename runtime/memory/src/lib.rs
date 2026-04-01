//! # IronGolem Memory
//!
//! Knowledge graph storage and queries. Manages the four graph structures:
//! event log, preference graph, relationship graph, and knowledge graph.

pub mod graph;
pub mod sqlite_store;
pub mod store;

pub use graph::{Edge, EdgeKind, GraphNode, NodeKind};
pub use sqlite_store::SqliteMemoryStore;
pub use store::MemoryStore;
