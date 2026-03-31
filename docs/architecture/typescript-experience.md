# TypeScript Experience Domain

The TypeScript layer owns **everything the user sees and interacts with**. Its
mission is to make a powerful platform feel simple, safe, and understandable.

## Purpose

- Onboarding and guided setup
- Recipe browsing and activation
- Approval inbox and action timeline
- Research center and memory explorer
- Health and security centers
- Admin console for team mode
- Desktop shell via Tauri

## Key Submodules

### Web App Shell (`apps/web/`)
The primary React application providing:
- Navigation and layout
- Theming (calm neutral surfaces with semantic color)
- Progressive disclosure (basic vs. advanced mode)
- Command palette for power users

### Desktop Shell (`apps/desktop/`)
Tauri-based wrapper for local-first deployment:
- Native system tray integration
- Local notifications
- File system access for solo mode
- Auto-update capability

### Admin Console (`apps/admin-console/`)
Team-mode administration:
- Workspace management
- Member roles and permissions
- Connector assignment by workspace
- Policy management and visualization
- Trace and audit exploration

### Shared UI Package (`packages/ui/`)
Reusable component library:
- Safety cards, policy cards, research cards
- Timeline components with v2 states
- Heartbeat status indicators
- Approval flow components
- Graph visualization (optional, not default)

### Design Tokens (`packages/design-tokens/`)
Centralized design system tokens:
- Colors (semantic: Safe, Warning, Blocked, Recovered, Quarantined)
- Typography scales
- Spacing and layout primitives
- Component-level tokens

### Policy Explainer Layer
Translates the five-layer permission model into plain language:
- "Who can trigger this?"
- "Which agent acts?"
- "Which tools are allowed?"
- "What always needs approval?"

## Design Principles

- **Informed, not overwhelmed** - summaries for basic users, depth for advanced
- **Protected, not restricted** - show risk clearly, explain blocks
- **Assisted, not replaced** - human-readable contracts for every automation
- **In control** - background operations remain visible and explainable
- Plain language everywhere; no jargon on default surfaces
- Mobile-first responsive design

## Canonical Reference

See [specs/05-ui-ux-design-guide-v2.md](../specs/05-ui-ux-design-guide-v2.md)
for the full UX design guide.
