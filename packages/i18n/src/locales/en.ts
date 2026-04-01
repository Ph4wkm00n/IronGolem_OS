/**
 * English translations (default locale) for IronGolem OS.
 *
 * This file is the source of truth for all translation keys. Other
 * locale files must use the same keys.
 */

export const en = {
  // ---------------------------------------------------------------------------
  // Primary navigation
  // ---------------------------------------------------------------------------
  "nav.home": "Home",
  "nav.inbox": "Inbox",
  "nav.recipes": "Recipes",
  "nav.squads": "Squads",
  "nav.health": "Health Center",
  "nav.settings": "Settings",
  "nav.admin": "Admin Console",
  "nav.plugins": "Plugins",
  "nav.research": "Research",
  "nav.history": "History",
  "nav.connectors": "Connectors",
  "nav.fleet": "Fleet",

  // ---------------------------------------------------------------------------
  // Heartbeat status labels
  // ---------------------------------------------------------------------------
  "heartbeat.healthy": "Healthy",
  "heartbeat.quietly_recovering": "Quietly Recovering",
  "heartbeat.needs_attention": "Needs Attention",
  "heartbeat.paused": "Paused",
  "heartbeat.quarantined": "Quarantined",
  "heartbeat.unknown": "Unknown",
  "heartbeat.last_seen": "Last seen {{time}}",
  "heartbeat.never_reported": "Never reported",

  // ---------------------------------------------------------------------------
  // Risk level labels
  // ---------------------------------------------------------------------------
  "risk.none": "No Risk",
  "risk.low": "Low Risk",
  "risk.medium": "Medium Risk",
  "risk.high": "High Risk",
  "risk.critical": "Critical Risk",
  "risk.description": "Risk Level: {{level}}",

  // ---------------------------------------------------------------------------
  // Safety card section headers
  // ---------------------------------------------------------------------------
  "safety.what_it_does": "What it does",
  "safety.what_it_needs": "What it needs",
  "safety.what_could_go_wrong": "What could go wrong",
  "safety.how_to_undo": "How to undo",
  "safety.permissions_required": "Permissions required",
  "safety.data_accessed": "Data accessed",
  "safety.estimated_impact": "Estimated impact",
  "safety.rollback_available": "Rollback available",

  // ---------------------------------------------------------------------------
  // Approval action labels
  // ---------------------------------------------------------------------------
  "approval.approve": "Approve",
  "approval.reject": "Reject",
  "approval.approve_once": "Approve Once",
  "approval.approve_always": "Always Approve",
  "approval.review_details": "Review Details",
  "approval.pending": "Pending Approval",
  "approval.approved": "Approved",
  "approval.rejected": "Rejected",
  "approval.expired": "Expired",
  "approval.request_from": "Request from {{agent}}",
  "approval.action_description": "Action: {{action}}",

  // ---------------------------------------------------------------------------
  // Recipe labels
  // ---------------------------------------------------------------------------
  "recipe.activate": "Activate Recipe",
  "recipe.deactivate": "Deactivate Recipe",
  "recipe.edit": "Edit Recipe",
  "recipe.duplicate": "Duplicate Recipe",
  "recipe.delete": "Delete Recipe",
  "recipe.active": "Active",
  "recipe.inactive": "Inactive",
  "recipe.draft": "Draft",
  "recipe.running": "Running",
  "recipe.paused": "Paused",
  "recipe.created_by": "Created by {{author}}",
  "recipe.last_run": "Last run {{time}}",

  // ---------------------------------------------------------------------------
  // Squad labels
  // ---------------------------------------------------------------------------
  "squad.inbox": "Inbox Squad",
  "squad.research": "Research Squad",
  "squad.ops": "Ops Squad",
  "squad.security": "Security Squad",
  "squad.executive_assistant": "Executive Assistant Squad",
  "squad.agents_count": "{{count}} agents",
  "squad.active_tasks": "{{count}} active tasks",

  // ---------------------------------------------------------------------------
  // Common UI phrases
  // ---------------------------------------------------------------------------
  "common.loading": "Loading...",
  "common.error": "An error occurred",
  "common.retry": "Retry",
  "common.cancel": "Cancel",
  "common.save": "Save",
  "common.delete": "Delete",
  "common.confirm": "Confirm",
  "common.close": "Close",
  "common.back": "Back",
  "common.next": "Next",
  "common.search": "Search",
  "common.filter": "Filter",
  "common.sort": "Sort",
  "common.refresh": "Refresh",
  "common.no_results": "No results found",
  "common.created_at": "Created {{time}}",
  "common.updated_at": "Updated {{time}}",
  "common.version": "Version {{version}}",
  "common.enabled": "Enabled",
  "common.disabled": "Disabled",
  "common.on": "On",
  "common.off": "Off",
  "common.yes": "Yes",
  "common.no": "No",
  "common.unknown": "Unknown",
  "common.all": "All",
  "common.none": "None",
  "common.more": "More",
  "common.less": "Less",
  "common.show_all": "Show All",
  "common.collapse": "Collapse",
  "common.expand": "Expand",

  // ---------------------------------------------------------------------------
  // Settings
  // ---------------------------------------------------------------------------
  "settings.general": "General",
  "settings.appearance": "Appearance",
  "settings.language": "Language",
  "settings.notifications": "Notifications",
  "settings.security": "Security",
  "settings.integrations": "Integrations",
  "settings.workspace": "Workspace",
  "settings.deployment_mode": "Deployment Mode",
  "settings.solo_mode": "Solo Mode",
  "settings.household_mode": "Household Mode",
  "settings.team_mode": "Team Mode",

  // ---------------------------------------------------------------------------
  // Plugins
  // ---------------------------------------------------------------------------
  "plugin.install": "Install Plugin",
  "plugin.uninstall": "Uninstall Plugin",
  "plugin.enable": "Enable Plugin",
  "plugin.disable": "Disable Plugin",
  "plugin.installed": "Installed",
  "plugin.available": "Available",
  "plugin.update_available": "Update Available",
  "plugin.permissions": "Plugin Permissions",
  "plugin.by_author": "By {{author}}",
} as const;
