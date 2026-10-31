package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"

	"devvault/internal/apisvc"
	"devvault/internal/containers"
	"devvault/internal/dashsvc"
	"devvault/internal/dbsvc"
	"devvault/internal/envsvc"
	"devvault/internal/gitsvc"
	"devvault/internal/localapi"
	"devvault/internal/plugins"
	"devvault/internal/procs"
	"devvault/internal/projects"
	"devvault/internal/relaywire"
	"devvault/internal/remote"
	"devvault/internal/searchsvc"
	"devvault/internal/secrets"
	"devvault/internal/snippets"
	"devvault/internal/store"
	"devvault/internal/tasks"
	"devvault/internal/terminal"
)

// App is the bound application facade. Every method is exposed to the
// frontend as window.go.main.App.<Method> by Wails.
type App struct {
	ctx        context.Context
	log        *slog.Logger
	dataDir    string
	sink       *wailsSink
	st         *store.Store
	secret     secrets.Store
	projects   *projects.Service
	gitsvcOK   bool
	terms      *terminal.Manager
	dbs        *dbsvc.Service
	apis       *apisvc.Service
	env        *envsvc.Service
	snips      *snippets.Service
	tasks      *tasks.Service
	procs      *procs.Service
	containers *containers.Service
	search     *searchsvc.Service
	dash       *dashsvc.Service
	remote     *remote.Agent
	plugins    *plugins.Service
	localAPI   *localapi.Server
}

// NewApp opens storage and constructs all services.
func NewApp(log *slog.Logger) (*App, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		base = "."
	}
	dataDir := filepath.Join(base, "DevVault")
	st, err := store.Open(filepath.Join(dataDir, "devvault.db"))
	if err != nil {
		return nil, err
	}
	sink := &wailsSink{}
	secret := secrets.New(filepath.Join(dataDir, "secrets"))

	proj := projects.New(st.DB())
	a := &App{
		log: log, dataDir: dataDir, sink: sink, st: st, secret: secret,
		projects:   proj,
		terms:      terminal.NewManager(sink),
		dbs:        dbsvc.New(st.DB(), secret),
		apis:       apisvc.New(st.DB()),
		env:        envsvc.New(st.DB()),
		snips:      snippets.New(st.DB()),
		tasks:      tasks.New(st.DB()),
		procs:      procs.New(st.DB(), sink),
		containers: containers.New(),
		search:     searchsvc.New(st.DB()),
		localAPI:   localapi.New(st.DB(), proj, snippets.New(st.DB())),
	}
	a.gitsvcOK = gitsvc.Available()
	a.remote = remote.NewAgent(dataDir, st.DB(), sink, log)
	a.plugins = plugins.New(st.DB(), sink, filepath.Join(dataDir, "plugins"))
	a.dash = dashsvc.New(st.DB())
	a.dash.RemoteStatus = a.remoteDashboardInfo
	return a, nil
}

func (a *App) remoteDashboardInfo() dashsvc.RemoteInfo {
	st := a.remote.Status()
	info := dashsvc.RemoteInfo{Enabled: st.Enabled, Connected: st.Connected}
	info.OnlineDevices = 0
	if st.Connected {
		info.OnlineDevices = 1 // this device is online at the relay
	}
	info.PendingRequests = len(st.Pending)
	if st.Enabled && !st.Connected {
		info.Detail = "enabled but relay unreachable"
	}
	return info
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.sink.ctx = ctx
	_ = store.SetSetting(a.st.DB(), "app.version", "1.0.0")
	_ = store.SetSetting(a.st.DB(), "app.platform", runtime.GOOS+"/"+runtime.GOARCH)
	_ = store.Audit(a.st.DB(), "system", "app.start", "DevVault 1.0.0 "+runtime.GOOS+"/"+runtime.GOARCH)

	// Auto-start remote access only if the user explicitly enabled it before.
	if v, ok := store.Setting(a.st.DB(), "remote.enabled"); ok && v == "1" {
		if err := a.remote.Start(); err != nil {
			a.log.Warn("remote access auto-start failed", "err", err)
		}
	}
	// Start previously enabled plugins.
	go func() {
		if list, err := a.plugins.Discover(); err == nil {
			for _, p := range list {
				if p.Enabled {
					_ = a.plugins.Start(p.ID)
				}
			}
		}
	}()
	// Restore the opt-in local API.
	if v, ok := store.Setting(a.st.DB(), "localapi.enabled"); ok && v == "1" {
		if token, err := a.localAPI.Start(7920); err == nil {
			_ = store.SetSetting(a.st.DB(), "localapi.token", token)
		}
	}
}

func (a *App) shutdown(ctx context.Context) {
	a.terms.CloseAll()
	_ = a.remote.Stop()
	a.plugins.StopAll()
	a.localAPI.Stop()
	_ = store.Audit(a.st.DB(), "system", "app.stop", "")
	_ = a.st.Close()
}

// ---- settings & audit ----

func (a *App) SettingsGet(key string) string {
	v, _ := store.Setting(a.st.DB(), key)
	return v
}

func (a *App) SettingsSet(key, value string) error {
	err := store.SetSetting(a.st.DB(), key, value)
	if err == nil {
		_ = store.Audit(a.st.DB(), "user", "settings.set", key)
	}
	return err
}

func (a *App) SettingsAll() (map[string]string, error) { return store.AllSettings(a.st.DB()) }

func (a *App) AuditList(limit int) ([]store.AuditEntry, error) {
	return store.ListAudit(a.st.DB(), limit)
}

// ---- projects ----

func (a *App) ProjectsList() ([]projects.Project, error) { return a.projects.List() }

func (a *App) ProjectsGet(id string) (projects.Project, error) { return a.projects.Get(id) }

func (a *App) ProjectsAdd(path, name, group string, tags []string) (projects.Project, error) {
	p, err := a.projects.Add(path, name, group, tags)
	if err == nil {
		_ = store.Audit(a.st.DB(), "user", "project.add", p.Name+" "+p.Path)
	}
	return p, err
}

func (a *App) ProjectsUpdate(id, name, group string, tags []string, favorite bool, notes string) (projects.Project, error) {
	return a.projects.Update(id, name, group, tags, favorite, notes)
}

func (a *App) ProjectsRemove(id string) error {
	err := a.projects.Remove(id)
	if err == nil {
		_ = store.Audit(a.st.DB(), "user", "project.remove", id)
	}
	return err
}

func (a *App) ProjectsGroups() ([]string, error) { return a.projects.Groups() }

// ---- git ----

func (a *App) GitAvailable() bool { return a.gitsvcOK }

func (a *App) GitStatus(path string) (*gitsvc.StatusResult, error) { return gitsvc.Status(path) }

func (a *App) GitLog(path string, limit int) ([]gitsvc.CommitEntry, error) {
	return gitsvc.Log(path, limit)
}

func (a *App) GitBranches(path string) ([]gitsvc.Branch, error) { return gitsvc.Branches(path) }

func (a *App) GitRemotes(path string) (map[string]string, error) { return gitsvc.Remotes(path) }

func (a *App) GitStage(path string, files []string) error { return gitsvc.Stage(path, files) }

func (a *App) GitUnstage(path string, files []string) error { return gitsvc.Unstage(path, files) }

func (a *App) GitCommit(path, message string, amend bool) (string, error) {
	out, err := gitsvc.Commit(path, message, amend)
	if err == nil {
		_ = store.Audit(a.st.DB(), "user", "git.commit", path+": "+firstLine(message))
	}
	return out, err
}

func (a *App) GitCheckout(path, branch string, create bool) error {
	return gitsvc.Checkout(path, branch, create)
}

func (a *App) GitDeleteBranch(path, branch string, force bool) error {
	return gitsvc.DeleteBranch(path, branch, force)
}

func (a *App) GitPush(path, remote, branch string, setUpstream bool) error {
	err := gitsvc.Push(path, remote, branch, setUpstream)
	if err == nil {
		_ = store.Audit(a.st.DB(), "user", "git.push", path+" "+remote+" "+branch)
	}
	return err
}

func (a *App) GitPull(path, remote, branch string) error {
	err := gitsvc.Pull(path, remote, branch)
	if err == nil {
		_ = store.Audit(a.st.DB(), "user", "git.pull", path)
	}
	return err
}

func (a *App) GitFetch(path string) error { return gitsvc.Fetch(path) }

func (a *App) GitClone(url, dest string) (string, error) {
	p, err := gitsvc.Clone(nil, url, dest)
	if err == nil {
		_ = store.Audit(a.st.DB(), "user", "git.clone", url+" -> "+p)
	}
	return p, err
}

func (a *App) GitDiff(path, file string, cached bool) (string, error) {
	return gitsvc.Diff(path, file, cached)
}

func (a *App) GitInit(path string) error { return gitsvc.Init(path) }

func firstLine(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			return s[:i]
		}
	}
	return s
}

// ---- terminal ----

func (a *App) TermSupported() bool { return terminal.IsSupported() }

func (a *App) TermStart(cwd, shell string) (string, error) { return a.terms.Start(cwd, shell) }

func (a *App) TermWrite(id, data string) error { return a.terms.Write(id, data) }

func (a *App) TermResize(id string, cols, rows int) error { return a.terms.Resize(id, cols, rows) }

func (a *App) TermClose(id string) error { return a.terms.Close(id) }

func (a *App) TermList() []string { return a.terms.List() }

// ---- database workbench ----

func (a *App) DBListConnections() ([]dbsvc.Connection, error) { return a.dbs.ListConnections() }

func (a *App) DBCreateConnection(name, typ, host string, port int, user, password, database, path string) (dbsvc.Connection, error) {
	c, err := a.dbs.CreateConnection(name, typ, host, port, user, password, database, path)
	if err == nil {
		_ = store.Audit(a.st.DB(), "user", "db.connection.create", name)
	}
	return c, err
}

func (a *App) DBDeleteConnection(id string) error { return a.dbs.DeleteConnection(id) }

func (a *App) DBTest(id string) error { return a.dbs.TestConnection(id) }

func (a *App) DBQuery(id, sqlText string, maxRows int) (*dbsvc.QueryResult, error) {
	return a.dbs.Query(id, sqlText, maxRows)
}

func (a *App) DBHistory(connID string, limit int) ([]dbsvc.HistoryRow, error) {
	return a.dbs.History(connID, limit)
}

func (a *App) DBListTables(id string) ([]string, error) { return a.dbs.ListTables(id) }

func (a *App) DBExport(id, sqlText, format string) (string, error) {
	return a.dbs.Export(id, sqlText, format, filepath.Join(a.dataDir, "exports"))
}

// ---- API workbench ----

func (a *App) APICollections() ([]apisvc.Collection, error) { return a.apis.ListCollections() }

func (a *App) APICreateCollection(name string) (apisvc.Collection, error) {
	return a.apis.CreateCollection(name)
}

func (a *App) APIDeleteCollection(id string) error { return a.apis.DeleteCollection(id) }

func (a *App) APIRequests(collectionID string) ([]apisvc.Request, error) {
	return a.apis.ListRequests(collectionID)
}

func (a *App) APISaveRequest(r apisvc.Request) (apisvc.Request, error) { return a.apis.SaveRequest(r) }

func (a *App) APIDeleteRequest(id string) error { return a.apis.DeleteRequest(id) }

func (a *App) APISend(r apisvc.Request, env map[string]string) (*apisvc.Response, error) {
	return a.apis.Send(context.Background(), r, env)
}

func (a *App) APIHistory(limit int) ([]apisvc.HistoryItem, error) { return a.apis.History(limit) }

func (a *App) APISetEnvironment(collectionID, name string, vars map[string]string) error {
	return a.apis.SetEnvironment(collectionID, name, vars)
}

func (a *App) APIEnvironments(collectionID string) ([]map[string]any, error) {
	return a.apis.ListEnvironments(collectionID)
}

// ---- env manager ----

func (a *App) EnvEnvironments(projectID string) ([]string, error) {
	return a.env.Environments(projectID)
}

func (a *App) EnvGet(projectID, env string) ([]envsvc.Var, error) { return a.env.Get(projectID, env) }

func (a *App) EnvSet(projectID, env, key, value string, secret bool) error {
	return a.env.SetVar(projectID, env, key, value, secret)
}

func (a *App) EnvDelete(projectID, env, key string) error {
	return a.env.DeleteVar(projectID, env, key)
}

func (a *App) EnvReveal(projectID, env, key string) (string, error) {
	_ = store.Audit(a.st.DB(), "user", "env.reveal", key+"@"+env)
	return a.env.Reveal(projectID, env, key)
}

func (a *App) EnvExport(projectID, env string) (string, error) {
	return a.env.ExportDotenv(projectID, env)
}

func (a *App) EnvImport(projectID, env, text string) (int, error) {
	return a.env.ImportDotenv(projectID, env, text)
}

// ---- snippets & notes ----

func (a *App) SnippetsList() ([]snippets.Snippet, error) { return a.snips.ListSnippets() }

func (a *App) SnippetGet(id string) (snippets.Snippet, error) { return a.snips.GetSnippet(id) }

func (a *App) SnippetSave(id, title, language, code string, tags []string, favorite bool) (snippets.Snippet, error) {
	return a.snips.SaveSnippet(id, title, language, code, tags, favorite)
}

func (a *App) SnippetDelete(id string) error { return a.snips.DeleteSnippet(id) }

func (a *App) NotesList() ([]snippets.Note, error) { return a.snips.ListNotes() }

func (a *App) NoteGet(id string) (snippets.Note, error) { return a.snips.GetNote(id) }

func (a *App) NoteSave(id, title, body string, tags []string, pinned bool) (snippets.Note, error) {
	return a.snips.SaveNote(id, title, body, tags, pinned)
}

func (a *App) NoteDelete(id string) error { return a.snips.DeleteNote(id) }

// ---- tasks ----

func (a *App) TasksList() ([]tasks.Task, error) { return a.tasks.List() }

func (a *App) TaskCreate(title, description, status, priority, dueDate, projectID string) (tasks.Task, error) {
	return a.tasks.Create(title, description, status, priority, dueDate, projectID)
}

func (a *App) TaskUpdate(id, title, description, status, priority, dueDate string) (tasks.Task, error) {
	return a.tasks.Update(id, title, description, status, priority, dueDate)
}

func (a *App) TaskMove(id, status string, order int) error { return a.tasks.Move(id, status, order) }

func (a *App) TaskDelete(id string) error { return a.tasks.Delete(id) }

func (a *App) TasksSummary() (map[string]int, error) { return a.tasks.Summary() }

// ---- processes ----

func (a *App) ProcsList() ([]procs.Proc, error) { return a.procs.List() }

func (a *App) ProcRegister(name, command string, args []string, cwd, projectID string) (procs.Proc, error) {
	return a.procs.Register(name, command, args, cwd, projectID)
}

func (a *App) ProcUpdate(id, name, command string, args []string, cwd string) error {
	return a.procs.Update(id, name, command, args, cwd)
}

func (a *App) ProcDelete(id string) error { return a.procs.Delete(id) }

func (a *App) ProcStart(id string) error { return a.procs.Start(id) }

func (a *App) ProcStop(id string, confirmed bool) error {
	err := a.procs.Stop(id, confirmed)
	if err == nil {
		_ = store.Audit(a.st.DB(), "user", "proc.stop", id)
	}
	return err
}

func (a *App) ProcStatus(id string) (procs.Status, error) { return a.procs.Status(id) }

func (a *App) ProcStatusAll() ([]procs.Status, error) { return a.procs.StatusAll() }

func (a *App) ProcLogs(id string) ([]string, error) { return a.procs.Logs(id) }

// ---- containers ----

func (a *App) ContainersDetect() (bool, string) { return a.containers.Detect() }

func (a *App) ContainersList() ([]containers.Container, error) { return a.containers.List() }

func (a *App) ContainersImages() ([]containers.Image, error) { return a.containers.Images() }

func (a *App) ContainerStart(id string) error { return a.containers.Start(id) }

func (a *App) ContainerStop(id string) error { return a.containers.Stop(id) }

func (a *App) ContainerLogs(id string, tail int) (string, error) { return a.containers.Logs(id, tail) }

// ---- search & dashboard ----

func (a *App) Search(q string) ([]searchsvc.Hit, error) { return a.search.Global(q) }

func (a *App) DashboardStats() (*dashsvc.Stats, error) { return a.dash.Get() }

func (a *App) DataDir() string { return a.dataDir }

// ---- DevVault Anywhere ----

func (a *App) RemoteStatus() remote.Status { return a.remote.Status() }

func (a *App) RemoteConfigure(relayAddr, fingerprint, deviceName string) error {
	return a.remote.Configure(relayAddr, fingerprint, deviceName)
}

func (a *App) RemoteStart() error {
	if err := a.remote.Start(); err != nil {
		return err
	}
	if err := store.SetSetting(a.st.DB(), "remote.enabled", "1"); err != nil {
		return err
	}
	return nil
}

func (a *App) RemoteStop() error {
	if err := a.remote.Stop(); err != nil {
		return err
	}
	return store.SetSetting(a.st.DB(), "remote.enabled", "0")
}

func (a *App) RemotePairingCode() (string, error) {
	code, err := a.remote.CreatePairing()
	if err == nil {
		_ = store.Audit(a.st.DB(), "user", "remote.pairing.created", "code generated")
	}
	return code, err
}

func (a *App) RemoteApproveClient(clientID string, approve bool) error {
	return a.remote.ApproveClient(clientID, approve)
}

func (a *App) RemoteClients() ([]relaywire.ClientInfo, error) {
	return a.remote.ListClients()
}

func (a *App) LocalAPIStatus() map[string]any {
	token, _ := a.localAPI.Token()
	return map[string]any{
		"running": a.localAPI.Running(),
		"port":    a.localAPI.Port(),
		"token":   token,
	}
}

func (a *App) LocalAPIEnable(port int) (string, error) {
	token, err := a.localAPI.Start(port)
	if err == nil {
		_ = store.SetSetting(a.st.DB(), "localapi.enabled", "1")
		_ = store.SetSetting(a.st.DB(), "localapi.token", token)
	}
	return token, err
}

func (a *App) LocalAPIDisable() {
	a.localAPI.Stop()
	_ = store.SetSetting(a.st.DB(), "localapi.enabled", "0")
	_ = store.DeleteSetting(a.st.DB(), "localapi.token")
}

// ---- plugins ----

func (a *App) PluginsList() ([]plugins.Plugin, error) { return a.plugins.Discover() }

func (a *App) PluginSetEnabled(id string, enabled bool) error {
	return a.plugins.SetEnabled(id, enabled)
}

func (a *App) PluginPing(id string) (string, error) { return a.plugins.Ping(id) }

func (a *App) PluginsUserDir() string { return a.plugins.UserDir() }
