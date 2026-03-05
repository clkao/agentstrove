// ABOUTME: UI preferences state management.
// ABOUTME: Controls theme selection for the application.

type Theme = "light" | "dark" | "system";

class UIStore {
  theme = $state<Theme>("system");

  setTheme(t: Theme): void {
    this.theme = t;
  }
}

export const ui = new UIStore();
