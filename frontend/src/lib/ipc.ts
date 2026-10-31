// Typed bridge to the Go backend. Wails injects window.go.main.App.* at
// runtime (dev and production); this module is the single access point.

/* eslint-disable @typescript-eslint/no-explicit-any */
function backend(): any {
  const g = (window as any).go;
  if (!g?.main?.App) {
    throw new Error(
      "DevVault backend bridge not available. Launch the app via `wails dev` or the built binary."
    );
  }
  return g.main.App;
}

export async function call<T = any>(method: string, ...args: any[]): Promise<T> {
  const app = backend();
  const fn = app[method];
  if (typeof fn !== "function") {
    throw new Error(`Unknown backend method: ${method}`);
  }
  return fn(...args) as Promise<T>;
}

export function onEvent(name: string, cb: (data: any) => void): () => void {
  const rt = (window as any).runtime;
  if (!rt?.EventsOn) return () => {};
  rt.EventsOn(name, cb);
  return () => rt.EventsOff(name);
}

// ---- entity types (mirroring Go structs) ----

export interface Project {
  id: string;
  path: string;
  name: string;
  group: string;
  tags: string[];
  favorite: boolean;
  notes: string;
  createdAt: string;
  lastOpenedAt: string;
}

export interface GitStatusResult {
  branch: string;
  upstream?: string;
  ahead: number;
  behind: number;
  entries: { x: string; y: string; path: string; orig?: string; staged: boolean }[];
  clean: boolean;
}

export interface CommitEntry {
  hash: string;
  short: string;
  author: string;
  date: string;
  subject: string;
}

export interface Branch {
  name: string;
  current: boolean;
  isRemote: boolean;
}

export interface Connection {
  id: string;
  name: string;
  type: string;
  host: string;
  port: number;
  user: string;
  database: string;
  path: string;
  hasPassword: boolean;
}

export interface QueryResult {
  columns: string[];
  rows: any[][];
  rowsAffected: number;
  elapsedMs: number;
  truncatedRows: boolean;
}

export interface ApiRequest {
  id: string;
  collectionId: string;
  name: string;
  method: string;
  url: string;
  headers: Record<string, string>;
  query: Record<string, string>;
  body: string;
  authType: string;
  authValue: string;
  order: number;
}

export interface ApiResponse {
  status: number;
  statusText: string;
  headers: Record<string, string>;
  body: string;
  elapsedMs: number;
  sizeBytes: number;
  error?: string;
}

export interface EnvVar {
  key: string;
  value: string;
  secret: boolean;
}

export interface Snippet {
  id: string;
  title: string;
  language: string;
  code: string;
  tags: string[];
  favorite: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface Note {
  id: string;
  title: string;
  body: string;
  tags: string[];
  pinned: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface Task {
  id: string;
  title: string;
  description: string;
  status: string;
  priority: string;
  dueDate: string;
  projectId: string;
  order: number;
  createdAt: string;
  updatedAt: string;
}

export interface Proc {
  id: string;
  name: string;
  command: string;
  args: string[];
  cwd: string;
  projectId: string;
}

export interface ProcStatus {
  id: string;
  running: boolean;
  pid?: number;
  started?: string;
}

export interface Container {
  id: string;
  name: string;
  image: string;
  state: string;
  status: string;
  ports: string;
}

export interface Image {
  id: string;
  repo: string;
  tag: string;
  size: string;
  createdAt: string;
}

export interface RemoteStatus {
  enabled: boolean;
  connected: boolean;
  relayAddr: string;
  deviceId: string;
  deviceName: string;
  fingerprint: string;
  apiPort: number;
  activeTunnels: number;
  pending?: string[];
}

export interface ClientInfo {
  client_id: string;
  name: string;
  approved: boolean;
  last_seen?: string;
}

export interface Stats {
  projects: number;
  snippets: number;
  tasksByStatus: Record<string, number>;
  remoteEnabled: boolean;
  cpuPercent: number;
  memPercent: number;
  memTotal: number;
  memUsed: number;
  goVersion: string;
  numCPU: number;
  generatedAt: string;
  remote: {
    enabled: boolean;
    connected: boolean;
    onlineDevices: number;
    pendingRequests: number;
    detail?: string;
  };
}

export interface SearchHit {
  type: string;
  id: string;
  title: string;
  subtitle: string;
}

export interface AuditEntry {
  id: number;
  ts: string;
  actor: string;
  action: string;
  detail: string;
}

// ---- facade ----

export const api = {
  // settings & audit
  settingsGet: (k: string) => call<string>("SettingsGet", k),
  settingsSet: (k: string, v: string) => call<void>("SettingsSet", k, v),
  settingsAll: () => call<Record<string, string>>("SettingsAll"),
  auditList: (limit = 200) => call<AuditEntry[]>("AuditList", limit),

  // projects
  projectsList: () => call<Project[]>("ProjectsList"),
  projectsAdd: (path: string, name: string, group: string, tags: string[]) =>
    call<Project>("ProjectsAdd", path, name, group, tags),
  projectsUpdate: (
    id: string,
    name: string,
    group: string,
    tags: string[],
    favorite: boolean,
    notes: string
  ) => call<Project>("ProjectsUpdate", id, name, group, tags, favorite, notes),
  projectsRemove: (id: string) => call<void>("ProjectsRemove", id),
  projectsGroups: () => call<string[]>("ProjectsGroups"),

  // git
  gitAvailable: () => call<boolean>("GitAvailable"),
  gitStatus: (path: string) => call<GitStatusResult>("GitStatus", path),
  gitLog: (path: string, limit = 50) => call<CommitEntry[]>("GitLog", path, limit),
  gitBranches: (path: string) => call<Branch[]>("GitBranches", path),
  gitRemotes: (path: string) => call<Record<string, string>>("GitRemotes", path),
  gitStage: (path: string, files: string[]) => call<void>("GitStage", path, files),
  gitUnstage: (path: string, files: string[]) => call<void>("GitUnstage", path, files),
  gitCommit: (path: string, message: string, amend = false) =>
    call<string>("GitCommit", path, message, amend),
  gitCheckout: (path: string, branch: string, create: boolean) =>
    call<void>("GitCheckout", path, branch, create),
  gitDeleteBranch: (path: string, branch: string, force: boolean) =>
    call<void>("GitDeleteBranch", path, branch, force),
  gitPush: (path: string, remote: string, branch: string, setUpstream: boolean) =>
    call<void>("GitPush", path, remote, branch, setUpstream),
  gitPull: (path: string, remote: string, branch: string) =>
    call<void>("GitPull", path, remote, branch),
  gitFetch: (path: string) => call<void>("GitFetch", path),
  gitClone: (url: string, dest: string) => call<string>("GitClone", url, dest),
  gitDiff: (path: string, file: string, cached: boolean) =>
    call<string>("GitDiff", path, file, cached),
  gitInit: (path: string) => call<void>("GitInit", path),

  // terminal
  termSupported: () => call<boolean>("TermSupported"),
  termStart: (cwd: string, shell: string) => call<string>("TermStart", cwd, shell),
  termWrite: (id: string, data: string) => call<void>("TermWrite", id, data),
  termResize: (id: string, cols: number, rows: number) =>
    call<void>("TermResize", id, cols, rows),
  termClose: (id: string) => call<void>("TermClose", id),

  // database workbench
  dbListConnections: () => call<Connection[]>("DBListConnections"),
  dbCreateConnection: (
    name: string,
    type: string,
    host: string,
    port: number,
    user: string,
    password: string,
    database: string,
    path: string
  ) => call<Connection>("DBCreateConnection", name, type, host, port, user, password, database, path),
  dbDeleteConnection: (id: string) => call<void>("DBDeleteConnection", id),
  dbTest: (id: string) => call<void>("DBTest", id),
  dbQuery: (id: string, sql: string, maxRows = 500) =>
    call<QueryResult>("DBQuery", id, sql, maxRows),
  dbHistory: (connId: string, limit = 100) => call<any[]>("DBHistory", connId, limit),
  dbListTables: (id: string) => call<string[]>("DBListTables", id),
  dbExport: (id: string, sql: string, format: string) =>
    call<string>("DBExport", id, sql, format),

  // api workbench
  apiCollections: () => call<{ id: string; name: string }[]>("APICollections"),
  apiCreateCollection: (name: string) =>
    call<{ id: string; name: string }>("APICreateCollection", name),
  apiDeleteCollection: (id: string) => call<void>("APIDeleteCollection", id),
  apiRequests: (collectionId: string) => call<ApiRequest[]>("APIRequests", collectionId),
  apiSaveRequest: (r: Partial<ApiRequest>) => call<ApiRequest>("APISaveRequest", r),
  apiDeleteRequest: (id: string) => call<void>("APIDeleteRequest", id),
  apiSend: (r: Partial<ApiRequest>, env: Record<string, string>) =>
    call<ApiResponse>("APISend", r, env),
  apiHistory: (limit = 100) =>
    call<{ id: number; ts: string; method: string; url: string; status: number; elapsedMs: number; error?: string }[]>(
      "APIHistory",
      limit
    ),
  apiSetEnvironment: (collectionId: string, name: string, vars: Record<string, string>) =>
    call<void>("APISetEnvironment", collectionId, name, vars),
  apiEnvironments: (collectionId: string) =>
    call<{ id: string; name: string; variables: Record<string, string> }[]>(
      "APIEnvironments",
      collectionId
    ),

  // env manager
  envEnvironments: (projectId: string) => call<string[]>("EnvEnvironments", projectId),
  envGet: (projectId: string, env: string) => call<EnvVar[]>("EnvGet", projectId, env),
  envSet: (projectId: string, env: string, key: string, value: string, secret: boolean) =>
    call<void>("EnvSet", projectId, env, key, value, secret),
  envDelete: (projectId: string, env: string, key: string) =>
    call<void>("EnvDelete", projectId, env, key),
  envReveal: (projectId: string, env: string, key: string) =>
    call<string>("EnvReveal", projectId, env, key),
  envExport: (projectId: string, env: string) => call<string>("EnvExport", projectId, env),
  envImport: (projectId: string, env: string, text: string) =>
    call<number>("EnvImport", projectId, env, text),

  // snippets & notes
  snippetsList: () => call<Snippet[]>("SnippetsList"),
  snippetSave: (
    id: string,
    title: string,
    language: string,
    code: string,
    tags: string[],
    favorite: boolean
  ) => call<Snippet>("SnippetSave", id, title, language, code, tags, favorite),
  snippetDelete: (id: string) => call<void>("SnippetDelete", id),
  notesList: () => call<Note[]>("NotesList"),
  noteSave: (id: string, title: string, body: string, tags: string[], pinned: boolean) =>
    call<Note>("NoteSave", id, title, body, tags, pinned),
  noteDelete: (id: string) => call<void>("NoteDelete", id),

  // tasks
  tasksList: () => call<Task[]>("TasksList"),
  taskCreate: (
    title: string,
    description: string,
    status: string,
    priority: string,
    dueDate: string,
    projectId: string
  ) => call<Task>("TaskCreate", title, description, status, priority, dueDate, projectId),
  taskUpdate: (
    id: string,
    title: string,
    description: string,
    status: string,
    priority: string,
    dueDate: string
  ) => call<Task>("TaskUpdate", id, title, description, status, priority, dueDate),
  taskMove: (id: string, status: string, order: number) => call<void>("TaskMove", id, status, order),
  taskDelete: (id: string) => call<void>("TaskDelete", id),
  tasksSummary: () => call<Record<string, number>>("TasksSummary"),

  // processes
  procsList: () => call<Proc[]>("ProcsList"),
  procRegister: (name: string, command: string, args: string[], cwd: string, projectId: string) =>
    call<Proc>("ProcRegister", name, command, args, cwd, projectId),
  procDelete: (id: string) => call<void>("ProcDelete", id),
  procStart: (id: string) => call<void>("ProcStart", id),
  procStop: (id: string, confirmed: boolean) => call<void>("ProcStop", id, confirmed),
  procStatusAll: () => call<ProcStatus[]>("ProcStatusAll"),
  procLogs: (id: string) => call<string[]>("ProcLogs", id),

  // containers
  containersDetect: () => call<[boolean, string]>("ContainersDetect"),
  containersList: () => call<Container[]>("ContainersList"),
  containersImages: () => call<Image[]>("ContainersImages"),
  containerStart: (id: string) => call<void>("ContainerStart", id),
  containerStop: (id: string) => call<void>("ContainerStop", id),
  containerLogs: (id: string, tail = 200) => call<string>("ContainerLogs", id, tail),

  // search & dashboard
  search: (q: string) => call<SearchHit[]>("Search", q),
  dashboardStats: () => call<Stats>("DashboardStats"),
  dataDir: () => call<string>("DataDir"),

  // remote
  remoteStatus: () => call<RemoteStatus>("RemoteStatus"),
  remoteConfigure: (relayAddr: string, fingerprint: string, deviceName: string) =>
    call<void>("RemoteConfigure", relayAddr, fingerprint, deviceName),
  remoteStart: () => call<void>("RemoteStart"),
  remoteStop: () => call<void>("RemoteStop"),
  remotePairingCode: () => call<string>("RemotePairingCode"),
  remoteApproveClient: (clientId: string, approve: boolean) =>
    call<void>("RemoteApproveClient", clientId, approve),
  remoteClients: () => call<ClientInfo[]>("RemoteClients"),

  // local api
  localApiStatus: () =>
    call<{ running: boolean; port: number; token: string }>("LocalAPIStatus"),
  localApiEnable: (port: number) => call<string>("LocalAPIEnable", port),
  localApiDisable: () => call<void>("LocalAPIDisable"),

  // plugins
  pluginsList: () =>
    call<
      {
        id: string;
        name: string;
        version: string;
        description: string;
        permissions: string[];
        enabled: boolean;
        path: string;
        running: boolean;
      }[]
    >("PluginsList"),
  pluginSetEnabled: (id: string, enabled: boolean) =>
    call<void>("PluginSetEnabled", id, enabled),
  pluginPing: (id: string) => call<string>("PluginPing", id),
  pluginsUserDir: () => call<string>("PluginsUserDir"),
};
