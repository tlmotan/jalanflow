// The text column shows one view at a time (docs/DESIGN.md §1): the main
// view (landing, running or result, by state) or one of list, detail and
// "How this works", each with a Back button. Escape also goes back. Opening
// a view moves focus to its Back button; going back returns focus to where
// it was (§8). style.css shows the right block from the body's attributes.

export type State = "home" | "running" | "result";
export type View = "main" | "list" | "detail" | "how";

export interface Views {
  readonly state: State;
  readonly view: View;
  setState: (s: State) => void;
  /** Open a view on top of the current one. */
  open: (v: Exclude<View, "main">) => void;
  back: () => void;
  /** Close every view and show the main one. */
  home: () => void;
}

interface Entry {
  view: View;
  opener: HTMLElement | null; // gets focus again on Back
}

export function initViews(onChange: (view: View, state: State) => void): Views {
  const body = document.body;
  let state: State = "home";
  let view: View = "main";
  const stack: Entry[] = [];

  const apply = (): void => {
    body.dataset["state"] = state;
    body.dataset["view"] = view;
    onChange(view, state);
  };

  const back = (): void => {
    const from = stack.pop();
    if (!from) return;
    view = from.view;
    apply();
    if (from.opener?.isConnected) from.opener.focus();
  };

  document.querySelectorAll<HTMLElement>("[data-back]").forEach((b) => b.addEventListener("click", back));
  window.addEventListener("keydown", (e) => {
    if (e.key === "Escape" && view !== "main") back();
  });

  // Phones: the map takes the space the text leaves (style.css).
  const text = document.getElementById("text")!;
  new ResizeObserver(() => {
    document.documentElement.style.setProperty("--text-h", `${text.offsetHeight}px`);
  }).observe(text);

  apply();
  return {
    get state() {
      return state;
    },
    get view() {
      return view;
    },
    setState(s) {
      state = s;
      apply();
    },
    open(v) {
      if (v === view) return;
      const active = document.activeElement;
      stack.push({ view, opener: active instanceof HTMLElement ? active : null });
      view = v;
      apply();
      document.querySelector<HTMLElement>(`[data-v="${v}"] [data-back]`)?.focus();
    },
    back,
    home() {
      stack.length = 0;
      view = "main";
      apply();
    },
  };
}
