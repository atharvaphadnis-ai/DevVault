import { useEffect, useState } from "react";
import { Plus, Trash2, ListTodo } from "lucide-react";
import { api, Task } from "@/lib/ipc";
import { Card, PageHeader, Spinner, ErrorNote, EmptyState, Modal, Badge, ConfirmDialog } from "@/components/ui";
import { useApp, errMessage } from "@/lib/store";

const COLUMNS = [
  { id: "todo", label: "To Do" },
  { id: "in_progress", label: "In Progress" },
  { id: "review", label: "Review" },
  { id: "done", label: "Done" },
];

const PRIORITIES: Record<string, "info" | "warn" | "danger" | "ok"> = {
  low: "info", medium: "info", high: "warn", urgent: "danger",
};

export default function TasksPage() {
  const [tasks, setTasks] = useState<Task[]>([]);
  const [view, setView] = useState<"board" | "list">("board");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [addOpen, setAddOpen] = useState(false);
  const [delTask, setDelTask] = useState<Task | null>(null);
  const toast = useApp((s) => s.toast);

  const load = async () => {
    try {
      setTasks(await api.tasksList());
      setError("");
    } catch (e) {
      setError(errMessage(e));
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => { load(); }, []);

  const move = async (task: Task, status: string) => {
    if (task.status === status) return;
    try {
      await api.taskMove(task.id, status, task.order);
      load();
    } catch (e) {
      toast(errMessage(e), "error");
    }
  };

  return (
    <div>
      <PageHeader
        title="Tasks"
        subtitle="Kanban and list views for your work items."
        actions={
          <>
            <div className="flex rounded-lg border border-line overflow-hidden">
              <button className={`px-3 py-2 text-sm ${view === "board" ? "bg-surface2 text-ink" : "text-muted"}`} onClick={() => setView("board")}>Board</button>
              <button className={`px-3 py-2 text-sm ${view === "list" ? "bg-surface2 text-ink" : "text-muted"}`} onClick={() => setView("list")}>List</button>
            </div>
            <button className="btn-primary" onClick={() => setAddOpen(true)}><Plus className="w-4 h-4" /> New task</button>
          </>
        }
      />
      {error && <ErrorNote message={error} />}
      {loading && <Spinner />}
      {!loading && tasks.length === 0 && !error && (
        <EmptyState icon={<ListTodo className="w-10 h-10" />} title="No tasks yet" hint="Create your first task to see the board come alive." />
      )}
      {!loading && tasks.length > 0 && view === "board" && (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-4">
          {COLUMNS.map((col) => {
            const items = tasks.filter((t) => t.status === col.id);
            return (
              <div
                key={col.id}
                className="card p-3 min-h-40"
                onDragOver={(e) => e.preventDefault()}
                onDrop={(e) => {
                  e.preventDefault();
                  const id = e.dataTransfer.getData("text/task");
                  const task = tasks.find((t) => t.id === id);
                  if (task) move(task, col.id);
                }}
              >
                <div className="flex items-center justify-between mb-3 px-1">
                  <span className="section-title">{col.label}</span>
                  <span className="chip">{items.length}</span>
                </div>
                <div className="space-y-2">
                  {items.map((t) => (
                    <div
                      key={t.id}
                      draggable
                      onDragStart={(e) => e.dataTransfer.setData("text/task", t.id)}
                      className="bg-surface2 border border-line rounded-lg p-3 text-sm cursor-grab active:cursor-grabbing hover:border-accent transition-colors group"
                    >
                      <div className="font-medium">{t.title}</div>
                      {t.description && <div className="text-xs text-muted mt-1 line-clamp-2">{t.description}</div>}
                      <div className="flex items-center gap-1.5 mt-2">
                        <Badge tone={PRIORITIES[t.priority] ?? "info"}>{t.priority}</Badge>
                        {t.dueDate && <span className="chip">due {t.dueDate}</span>}
                        <Trash2 className="w-3.5 h-3.5 ml-auto text-muted opacity-0 group-hover:opacity-100 hover:!text-danger" onClick={() => setDelTask(t)} />
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            );
          })}
        </div>
      )}
      {!loading && tasks.length > 0 && view === "list" && (
        <Card className="p-0 overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-surface2">
              <tr>
                <th className="text-left px-4 py-2.5 border-b border-line">Task</th>
                <th className="text-left px-4 py-2.5 border-b border-line">Status</th>
                <th className="text-left px-4 py-2.5 border-b border-line">Priority</th>
                <th className="text-left px-4 py-2.5 border-b border-line">Due</th>
                <th className="px-4 py-2.5 border-b border-line w-10"></th>
              </tr>
            </thead>
            <tbody>
              {tasks.map((t) => (
                <tr key={t.id} className="hover:bg-surface2/60">
                  <td className="px-4 py-2.5">
                    <div className="font-medium">{t.title}</div>
                    {t.description && <div className="text-xs text-muted">{t.description}</div>}
                  </td>
                  <td className="px-4 py-2.5">
                    <select className="input !py-1 text-xs" value={t.status} onChange={(e) => move(t, e.target.value)}>
                      {COLUMNS.map((c) => <option key={c.id} value={c.id}>{c.label}</option>)}
                    </select>
                  </td>
                  <td className="px-4 py-2.5"><Badge tone={PRIORITIES[t.priority] ?? "info"}>{t.priority}</Badge></td>
                  <td className="px-4 py-2.5 text-muted text-xs">{t.dueDate || "—"}</td>
                  <td className="px-4 py-2.5"><Trash2 className="w-4 h-4 text-muted hover:text-danger cursor-pointer" onClick={() => setDelTask(t)} /></td>
                </tr>
              ))}
            </tbody>
          </table>
        </Card>
      )}

      <AddTaskModal open={addOpen} onClose={() => setAddOpen(false)} onSaved={() => { setAddOpen(false); load(); }} />
      <ConfirmDialog
        open={!!delTask}
        title="Delete task"
        message={`Delete "${delTask?.title}"? This cannot be undone.`}
        confirmLabel="Delete"
        danger
        onCancel={() => setDelTask(null)}
        onConfirm={async () => {
          await api.taskDelete(delTask!.id);
          setDelTask(null);
          load();
        }}
      />
    </div>
  );
}

function AddTaskModal({ open, onClose, onSaved }: { open: boolean; onClose: () => void; onSaved: () => void }) {
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [status, setStatus] = useState("todo");
  const [priority, setPriority] = useState("medium");
  const [dueDate, setDueDate] = useState("");
  const [err, setErr] = useState("");
  return (
    <Modal open={open} onClose={onClose} title="New task">
      <div className="space-y-3">
        <div>
          <label className="text-sm text-muted">Title *</label>
          <input className="input w-full mt-1" value={title} onChange={(e) => setTitle(e.target.value)} />
        </div>
        <div>
          <label className="text-sm text-muted">Description</label>
          <textarea className="input w-full mt-1 h-20" value={description} onChange={(e) => setDescription(e.target.value)} />
        </div>
        <div className="grid grid-cols-3 gap-3">
          <div>
            <label className="text-sm text-muted">Status</label>
            <select className="input w-full mt-1" value={status} onChange={(e) => setStatus(e.target.value)}>
              {COLUMNS.map((c) => <option key={c.id} value={c.id}>{c.label}</option>)}
            </select>
          </div>
          <div>
            <label className="text-sm text-muted">Priority</label>
            <select className="input w-full mt-1" value={priority} onChange={(e) => setPriority(e.target.value)}>
              <option>low</option><option>medium</option><option>high</option><option>urgent</option>
            </select>
          </div>
          <div>
            <label className="text-sm text-muted">Due date</label>
            <input className="input w-full mt-1" type="date" value={dueDate} onChange={(e) => setDueDate(e.target.value)} />
          </div>
        </div>
        {err && <ErrorNote message={err} />}
        <div className="flex justify-end gap-2">
          <button className="btn-ghost" onClick={onClose}>Cancel</button>
          <button className="btn-primary" disabled={!title.trim()} onClick={async () => {
            try {
              await api.taskCreate(title.trim(), description, status, priority, dueDate, "");
              onSaved();
            } catch (e) { setErr(errMessage(e)); }
          }}>Create</button>
        </div>
      </div>
    </Modal>
  );
}
