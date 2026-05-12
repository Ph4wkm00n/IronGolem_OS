// icons.jsx — inline Heroicons-style SVG (stroke=1.5, currentColor).
// No third-party icon library, per spec.

const Icon = (() => {
  const make = (paths, viewBox = "0 0 24 24") => ({ size = 16, className = "", ...rest }) => (
    <svg xmlns="http://www.w3.org/2000/svg" viewBox={viewBox} width={size} height={size}
         fill="none" stroke="currentColor" strokeWidth="1.5"
         strokeLinecap="round" strokeLinejoin="round"
         className={className} aria-hidden="true" {...rest}>
      {paths}
    </svg>
  );
  return {
    // Layout
    Home: make(<><path d="M3 11.5 12 4l9 7.5" /><path d="M5 10v9a1 1 0 0 0 1 1h4v-6h4v6h4a1 1 0 0 0 1-1v-9" /></>),
    Inbox: make(<><path d="M3 12V5a1 1 0 0 1 1-1h16a1 1 0 0 1 1 1v7" /><path d="M3 12h5l1.5 2.5h5L16 12h5v6a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1v-6Z" /></>),
    Timeline: make(<><path d="M4 6h16" /><path d="M4 12h10" /><path d="M4 18h13" /><circle cx="20" cy="12" r="1.5" /><circle cx="20" cy="18" r="1.5" /></>),
    Teams: make(<><circle cx="9" cy="9" r="3.2" /><path d="M3 19c0-2.8 2.7-5 6-5s6 2.2 6 5" /><circle cx="17" cy="8" r="2.4" /><path d="M15 14.5c2.4.4 4.5 1.8 4.5 4.5" /></>),
    Research: make(<><circle cx="11" cy="11" r="6" /><path d="m20 20-4.3-4.3" /></>),
    Safety: make(<><path d="M12 3 5 6v6c0 4 3 7.5 7 9 4-1.5 7-5 7-9V6l-7-3Z" /><path d="m9 12 2 2 4-4" /></>),
    Settings: make(<><circle cx="12" cy="12" r="2.5" /><path d="M19.4 15a1.7 1.7 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.8-.3 1.7 1.7 0 0 0-1 1.5V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-1-1.5 1.7 1.7 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0 .3-1.8 1.7 1.7 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.5-1 1.7 1.7 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.8.3h.1a1.7 1.7 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.8v.1a1.7 1.7 0 0 0 1.5 1H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1Z" /></>),

    // Status / actions
    Check: make(<path d="m5 12 5 5L20 7" />),
    CheckCircle: make(<><circle cx="12" cy="12" r="9" /><path d="m8 12 3 3 5-6" /></>),
    XCircle: make(<><circle cx="12" cy="12" r="9" /><path d="m9 9 6 6M15 9l-6 6" /></>),
    AlertTriangle: make(<><path d="M12 4 2.5 20h19L12 4Z" /><path d="M12 10v4" /><circle cx="12" cy="17.5" r=".5" fill="currentColor" stroke="none" /></>),
    Shield: make(<><path d="M12 3 5 6v6c0 4 3 7.5 7 9 4-1.5 7-5 7-9V6l-7-3Z" /></>),
    ShieldOff: make(<><path d="M12 3 5 6v6c0 4 3 7.5 7 9 4-1.5 7-5 7-9V6l-7-3Z" /><path d="m5 5 14 14" /></>),
    Wand: make(<><path d="m3 21 12-12" /><path d="m13 5 2 2" /><path d="m17 9 2 2" /><path d="m9 1 1 2 2 1-2 1-1 2-1-2-2-1 2-1 1-2Z" /></>),
    Box: make(<><path d="m12 3 8 4.5v9L12 21l-8-4.5v-9L12 3Z" /><path d="m4 7.5 8 4.5 8-4.5" /><path d="M12 12v9" /></>),
    Eye: make(<><path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12Z" /><circle cx="12" cy="12" r="3" /></>),
    Sparkles: make(<><path d="M12 3v3M12 18v3M3 12h3M18 12h3M5.6 5.6l2.1 2.1M16.3 16.3l2.1 2.1M5.6 18.4l2.1-2.1M16.3 7.7l2.1-2.1" /></>),
    Clock: make(<><circle cx="12" cy="12" r="9" /><path d="M12 7v5l3 2" /></>),
    Chevron: make(<path d="m9 6 6 6-6 6" />),
    ChevronDown: make(<path d="m6 9 6 6 6-6" />),
    ChevronUp: make(<path d="m6 15 6-6 6 6" />),
    ArrowRight: make(<><path d="M5 12h14" /><path d="m13 6 6 6-6 6" /></>),
    Plus: make(<><path d="M12 5v14" /><path d="M5 12h14" /></>),
    Filter: make(<path d="M4 5h16l-6 8v6l-4-2v-4L4 5Z" />),
    Search: make(<><circle cx="11" cy="11" r="6" /><path d="m20 20-4.3-4.3" /></>),
    Bell: make(<><path d="M18 16v-5a6 6 0 1 0-12 0v5l-2 2h16l-2-2Z" /><path d="M10 20a2 2 0 0 0 4 0" /></>),
    Pause: make(<><path d="M9 5v14" /><path d="M15 5v14" /></>),
    Play: make(<path d="M7 4v16l13-8L7 4Z" />),
    Refresh: make(<><path d="M3 12a9 9 0 0 1 15-6.7L21 8" /><path d="M21 3v5h-5" /><path d="M21 12a9 9 0 0 1-15 6.7L3 16" /><path d="M3 21v-5h5" /></>),
    Lock: make(<><rect x="5" y="11" width="14" height="9" rx="2" /><path d="M8 11V8a4 4 0 1 1 8 0v3" /></>),
    HelpCircle: make(<><circle cx="12" cy="12" r="9" /><path d="M9.5 9.5a2.5 2.5 0 1 1 3.5 2.3c-.7.3-1 .9-1 1.7" /><circle cx="12" cy="17" r=".5" fill="currentColor" stroke="none" /></>),
    Link: make(<><path d="M10 14a4 4 0 0 0 5.7 0l3-3a4 4 0 0 0-5.7-5.7l-1 1" /><path d="M14 10a4 4 0 0 0-5.7 0l-3 3a4 4 0 0 0 5.7 5.7l1-1" /></>),
    Dot: make(<circle cx="12" cy="12" r="3" fill="currentColor" stroke="none" />),
    Database: make(<><ellipse cx="12" cy="6" rx="8" ry="3" /><path d="M4 6v6c0 1.7 3.6 3 8 3s8-1.3 8-3V6" /><path d="M4 12v6c0 1.7 3.6 3 8 3s8-1.3 8-3v-6" /></>),
    Cpu: make(<><rect x="5" y="5" width="14" height="14" rx="2" /><rect x="9" y="9" width="6" height="6" rx="1" /><path d="M9 2v3M15 2v3M9 19v3M15 19v3M2 9h3M2 15h3M19 9h3M19 15h3" /></>),
    Mail: make(<><rect x="3" y="5" width="18" height="14" rx="2" /><path d="m4 7 8 6 8-6" /></>),
    File: make(<><path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8l-5-5Z" /><path d="M14 3v5h5" /></>),
    Activity: make(<path d="M3 12h4l3-7 4 14 3-7h4" />),
    History: make(<><path d="M3 12a9 9 0 1 0 3-6.7" /><path d="M3 4v4h4" /><path d="M12 8v5l3 2" /></>),
    Layers: make(<><path d="m12 3 9 5-9 5-9-5 9-5Z" /><path d="m3 13 9 5 9-5" /><path d="m3 18 9 5 9-5" /></>),
    Slash: make(<path d="M5 19 19 5" />),
    Pin: make(<><path d="M12 17v5" /><path d="M9 3h6l-1 6h2l1 4H7l1-4h2L9 3Z" /></>),
    Logo: make(<>
      <rect x="3"  y="6"  width="6"  height="2.5" rx="1" />
      <rect x="3"  y="11" width="14" height="2.5" rx="1" />
      <rect x="3"  y="16" width="9"  height="2.5" rx="1" />
    </>, "0 0 20 24"),
  };
})();

window.Icon = Icon;
