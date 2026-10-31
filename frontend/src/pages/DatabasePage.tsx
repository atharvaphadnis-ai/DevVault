import { useEffect, useState } from "react";
import { Plus, Play, Trash2, Download, Database as DbIcon, History } from "lucide-react";
import CodeMirror from "@uiw/react-codemirror";
import { sql } from "@codemirror/lang-sql";
import { api, Connection, QueryResult } from "@/lib/ipc";
import { Card, PageHeader, Spinner, ErrorNote, Modal, Badge, EmptyState } from "@/components/ui";
import { useApp, errMessage } from "@/lib/store";

export default function DatabasePage() {
  const [conns, setConns] = useState<Connection[]>([]);
  const [selected, setSelected] = useState<Connection | null>(null);
  const [tables, setTables] = useState<string[]>([]);
  const [sqlText, setSqlText] = useState("SELECT 1");
  const [result, setResult] = useState<QueryResult | null>(null);
  const [history, setHistory] = useState<any[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [addOpen, setAddOpen] = useState(false);
  const toast = useApp((s) => s.toast);

  const load = async () => {
    try {
      const list = await api.dbListConnections();
      setConns(list);
      if (list.length && !selected) pick(list[0]);
    } catch (e) {
      setError(errMessage(e));
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  };
  useEffect(() => { load(); }, []);

  const pick = async (c: Connection) => {
    setSelected(c);
    setResult(null);
    try {
      setTables(await api.dbListTables(c.id));
      setHistory(await api.dbHistory(c.id, 50));
    } catch (e) {
      setTables([]);
      setError(errMessage(e));
    }
  };

  const run = async () => {
    if (!selected || !sqlText.trim()) return;
    setBusy(true);
    setError("");
    try {
      setResult(await api.dbQuery(selected.id, sqlText, 500));
      setHistory(await api.dbHistory(selected.id, 50));
    } catch (e) {
      setError(errMessage(e));
      setResult(null);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div>
      <PageHeader
        title="Database Workbench"
        subtitle="SQLite, PostgreSQL, MySQL & MariaDB — query, browse, export. Passwords live in your OS keyring (or encrypted file fallback)."
        actions={
          <button className="btn-primary" onClick={() => setAddOpen(true)}>
            <Plus className="w-4 h-4" /> New connection
          </button>
        }
      />
      {error && <ErrorNote message={error} />}
      <div className="grid grid-cols-12 gap-4">
        {/* connections sidebar */}
        <div className="col-span-12 md:col-span-3 space-y-2">
          {conns.length === 0 && (
            <Card><div className="text-sm text-muted">No connections yet.</div></Card>
          )}
          {conns.map((c) => (
            <button
              key={c.id}
              className={`w-full text-left card p-3 hover:border-accent transition-colors ${selected?.id === c.id ? "border-accent" : ""}`}
              onClick={() => pick(c)}
            >
              <div className="font-medium text-sm flex items-center gap-2">
                <DbIcon className="w-3.5 h-3.5 text-muted" /> {c.name}
              </div>
              <div className="text-xs text-muted mt-1 font-mono truncate">
                {c.type === "sqlite" ? c.path : `${c.host}:${c.port}/${c.database}`}
              </div>
              <div className="text-xs text-muted mt-1 capitalize">{c.type}</div>
            </button>
          ))}
        </div>

        {/* editor + result */}
        <div className="col-span-12 md:col-span-9 space-y-4">
          {selected && (
            <>
              <Card className="p-0 overflow-hidden">
                <div className="flex items-center justify-between px-4 py-2 border-b border-line">
                  <div className="flex items-center gap-2 text-sm">
                    <b>{selected.name}</b>
                    <Badge tone="info">{selected.type}</Badge>
                    <button className="btn-ghost !py-1 !px-2 text-xs" onClick={async () => {
                      try { await api.dbTest(selected.id); toast("Connection OK", "success"); }
                      catch (e) { toast(errMessage(e), "error"); }
                    }}>Test</button>
                  </div>
                  <div className="flex gap-2">
                    <button className="btn-ghost !py-1 !px-2 text-xs" onClick={async () => {
                      try {
                        const p = await api.dbExport(selected.id, sqlText, "csv");
                        toast(`Exported to ${p}`, "success");
                      } catch (e) { toast(errMessage(e), "error"); }
                    }}><Download className="w-3 h-3" /> CSV</button>
                    <button className="btn-ghost !py-1 !px-2 text-xs" onClick={async () => {
                      try {
                        const p = await api.dbExport(selected.id, sqlText, "json");
                        toast(`Exported to ${p}`, "success");
                      } catch (e) { toast(errMessage(e), "error"); }
                    }}><Download className="w-3 h-3" /> JSON</button>
                    <button className="btn-ghost !py-1 !px-2 text-xs hover:!text-danger" onClick={async () => {
                      await api.dbDeleteConnection(selected.id);
                      setSelected(null); setTables([]); load();
                    }}><Trash2 className="w-3 h-3" /></button>
                  </div>
                </div>
                <CodeMirror
                  value={sqlText}
                  height="160px"
                  extensions={[sql()]}
                  theme="dark"
                  onChange={(v) => setSqlText(v)}
                  basicSetup={{ lineNumbers: true, foldGutter: false }}
                />
                <div className="flex items-center gap-3 px-4 py-2 border-t border-line">
                  <button className="btn-primary !py-1.5" onClick={run} disabled={busy}>
                    <Play className="w-4 h-4" /> {busy ? "Running…" : "Run (Ctrl+Enter)"}
                  </button>
                  {tables.length > 0 && (
                    <div className="flex gap-1.5 flex-wrap">
                      {tables.slice(0, 8).map((t) => (
                        <button key={t} className="chip hover:text-ink" onClick={() => setSqlText(`SELECT * FROM ${t} LIMIT 50`)}>
                          {t}
                        </button>
                      ))}
                    </div>
                  )}
                </div>
              </Card>

              {result && (
                <Card className="p-0 overflow-hidden">
                  <div className="px-4 py-2 border-b border-line text-xs text-muted flex justify-between">
                    <span>
                      {result.rowsAffected > 0
                        ? `${result.rowsAffected} row(s) affected`
                        : `${result.rows.length} row(s)`}
                      {result.truncatedRows && " (truncated)"}
                    </span>
                    <span>{result.elapsedMs} ms</span>
                  </div>
                  <div className="overflow-auto max-h-96">
                    <table className="w-full text-sm">
                      <thead className="bg-surface2 sticky top-0">
                        <tr>
                          {result.columns.map((c) => (
                            <th key={c} className="text-left px-3 py-2 font-medium border-b border-line whitespace-nowrap">{c}</th>
                          ))}
                        </tr>
                      </thead>
                      <tbody>
                        {result.rows.map((row, i) => (
                          <tr key={i} className="hover:bg-surface2/60">
                            {row.map((v, j) => (
                              <td key={j} className="px-3 py-1.5 border-b border-line/50 font-mono text-xs whitespace-nowrap">
                                {v === null ? <span className="text-muted italic">NULL</span> : String(v)}
                              </td>
                            ))}
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </Card>
              )}

              <Card>
                <div className="flex items-center gap-2 mb-3">
                  <History className="w-4 h-4 text-muted" />
                  <span className="section-title">Query history</span>
                </div>
                {history.length === 0 ? (
                  <div className="text-sm text-muted">Nothing yet.</div>
                ) : (
                  <div className="space-y-1.5 max-h-60 overflow-y-auto">
                    {history.map((h) => (
                      <button
                        key={h.id}
                        className="w-full text-left px-3 py-2 rounded-lg hover:bg-surface2 text-xs font-mono flex items-center gap-2"
                        onClick={() => setSqlText(h.sql)}
                      >
                        <Badge tone={h.ok ? "ok" : "danger"}>{h.ok ? "ok" : "err"}</Badge>
                        <span className="truncate flex-1">{h.sql}</span>
                        <span className="text-muted shrink-0">{h.elapsedMs}ms</span>
                      </button>
                    ))}
                  </div>
                )}
              </Card>
            </>
          )}
          {!selected && conns.length === 0 && (
            <EmptyState icon={<DbIcon className="w-10 h-10" />} title="No connection selected" hint="Create a connection to get started." />
          )}
        </div>
      </div>
      {busy && <Spinner label="Executing…" />}
      <AddConnectionModal open={addOpen} onClose={() => setAddOpen(false)} onAdded={() => { setAddOpen(false); load(); }} />
    </div>
  );
}

function AddConnectionModal({ open, onClose, onAdded }: { open: boolean; onClose: () => void; onAdded: () => void }) {
  const [type, setType] = useState("sqlite");
  const [name, setName] = useState("");
  const [host, setHost] = useState("");
  const [port, setPort] = useState(5432);
  const [user, setUser] = useState("");
  const [password, setPassword] = useState("");
  const [database, setDatabase] = useState("");
  const [path, setPath] = useState("");
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);

  return (
    <Modal open={open} onClose={onClose} title="New database connection">
      <div className="space-y-3">
        <div className="grid grid-cols-2 gap-3">
          <div>
            <label className="text-sm text-muted">Name *</label>
            <input className="input w-full mt-1" value={name} onChange={(e) => setName(e.target.value)} placeholder="Local Postgres" />
          </div>
          <div>
            <label className="text-sm text-muted">Type *</label>
            <select className="input w-full mt-1" value={type} onChange={(e) => setType(e.target.value)}>
              <option value="sqlite">SQLite</option>
              <option value="postgres">PostgreSQL</option>
              <option value="mysql">MySQL / MariaDB</option>
            </select>
          </div>
        </div>
        {type === "sqlite" ? (
          <div>
            <label className="text-sm text-muted">Database file path *</label>
            <input className="input w-full mt-1 font-mono" value={path} onChange={(e) => setPath(e.target.value)} placeholder="/path/to/data.sqlite" />
          </div>
        ) : (
          <>
            <div className="grid grid-cols-3 gap-3">
              <div className="col-span-2">
                <label className="text-sm text-muted">Host *</label>
                <input className="input w-full mt-1" value={host} onChange={(e) => setHost(e.target.value)} placeholder="localhost" />
              </div>
              <div>
                <label className="text-sm text-muted">Port</label>
                <input className="input w-full mt-1" type="number" value={port} onChange={(e) => setPort(Number(e.target.value))} />
              </div>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="text-sm text-muted">User</label>
                <input className="input w-full mt-1" value={user} onChange={(e) => setUser(e.target.value)} />
              </div>
              <div>
                <label className="text-sm text-muted">Password</label>
                <input className="input w-full mt-1" type="password" value={password} onChange={(e) => setPassword(e.target.value)} />
              </div>
            </div>
            <div>
              <label className="text-sm text-muted">Database *</label>
              <input className="input w-full mt-1" value={database} onChange={(e) => setDatabase(e.target.value)} />
            </div>
          </>
        )}
        {err && <ErrorNote message={err} />}
        <div className="flex justify-end gap-2">
          <button className="btn-ghost" onClick={onClose}>Cancel</button>
          <button className="btn-primary" disabled={busy || !name.trim()} onClick={async () => {
            setBusy(true); setErr("");
            try {
              await api.dbCreateConnection(name.trim(), type, host.trim(), port, user, password, database.trim(), path.trim());
              onAdded();
            } catch (e) { setErr(errMessage(e)); }
            finally { setBusy(false); }
          }}>{busy ? "Saving…" : "Create"}</button>
        </div>
      </div>
    </Modal>
  );
}
