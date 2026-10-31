import { useEffect, useState } from "react";
import { Plus, Star, Trash2, FolderOpen, Tag } from "lucide-react";
import { api, Project } from "@/lib/ipc";
import { Card, PageHeader, Spinner, ErrorNote, Modal, ConfirmDialog, EmptyState } from "@/components/ui";
import { useApp, errMessage } from "@/lib/store";

export default function Projects() {
  const [projects, setProjects] = useState<Project[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [addOpen, setAddOpen] = useState(false);
  const [remove, setRemove] = useState<Project | null>(null);
  const toast = useApp((s) => s.toast);

  const load = async () => {
    try {
      setProjects(await api.projectsList());
      setError("");
    } catch (e) {
      setError(errMessage(e));
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => {
    load();
  }, []);

  const groups = Array.from(new Set(projects.map((p) => p.group).filter(Boolean)));

  return (
    <div>
      <PageHeader
        title="Projects"
        subtitle="Registered working directories. DevVault only stores metadata — it never copies or silently modifies your files."
        actions={
          <button className="btn-primary" onClick={() => setAddOpen(true)}>
            <Plus className="w-4 h-4" /> Add project
          </button>
        }
      />
      {error && <ErrorNote message={error} />}
      {loading && <Spinner />}
      {!loading && projects.length === 0 && !error && (
        <EmptyState
          icon={<FolderOpen className="w-10 h-10" />}
          title="No projects yet"
          hint="Register a folder to see it here. Use the full absolute path, e.g. /home/you/dev/my-app."
          action={
            <button className="btn-primary" onClick={() => setAddOpen(true)}>
              Add your first project
            </button>
          }
        />
      )}
      {groups.length > 0 && (
        <div className="flex flex-wrap gap-2 mb-4">
          {groups.map((g) => (
            <span key={g} className="chip">
              <Tag className="w-3 h-3" /> {g}
            </span>
          ))}
        </div>
      )}
      <div className="grid grid-cols-1 lg:grid-cols-2 xl:grid-cols-3 gap-4">
        {projects.map((p) => (
          <Card key={p.id} className="flex flex-col">
            <div className="flex items-start justify-between gap-2">
              <div className="min-w-0">
                <div className="font-semibold truncate flex items-center gap-2">
                  {p.name}
                  {p.favorite && <Star className="w-3.5 h-3.5 text-warn fill-warn" />}
                </div>
                <div className="text-xs text-muted truncate mt-0.5 font-mono">{p.path}</div>
              </div>
              <div className="flex gap-1 shrink-0">
                <button
                  title="Toggle favorite"
                  className="p-1.5 rounded hover:bg-surface2 text-muted"
                  onClick={async () => {
                    try {
                      await api.projectsUpdate(p.id, p.name, p.group, p.tags, !p.favorite, p.notes);
                      load();
                    } catch (e) {
                      toast(errMessage(e), "error");
                    }
                  }}
                >
                  <Star className={`w-4 h-4 ${p.favorite ? "text-warn fill-warn" : ""}`} />
                </button>
                <button
                  title="Remove"
                  className="p-1.5 rounded hover:bg-surface2 text-muted hover:text-danger"
                  onClick={() => setRemove(p)}
                >
                  <Trash2 className="w-4 h-4" />
                </button>
              </div>
            </div>
            {p.group && <div className="mt-2"><span className="chip">{p.group}</span></div>}
            {p.tags.length > 0 && (
              <div className="flex flex-wrap gap-1.5 mt-2">
                {p.tags.map((t) => (
                  <span key={t} className="chip">{t}</span>
                ))}
              </div>
            )}
            {p.notes && <p className="text-sm text-muted mt-3 line-clamp-2">{p.notes}</p>}
            <div className="mt-auto pt-3 text-xs text-muted">Last opened {p.lastOpenedAt}</div>
          </Card>
        ))}
      </div>

      <AddProjectModal
        open={addOpen}
        onClose={() => setAddOpen(false)}
        onAdded={() => {
          setAddOpen(false);
          load();
        }}
      />
      <ConfirmDialog
        open={!!remove}
        title="Remove project"
        message={`Unregister "${remove?.name}"? Files on disk are NOT touched — this only removes the DevVault entry.`}
        confirmLabel="Remove"
        danger
        onCancel={() => setRemove(null)}
        onConfirm={async () => {
          try {
            await api.projectsRemove(remove!.id);
            toast("Project removed", "success");
          } catch (e) {
            toast(errMessage(e), "error");
          }
          setRemove(null);
          load();
        }}
      />
    </div>
  );
}

function AddProjectModal({ open, onClose, onAdded }: { open: boolean; onClose: () => void; onAdded: () => void }) {
  const [path, setPath] = useState("");
  const [name, setName] = useState("");
  const [group, setGroup] = useState("");
  const [tags, setTags] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  const toast = useApp((s) => s.toast);

  const submit = async () => {
    setBusy(true);
    setErr("");
    try {
      const p = await api.projectsAdd(
        path.trim(),
        name.trim(),
        group.trim(),
        tags.split(",").map((t) => t.trim()).filter(Boolean)
      );
      toast(`Added ${p.name}`, "success");
      onAdded();
    } catch (e) {
      setErr(errMessage(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Modal open={open} onClose={onClose} title="Add project">
      <div className="space-y-3">
        <div>
          <label className="text-sm text-muted">Absolute path *</label>
          <input className="input w-full mt-1 font-mono" placeholder="/home/you/dev/my-app" value={path} onChange={(e) => setPath(e.target.value)} />
        </div>
        <div className="grid grid-cols-2 gap-3">
          <div>
            <label className="text-sm text-muted">Display name</label>
            <input className="input w-full mt-1" placeholder="auto from folder" value={name} onChange={(e) => setName(e.target.value)} />
          </div>
          <div>
            <label className="text-sm text-muted">Group</label>
            <input className="input w-full mt-1" placeholder="work / personal" value={group} onChange={(e) => setGroup(e.target.value)} />
          </div>
        </div>
        <div>
          <label className="text-sm text-muted">Tags (comma separated)</label>
          <input className="input w-full mt-1" placeholder="go, api" value={tags} onChange={(e) => setTags(e.target.value)} />
        </div>
        {err && <ErrorNote message={err} />}
        <div className="flex justify-end gap-2 pt-1">
          <button className="btn-ghost" onClick={onClose}>Cancel</button>
          <button className="btn-primary" disabled={!path.trim() || busy} onClick={submit}>
            {busy ? "Adding…" : "Add project"}
          </button>
        </div>
      </div>
    </Modal>
  );
}
