# Visual System

## Design Philosophy

IronGolem OS should feel **calm, professional, and trustworthy**. The visual
system prioritizes clarity and readability over flashiness.

### Do
- Maintain calm neutral surfaces with one accent color
- Use semantic color carefully and purposefully
- Emphasize status through text, icons, and layout together
- Keep the interface feeling spacious and breathable

### Don't
- Use glowing, cyber, or surveillance aesthetics
- Rely on color alone to convey meaning
- Create visual noise with competing colors or animations
- Make the interface feel like an enterprise SIEM or monitoring tool

## Semantic Colors

| Token | Meaning | Usage |
|-------|---------|-------|
| `safe` | Healthy, approved, successful | Heartbeat healthy, action taken |
| `warning` | Attention needed, recovering | Heartbeat recovering, needs attention |
| `blocked` | Denied, restricted | Action blocked, policy violation |
| `recovered` | Healed, restored | Self-healing success |
| `quarantined` | Isolated for safety | Quarantine status |
| `neutral` | Default, no special status | Normal state, informational |
| `accent` | Primary brand accent | Interactive elements, CTAs |

## Typography

| Level | Usage |
|-------|-------|
| Page title | Screen headings (Home, Inbox, Recipes) |
| Section title | Card and section headings |
| Body | Primary content text |
| Caption | Secondary information, timestamps, metadata |
| Label | Form labels, status labels, badges |
| Code | Technical content in advanced mode |

### Font Selection Criteria
- High readability at small sizes
- Clear distinction between similar characters
- Good internationalization support (for multilingual UX in Phase 5)
- Available as a web font with good performance

## Iconography

- Use a consistent icon set throughout the application
- Icons should be simple, recognizable, and work at small sizes
- Always pair icons with text labels (icons alone are insufficient)
- Status icons must have textual alternatives for accessibility

### Key Icon Categories
- Navigation (home, inbox, recipes, research, memory, health, security)
- Status (healthy, warning, blocked, recovered, quarantined)
- Actions (approve, reject, pause, inspect, edit, export)
- Agent roles (planner, executor, verifier, researcher, defender)

## Spacing and Layout

- Consistent spacing scale based on design tokens
- Cards with adequate padding for readability
- Clear visual hierarchy through spacing and typography
- Generous whitespace to prevent feeling cluttered

## Component Theming

All components use design tokens from `packages/design-tokens/`:
- Colors reference semantic tokens, not raw values
- Spacing uses scale tokens
- Typography uses type scale tokens
- Dark mode support planned (not Phase 1)

## Canonical Reference

See [specs/05-ui-ux-design-guide-v2.md](../specs/05-ui-ux-design-guide-v2.md).
