import { useEffect, useState } from "react";
import { Moon, Sun, ShieldCheck, ScrollText, Server, Copy } from "lucide-react";
import { api } from "@/lib/ipc";
import { Card, PageHeader, Spinner, ErrorNote, Badge } from "@/components/ui";
import { useApp, errMessage } from "@/lib/store";

export default function SettingsPage() {
  const [settings, setSettings] = useState<Record<string, string>>({});
  const [audit, setAudit] = useState<any[]>([]);
  const [dataDir, setDataDir] = useState("");
  const [localApi, setLocalApi] = useState<{ running: boolean; port: number; token: string } | null>(null);
  const [apiPort, setApiPort] = useState(7920);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const { theme, toggleTheme, toast } = useApp();

  const load = async () => {
    try {
      const [s, a, d, l] = await Promise.all([
        api.settingsAll(),
        api.auditList(80),
        api.dataDir(),
        api.localApiStatus(),
      ]);
      setSettings(s);
      setAudit(a);
      setDataDir(d);
      setLocalApi(l);
      setError("");
    } catch (e) {
      setError(errMessage(e));
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => { load(); }, []);

  return (
    <div>
      <PageHeader title="Settings" subtitle="Appearance, local API and the audit trail." />
      {error && <ErrorNote message={error} />}
      {loading && <Spinner />}
      {!loading && (
        <div className="space-y-4">
          <Card>
            <div className="section-title mb-3">Appearance</div>
            <button className="btn-ghost" onClick={toggleTheme}>
              {theme === "dark" ? <Sun className="w-4 h-4" /> : <Moon className="w-4 h-4" />}
              {theme === "dark" ? "Switch to light theme" : "Switch to dark theme"}
            </button>
            <p className="text-xs text-muted mt-2">The choice is persisted in the DevVault database and follows you across restarts.</p>
          </Card>

          <Card>
            <div className="flex items-center gap-2 mb-3">
              <Server className="w-4 h-4 text-muted" />
              <span className="section-title !mb-0">Local REST API (opt-in, default off)</span>
            </div>
            {localApi?.running ? (
              <div className="space-y-2">
                <div className="flex items-center gap-2 text-sm">
                  <Badge tone="ok">running</Badge>
                  <span className="font-mono text-xs text-muted">http://127.0.0.1:{localApi.port}</span>
                </div>
                <div className="flex items-center gap-2 text-xs">
                  <code className="chip font-mono break-all">Bearer {localApi.token}</code>
                  <button className="p-1 text-muted hover:text-ink" title="Copy token" onClick={() => {
                    navigator.clipboard?.writeText(localApi.token).then(() => toast("Token copied", "success")).catch(() => {});
                  }}><Copy className="w-3.5 h-3.5" /></button>
                </div>
                <button className="btn-danger !py-1.5 text-xs" onClick={async () => {
                  await api.localApiDisable();
                  toast("Local API disabled", "success");
                  load();
                }}>Disable</button>
              </div>
            ) : (
              <div className="space-y-3">
                <p className="text-xs text-muted leading-relaxed">
                  When enabled, DevVault serves a read-only JSON API on <b>127.0.0.1 only</b>, protected by a bearer token.
                  It is disabled by default and every enable/disable is audited.
                </p>
                <div className="flex gap-2 items-end">
                  <div>
                    <label className="text-xs text-muted">Port</label>
                    <input className="input w-28 mt-1" type="number" value={apiPort} onChange={(e) => setApiPort(Number(e.target.value))} />
                  </div>
                  <button className="btn-primary" onClick={async () => {
                    try {
                      await api.localApiEnable(apiPort);
                      toast(`Local API enabled on 127.0.0.1:${apiPort}`, "success");
                      load();
                    } catch (e) { toast(errMessage(e), "error"); }
                  }}>Enable</button>
                </div>
              </div>
            )}
          </Card>

          <Card>
            <div className="flex items-center gap-2 mb-3">
              <ShieldCheck className="w-4 h-4 text-muted" />
              <span className="section-title !mb-0">Storage</span>
            </div>
            <div className="text-sm text-muted space-y-1">
              <div>Data directory: <code className="chip font-mono">{dataDir}</code></div>
              <div className="text-xs">SQLite database, encrypted secrets and the audit log live here. Deleting the directory resets DevVault (projects are never modified).</div>
            </div>
          </Card>

          <Card className="p-0 overflow-hidden">
            <div className="px-4 py-3 border-b border-line flex items-center gap-2">
              <ScrollText className="w-4 h-4 text-muted" />
              <span className="section-title !mb-0">Audit log (latest 80)</span>
            </div>
            <div className="overflow-y-auto max-h-96">
              <table className="w-full text-xs">
                <tbody>
                  {audit.map((e) => (
                    <tr key={e.id} className="border-b border-line/40 hover:bg-surface2/50">
                      <td className="px-4 py-1.5 text-muted whitespace-nowrap w-44">{e.ts}</td>
                      <td className="px-4 py-1.5 font-mono">{e.action}</td>
                      <td className="px-4 py-1.5 text-muted">{e.detail}</td>
                      <td className="px-4 py-1.5 w-16 text-right text-muted">{e.actor}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
              {audit.length === 0 && <div className="px-4 py-6 text-sm text-muted text-center">No audit entries yet.</div>}
            </div>
          </Card>

          <Card>
            <div className="section-title mb-3">All settings (raw)</div>
            <pre className="text-xs font-mono bg-surface2 rounded-lg p-3 overflow-auto max-h-60">{JSON.stringify(settings, null, 2)}</pre>
          </Card>
        </div>
      )}
    </div>
  );
}
