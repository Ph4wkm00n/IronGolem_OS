# Mobile and Accessibility

## Mobile Behavior

IronGolem OS must work well on mobile devices, prioritizing the most
time-sensitive interactions.

### Mobile Priorities (in order)

1. **Approvals** - Quick approve/reject from notifications
2. **Incident summaries** - Understand what happened at a glance
3. **Heartbeat status** - Is everything healthy?
4. **Inbox actions** - Swipeable cards for quick decisions

### Mobile-Specific Patterns

| Pattern | Implementation |
|---------|---------------|
| Navigation | Bottom navigation bar for key areas |
| Inbox | Swipeable cards for approve/reject/defer |
| Heartbeats | Compressed summary with expand-on-tap |
| Security alerts | Full-screen sheets for clarity and focus |
| Recipes | Simplified gallery with essential info only |
| Graphs | List view default; graph visualization hidden on small screens |

### Responsive Breakpoints

| Breakpoint | Target |
|-----------|--------|
| < 640px | Mobile (single column, bottom nav) |
| 640-1024px | Tablet (two column where appropriate) |
| > 1024px | Desktop (full layout with sidebar) |

## Accessibility Requirements

### Screen Reader Support
- All security and heartbeat statuses must include descriptive text
- Status indicators use aria-labels, not just color
- Timeline entries have full textual descriptions
- Graph views provide list/table alternatives

### Interaction Requirements
- Policy cards readable without hover interactions
- All interactive elements have minimum 44x44px touch targets
- Focus indicators visible on all interactive elements
- Keyboard navigation for all features

### Motion and Visual
- Reduced-motion mode simplifies graph and timeline transitions
- No information conveyed by animation alone
- Semantic color used carefully; status conveyed through text + icons + layout
- Sufficient contrast ratios on all text and indicators

### Cognitive Accessibility
- Plain language on all default surfaces
- Progressive disclosure reduces cognitive load
- Consistent patterns across all screens
- Clear hierarchy: primary action is always obvious

## Testing Requirements

- Screen reader testing with VoiceOver (macOS/iOS) and NVDA (Windows)
- Keyboard-only navigation testing
- Color contrast validation (WCAG 2.1 AA minimum)
- Mobile usability testing on actual devices
- Reduced-motion preference testing

## Canonical Reference

See [specs/05-ui-ux-design-guide-v2.md](../specs/05-ui-ux-design-guide-v2.md).
