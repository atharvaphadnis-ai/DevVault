import { useEffect, useState } from "react";
import { Eye, EyeOff, Plus, Trash2, FileDown, FileUp, KeyRound } from "lucide-react";
import { api, EnvVar, Project } from "@/lib/ipc";
import { Card, PageHeader, Spinner, ErrorNote, EmptyState, Modal, Badge } from "@/components/ui";
import { useApp, errMessage } from "@/lib/store";

export default function EnvPage() {
  const [projects, setProjects] = useState<Project[]>([]);
  const [projectId, setProjectId] = useState("");
  const [envs, setEnvs] = useState<string[]>([]);
  const [env, setEnv] = useState("development");
  const [vars, setVars] = useState<EnvVar[]>([]);
  const [revealed, setRevealed] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [importOpen, setImportOpen] = useState(false);
  const toast = useApp((s) => s.toast);

  useEffect(() => {
    (async () => {
      try {
        const list = await api.projectsList();
        setProjects(list);
        if (list.length) setProjectId(list[0].id);
      } catch (e) {
        setError(errMessage(e));
      }
    })();
  }, []);

  const loadEnvs = async (pid: string) => {
    try {
      const list = await api.envEnvironments(pid);
      setEnvs(list);
      setEnv(list.includes("development") ? "development" : list[0] ?? "development");
    } catch (e) {
      setError(errMessage(e));
    }
  };

  const loadVars = async () => {
    if (!projectId) return;
    setLoading(true);
    setRevealed({});
    try {
      setVars(await api.envGet(projectId, env));
      setError("");
    } catch (e) {
      setError(errMessage(e));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { loadEnvs(projectId); }, [projectId]);
  useEffect(() => { loadVars(); /* eslint-disable-next-line */ }, [projectId, env]);

  const reveal = async (key: string) => {
    try {
      const v = await api.envReveal(projectId, env, key);
      setRevealed((r) => ({ ...r, [key]: v }));
      setTimeout(() => setRevealed((r) => {
        const c = { ...r };
        delete c[key];
        return c;
      }), 10000); // auto re-mask after 10s
    } catch (e) {
      toast(errMessage(e), "error");
    }
  };

  return (
    <div>
      <PageHeader
        title="Environment Variables"
        subtitle="Per-project, per-environment config. Secret values are masked and reveals are audited."
        actions={
          <>
            <select className="input" value={projectId} onChange={(e) => setProjectId(e.target.value)}>
              {projects.map((p) => <option key={p.id} value={p.id}>{p.name}</option>)}
            </select>
            <select className="input" value={env} onChange={(e) => setEnv(e.target.value)}>
              {(envs.length ? envs : ["development"]).map((n) => <option key={n}>{n}</option>)}
              {!envs.includes("development") && <option>development</option>}
            </select>
            <button className="btn-ghost" onClick={() => setImportOpen(true)}>
              <FileUp className="w-4 h-4" /> Import .env
            </button>
            <button className="btn-ghost" onClick={async () => {
              try {
                const text = await api.envExport(projectId, env);
                const blob = new Blob([text], { type: "text/plain" });
                const a = document.createElement("a");
                a.href = URL.createObjectURL(blob);
                a.download = `.env.${env}`;
                a.click();
                toast(`Exported .env.${env}`, "success");
              } catch (e) { toast(errMessage(e), "error"); }
            }}><FileDown className="w-4 h-4" /> Export</button>
          </>
        }
      />
      {error && <ErrorNote message={error} />}
      {!projectId && <EmptyState icon={<KeyRound className="w-10 h-10" />} title="No project selected" hint="Add a project first — environment variables are stored per project." />}
      {projectId && (
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
          <Card className="lg:col-span-2 p-0 overflow-hidden">
            <div className="px-4 py-3 border-b border-line flex items-center justify-between">
              <span className="section-title">Variables — {env}</span>
              <Badge tone="info">{vars.length} entries</Badge>
            </div>
            {loading ? <Spinner /> : vars.length === 0 ? (
              <EmptyState title="No variables yet" hint="Add your first variable on the right." />
            ) : (
              <table className="w-full text-sm">
                <tbody>
                  {vars.map((v) => (
                    <tr key={v.key} className="border-b border-line/50 hover:bg-surface2/50">
                      <td className="px-4 py-2 font-mono text-xs w-1/3">{v.key}</td>
                      <td className="px-4 py-2 font-mono text-xs">
                        {v.secret && !revealed[v.key] ? (
                          <span className="text-muted tracking-widest">••••••••</span>
                        ) : (
                          <span>{revealed[v.key] ?? v.value}</span>
                        )}
                      </td>
                      <td className="px-4 py-2 text-right whitespace-nowrap">
                        {v.secret && (
                          <button className="p-1.5 text-muted hover:text-ink" title="Reveal for 10s (audited)" onClick={() => reveal(v.key)}>
                            {revealed[v.key] ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                          </button>
                        )}
                        <button className="p-1.5 text-muted hover:text-danger" title="Delete" onClick={async () => {
                          await api.envDelete(projectId, env, v.key);
                          loadVars();
                        }}><Trash2 className="w-4 h-4" /></button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </Card>
          <Card>
            <div className="section-title mb-3">Add variable</div>
            <AddVar
              onAdd={async (key, value, secret) => {
                await api.envSet(projectId, env, key, value, secret);
                toast(`${key} saved`, "success");
                loadVars();
              }}
            />
            <div className="mt-6 text-xs text-muted leading-relaxed">
              Keys matching secret/token/password/api_key are automatically flagged as secret.
              Masked values sent back from the UI keep their stored value — nothing is clobbered.
            </div>
          </Card>
        </div>
      )}

      <Modal open={importOpen} onClose={() => setImportOpen(false)} title={`Import .env into "${env}"`}>
        <ImportEnv onImport={async (text) => {
          const n = await api.envImport(projectId, env, text);
          toast(`Imported ${n} variables`, "success");
          setImportOpen(false);
          loadVars();
        }} />
      </Modal>
    </div>
  );
}

function AddVar({ onAdd }: { onAdd: (key: string, value: string, secret: boolean) => Promise<void> }) {
  const [key, setKey] = useState("");
  const [value, setValue] = useState("");
  const [secret, setSecret] = useState(false);
  return (
    <div className="space-y-2">
      <input className="input w-full font-mono" placeholder="KEY_NAME" value={key} onChange={(e) => setKey(e.target.value)} />
      <input className="input w-full font-mono" placeholder="value" value={value} onChange={(e) => setValue(e.target.value)} />
      <label className="flex items-center gap-2 text-sm text-muted">
        <input type="checkbox" checked={secret} onChange={(e) => setSecret(e.target.checked)} />
        Mark as secret (masked in UI)
      </label>
      <button className="btn-primary w-full justify-center" disabled={!key.trim()} onClick={async () => {
        await onAdd(key.trim(), value, secret);
        setKey(""); setValue(""); setSecret(false);
      }}><Plus className="w-4 h-4" /> Add</button>
    </div>
  );
}

function ImportEnv({ onImport }: { onImport: (text: string) => Promise<void> }) {
  const [text, setText] = useState("# paste .env content here\nAPI_URL=https://api.example.com\nAPI_TOKEN=secret123");
  return (
    <div className="space-y-3">
      <textarea className="input w-full font-mono text-xs h-48" value={text} onChange={(e) => setText(e.target.value)} />
      <div className="flex justify-end">
        <button className="btn-primary" onClick={() => onImport(text)}>Import</button>
      </div>
    </div>
  );
}
