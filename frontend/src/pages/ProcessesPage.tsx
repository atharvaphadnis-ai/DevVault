import { useEffect, useState } from "react";
import { Plus, Play, Square, Trash2, RefreshCw, PlayCircle } from "lucide-react";
import { api, Proc, ProcStatus } from "@/lib/ipc";
import { Card, PageHeader, Spinner, ErrorNote, EmptyState, Modal, ConfirmDialog, Badge } from "@/components/ui";
import { onEvent } from "@/lib/ipc";
import { useApp, errMessage } from "@/lib/store";

export default function ProcessesPage() {
  const [procs, setProcs] = useState<Proc[]>([]);
  const [statuses, setStatuses] = useState<Record<string, ProcStatus>>({});
  const [logs, setLogs] = useState<Record<string, string[]>>({});
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [addOpen, setAddOpen] = useState(false);
  const [stopTarget, setStopTarget] = useState<Proc | null>(null);
  const [active, setActive] = useState<string>("");
  const toast = useApp((s) => s.toast);

  const load = async () => {
    try {
      const list = await api.procsList();
      setProcs(list);
      const sts = await api.procStatusAll();
      const map: Record<string, ProcStatus> = {};
      sts.forEach((s) => (map[s.id] = s));
      setStatuses(map);
      if (list.length && !active) setActive(list[0].id);
      if (active) setLogs({ ...logs, [active]: await api.procLogs(active) });
      setError("");
    } catch (e) {
      setError(errMessage(e));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, []); // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => {
    const offLog = onEvent("proc:log", (p: any) => {
      setLogs((prev) => ({
        ...prev,
        [p.id]: [...(prev[p.id] ?? []).slice(-400), `${p.stream}: ${p.line}`],
      }));
    });
    const offStatus = onEvent("proc:status", () => load());
    const offExit = onEvent("proc:exit", (p: any) => {
      toast(`Process ${p.id.slice(0, 8)} exited ${p.err ? `(${p.err})` : ""}`, "info");
      load();
    });
    const t = setInterval(load, 5000);
    return () => { offLog(); offStatus(); offExit(); clearInterval(t); };
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <div>
      <PageHeader
        title="Processes & Services"
        subtitle="Register dev servers and long-running commands. Stopping always requires explicit confirmation — DevVault only ever stops what it started."
        actions={
          <>
            <button className="btn-ghost" onClick={load}><RefreshCw className="w-4 h-4" /></button>
            <button className="btn-primary" onClick={() => setAddOpen(true)}><Plus className="w-4 h-4" /> Register command</button>
          </>
        }
      />
      {error && <ErrorNote message={error} />}
      {loading && <Spinner />}
      {!loading && procs.length === 0 && !error && (
        <EmptyState
          icon={<PlayCircle className="w-10 h-10" />}
          title="No registered commands"
          hint='Example: name "API server", command "npm", args ["run","dev"], working dir /home/you/dev/api.'
        />
      )}
      {procs.length > 0 && (
        <div className="grid grid-cols-1 xl:grid-cols-3 gap-4">
          <div className="xl:col-span-1 space-y-2">
            {procs.map((p) => {
              const st = statuses[p.id];
              return (
                <div key={p.id} className={`card p-3 cursor-pointer hover:border-accent ${active === p.id ? "border-accent" : ""}`} onClick={() => setActive(p.id)}>
                  <div className="flex items-center justify-between gap-2">
                    <div className="min-w-0">
                      <div className="font-medium text-sm truncate">{p.name}</div>
                      <div className="text-xs text-muted font-mono truncate">{p.command} {p.args.join(" ")}</div>
                      {p.cwd && <div className="text-xs text-muted font-mono truncate">{p.cwd}</div>}
                    </div>
                    <div className="flex items-center gap-1.5 shrink-0">
                      {st?.running ? <Badge tone="ok">running</Badge> : <Badge tone="info">stopped</Badge>}
                      {st?.running ? (
                        <button className="btn-danger !p-2" title="Stop (confirmation required)" onClick={(e) => { e.stopPropagation(); setStopTarget(p); }}>
                          <Square className="w-3.5 h-3.5" />
                        </button>
                      ) : (
                        <button className="btn-primary !p-2" title="Start" onClick={async (e) => {
                          e.stopPropagation();
                          try { await api.procStart(p.id); load(); }
                          catch (err) { toast(errMessage(err), "error"); }
                        }}><Play className="w-3.5 h-3.5" /></button>
                      )}
                      <button className="p-2 text-muted hover:text-danger" title="Delete (must be stopped)" onClick={async (e) => {
                        e.stopPropagation();
                        try { await api.procDelete(p.id); load(); }
                        catch (err) { toast(errMessage(err), "error"); }
                      }}><Trash2 className="w-3.5 h-3.5" /></button>
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
          <Card className="xl:col-span-2 p-0 overflow-hidden flex flex-col max-h-[70vh]">
            <div className="px-4 py-2 border-b border-line text-sm flex items-center gap-2">
              <span className="section-title !mb-0">Logs</span>
              <span className="text-xs text-muted font-mono">{procs.find((p) => p.id === active)?.name}</span>
            </div>
            <div className="flex-1 overflow-y-auto p-4 bg-surface2/40 font-mono text-xs leading-relaxed">
              {(logs[active] ?? []).length === 0 ? (
                <div className="text-muted">No output yet. Start the process to stream live logs.</div>
              ) : (
                (logs[active] ?? []).map((l, i) => (
                  <div key={i} className={l.startsWith("stderr") ? "text-danger/90" : ""}>{l}</div>
                ))
              )}
            </div>
          </Card>
        </div>
      )}

      <AddProcModal open={addOpen} onClose={() => setAddOpen(false)} onSaved={() => { setAddOpen(false); load(); }} />
      <ConfirmDialog
        open={!!stopTarget}
        title={`Stop "${stopTarget?.name}"?`}
        message="This terminates the process (and its process group on Linux/macOS). Only proceed if you are sure."
        confirmLabel="Stop process"
        danger
        onCancel={() => setStopTarget(null)}
        onConfirm={async () => {
          try {
            await api.procStop(stopTarget!.id, true); // explicit confirmation gate
            toast("Stopped", "success");
          } catch (e) {
            toast(errMessage(e), "error");
          }
          setStopTarget(null);
          load();
        }}
      />
    </div>
  );
}

function AddProcModal({ open, onClose, onSaved }: { open: boolean; onClose: () => void; onSaved: () => void }) {
  const [name, setName] = useState("");
  const [command, setCommand] = useState("");
  const [args, setArgs] = useState("");
  const [cwd, setCwd] = useState("");
  const [err, setErr] = useState("");
  return (
    <Modal open={open} onClose={onClose} title="Register command">
      <div className="space-y-3">
        <div className="grid grid-cols-2 gap-3">
          <div>
            <label className="text-sm text-muted">Name *</label>
            <input className="input w-full mt-1" placeholder="API server" value={name} onChange={(e) => setName(e.target.value)} />
          </div>
          <div>
            <label className="text-sm text-muted">Command *</label>
            <input className="input w-full mt-1 font-mono" placeholder="npm" value={command} onChange={(e) => setCommand(e.target.value)} />
          </div>
        </div>
        <div>
          <label className="text-sm text-muted">Arguments (space separated)</label>
          <input className="input w-full mt-1 font-mono" placeholder='run dev' value={args} onChange={(e) => setArgs(e.target.value)} />
        </div>
        <div>
          <label className="text-sm text-muted">Working directory</label>
          <input className="input w-full mt-1 font-mono" placeholder="/home/you/dev/api" value={cwd} onChange={(e) => setCwd(e.target.value)} />
        </div>
        {err && <ErrorNote message={err} />}
        <div className="flex justify-end gap-2">
          <button className="btn-ghost" onClick={onClose}>Cancel</button>
          <button className="btn-primary" disabled={!name.trim() || !command.trim()} onClick={async () => {
            try {
              await api.procRegister(name.trim(), command.trim(), args.split(" ").map((a) => a.trim()).filter(Boolean), cwd.trim(), "");
              onSaved();
            } catch (e) { setErr(errMessage(e)); }
          }}>Register</button>
        </div>
      </div>
    </Modal>
  );
}
