import { useEffect, useState } from "react";
import { Plus, Trash2, Star, BookOpen, StickyNote } from "lucide-react";
import CodeMirror from "@uiw/react-codemirror";
import { javascript } from "@codemirror/lang-javascript";
import { markdown } from "@codemirror/lang-markdown";
import { sql } from "@codemirror/lang-sql";
import { marked } from "marked";
import DOMPurify from "dompurify";
import { api, Snippet, Note } from "@/lib/ipc";
import { Card, PageHeader, Spinner, ErrorNote, EmptyState, Modal, Badge } from "@/components/ui";
import { useApp, errMessage } from "@/lib/store";

const LANGS = ["plaintext", "javascript", "typescript", "sql", "bash", "go", "json", "markdown"];

export default function SnippetsPage() {
  const [tab, setTab] = useState<"snippets" | "notes">("snippets");
  const [snippets, setSnippets] = useState<Snippet[]>([]);
  const [notes, setNotes] = useState<Note[]>([]);
  const [selected, setSelected] = useState<Snippet | Note | null>(null);
  const [editOpen, setEditOpen] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const toast = useApp((s) => s.toast);

  const load = async () => {
    setLoading(true);
    try {
      const [sn, nt] = await Promise.all([api.snippetsList(), api.notesList()]);
      setSnippets(sn);
      setNotes(nt);
      setError("");
    } catch (e) {
      setError(errMessage(e));
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => { load(); }, []);

  const isNote = (x: Snippet | Note): x is Note => "body" in x;

  return (
    <div>
      <PageHeader
        title="Snippets & Notes"
        subtitle="Reusable code with syntax highlighting and markdown notes."
        actions={
          <>
            <div className="flex rounded-lg border border-line overflow-hidden">
              <button className={`px-3 py-2 text-sm ${tab === "snippets" ? "bg-surface2 text-ink" : "text-muted"}`} onClick={() => { setTab("snippets"); setSelected(null); }}>
                <StickyNote className="w-4 h-4 inline mr-1 -mt-0.5" /> Snippets
              </button>
              <button className={`px-3 py-2 text-sm ${tab === "notes" ? "bg-surface2 text-ink" : "text-muted"}`} onClick={() => { setTab("notes"); setSelected(null); }}>
                <BookOpen className="w-4 h-4 inline mr-1 -mt-0.5" /> Notes
              </button>
            </div>
            <button className="btn-primary" onClick={() => setEditOpen(true)}>
              <Plus className="w-4 h-4" /> New {tab === "snippets" ? "snippet" : "note"}
            </button>
          </>
        }
      />
      {error && <ErrorNote message={error} />}
      {loading && <Spinner />}
      {!loading && (
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
          <div className="space-y-2">
            {tab === "snippets" &&
              snippets.map((s) => (
                <button key={s.id} className={`w-full text-left card p-3 hover:border-accent ${selected?.id === s.id ? "border-accent" : ""}`} onClick={() => setSelected(s)}>
                  <div className="font-medium text-sm flex items-center gap-2">
                    {s.favorite && <Star className="w-3.5 h-3.5 text-warn fill-warn" />}
                    <span className="truncate">{s.title}</span>
                  </div>
                  <div className="flex gap-1.5 mt-1.5 flex-wrap">
                    <Badge tone="info">{s.language}</Badge>
                    {s.tags.map((t) => <span key={t} className="chip">{t}</span>)}
                  </div>
                </button>
              ))}
            {tab === "notes" &&
              notes.map((n) => (
                <button key={n.id} className={`w-full text-left card p-3 hover:border-accent ${selected?.id === n.id ? "border-accent" : ""}`} onClick={() => setSelected(n)}>
                  <div className="font-medium text-sm truncate">{n.pinned ? "📌 " : ""}{n.title}</div>
                  <div className="text-xs text-muted mt-1 line-clamp-2">{n.body.slice(0, 100)}</div>
                </button>
              ))}
            {(tab === "snippets" ? snippets.length === 0 : notes.length === 0) && (
              <EmptyState title={`No ${tab} yet`} hint={`Create your first ${tab === "snippets" ? "snippet" : "note"}.`} />
            )}
          </div>

          <div className="lg:col-span-2">
            {selected ? (
              <Card>
                <div className="flex items-start justify-between gap-3 mb-4">
                  <div>
                    <h2 className="font-semibold text-lg">{isNote(selected) ? selected.title : (selected as Snippet).title}</h2>
                    {!isNote(selected) && (
                      <div className="flex gap-1.5 mt-1.5 flex-wrap">
                        <Badge tone="info">{(selected as Snippet).language}</Badge>
                        {(selected as Snippet).tags.map((t) => <span key={t} className="chip">{t}</span>)}
                      </div>
                    )}
                    {isNote(selected) && (
                      <div className="flex gap-1.5 mt-1.5 flex-wrap">
                        {selected.tags.map((t) => <span key={t} className="chip">{t}</span>)}
                      </div>
                    )}
                  </div>
                  <div className="flex gap-2 shrink-0">
                    <button className="btn-ghost !py-1.5" onClick={() => setEditOpen(true)}>Edit</button>
                    <button className="btn-ghost !py-1.5 hover:!text-danger" onClick={async () => {
                      if (isNote(selected)) await api.noteDelete(selected.id);
                      else await api.snippetDelete(selected.id);
                      setSelected(null);
                      load();
                    }}><Trash2 className="w-4 h-4" /></button>
                  </div>
                </div>
                {isNote(selected) ? (
                  <div className="prose prose-invert max-w-none text-sm" dangerouslySetInnerHTML={{ __html: DOMPurify.sanitize(marked.parse(selected.body) as string) }} />
                ) : (
                  <CodeMirror
                    value={(selected as Snippet).code}
                    height="400px"
                    extensions={[
                      (selected as Snippet).language === "sql" ? sql() :
                      (selected as Snippet).language.startsWith("java") || (selected as Snippet).language === "typescript" ? javascript() :
                      (selected as Snippet).language === "markdown" ? markdown() : [],
                    ]}
                    theme="dark"
                    editable={false}
                  />
                )}
              </Card>
            ) : (
              <EmptyState icon={<Code2Placeholder />} title="Select an item" hint="Pick a snippet or note from the list." />
            )}
          </div>
        </div>
      )}

      <EditorModal
        open={editOpen}
        mode={tab}
        editing={selected}
        onClose={() => setEditOpen(false)}
        onSaved={() => { setEditOpen(false); load(); }}
      />
    </div>
  );
}

function Code2Placeholder() {
  return <StickyNote className="w-10 h-10" />;
}

function EditorModal({ open, mode, editing, onClose, onSaved }: {
  open: boolean;
  mode: "snippets" | "notes";
  editing: Snippet | Note | null;
  onClose: () => void;
  onSaved: () => void;
}) {
  const isNote = mode === "notes";
  const [title, setTitle] = useState("");
  const [language, setLanguage] = useState("plaintext");
  const [code, setCode] = useState("");
  const [body, setBody] = useState("");
  const [tags, setTags] = useState("");
  const [favorite, setFavorite] = useState(false);
  const [pinned, setPinned] = useState(false);
  const toast = useApp((s) => s.toast);

  useEffect(() => {
    if (!open) return;
    if (editing) {
      setTitle(editing.title);
      setTags(editing.tags.join(", "));
      if ("body" in editing) {
        setBody(editing.body);
        setPinned(editing.pinned);
      } else {
        setCode((editing as Snippet).code);
        setLanguage((editing as Snippet).language);
        setFavorite((editing as Snippet).favorite);
      }
    } else {
      setTitle(""); setCode(""); setBody(""); setTags(""); setFavorite(false); setPinned(false); setLanguage("plaintext");
    }
  }, [open, editing]);

  const save = async () => {
    try {
      const tagList = tags.split(",").map((t) => t.trim()).filter(Boolean);
      if (isNote) {
        await api.noteSave(editing?.id ?? "", title.trim(), body, tagList, pinned);
      } else {
        await api.snippetSave(editing?.id ?? "", title.trim(), language, code, tagList, favorite);
      }
      toast("Saved", "success");
      onSaved();
    } catch (e) {
      toast(errMessage(e), "error");
    }
  };

  return (
    <Modal open={open} onClose={onClose} title={`${editing ? "Edit" : "New"} ${isNote ? "note" : "snippet"}`} wide>
      <div className="space-y-3">
        <div className="grid grid-cols-3 gap-3">
          <div className="col-span-2">
            <label className="text-sm text-muted">Title *</label>
            <input className="input w-full mt-1" value={title} onChange={(e) => setTitle(e.target.value)} />
          </div>
          {!isNote && (
            <div>
              <label className="text-sm text-muted">Language</label>
              <select className="input w-full mt-1" value={language} onChange={(e) => setLanguage(e.target.value)}>
                {LANGS.map((l) => <option key={l}>{l}</option>)}
              </select>
            </div>
          )}
        </div>
        {isNote ? (
          <div>
            <label className="text-sm text-muted">Markdown body</label>
            <textarea className="input w-full mt-1 font-mono text-xs h-64" value={body} onChange={(e) => setBody(e.target.value)} />
          </div>
        ) : (
          <div>
            <label className="text-sm text-muted">Code</label>
            <div className="mt-1 border border-line rounded-lg overflow-hidden">
              <CodeMirror
                value={code}
                height="260px"
                extensions={[language === "sql" ? sql() : language.startsWith("java") || language === "typescript" ? javascript() : markdown()]}
                theme="dark"
                onChange={(v) => setCode(v)}
              />
            </div>
          </div>
        )}
        <div className="flex items-center gap-4">
          <div className="flex-1">
            <label className="text-sm text-muted">Tags (comma separated)</label>
            <input className="input w-full mt-1" value={tags} onChange={(e) => setTags(e.target.value)} />
          </div>
          {isNote ? (
            <label className="flex items-center gap-2 text-sm text-muted mt-5">
              <input type="checkbox" checked={pinned} onChange={(e) => setPinned(e.target.checked)} /> Pinned
            </label>
          ) : (
            <label className="flex items-center gap-2 text-sm text-muted mt-5">
              <input type="checkbox" checked={favorite} onChange={(e) => setFavorite(e.target.checked)} /> Favorite
            </label>
          )}
        </div>
        <div className="flex justify-end gap-2">
          <button className="btn-ghost" onClick={onClose}>Cancel</button>
          <button className="btn-primary" disabled={!title.trim()} onClick={save}>Save</button>
        </div>
      </div>
    </Modal>
  );
}
