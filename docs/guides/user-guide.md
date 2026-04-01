# IronGolem OS User Guide

This guide is for non-technical users getting started with IronGolem OS.

## First-Time Setup

1. Open `http://localhost:3000` in your browser (or launch the Tauri desktop app).
2. The onboarding wizard walks you through three steps:
   - **Name your assistant** -- pick a name for your IronGolem instance.
   - **Connect a channel** -- link at least one channel (email, Telegram, calendar, etc.).
   - **Activate a starter recipe** -- choose a pre-built automation to try first.
3. After onboarding you land on the Home dashboard.

## Home Dashboard

The Home dashboard gives you a single view of everything happening:

| Section | What it shows |
|---------|--------------|
| Activity timeline | Recent actions your assistant has taken |
| Pending approvals | Actions waiting for your "yes" or "no" |
| Heartbeat status | Overall system health at a glance |
| Active recipes | Automations currently running |

## Activating a Recipe

Recipes are pre-built automations with plain-language safety summaries.

1. Go to **Recipe Gallery** from the sidebar.
2. Browse categories (Inbox, Research, Ops, Security, Executive Assistant).
3. Click a recipe card to see its **safety summary** -- what it can do, what it cannot do, and what permissions it needs.
4. Click **Activate**. Some recipes start immediately; others ask you to configure options first (e.g., which email folder to watch).
5. The recipe appears on your Home dashboard under "Active recipes."

To pause or deactivate a recipe, click its card and choose **Pause** or **Deactivate**.

## Managing the Inbox

The Inbox is where your assistant asks for approval before taking sensitive actions.

- **Approve** -- the assistant proceeds with the action.
- **Reject** -- the action is cancelled and logged.
- **Details** -- expand the card to see exactly what the assistant wants to do and why.

Actions that do not require approval run automatically and appear in the Activity timeline.

## Health Center

The Health Center shows heartbeat status for every component.

| State | Meaning |
|-------|---------|
| **Healthy** | Everything is working normally |
| **Quietly Recovering** | A minor issue was detected and is being fixed automatically |
| **Needs Attention** | Something requires your input or investigation |
| **Paused** | A component has been paused (by you or by the system) |
| **Quarantined** | A component was isolated for safety; check the Security Center |

No action is needed for Healthy or Quietly Recovering states. For Needs Attention, the Health Center provides a suggested action.

## Security Center

The Security Center shows what your assistant has blocked or flagged.

- **Blocked actions** -- requests that violated a security policy (e.g., prompt injection attempts, unauthorized tool access).
- **Quarantined components** -- agents or connectors isolated after suspicious behavior.
- **Incidents** -- a timeline of security events with explanations.

You can review any incident and choose to **allowlist** a blocked action if it was a false positive, or **rollback** a change made before quarantine.

## Research Center

The Research Center lets you track topics your assistant monitors.

1. **Add a topic** -- tell your assistant what to research (e.g., "competitor pricing changes").
2. **View briefs** -- the assistant produces summaries from fetched sources.
3. **Contradictions** -- if sources disagree, the assistant flags the conflict for your review.

All research is evidence-backed with source links.

## Memory

The Memory section shows what your assistant remembers about your preferences and past interactions.

- Each memory entry has a **confidence score** and **freshness indicator**.
- You can **delete** any memory entry you no longer want retained.
- Memory is stored locally in Solo mode; it never leaves your machine.

## Settings

Key settings to be aware of:

| Setting | What it controls |
|---------|-----------------|
| Deployment mode | Solo, Household, or Team |
| Connected channels | Which services your assistant can access |
| Approval policies | What requires your approval vs. runs automatically |
| Security policies | Tool restrictions, blocked domains, rate limits |
| Notification preferences | How and when you get alerts |

## FAQ

**Q: Can my assistant take actions without asking me?**
A: Only for actions within its approved policy. Sensitive actions always require your approval in the Inbox.

**Q: Where is my data stored?**
A: In Solo mode, everything stays in a local SQLite file on your machine. In Team mode, data is in your PostgreSQL database.

**Q: Can I undo something my assistant did?**
A: Yes. Every action is recorded via event sourcing. Use the rollback control on any action in the Activity timeline.

**Q: What happens if something breaks?**
A: The self-healing loop detects failures automatically. Check the Health Center for current status. Most issues resolve without your involvement.

**Q: How do I stop everything immediately?**
A: Click the **Pause All** button in the Health Center. This suspends all autonomous loops until you resume them.
