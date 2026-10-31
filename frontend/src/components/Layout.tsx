import { useEffect, useState } from "react";
import { Outlet, NavLink, useNavigate } from "react-router-dom";
import {
  LayoutDashboard, FolderGit2, GitBranch, TerminalSquare, Database, Globe,
  KeyRound, Code2, ListTodo, PlayCircle, Container, Satellite, Puzzle, Settings as SettingsIcon,
  ChevronLeft, ChevronRight, Search, Moon, Sun, Vault,
} from "lucide-react";
import { motion, AnimatePresence } from "framer-motion";
import { useApp } from "@/lib/store";
import { ToastHost } from "./ui";
import { onEvent } from "@/lib/ipc";
import CommandPalette from "./CommandPalette";

const nav = [
  { to: "/", label: "Dashboard", icon: LayoutDashboard },
  { to: "/projects", label: "Projects", icon: FolderGit2 },
  { to: "/git", label: "Git", icon: GitBranch },
  { to: "/terminal", label: "Terminal", icon: TerminalSquare },
  { to: "/database", label: "Databases", icon: Database },
  { to: "/api", label: "API Workbench", icon: Globe },
  { to: "/env", label: "Environments", icon: KeyRound },
  { to: "/snippets", label: "Snippets & Notes", icon: Code2 },
  { to: "/tasks", label: "Tasks", icon: ListTodo },
  { to: "/processes", label: "Processes", icon: PlayCircle },
  { to: "/containers", label: "Containers", icon: Container },
  { to: "/remote", label: "DevVault Anywhere", icon: Satellite },
  { to: "/plugins", label: "Plugins", icon: Puzzle },
  { to: "/settings", label: "Settings", icon: SettingsIcon },
];

export default function Layout() {
  const { sidebarCollapsed, toggleSidebar, theme, toggleTheme, toast, hydrate } = useApp();
  const navigate = useNavigate();

  useEffect(() => {
    hydrate();
    const off = onEvent("remote:client-pending", (data) => {
      toast(`Remote client "${data?.name}" wants to pair — approve it in DevVault Anywhere`, "info");
    });
    return off;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        useApp.getState().setPaletteOpen(true);
      }
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, []);

  return (
    <div className="flex h-full">
      {/* Sidebar */}
      <div
        className={`h-full bg-surface border-r border-line flex flex-col transition-all duration-200 ${
          sidebarCollapsed ? "w-16" : "w-60"
        }`}
      >
        <div className="flex items-center gap-2.5 px-4 h-16 shrink-0">
          <div className="w-8 h-8 rounded-lg flex items-center justify-center text-white shrink-0"
               style={{ background: "linear-gradient(135deg, var(--accent), var(--accent2))" }}>
            <Vault className="w-4 h-4" />
          </div>
          {!sidebarCollapsed && (
            <span className="font-bold text-lg tracking-tight gradient-text">DevVault</span>
          )}
        </div>
        <nav className="flex-1 overflow-y-auto px-2 py-2 space-y-0.5">
          {nav.map(({ to, label, icon: Icon }) => (
            <NavLink
              key={to}
              to={to}
              title={label}
              className={({ isActive }) =>
                `flex items-center gap-3 rounded-lg px-3 py-2 text-sm transition-colors ${
                  isActive
                    ? "bg-surface2 text-ink font-medium border border-line"
                    : "text-muted hover:text-ink hover:bg-surface2"
                } ${sidebarCollapsed ? "justify-center" : ""}`
              }
            >
              <Icon className="w-4 h-4 shrink-0" />
              {!sidebarCollapsed && <span className="truncate">{label}</span>}
            </NavLink>
          ))}
        </nav>
        <div className="p-2 border-t border-line flex items-center justify-between gap-1">
          <button
            className="p-2 rounded-lg text-muted hover:text-ink hover:bg-surface2"
            title="Toggle theme"
            onClick={toggleTheme}
          >
            {theme === "dark" ? <Sun className="w-4 h-4" /> : <Moon className="w-4 h-4" />}
          </button>
          <button
            className="p-2 rounded-lg text-muted hover:text-ink hover:bg-surface2"
            title="Toggle sidebar"
            onClick={toggleSidebar}
          >
            {sidebarCollapsed ? <ChevronRight className="w-4 h-4" /> : <ChevronLeft className="w-4 h-4" />}
          </button>
        </div>
      </div>

      {/* Main */}
      <div className="flex-1 h-full overflow-hidden flex flex-col">
        <div className="h-14 shrink-0 border-b border-line bg-surface/70 backdrop-blur flex items-center px-4 gap-3">
          <button
            onClick={() => useApp.getState().setPaletteOpen(true)}
            className="input flex items-center gap-2 w-72 text-muted cursor-pointer"
          >
            <Search className="w-4 h-4" />
            <span className="text-sm">Search or jump to…</span>
            <kbd className="ml-auto chip">Ctrl K</kbd>
          </button>
        </div>
        <main className="flex-1 overflow-y-auto p-6">
          <AnimatePresence mode="wait">
            <motion.div
              key={location.pathname}
              initial={{ opacity: 0, y: 6 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.14 }}
            >
              <Outlet />
            </motion.div>
          </AnimatePresence>
        </main>
      </div>

      <CommandPalette onNavigate={(to) => navigate(to)} />
      <ToastHost />
    </div>
  );
}
