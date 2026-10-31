import { useEffect, useState } from "react";
import { Plus, Send, Trash2, Globe, Play } from "lucide-react";
import { api, ApiRequest, ApiResponse } from "@/lib/ipc";
import { Card, PageHeader, ErrorNote, Modal, Badge, EmptyState } from "@/components/ui";
import { useApp, errMessage } from "@/lib/store";

const METHODS = ["GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"];

export default function ApiPage() {
  const [collections, setCollections] = useState<{ id: string; name: string }[]>([]);
  const [colId, setColId] = useState("");
  const [requests, setRequests] = useState<ApiRequest[]>([]);
  const [envs, setEnvs] = useState<{ id: string; name: string; variables: Record<string, string> }[]>([]);
  const [activeEnv, setActiveEnv] = useState("");
  const [current, setCurrent] = useState<Partial<ApiRequest>>({ method: "GET", headers: {}, query: {}, authType: "none" });
  const [resp, setResp] = useState<ApiResponse | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [newCol, setNewCol] = useState("");
  const toast = useApp((s) => s.toast);

  const loadCollections = async () => {
    try {
      const list = await api.apiCollections();
      setCollections(list);
      if (list.length && !colId) pickCollection(list[0].id);
    } catch (e) {
      setError(errMessage(e));
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  };
  useEffect(() => { loadCollections(); }, []);

  const pickCollection = async (id: string) => {
    setColId(id);
    try {
      const [reqs, envList] = await Promise.all([api.apiRequests(id), api.apiEnvironments(id)]);
      setRequests(reqs);
      setEnvs(envList);
      if (reqs.length) setCurrent(reqs[0]);
      else setCurrent({ method: "GET", headers: {}, query: {}, authType: "none", collectionId: id });
    } catch (e) {
      setError(errMessage(e));
    }
  };

  const mergedEnv = (): Record<string, string> => {
    const e = envs.find((x) => x.name === activeEnv);
    return e ? e.variables : {};
  };

  const send = async () => {
    if (!current.url) return;
    setBusy(true);
    try {
      const saved = await api.apiSaveRequest({ ...current, collectionId: colId, name: current.name || "Quick request" });
      setCurrent(saved);
      setResp(await api.apiSend(saved, mergedEnv()));
      setRequests(await api.apiRequests(colId));
    } catch (e) {
      setError(errMessage(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div>
      <PageHeader
        title="API Workbench"
        subtitle="All HTTP methods, collections, environments and full request history."
        actions={
          <div className="flex gap-2">
            <input
              className="input w-40"
              placeholder="New collection…"
              value={newCol}
              onChange={(e) => setNewCol(e.target.value)}
              onKeyDown={async (e) => {
                if (e.key === "Enter" && newCol.trim()) {
                  const c = await api.apiCreateCollection(newCol.trim());
                  setNewCol("");
                  setCollections(await api.apiCollections());
                  pickCollection(c.id);
                }
              }}
            />
          </div>
        }
      />
      {error && <ErrorNote message={error} />}
      <div className="grid grid-cols-12 gap-4">
        <div className="col-span-12 md:col-span-3 space-y-3">
          <Card className="p-2">
            <div className="px-2 py-1 section-title">Collections</div>
            {collections.map((c) => (
              <div key={c.id} className={`group flex items-center justify-between rounded-lg px-2 py-1.5 text-sm cursor-pointer ${colId === c.id ? "bg-surface2" : "hover:bg-surface2"}`} onClick={() => pickCollection(c.id)}>
                <span className="truncate">{c.name}</span>
                <Trash2 className="w-3.5 h-3.5 text-muted opacity-0 group-hover:opacity-100 hover:!text-danger" onClick={async (e) => {
                  e.stopPropagation();
                  await api.apiDeleteCollection(c.id);
                  setCollections(await api.apiCollections());
                  if (colId === c.id) { setColId(""); setRequests([]); }
                }} />
              </div>
            ))}
            {collections.length === 0 && <div className="text-xs text-muted px-2 py-2">Type a name above to create one.</div>}
          </Card>
          {colId && (
            <Card className="p-2">
              <div className="px-2 py-1 section-title">Requests</div>
              {requests.map((r) => (
                <div key={r.id} className={`group flex items-center justify-between rounded-lg px-2 py-1.5 text-sm cursor-pointer ${current.id === r.id ? "bg-surface2" : "hover:bg-surface2"}`} onClick={() => setCurrent(r)}>
                  <span className="flex items-center gap-2 truncate">
                    <Badge tone="info">{r.method}</Badge>
                    <span className="truncate">{r.name}</span>
                  </span>
                  <Trash2 className="w-3.5 h-3.5 text-muted opacity-0 group-hover:opacity-100 hover:!text-danger" onClick={async (e) => {
                    e.stopPropagation();
                    await api.apiDeleteRequest(r.id);
                    setRequests(await api.apiRequests(colId));
                  }} />
                </div>
              ))}
              <button className="w-full text-left rounded-lg px-2 py-1.5 text-sm text-muted hover:text-ink hover:bg-surface2 flex items-center gap-2" onClick={() => setCurrent({ method: "GET", headers: {}, query: {}, authType: "none", collectionId: colId })}>
                <Plus className="w-3.5 h-3.5" /> New request
              </button>
            </Card>
          )}
        </div>

        <div className="col-span-12 md:col-span-9 space-y-4">
          <Card>
            <div className="flex gap-2 mb-3">
              <select className="input w-28" value={current.method ?? "GET"} onChange={(e) => setCurrent({ ...current, method: e.target.value })}>
                {METHODS.map((m) => <option key={m}>{m}</option>)}
              </select>
              <input className="input flex-1 font-mono" placeholder="https://api.example.com/v1/users?page={{page}}" value={current.url ?? ""} onChange={(e) => setCurrent({ ...current, url: e.target.value })} />
              <select className="input w-32" value={activeEnv} onChange={(e) => setActiveEnv(e.target.value)}>
                <option value="">No env</option>
                {envs.map((e) => <option key={e.id} value={e.name}>{e.name}</option>)}
              </select>
              <button className="btn-primary" onClick={send} disabled={busy || !current.url}>
                <Send className="w-4 h-4" /> {busy ? "Sending…" : "Send"}
              </button>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="text-xs text-muted">Request name</label>
                <input className="input w-full mt-1" placeholder="My request" value={current.name ?? ""} onChange={(e) => setCurrent({ ...current, name: e.target.value })} />
              </div>
              <div>
                <label className="text-xs text-muted">Auth (bearer token or user:pass)</label>
                <div className="flex gap-2 mt-1">
                  <select className="input w-24" value={current.authType ?? "none"} onChange={(e) => setCurrent({ ...current, authType: e.target.value })}>
                    <option value="none">none</option>
                    <option value="bearer">bearer</option>
                    <option value="basic">basic</option>
                  </select>
                  <input className="input flex-1 font-mono" value={current.authValue ?? ""} onChange={(e) => setCurrent({ ...current, authValue: e.target.value })} placeholder="{{token}}" />
                </div>
              </div>
              <div>
                <label className="text-xs text-muted">Headers (Key: Value per line)</label>
                <textarea
                  className="input w-full mt-1 font-mono text-xs h-20"
                  value={Object.entries(current.headers ?? {}).map(([k, v]) => `${k}: ${v}`).join("\n")}
                  onChange={(e) => {
                    const headers: Record<string, string> = {};
                    e.target.value.split("\n").forEach((line) => {
                      const i = line.indexOf(":");
                      if (i > 0) headers[line.slice(0, i).trim()] = line.slice(i + 1).trim();
                    });
                    setCurrent({ ...current, headers });
                  }}
                  placeholder="X-Custom: hello"
                />
              </div>
              <div>
                <label className="text-xs text-muted">Body</label>
                <textarea className="input w-full mt-1 font-mono text-xs h-20" value={current.body ?? ""} onChange={(e) => setCurrent({ ...current, body: e.target.value })} placeholder={'{"name": "value"}'} />
              </div>
            </div>
            <details className="mt-3">
              <summary className="text-xs text-muted cursor-pointer">Environment variables for "{activeEnv || "(create one)"}"</summary>
              <EnvEditor
                onSave={async (vars) => {
                  const name = activeEnv || "default";
                  await api.apiSetEnvironment(colId, name, vars);
                  setEnvs(await api.apiEnvironments(colId));
                  setActiveEnv(name);
                  toast(`Environment "${name}" saved`, "success");
                }}
                initial={envs.find((x) => x.name === activeEnv)?.variables ?? {}}
              />
            </details>
          </Card>

          {resp && (
            <Card className="p-0 overflow-hidden">
              <div className="flex items-center gap-3 px-4 py-2 border-b border-line text-sm">
                {resp.error ? <Badge tone="danger">network error</Badge> : (
                  <Badge tone={resp.status < 400 ? "ok" : "danger"}>{resp.status} {resp.statusText}</Badge>
                )}
                <span className="text-xs text-muted">{resp.elapsedMs} ms · {(resp.sizeBytes / 1024).toFixed(1)} KB</span>
              </div>
              {resp.error && <div className="px-4 py-3 text-sm text-danger">{resp.error}</div>}
              {!resp.error && (
                <pre className="text-xs font-mono p-4 overflow-auto max-h-96 whitespace-pre-wrap">{resp.body}</pre>
              )}
            </Card>
          )}

          {!resp && (
            <EmptyState icon={<Globe className="w-10 h-10" />} title="Send a request" hint="Use {{variables}} from the selected environment anywhere in URL, headers or body." />
          )}
        </div>
      </div>
    </div>
  );
}

function EnvEditor({ initial, onSave }: { initial: Record<string, string>; onSave: (vars: Record<string, string>) => Promise<void> }) {
  const [text, setText] = useState(
    Object.entries(initial).map(([k, v]) => `${k}=${v}`).join("\n")
  );
  const parse = () => {
    const vars: Record<string, string> = {};
    text.split("\n").forEach((l) => {
      const i = l.indexOf("=");
      if (i > 0) vars[l.slice(0, i).trim()] = l.slice(i + 1).trim();
    });
    return vars;
  };
  return (
    <div className="mt-2 flex gap-2">
      <textarea className="input flex-1 font-mono text-xs h-24" value={text} onChange={(e) => setText(e.target.value)} placeholder={"base=https://api.example.com\ntoken=abc123"} />
      <button className="btn-ghost self-start" onClick={() => onSave(parse()).catch((e) => useApp.getState().toast(errMessage(e), "error"))}>
        <Play className="w-3.5 h-3.5" /> Save env
      </button>
    </div>
  );
}
