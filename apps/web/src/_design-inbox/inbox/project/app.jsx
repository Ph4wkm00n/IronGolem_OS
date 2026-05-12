// app.jsx — Root that wires Tweaks + theme + WorkspaceDashboard.

const TWEAK_DEFAULTS = /*EDITMODE-BEGIN*/{
  "density": "cozy",
  "layoutVariant": "list",
  "permStyle": "inline",
  "trustStyle": "chip",
  "whyStyle": "link",
  "theme": "light",
  "dataVolume": 20,
  "showEmpty": false
}/*EDITMODE-END*/;

function App() {
  const [t, setTweak] = useTweaks(TWEAK_DEFAULTS);

  // Apply theme to <html>
  React.useEffect(() => {
    document.documentElement.dataset.theme = t.theme === "dark" ? "dark" : "light";
  }, [t.theme]);

  return (
    <>
      <WorkspaceDashboard tweaks={t} />

      <TweaksPanel title="Tweaks">
        <TweakSection label="Layout">
          <TweakRadio label="Density" value={t.density}
                      options={["compact", "cozy", "comfy"].map((v) =>
                        ({ value: v === "comfy" ? "comfortable" : v, label: v }))}
                      onChange={(v) => setTweak("density", v)} />
          <TweakRadio label="Activity layout" value={t.layoutVariant}
                      options={[
                        { value: "list",    label: "List" },
                        { value: "grouped", label: "Grouped" },
                        { value: "rail",    label: "Rail" },
                      ]}
                      onChange={(v) => setTweak("layoutVariant", v)} />
        </TweakSection>

        <TweakSection label="Permissions">
          <TweakSelect label="Surface" value={t.permStyle}
                       options={[
                         { value: "inline",  label: "Inline badge" },
                         { value: "popover", label: "Hover popover" },
                         { value: "rail",    label: "Dedicated rail" },
                         { value: "hidden",  label: "Hide" },
                       ]}
                       onChange={(v) => setTweak("permStyle", v)} />
        </TweakSection>

        <TweakSection label="Trust signal">
          <TweakRadio label="Style" value={t.trustStyle}
                      options={["chip", "banner", "inline"]}
                      onChange={(v) => setTweak("trustStyle", v)} />
        </TweakSection>

        <TweakSection label="Why did this happen?">
          <TweakRadio label="Style" value={t.whyStyle}
                      options={[
                        { value: "link",   label: "Link" },
                        { value: "expand", label: "Expand" },
                        { value: "drawer", label: "Drawer" },
                      ]}
                      onChange={(v) => setTweak("whyStyle", v)} />
        </TweakSection>

        <TweakSection label="Theme">
          <TweakRadio label="Mode" value={t.theme}
                      options={["light", "dark"]}
                      onChange={(v) => setTweak("theme", v)} />
        </TweakSection>

        <TweakSection label="Data">
          <TweakSlider label="Events shown" value={t.dataVolume}
                       min={0} max={20} step={1}
                       onChange={(v) => setTweak("dataVolume", v)} />
          <TweakToggle label="Preview empty state" value={t.showEmpty}
                       onChange={(v) => setTweak("showEmpty", v)} />
        </TweakSection>
      </TweaksPanel>
    </>
  );
}

const root = ReactDOM.createRoot(document.getElementById("root"));
root.render(<App />);
