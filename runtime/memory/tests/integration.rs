//! Integration tests for the IronGolem memory knowledge graph.

use chrono::{Duration, Utc};
use irongolem_core::types::WorkspaceId;
use irongolem_memory::graph::{Edge, EdgeKind, Evidence, GraphNode, NodeKind};
use irongolem_memory::sqlite_store::SqliteMemoryStore;
use irongolem_memory::store::MemoryStore;
use uuid::Uuid;

/// Create a basic in-memory store for tests.
fn test_store() -> SqliteMemoryStore {
    SqliteMemoryStore::in_memory().expect("failed to create in-memory store")
}

#[tokio::test]
async fn test_knowledge_graph_workflow() {
    let store = test_store();
    let ws = WorkspaceId::new();

    // Create nodes: Person, Topic, Source
    let alice = GraphNode::new(ws, NodeKind::Person, "Alice");
    let rust_topic = GraphNode::new(ws, NodeKind::Topic, "Rust Programming");
    let mut source = GraphNode::new(ws, NodeKind::Source, "The Rust Book");
    source.description = Some("Official Rust programming language book".into());

    let alice_id = alice.id;
    let topic_id = rust_topic.id;
    let source_id = source.id;

    store.upsert_node(&alice).await.unwrap();
    store.upsert_node(&rust_topic).await.unwrap();
    store.upsert_node(&source).await.unwrap();

    // Verify nodes were stored
    let loaded_alice = store
        .get_node(alice_id)
        .await
        .unwrap()
        .expect("alice should exist");
    assert_eq!(loaded_alice.name, "Alice");
    assert_eq!(loaded_alice.kind, NodeKind::Person);

    // Add edges: Alice -> RelatedTo -> Rust, Source -> Supports -> Rust
    let edge_related = Edge {
        id: Uuid::new_v4(),
        source_id: alice_id,
        target_id: topic_id,
        kind: EdgeKind::RelatedTo,
        weight: 1.0,
        metadata: serde_json::Value::Null,
        created_at: Utc::now(),
    };
    let edge_supports = Edge {
        id: Uuid::new_v4(),
        source_id: source_id,
        target_id: topic_id,
        kind: EdgeKind::Supports,
        weight: 0.9,
        metadata: serde_json::Value::Null,
        created_at: Utc::now(),
    };

    store.add_edge(&edge_related).await.unwrap();
    store.add_edge(&edge_supports).await.unwrap();

    // Verify graph traversal: edges from Alice
    let alice_edges = store.get_edges_from(alice_id).await.unwrap();
    assert_eq!(alice_edges.len(), 1);
    assert_eq!(alice_edges[0].target_id, topic_id);
    assert_eq!(alice_edges[0].kind, EdgeKind::RelatedTo);

    // Verify edges to Rust topic (should have 2: RelatedTo from Alice, Supports from Source)
    let topic_edges = store.get_edges_to(topic_id).await.unwrap();
    assert_eq!(topic_edges.len(), 2);

    // Verify find by kind
    let people = store
        .find_nodes_by_kind(ws, NodeKind::Person)
        .await
        .unwrap();
    assert_eq!(people.len(), 1);
    assert_eq!(people[0].name, "Alice");

    let topics = store.find_nodes_by_kind(ws, NodeKind::Topic).await.unwrap();
    assert_eq!(topics.len(), 1);
    assert_eq!(topics[0].name, "Rust Programming");

    let sources = store
        .find_nodes_by_kind(ws, NodeKind::Source)
        .await
        .unwrap();
    assert_eq!(sources.len(), 1);
    assert_eq!(sources[0].name, "The Rust Book");
}

#[tokio::test]
async fn test_preference_learning() {
    let store = test_store();
    let ws = WorkspaceId::new();

    // Create a preference node with initial confidence
    let mut pref = GraphNode::new(ws, NodeKind::Preference, "Prefers dark mode");
    pref.confidence = 0.6;
    pref.evidence.push(Evidence {
        source: "ui_interaction_log".into(),
        collected_at: Utc::now(),
        excerpt: Some("User toggled dark mode on".into()),
        trust_score: 0.8,
    });
    let pref_id = pref.id;

    store.upsert_node(&pref).await.unwrap();

    // Verify initial state
    let loaded = store
        .get_node(pref_id)
        .await
        .unwrap()
        .expect("preference should exist");
    assert_eq!(loaded.confidence, 0.6);
    assert_eq!(loaded.evidence.len(), 1);

    // Simulate additional evidence that increases confidence
    pref.confidence = 0.85;
    pref.evidence.push(Evidence {
        source: "settings_api".into(),
        collected_at: Utc::now(),
        excerpt: Some("User explicitly set theme=dark in settings".into()),
        trust_score: 0.95,
    });
    pref.updated_at = Utc::now();

    store.upsert_node(&pref).await.unwrap();

    // Verify updated state
    let updated = store
        .get_node(pref_id)
        .await
        .unwrap()
        .expect("preference should exist");
    assert_eq!(updated.confidence, 0.85);
    assert_eq!(updated.evidence.len(), 2);

    // Verify the evidence details round-trip correctly
    assert_eq!(updated.evidence[0].source, "ui_interaction_log");
    assert_eq!(updated.evidence[1].source, "settings_api");
    assert_eq!(updated.evidence[1].trust_score, 0.95);

    // Verify it shows up as a Preference kind
    let prefs = store
        .find_nodes_by_kind(ws, NodeKind::Preference)
        .await
        .unwrap();
    assert_eq!(prefs.len(), 1);
    assert_eq!(prefs[0].name, "Prefers dark mode");
}

#[tokio::test]
async fn test_contradiction_detection() {
    let store = test_store();
    let ws = WorkspaceId::new();

    // Create a claim with supporting evidence
    let mut claim = GraphNode::new(ws, NodeKind::Claim, "Coffee improves productivity");
    claim.evidence.push(Evidence {
        source: "study_2024_a".into(),
        collected_at: Utc::now(),
        excerpt: Some("Moderate caffeine intake correlated with higher output".into()),
        trust_score: 0.7,
    });
    store.upsert_node(&claim).await.unwrap();

    // Create a contradicting claim and mark the original as contradicted
    let mut counter_claim = GraphNode::new(ws, NodeKind::Claim, "Coffee harms productivity");
    counter_claim.evidence.push(Evidence {
        source: "study_2024_b".into(),
        collected_at: Utc::now(),
        excerpt: Some("High caffeine leads to anxiety and reduced focus".into()),
        trust_score: 0.75,
    });
    counter_claim.has_contradiction = true;
    store.upsert_node(&counter_claim).await.unwrap();

    // Mark the original claim as contradicted too
    claim.has_contradiction = true;
    store.upsert_node(&claim).await.unwrap();

    // Add a Contradicts edge between them
    let edge = Edge {
        id: Uuid::new_v4(),
        source_id: counter_claim.id,
        target_id: claim.id,
        kind: EdgeKind::Contradicts,
        weight: 1.0,
        metadata: serde_json::Value::Null,
        created_at: Utc::now(),
    };
    store.add_edge(&edge).await.unwrap();

    // Verify find_contradictions returns both flagged nodes
    let contradictions = store.find_contradictions(ws).await.unwrap();
    assert_eq!(contradictions.len(), 2);

    let names: Vec<&str> = contradictions.iter().map(|n| n.name.as_str()).collect();
    assert!(names.contains(&"Coffee improves productivity"));
    assert!(names.contains(&"Coffee harms productivity"));

    // Verify the Contradicts edge
    let edges_to_claim = store.get_edges_to(claim.id).await.unwrap();
    assert_eq!(edges_to_claim.len(), 1);
    assert_eq!(edges_to_claim[0].kind, EdgeKind::Contradicts);

    // A non-contradicted claim should not appear
    store
        .upsert_node(&GraphNode::new(ws, NodeKind::Claim, "Water is wet"))
        .await
        .unwrap();
    let contradictions_after = store.find_contradictions(ws).await.unwrap();
    assert_eq!(contradictions_after.len(), 2);
}

#[tokio::test]
async fn test_freshness_tracking() {
    let store = test_store();
    let ws = WorkspaceId::new();

    // Create a node with old freshness (90 days ago)
    let mut stale_node = GraphNode::new(ws, NodeKind::Source, "Old API Docs");
    stale_node.freshness = Utc::now() - Duration::days(90);
    store.upsert_node(&stale_node).await.unwrap();

    // Create a node with recent freshness
    let fresh_node = GraphNode::new(ws, NodeKind::Source, "Fresh API Docs");
    // freshness defaults to now
    store.upsert_node(&fresh_node).await.unwrap();

    // Create another stale node (45 days old)
    let mut somewhat_stale = GraphNode::new(ws, NodeKind::Topic, "Old Research");
    somewhat_stale.freshness = Utc::now() - Duration::days(45);
    store.upsert_node(&somewhat_stale).await.unwrap();

    // Find nodes stale for more than 30 days -> should return 2
    let stale_30 = store.find_stale_nodes(ws, 30).await.unwrap();
    assert_eq!(stale_30.len(), 2);

    // They should be sorted by freshness ascending (oldest first)
    assert_eq!(stale_30[0].name, "Old API Docs");
    assert_eq!(stale_30[1].name, "Old Research");

    // Find nodes stale for more than 60 days -> should return only the 90-day-old one
    let stale_60 = store.find_stale_nodes(ws, 60).await.unwrap();
    assert_eq!(stale_60.len(), 1);
    assert_eq!(stale_60[0].name, "Old API Docs");

    // Find nodes stale for more than 365 days -> should return none
    let stale_365 = store.find_stale_nodes(ws, 365).await.unwrap();
    assert!(stale_365.is_empty());
}
