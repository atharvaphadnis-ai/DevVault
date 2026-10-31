import { useEffect, useRef, useState } from "react";
import { Plus, X, TerminalSquare } from "lucide-react";
import { api, Project } from "@/lib/ipc";
import { PageHeader, EmptyState } from "@/components/ui";
import { useApp, errMessage } from "@/lib/store";

type Tab = { id: string; projectPath: string; name: string };

// xterm is loaded lazily so the main bundle stays fast.
async function loadXterm() {
  const [{ Terminal }, { FitAddon }] = await Promise.all([
    import("@xterm/xterm"),
    import("@xterm/addon-fit"),
  ]);
  await import("@xterm/xterm/css/xterm.css");
  return { Terminal, FitAddon };
}

export default function TerminalPage() {
  const [tabs, setTabs] = useState<Tab[]>([]);
  const [active, setActive] = useState("");
  const [supported, setSupported] = useState(true);
  const [projects, setProjects] = useState<Project[]>([]);
  const holderRef = useRef<HTMLDivElement>(null);
  const termRef = useRef<any>(null);
  const fitRef = useRef<any>(null);
  const toast = useApp((s) => s.toast);

  useEffect(() => {
    (async () => {
      try {
        setSupported(await api.termSupported());
        setProjects(await api.projectsList());
      } catch (e) {
        toast(errMessage(e), "error");
      }
    })();
  }, [toast]);

  const openSession = async (cwd: string, name: string) => {
    try {
      const id = await api.termStart(cwd, "");
      setTabs((t) => [...t, { id, projectPath: cwd, name }]);
      setActive(id);
    } catch (e) {
      toast(errMessage(e), "error");
    }
  };

  // attach / detach xterm to the active session
  useEffect(() => {
    let disposed = false;
    let offData: (() => void) | undefined;

    (async () => {
      if (!active || !holderRef.current) return;
      // clear holder
      holderRef.current.innerHTML = "";
      const { Terminal, FitAddon } = await loadXterm();
      if (disposed) return;
      const term = new Terminal({
        fontSize: 13,
        fontFamily: "'JetBrains Mono', monospace",
        cursorBlink: true,
        theme: {
          background: "#11151e",
          foreground: "#e6eaf2",
          cursor: "#22d3ee",
          selectionBackground: "#6366f155",
        },
      });
      const fit = new FitAddon();
      term.loadAddon(fit);
      term.open(holderRef.current);
      termRef.current = term;
      fitRef.current = fit;
      try {
        fit.fit();
        const cols = term.cols, rows = term.rows;
        api.termResize(active, cols, rows).catch(() => {});
      } catch { /* layout not ready */ }

      term.onData((data) => api.termWrite(active, data).catch(() => {}));

      const off = apiAppEvent("term:data", (payload: any) => {
        if (payload?.id !== active) return;
        const b64: string = payload.b64 ?? "";
        try {
          const bin = atob(b64);
          const bytes = new Uint8Array(bin.length);
          for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
          term.write(bytes);
        } catch { /* ignore bad chunk */ }
      });
      const offExit = apiAppEvent("term:exit", (payload: any) => {
        if (payload?.id !== active) return;
        term.write("\r\n\x1b[90m[session ended]\r\n");
      });
      offData = () => { off(); offExit(); };

      const ro = new ResizeObserver(() => {
        try {
          fit.fit();
          api.termResize(active, term.cols, term.rows).catch(() => {});
        } catch { /* ignore */ }
      });
      ro.observe(holderRef.current);
      return () => ro.disconnect();
    })();

    return () => {
      disposed = true;
      offData?.();
      termRef.current = null;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [active]);

  const closeTab = async (tab: Tab) => {
    try {
      await api.termClose(tab.id);
    } catch { /* already gone */ }
    setTabs((ts) => {
      const next = ts.filter((t) => t.id !== tab.id);
      if (active === tab.id) setActive(next.length ? next[next.length - 1].id : "");
      return next;
    });
  };

  if (!supported) {
    return (
      <div>
        <PageHeader title="Terminal" />
        <EmptyState
          icon={<TerminalSquare className="w-10 h-10" />}
          title="Integrated terminal is not supported on this platform yet"
          hint="Windows ConPTY support is planned. See docs/TROUBLESHOOTING.md. On Linux and macOS you get real PTY-backed shells."
        />
      </div>
    );
  }

  return (
    <div className="h-[calc(100vh-8.5rem)] flex flex-col">
      <PageHeader
        title="Terminal"
        subtitle="Real PTY sessions — nothing simulated. Output streams live from the shell."
        actions={
          <>
            {projects.length > 0 && (
              <select
                className="input text-xs"
                value=""
                onChange={(e) => {
                  const p = projects.find((x) => x.path === e.target.value);
                  if (p) openSession(p.path, p.name);
                }}
              >
                <option value="">New tab in project…</option>
                {projects.map((p) => (
                  <option key={p.id} value={p.path}>{p.name}</option>
                ))}
              </select>
            )}
            <button className="btn-primary" onClick={() => openSession("", "shell")}>
              <Plus className="w-4 h-4" /> New shell
            </button>
          </>
        }
      />
      {tabs.length === 0 ? (
        <EmptyState
          icon={<TerminalSquare className="w-10 h-10" />}
          title="No open sessions"
          hint="Open a new shell or pick a project to start in its directory."
        />
      ) : (
        <div className="card flex-1 min-h-0 flex flex-col overflow-hidden">
          <div className="flex border-b border-line overflow-x-auto shrink-0">
            {tabs.map((t) => (
              <div
                key={t.id}
                className={`flex items-center gap-2 px-3 py-2 text-sm border-r border-line cursor-pointer select-none ${
                  active === t.id ? "bg-surface2 text-ink" : "text-muted hover:text-ink"
                }`}
                onClick={() => setActive(t.id)}
              >
                <span className="truncate max-w-40">{t.name}</span>
                <X className="w-3.5 h-3.5 hover:text-danger" onClick={(e) => { e.stopPropagation(); closeTab(t); }} />
              </div>
            ))}
          </div>
          <div ref={holderRef} className="xterm-holder flex-1 min-h-0" />
        </div>
      )}
    </div>
  );
}

// small helper subscribing to backend events
function apiAppEvent(name: string, cb: (data: any) => void): () => void {
  const rt = (window as any).runtime;
  if (!rt?.EventsOn) return () => {};
  rt.EventsOn(name, cb);
  return () => {
    try { rt.EventsOff(name); } catch { /* noop */ }
  };
}
