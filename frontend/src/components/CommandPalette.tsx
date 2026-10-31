import { useEffect, useMemo, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { Search, CornerDownLeft } from "lucide-react";
import { AnimatePresence, motion } from "framer-motion";
import { api, SearchHit } from "@/lib/ipc";
import { useApp } from "@/lib/store";

interface Cmd {
  id: string;
  label: string;
  hint?: string;
  run: () => void;
}

const routes: { to: string; label: string }[] = [
  { to: "/", label: "Go to Dashboard" },
  { to: "/projects", label: "Go to Projects" },
  { to: "/git", label: "Go to Git" },
  { to: "/terminal", label: "Go to Terminal" },
  { to: "/database", label: "Go to Databases" },
  { to: "/api", label: "Go to API Workbench" },
  { to: "/env", label: "Go to Environments" },
  { to: "/snippets", label: "Go to Snippets & Notes" },
  { to: "/tasks", label: "Go to Tasks" },
  { to: "/processes", label: "Go to Processes" },
  { to: "/containers", label: "Go to Containers" },
  { to: "/remote", label: "Go to DevVault Anywhere" },
  { to: "/plugins", label: "Go to Plugins" },
  { to: "/settings", label: "Go to Settings" },
];

export default function CommandPalette({ onNavigate }: { onNavigate: (to: string) => void }) {
  const { paletteOpen, setPaletteOpen, toggleTheme, toast } = useApp();
  const [q, setQ] = useState("");
  const [hits, setHits] = useState<SearchHit[]>([]);
  const [cursor, setCursor] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);
  const navigate = useNavigate();

  const commands = useMemo<Cmd[]>(() => {
    const list: Cmd[] = routes.map((r) => ({
      id: "nav:" + r.to,
      label: r.label,
      run: () => onNavigate(r.to),
    }));
    list.push({ id: "theme", label: "Toggle dark/light theme", hint: "persisted", run: toggleTheme });
    return list;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [toggleTheme]);

  useEffect(() => {
    if (paletteOpen) {
      setQ("");
      setHits([]);
      setCursor(0);
      setTimeout(() => inputRef.current?.focus(), 30);
    }
  }, [paletteOpen]);

  // global search (backend) with debounce
  useEffect(() => {
    if (!paletteOpen || q.trim().length < 2) {
      setHits([]);
      return;
    }
    const t = setTimeout(async () => {
      try {
        setHits(await api.search(q.trim()));
      } catch {
        setHits([]);
      }
    }, 180);
    return () => clearTimeout(t);
  }, [q, paletteOpen]);

  const navMatches = commands.filter((c) =>
    c.label.toLowerCase().includes(q.toLowerCase())
  );
  const items: Cmd[] = [
    ...navMatches.slice(0, 8),
    ...hits.map((h) => ({
      id: `hit:${h.type}:${h.id}`,
      label: h.title,
      hint: `${h.type} — ${h.subtitle}`,
      run: () => {
        // jump to the most relevant module for the hit type
        const map: Record<string, string> = {
          project: "/projects",
          snippet: "/snippets",
          note: "/snippets",
          task: "/tasks",
          request: "/api",
          env: "/env",
        };
        onNavigate(map[h.type] ?? "/");
      },
    })),
  ];

  const execute = (i: number) => {
    const item = items[i];
    if (!item) return;
    setPaletteOpen(false);
    try {
      item.run();
    } catch (e) {
      toast(String(e), "error");
    }
  };

  return (
    <AnimatePresence>
      {paletteOpen && (
        <motion.div
          className="fixed inset-0 z-[70] flex items-start justify-center pt-24 px-4"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
        >
          <div className="absolute inset-0 bg-black/60 backdrop-blur-sm" onClick={() => setPaletteOpen(false)} />
          <motion.div
            initial={{ scale: 0.97, y: -8, opacity: 0 }}
            animate={{ scale: 1, y: 0, opacity: 1 }}
            exit={{ scale: 0.97, y: -8, opacity: 0 }}
            transition={{ duration: 0.14 }}
            className="card relative w-full max-w-xl overflow-hidden"
          >
            <div className="flex items-center gap-3 px-4 py-3 border-b border-line">
              <Search className="w-4 h-4 text-muted" />
              <input
                ref={inputRef}
                value={q}
                onChange={(e) => {
                  setQ(e.target.value);
                  setCursor(0);
                }}
                onKeyDown={(e) => {
                  if (e.key === "ArrowDown") {
                    e.preventDefault();
                    setCursor((c) => Math.min(c + 1, items.length - 1));
                  } else if (e.key === "ArrowUp") {
                    e.preventDefault();
                    setCursor((c) => Math.max(c - 1, 0));
                  } else if (e.key === "Enter") {
                    e.preventDefault();
                    execute(cursor);
                  } else if (e.key === "Escape") {
                    setPaletteOpen(false);
                  }
                }}
                placeholder="Search projects, snippets, tasks, or type a command…"
                className="flex-1 bg-transparent outline-none text-sm"
              />
              <kbd className="chip">ESC</kbd>
            </div>
            <div className="max-h-80 overflow-y-auto py-1">
              {items.length === 0 && (
                <div className="px-4 py-6 text-sm text-muted text-center">No matches</div>
              )}
              {items.map((item, i) => (
                <button
                  key={item.id}
                  onMouseEnter={() => setCursor(i)}
                  onClick={() => execute(i)}
                  className={`w-full text-left px-4 py-2.5 text-sm flex items-center gap-3 ${
                    i === cursor ? "bg-surface2" : ""
                  }`}
                >
                  <CornerDownLeft className="w-3.5 h-3.5 text-muted shrink-0" />
                  <span className="truncate">{item.label}</span>
                  {item.hint && <span className="ml-auto text-xs text-muted truncate">{item.hint}</span>}
                </button>
              ))}
            </div>
          </motion.div>
        </motion.div>
      )}
    </AnimatePresence>
  );
}
