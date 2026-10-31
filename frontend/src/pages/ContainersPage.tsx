import { useEffect, useState } from "react";
import { Play, Square, ScrollText, RefreshCw, Container as ContainerIcon } from "lucide-react";
import { api, Container } from "@/lib/ipc";
import { Card, PageHeader, Spinner, ErrorNote, EmptyState, Modal, Badge } from "@/components/ui";
import { useApp, errMessage } from "@/lib/store";

export default function ContainersPage() {
  const [available, setAvailable] = useState<boolean | null>(null);
  const [version, setVersion] = useState("");
  const [items, setItems] = useState<Container[]>([]);
  const [tab, setTab] = useState<"containers" | "images">("containers");
  const [images, setImages] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [logsFor, setLogsFor] = useState<Container | null>(null);
  const [logs, setLogs] = useState("");
  const toast = useApp((s) => s.toast);

  const load = async () => {
    setLoading(true);
    try {
      const [ok, v] = await api.containersDetect();
      setAvailable(ok);
      setVersion(v);
      if (ok) {
        setItems(await api.containersList());
        if (tab === "images") setImages(await api.containersImages());
      }
      setError("");
    } catch (e) {
      setError(errMessage(e));
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => { load(); }, []); // eslint-disable-line react-hooks/exhaustive-deps

  if (available === false) {
    return (
      <div>
        <PageHeader title="Containers" />
        <EmptyState
          icon={<ContainerIcon className="w-10 h-10" />}
          title="Docker not detected"
          hint="This module appears automatically when Docker is installed and running. Install Docker Desktop (or the docker engine), then restart DevVault."
        />
      </div>
    );
  }

  return (
    <div>
      <PageHeader
        title="Containers"
        subtitle={available ? `Docker ${version}` : "Checking Docker…"}
        actions={
          <>
            <div className="flex rounded-lg border border-line overflow-hidden">
              <button className={`px-3 py-2 text-sm ${tab === "containers" ? "bg-surface2 text-ink" : "text-muted"}`} onClick={() => { setTab("containers"); load(); }}>Containers</button>
              <button className={`px-3 py-2 text-sm ${tab === "images" ? "bg-surface2 text-ink" : "text-muted"}`} onClick={async () => { setTab("images"); try { setImages(await api.containersImages()); } catch (e) { setError(errMessage(e)); } }}>Images</button>
            </div>
            <button className="btn-ghost" onClick={load}><RefreshCw className="w-4 h-4" /></button>
          </>
        }
      />
      {error && <ErrorNote message={error} />}
      {loading && <Spinner label="Talking to Docker…" />}
      {!loading && available && tab === "containers" && (
        <Card className="p-0 overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-surface2">
              <tr>
                <th className="text-left px-4 py-2.5 border-b border-line">Name</th>
                <th className="text-left px-4 py-2.5 border-b border-line">Image</th>
                <th className="text-left px-4 py-2.5 border-b border-line">State</th>
                <th className="text-left px-4 py-2.5 border-b border-line">Status</th>
                <th className="text-left px-4 py-2.5 border-b border-line">Ports</th>
                <th className="px-4 py-2.5 border-b border-line w-32"></th>
              </tr>
            </thead>
            <tbody>
              {items.map((c) => (
                <tr key={c.id} className="hover:bg-surface2/60">
                  <td className="px-4 py-2.5 font-medium">{c.name}</td>
                  <td className="px-4 py-2.5 font-mono text-xs">{c.image}</td>
                  <td className="px-4 py-2.5">
                    <Badge tone={c.state === "running" ? "ok" : "info"}>{c.state}</Badge>
                  </td>
                  <td className="px-4 py-2.5 text-xs text-muted">{c.status}</td>
                  <td className="px-4 py-2.5 font-mono text-xs">{c.ports}</td>
                  <td className="px-4 py-2.5">
                    <div className="flex gap-1.5 justify-end">
                      {c.state === "running" ? (
                        <button className="btn-ghost !py-1 !px-2 text-xs" onClick={async () => {
                          try { await api.containerStop(c.id); load(); } catch (e) { toast(errMessage(e), "error"); }
                        }}><Square className="w-3 h-3" /> Stop</button>
                      ) : (
                        <button className="btn-ghost !py-1 !px-2 text-xs" onClick={async () => {
                          try { await api.containerStart(c.id); load(); } catch (e) { toast(errMessage(e), "error"); }
                        }}><Play className="w-3 h-3" /> Start</button>
                      )}
                      <button className="btn-ghost !py-1 !px-2 text-xs" onClick={async () => {
                        try {
                          setLogs(await api.containerLogs(c.id, 200));
                          setLogsFor(c);
                        } catch (e) { toast(errMessage(e), "error"); }
                      }}><ScrollText className="w-3 h-3" /></button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          {items.length === 0 && <EmptyState title="No containers" hint="Pull or run an image with the docker CLI, then refresh." />}
        </Card>
      )}
      {!loading && available && tab === "images" && (
        <Card className="p-0 overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-surface2">
              <tr>
                <th className="text-left px-4 py-2.5 border-b border-line">Repository</th>
                <th className="text-left px-4 py-2.5 border-b border-line">Tag</th>
                <th className="text-left px-4 py-2.5 border-b border-line">Size</th>
                <th className="text-left px-4 py-2.5 border-b border-line">Created</th>
              </tr>
            </thead>
            <tbody>
              {images.map((img, i) => (
                <tr key={i} className="hover:bg-surface2/60">
                  <td className="px-4 py-2.5 font-mono text-xs">{img.repo}</td>
                  <td className="px-4 py-2.5 font-mono text-xs">{img.tag}</td>
                  <td className="px-4 py-2.5 text-xs">{img.size}</td>
                  <td className="px-4 py-2.5 text-xs text-muted">{img.createdAt}</td>
                </tr>
              ))}
            </tbody>
          </table>
          {images.length === 0 && <EmptyState title="No images" />}
        </Card>
      )}

      <Modal open={!!logsFor} onClose={() => setLogsFor(null)} title={`Logs — ${logsFor?.name}`} wide>
        <pre className="text-xs font-mono bg-surface2 rounded-lg p-4 overflow-auto max-h-[60vh] whitespace-pre-wrap">{logs || "No logs."}</pre>
      </Modal>
    </div>
  );
}
