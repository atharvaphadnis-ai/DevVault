import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { FolderGit2, Code2, ListTodo, Satellite, Cpu, MemoryStick, RefreshCw } from "lucide-react";
import { api, Stats } from "@/lib/ipc";
import { Card, PageHeader, Spinner, ErrorNote, Badge, StatCard } from "@/components/ui";
import { useApp, errMessage } from "@/lib/store";

function fmtBytes(n: number): string {
  if (!n) return "—";
  const gb = n / 1024 ** 3;
  return gb >= 1 ? `${gb.toFixed(1)} GB` : `${(n / 1024 ** 2).toFixed(0)} MB`;
}

export default function Dashboard() {
  const [stats, setStats] = useState<Stats | null>(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const toast = useApp((s) => s.toast);

  const load = async () => {
    setLoading(true);
    try {
      setStats(await api.dashboardStats());
      setError("");
    } catch (e) {
      setError(errMessage(e));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
    const t = setInterval(load, 15000);
    return () => clearInterval(t);
  }, []);

  return (
    <div>
      <PageHeader
        title="Dashboard"
        subtitle="Your workbench at a glance"
        actions={
          <button className="btn-ghost" onClick={load}>
            <RefreshCw className="w-4 h-4" /> Refresh
          </button>
        }
      />
      {error && <ErrorNote message={error} />}
      {loading && !stats && <Spinner />}
      {stats && (
        <div className="space-y-6">
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            <StatCard label="Projects" value={stats.projects} hint="registered workspaces" />
            <StatCard label="Snippets" value={stats.snippets} hint="reusable code blocks" />
            <StatCard
              label="Open tasks"
              value={
                (stats.tasksByStatus.todo ?? 0) +
                (stats.tasksByStatus.in_progress ?? 0) +
                (stats.tasksByStatus.review ?? 0)
              }
              hint={`${stats.tasksByStatus.done ?? 0} done`}
            />
            <div className="card p-4">
              <div className="text-xs text-muted uppercase tracking-wide flex items-center gap-1.5">
                <Satellite className="w-3.5 h-3.5" /> Remote access
              </div>
              <div className="mt-2">
                {stats.remote.connected ? (
                  <Badge tone="ok">Connected to relay</Badge>
                ) : stats.remote.enabled ? (
                  <Badge tone="warn">Reconnecting…</Badge>
                ) : (
                  <Badge tone="info">Disabled</Badge>
                )}
              </div>
              {stats.remote.detail && <div className="text-xs text-muted mt-1">{stats.remote.detail}</div>}
            </div>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <Card>
              <div className="flex items-center gap-2 mb-4">
                <Cpu className="w-4 h-4 text-muted" />
                <span className="section-title">System</span>
              </div>
              <div className="space-y-4">
                <div>
                  <div className="flex justify-between text-sm mb-1">
                    <span className="text-muted">CPU</span>
                    <span>{stats.cpuPercent.toFixed(1)}%</span>
                  </div>
                  <div className="h-2 rounded-full bg-surface2 overflow-hidden">
                    <div className="h-full rounded-full" style={{ width: `${stats.cpuPercent}%`, background: "linear-gradient(90deg, var(--accent), var(--accent2))" }} />
                  </div>
                </div>
                <div>
                  <div className="flex justify-between text-sm mb-1">
                    <span className="text-muted flex items-center gap-1">
                      <MemoryStick className="w-3.5 h-3.5" /> Memory
                    </span>
                    <span>
                      {stats.memPercent.toFixed(1)}% · {fmtBytes(stats.memUsed)} / {fmtBytes(stats.memTotal)}
                    </span>
                  </div>
                  <div className="h-2 rounded-full bg-surface2 overflow-hidden">
                    <div className="h-full rounded-full" style={{ width: `${stats.memPercent}%`, background: "linear-gradient(90deg, var(--accent), var(--accent2))" }} />
                  </div>
                </div>
                <div className="text-xs text-muted">
                  {stats.numCPU} cores · backend Go {stats.goVersion.replace("go", "")}
                </div>
              </div>
            </Card>

            <Card>
              <div className="section-title mb-4">Quick actions</div>
              <div className="grid grid-cols-2 gap-3">
                <Link to="/projects" className="btn-ghost justify-start">
                  <FolderGit2 className="w-4 h-4" /> Open projects
                </Link>
                <Link to="/terminal" className="btn-ghost justify-start">
                  Open terminal
                </Link>
                <Link to="/tasks" className="btn-ghost justify-start">
                  <ListTodo className="w-4 h-4" /> Plan tasks
                </Link>
                <Link to="/remote" className="btn-ghost justify-start">
                  <Satellite className="w-4 h-4" /> Pair a device
                </Link>
              </div>
              <div className="mt-4 text-xs text-muted">
                Tip: press <kbd className="chip">Ctrl K</kbd> anywhere to search everything.
              </div>
            </Card>
          </div>

          <Card>
            <div className="section-title mb-3">Task board summary</div>
            <div className="flex flex-wrap gap-2">
              {["todo", "in_progress", "review", "done"].map((s) => (
                <span key={s} className="chip">
                  {s.replace("_", " ")}: <b className="text-ink">{stats.tasksByStatus[s] ?? 0}</b>
                </span>
              ))}
            </div>
            <div className="mt-3">
              <Link to="/tasks" className="text-sm text-accent2 hover:underline">
                Open task board →
              </Link>
            </div>
          </Card>
        </div>
      )}
      <div className="h-2" />
      {stats && !stats.remote.connected && !stats.remote.enabled && (
        <button className="btn-ghost mt-4" onClick={() => toast("Enable DevVault Anywhere from the Remote page", "info")}>
          Learn how to enable remote access
        </button>
      )}
    </div>
  );
}
