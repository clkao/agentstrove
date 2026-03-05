// ABOUTME: UI preferences state management.
// ABOUTME: Controls theme toggle with localStorage persistence and DOM class sync.

type Theme = "light" | "dark";

function readStoredTheme(): Theme | null {
  try {
    const val = localStorage?.getItem("theme");
    if (val === "light" || val === "dark") return val;
  } catch {
    // ignore
  }
  return null;
}

class UIStore {
  theme: Theme = $state(readStoredTheme() || "light");

  constructor() {
    $effect.root(() => {
      $effect(() => {
        const root = document.documentElement;
        if (this.theme === "dark") {
          root.classList.add("dark");
        } else {
          root.classList.remove("dark");
        }
        try {
          localStorage?.setItem("theme", this.theme);
        } catch {
          // ignore
        }
      });
    });
  }

  toggleTheme(): void {
    this.theme = this.theme === "light" ? "dark" : "light";
  }
}

export const ui = new UIStore();
