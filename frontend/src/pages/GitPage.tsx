import { useEffect, useState } from "react";
import { RefreshCw, Upload, Download, GitCommitHorizontal, GitBranch, ChevronDown, Plus, Trash2, CloudUpload, CloudDownload } from "lucide-react";
import { api, GitStatusResult, CommitEntry, Branch, Project } from "@/lib/ipc";
import { Card, PageHeader, Spinner, ErrorNote, Modal, EmptyState } from "@/components/ui";
import { useApp, errMessage } from "@/lib/store";

export default function GitPage() {
  const [projects, setProjects] = useState<Project[]>([]);
  const [selected, setSelected] = useState<string>("");
  const [gitOK, setGitOK] = useState(true);
  const [status, setStatus] = useState<GitStatusResult | null>(null);
  const [log, setLog] = useState<CommitEntry[]>([]);
  const [branches, setBranches] = useState<Branch[]>([]);
  const [remotes, setRemotes] = useState<Record<string, string>>({});
  const [selectedFile, setSelectedFile] = useState<string>("");
  const [diff, setDiff] = useState("");
  const [diffCached, setDiffCached] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [commitMsg, setCommitMsg] = useState("");
  const [cloneOpen, setCloneOpen] = useState(false);
  const [branchOpen, setBranchOpen] = useState(false);
  const [newBranch, setNewBranch] = useState("");
  const toast = useApp((s) => s.toast);

  useEffect(() => {
    (async () => {
      try {
        setGitOK(await api.gitAvailable());
        const list = await api.projectsList();
        setProjects(list);
        if (list.length > 0) setSelected(list[0].path);
      } catch (e) {
        setError(errMessage(e));
      }
    })();
  }, []);

  const refresh = async () => {
    if (!selected) return;
    setLoading(true);
    setError("");
    try {
      const [st, lg, br, rm] = await Promise.all([
        api.gitStatus(selected),
        api.gitLog(selected, 30),
        api.gitBranches(selected),
        api.gitRemotes(selected),
      ]);
      setStatus(st);
      setLog(lg);
      setBranches(br);
      setRemotes(rm);
    } catch (e) {
      setError(errMessage(e));
      setStatus(null);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    refresh();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selected]);

  const showDiff = async (file: string, cached: boolean) => {
    setSelectedFile(file);
    setDiffCached(cached);
    try {
      setDiff(await api.gitDiff(selected, file, cached));
    } catch (e) {
      setDiff(errMessage(e));
    }
  };

  if (!gitOK) {
    return (
      <div>
        <PageHeader title="Git" />
        <EmptyState
          icon={<GitBranch className="w-10 h-10" />}
          title="Git executable not found"
          hint="DevVault uses your system git. Install git (https://git-scm.com) and restart DevVault."
        />
      </div>
    );
  }

  const current = branches.find((b) => b.current);

  return (
    <div>
      <PageHeader
        title="Git"
        subtitle="Full git workflow on your registered repositories (system git executable)"
        actions={
          <>
            <select className="input max-w-56 font-mono text-xs" value={selected} onChange={(e) => setSelected(e.target.value)}>
              {projects.map((p) => (
                <option key={p.id} value={p.path}>{p.name}</option>
              ))}
            </select>
            <button className="btn-ghost" onClick={refresh}>
              <RefreshCw className="w-4 h-4" />
            </button>
            <button className="btn-primary" onClick={() => setCloneOpen(true)}>
              <Download className="w-4 h-4" /> Clone
            </button>
          </>
        }
      />
      {!selected && (
        <EmptyState icon={<GitBranch className="w-10 h-10" />} title="No project selected" hint="Add a project first, then come back." />
      )}
      {selected && (
        <>
          {error && <ErrorNote message={error} />}
          {loading && <Spinner label="Running git…" />}
          {status && !loading && (
            <div className="grid grid-cols-1 xl:grid-cols-3 gap-4">
              {/* Working tree */}
              <Card className="xl:col-span-2">
                <div className="flex items-center justify-between mb-4 gap-2 flex-wrap">
                  <div className="flex items-center gap-2">
                    <GitBranch className="w-4 h-4 text-muted" />
                    <b>{status.branch || "HEAD"}</b>
                    {status.upstream && (
                      <span className="chip">
                        {status.upstream}
                        {status.ahead > 0 && ` ↑${status.ahead}`}
                        {status.behind > 0 && ` ↓${status.behind}`}
                      </span>
                    )}
                    {status.clean && <span className="chip text-ok">clean</span>}
                  </div>
                  <div className="flex gap-2">
                    <button className="btn-ghost" onClick={() => setBranchOpen(true)}>
                      <ChevronDown className="w-4 h-4" /> Branches
                    </button>
                    <button
                      className="btn-ghost"
                      onClick={async () => {
                        try {
                          await api.gitFetch(selected);
                          toast("Fetched all remotes", "success");
                          refresh();
                        } catch (e) {
                          toast(errMessage(e), "error");
                        }
                      }}
                    >
                      Fetch
                    </button>
                    <button
                      className="btn-ghost"
                      onClick={async () => {
                        try {
                          await api.gitPull(selected, "origin", "");
                          toast("Pulled", "success");
                          refresh();
                        } catch (e) {
                          toast(errMessage(e), "error");
                        }
                      }}
                    >
                      <CloudDownload className="w-4 h-4" /> Pull
                    </button>
                    <button
                      className="btn-ghost"
                      onClick={async () => {
                        try {
                          const branch = status.branch || "HEAD";
                          await api.gitPush(selected, "origin", branch, !status.upstream);
                          toast("Pushed", "success");
                          refresh();
                        } catch (e) {
                          toast(errMessage(e), "error");
                        }
                      }}
                    >
                      <CloudUpload className="w-4 h-4" /> Push
                    </button>
                  </div>
                </div>
                {status.entries.length === 0 ? (
                  <EmptyState title="Working tree clean" hint="No changes to commit." />
                ) : (
                  <div className="space-y-1.5 max-h-96 overflow-y-auto">
                    {status.entries.map((e, i) => (
                      <div key={i} className="flex items-center gap-2 px-2 py-1.5 rounded-lg hover:bg-surface2 text-sm font-mono">
                        <span className="text-xs w-8 text-muted shrink-0">{e.x}{e.y}</span>
                        <button
                          className="truncate hover:underline text-left flex-1"
                          onClick={() => showDiff(e.path, e.staged)}
                        >
                          {e.path}
                        </button>
                        {e.staged ? (
                          <button
                            className="btn-ghost !py-1 !px-2 text-xs"
                            onClick={async () => {
                              await api.gitUnstage(selected, [e.path]);
                              refresh();
                            }}
                          >
                            Unstage
                          </button>
                        ) : (
                          <button
                            className="btn-ghost !py-1 !px-2 text-xs"
                            onClick={async () => {
                              await api.gitStage(selected, [e.path]);
                              refresh();
                            }}
                          >
                            <Upload className="w-3 h-3" /> Stage
                          </button>
                        )}
                      </div>
                    ))}
                  </div>
                )}
                <div className="mt-4 flex gap-2">
                  <input
                    className="input flex-1"
                    placeholder="Commit message (amend with — leaves empty for --amend --no-edit)"
                    value={commitMsg}
                    onChange={(e) => setCommitMsg(e.target.value)}
                  />
                  <button
                    className="btn-ghost"
                    disabled={status.entries.length === 0 && !commitMsg}
                    onClick={async () => {
                      try {
                        const out = await api.gitCommit(selected, commitMsg, commitMsg.trim() === "");
                        toast(out.trim() || "Committed", "success");
                        setCommitMsg("");
                        refresh();
                      } catch (e) {
                        toast(errMessage(e), "error");
                      }
                    }}
                  >
                    <GitCommitHorizontal className="w-4 h-4" /> Commit / Amend
                  </button>
                  <button
                    className="btn-primary"
                    disabled={!commitMsg.trim() || status.entries.length === 0}
                    onClick={async () => {
                      try {
                        await api.gitStage(selected, []);
                        await api.gitCommit(selected, commitMsg, false);
                        toast("Committed", "success");
                        setCommitMsg("");
                        refresh();
                      } catch (e) {
                        toast(errMessage(e), "error");
                      }
                    }}
                  >
                    Stage all + Commit
                  </button>
                </div>
              </Card>

              {/* Log */}
              <Card>
                <div className="section-title mb-3">Recent commits</div>
                <div className="space-y-2 max-h-[26rem] overflow-y-auto">
                  {log.map((c) => (
                    <div key={c.hash} className="text-sm border-b border-line/60 pb-2">
                      <div className="font-mono text-xs text-accent2">{c.short}</div>
                      <div className="truncate">{c.subject}</div>
                      <div className="text-xs text-muted truncate">{c.author} · {c.date}</div>
                    </div>
                  ))}
                  {log.length === 0 && <div className="text-sm text-muted">No commits yet.</div>}
                </div>
              </Card>
            </div>
          )}
        </>
      )}

      {/* Diff modal */}
      <Modal open={!!selectedFile} onClose={() => setSelectedFile("")} title={`Diff: ${selectedFile}${diffCached ? " (staged)" : ""}`} wide>
        <pre className="text-xs font-mono bg-surface2 rounded-lg p-4 overflow-auto max-h-[60vh] whitespace-pre-wrap">{diff || "No diff."}</pre>
      </Modal>

      {/* Branch modal */}
      <Modal open={branchOpen} onClose={() => setBranchOpen(false)} title={`Branches — ${current?.name ?? ""}`}>
        <div className="space-y-2 max-h-72 overflow-y-auto mb-4">
          {branches.map((b) => (
            <div key={b.name} className="flex items-center justify-between px-3 py-2 rounded-lg hover:bg-surface2 text-sm font-mono">
              <span>
                {b.current ? <b className="text-accent2">{b.name} ●</b> : b.name}
                {b.isRemote && <span className="text-muted"> (remote)</span>}
              </span>
              <div className="flex gap-1">
                {!b.isRemote && !b.current && (
                  <>
                    <button className="btn-ghost !py-1 !px-2 text-xs" onClick={async () => {
                      try { await api.gitCheckout(selected, b.name, false); setBranchOpen(false); refresh(); }
                      catch (e) { toast(errMessage(e), "error"); }
                    }}>Switch</button>
                    <button className="btn-ghost !py-1 !px-2 text-xs hover:!text-danger" onClick={async () => {
                      try { await api.gitDeleteBranch(selected, b.name, false); refresh(); }
                      catch (e) { toast(errMessage(e), "error"); }
                    }}><Trash2 className="w-3 h-3" /></button>
                  </>
                )}
              </div>
            </div>
          ))}
        </div>
        <div className="flex gap-2">
          <input className="input flex-1 font-mono" placeholder="new-branch-name" value={newBranch} onChange={(e) => setNewBranch(e.target.value)} />
          <button className="btn-primary" disabled={!newBranch.trim()} onClick={async () => {
            try {
              await api.gitCheckout(selected, newBranch.trim(), true);
              toast(`Created ${newBranch.trim()}`, "success");
              setNewBranch("");
              setBranchOpen(false);
              refresh();
            } catch (e) { toast(errMessage(e), "error"); }
          }}><Plus className="w-4 h-4" /> Create</button>
        </div>
        {Object.keys(remotes).length > 0 && (
          <div className="mt-4 text-xs text-muted">
            Remotes: {Object.entries(remotes).map(([k, v]) => `${k} → ${v}`).join("  ·  ")}
          </div>
        )}
      </Modal>

      <CloneModal open={cloneOpen} onClose={() => setCloneOpen(false)} onDone={(path) => {
        setCloneOpen(false);
        toast(`Cloned to ${path}`, "success");
        (async () => {
          const list = await api.projectsList();
          setProjects(list);
          setSelected(path);
        })();
      }} />
    </div>
  );
}

function CloneModal({ open, onClose, onDone }: { open: boolean; onClose: () => void; onDone: (path: string) => void }) {
  const [url, setUrl] = useState("");
  const [dest, setDest] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  return (
    <Modal open={open} onClose={onClose} title="Clone repository">
      <div className="space-y-3">
        <div>
          <label className="text-sm text-muted">Repository URL *</label>
          <input className="input w-full mt-1 font-mono" placeholder="https://github.com/user/repo.git" value={url} onChange={(e) => setUrl(e.target.value)} />
        </div>
        <div>
          <label className="text-sm text-muted">Destination (optional)</label>
          <input className="input w-full mt-1 font-mono" placeholder="auto from repo name" value={dest} onChange={(e) => setDest(e.target.value)} />
        </div>
        {err && <ErrorNote message={err} />}
        <div className="flex justify-end gap-2">
          <button className="btn-ghost" onClick={onClose}>Cancel</button>
          <button className="btn-primary" disabled={!url.trim() || busy} onClick={async () => {
            setBusy(true); setErr("");
            try { onDone(await api.gitClone(url.trim(), dest.trim())); }
            catch (e) { setErr(errMessage(e)); }
            finally { setBusy(false); }
          }}>{busy ? "Cloning…" : "Clone"}</button>
        </div>
      </div>
    </Modal>
  );
}
