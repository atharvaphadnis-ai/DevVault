import { useEffect, useState } from "react";
import { Puzzle, RefreshCw, FolderOpen } from "lucide-react";
import { api } from "@/lib/ipc";
import { Card, PageHeader, Spinner, ErrorNote, EmptyState, Badge } from "@/components/ui";
import { useApp, errMessage } from "@/lib/store";

interface Plugin {
  id: string;
  name: string;
  version: string;
  description: string;
  permissions: string[];
  enabled: boolean;
  path: string;
  running: boolean;
}

export default function PluginsPage() {
  const [plugins, setPlugins] = useState<Plugin[]>([]);
  const [userDir, setUserDir] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const toast = useApp((s) => s.toast);

  const load = async () => {
    try {
      setPlugins(await api.pluginsList());
      setUserDir(await api.pluginsUserDir());
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
      <PageHeader
        title="Plugins"
        subtitle="Out-of-process plugins speaking JSON-RPC over stdio, with manifest-declared permissions."
        actions={<button className="btn-ghost" onClick={load}><RefreshCw className="w-4 h-4" /> Rescan</button>}
      />
      {error && <ErrorNote message={error} />}
      {loading && <Spinner />}
      {!loading && (
        <>
          <Card className="mb-4">
            <div className="flex items-center gap-2 text-sm">
              <FolderOpen className="w-4 h-4 text-muted" />
              <span className="text-muted">Plugins directory:</span>
              <code className="chip font-mono">{userDir || "…"}</code>
            </div>
            <p className="text-xs text-muted mt-2 leading-relaxed">
              Copy a plugin folder (containing <code>plugin.json</code> and its entry file) into this directory and hit Rescan.
              The bundled example lives at <code>plugins/hello-plugin</code> in the repository.
            </p>
          </Card>
          {plugins.length === 0 ? (
            <EmptyState
              icon={<Puzzle className="w-10 h-10" />}
              title="No plugins discovered"
              hint="Install the bundled hello-plugin to see the host in action: copy plugins/hello-plugin into the directory above, then Rescan and enable it."
            />
          ) : (
            <div className="space-y-2">
              {plugins.map((p) => (
                <Card key={p.id} className="flex items-center justify-between gap-4">
                  <div className="min-w-0">
                    <div className="font-medium flex items-center gap-2">
                      {p.name} <Badge tone="info">v{p.version}</Badge>
                      {p.running && <Badge tone="ok">running</Badge>}
                    </div>
                    <div className="text-sm text-muted mt-0.5">{p.description}</div>
                    <div className="flex flex-wrap gap-1.5 mt-1.5">
                      {p.permissions.map((perm) => (
                        <span key={perm} className="chip font-mono text-[10px]">{perm}</span>
                      ))}
                    </div>
                    <div className="text-xs text-muted font-mono mt-1 truncate">{p.path}</div>
                  </div>
                  <div className="flex items-center gap-2 shrink-0">
                    {p.enabled && (
                      <button className="btn-ghost !py-1.5 text-xs" onClick={async () => {
                        try {
                          const msg = await api.pluginPing(p.id);
                          toast(msg, "success");
                        } catch (e) {
                          toast(errMessage(e), "error");
                        }
                      }}>Ping</button>
                    )}
                    <button
                      className={p.enabled ? "btn-ghost" : "btn-primary"}
                      onClick={async () => {
                        try {
                          await api.pluginSetEnabled(p.id, !p.enabled);
                          toast(p.enabled ? "Disabled" : "Enabled", "success");
                          load();
                        } catch (e) {
                          toast(errMessage(e), "error");
                        }
                      }}
                    >
                      {p.enabled ? "Disable" : "Enable"}
                    </button>
                  </div>
                </Card>
              ))}
            </div>
          )}
        </>
      )}
    </div>
  );
}
