import { create } from "zustand";
import { api } from "./ipc";

export type Theme = "dark" | "light";

export interface Toast {
  id: number;
  kind: "info" | "success" | "error";
  message: string;
}

interface AppState {
  theme: Theme;
  sidebarCollapsed: boolean;
  paletteOpen: boolean;
  toasts: Toast[];
  setTheme: (t: Theme) => void;
  toggleTheme: () => void;
  toggleSidebar: () => void;
  setPaletteOpen: (open: boolean) => void;
  toast: (message: string, kind?: Toast["kind"]) => void;
  dismiss: (id: number) => void;
  hydrate: () => Promise<void>;
}

let toastId = 0;

export const useApp = create<AppState>((set, get) => ({
  theme: "dark",
  sidebarCollapsed: false,
  paletteOpen: false,
  toasts: [],
  setTheme: (t) => {
    document.documentElement.setAttribute("data-theme", t);
    set({ theme: t });
  },
  toggleTheme: () => {
    const next = get().theme === "dark" ? "light" : "dark";
    document.documentElement.setAttribute("data-theme", next);
    set({ theme: next });
    api.settingsSet("ui.theme", next).catch(() => {});
  },
  toggleSidebar: () => set((s) => ({ sidebarCollapsed: !s.sidebarCollapsed })),
  setPaletteOpen: (open) => set({ paletteOpen: open }),
  toast: (message, kind = "info") => {
    const id = ++toastId;
    set((s) => ({ toasts: [...s.toasts, { id, kind, message }] }));
    setTimeout(() => get().dismiss(id), 4200);
  },
  dismiss: (id) => set((s) => ({ toasts: s.toasts.filter((t) => t.id !== id) })),
  hydrate: async () => {
    try {
      const t = await api.settingsGet("ui.theme");
      const theme: Theme = t === "light" ? "light" : "dark";
      document.documentElement.setAttribute("data-theme", theme);
      set({ theme });
    } catch {
      // backend not ready (e.g. plain browser dev); keep dark
    }
  },
}));

export function errMessage(e: unknown): string {
  if (e instanceof Error) return e.message;
  return String(e);
}
