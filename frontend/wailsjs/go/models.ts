export namespace apisvc {
	
	export class Collection {
	    id: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new Collection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	    }
	}
	export class HistoryItem {
	    id: number;
	    ts: string;
	    method: string;
	    url: string;
	    status: number;
	    elapsedMs: number;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new HistoryItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.ts = source["ts"];
	        this.method = source["method"];
	        this.url = source["url"];
	        this.status = source["status"];
	        this.elapsedMs = source["elapsedMs"];
	        this.error = source["error"];
	    }
	}
	export class Request {
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
	
	    static createFrom(source: any = {}) {
	        return new Request(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.collectionId = source["collectionId"];
	        this.name = source["name"];
	        this.method = source["method"];
	        this.url = source["url"];
	        this.headers = source["headers"];
	        this.query = source["query"];
	        this.body = source["body"];
	        this.authType = source["authType"];
	        this.authValue = source["authValue"];
	        this.order = source["order"];
	    }
	}
	export class Response {
	    status: number;
	    statusText: string;
	    headers: Record<string, string>;
	    body: string;
	    elapsedMs: number;
	    sizeBytes: number;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new Response(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.statusText = source["statusText"];
	        this.headers = source["headers"];
	        this.body = source["body"];
	        this.elapsedMs = source["elapsedMs"];
	        this.sizeBytes = source["sizeBytes"];
	        this.error = source["error"];
	    }
	}

}

export namespace containers {
	
	export class Container {
	    id: string;
	    name: string;
	    image: string;
	    state: string;
	    status: string;
	    ports: string;
	
	    static createFrom(source: any = {}) {
	        return new Container(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.image = source["image"];
	        this.state = source["state"];
	        this.status = source["status"];
	        this.ports = source["ports"];
	    }
	}
	export class Image {
	    id: string;
	    repo: string;
	    tag: string;
	    size: string;
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new Image(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.repo = source["repo"];
	        this.tag = source["tag"];
	        this.size = source["size"];
	        this.createdAt = source["createdAt"];
	    }
	}

}

export namespace dashsvc {
	
	export class RemoteInfo {
	    enabled: boolean;
	    connected: boolean;
	    onlineDevices: number;
	    pendingRequests: number;
	    detail?: string;
	
	    static createFrom(source: any = {}) {
	        return new RemoteInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.connected = source["connected"];
	        this.onlineDevices = source["onlineDevices"];
	        this.pendingRequests = source["pendingRequests"];
	        this.detail = source["detail"];
	    }
	}
	export class Stats {
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
	    remote: RemoteInfo;
	
	    static createFrom(source: any = {}) {
	        return new Stats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.projects = source["projects"];
	        this.snippets = source["snippets"];
	        this.tasksByStatus = source["tasksByStatus"];
	        this.remoteEnabled = source["remoteEnabled"];
	        this.cpuPercent = source["cpuPercent"];
	        this.memPercent = source["memPercent"];
	        this.memTotal = source["memTotal"];
	        this.memUsed = source["memUsed"];
	        this.goVersion = source["goVersion"];
	        this.numCPU = source["numCPU"];
	        this.generatedAt = source["generatedAt"];
	        this.remote = this.convertValues(source["remote"], RemoteInfo);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace dbsvc {
	
	export class Connection {
	    id: string;
	    name: string;
	    type: string;
	    host: string;
	    port: number;
	    user: string;
	    database: string;
	    path: string;
	    hasPassword: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Connection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.type = source["type"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.user = source["user"];
	        this.database = source["database"];
	        this.path = source["path"];
	        this.hasPassword = source["hasPassword"];
	    }
	}
	export class HistoryRow {
	    id: number;
	    connId: string;
	    sql: string;
	    ok: boolean;
	    error?: string;
	    rows: number;
	    elapsedMs: number;
	    ts: string;
	
	    static createFrom(source: any = {}) {
	        return new HistoryRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.connId = source["connId"];
	        this.sql = source["sql"];
	        this.ok = source["ok"];
	        this.error = source["error"];
	        this.rows = source["rows"];
	        this.elapsedMs = source["elapsedMs"];
	        this.ts = source["ts"];
	    }
	}
	export class QueryResult {
	    columns: string[];
	    rows: any[][];
	    rowsAffected: number;
	    elapsedMs: number;
	    truncatedRows: boolean;
	
	    static createFrom(source: any = {}) {
	        return new QueryResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.columns = source["columns"];
	        this.rows = source["rows"];
	        this.rowsAffected = source["rowsAffected"];
	        this.elapsedMs = source["elapsedMs"];
	        this.truncatedRows = source["truncatedRows"];
	    }
	}

}

export namespace envsvc {
	
	export class Var {
	    key: string;
	    value: string;
	    secret: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Var(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.value = source["value"];
	        this.secret = source["secret"];
	    }
	}

}

export namespace gitsvc {
	
	export class Branch {
	    name: string;
	    current: boolean;
	    isRemote: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Branch(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.current = source["current"];
	        this.isRemote = source["isRemote"];
	    }
	}
	export class CommitEntry {
	    hash: string;
	    short: string;
	    author: string;
	    date: string;
	    subject: string;
	
	    static createFrom(source: any = {}) {
	        return new CommitEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hash = source["hash"];
	        this.short = source["short"];
	        this.author = source["author"];
	        this.date = source["date"];
	        this.subject = source["subject"];
	    }
	}
	export class StatusEntry {
	    x: string;
	    y: string;
	    path: string;
	    orig?: string;
	    staged: boolean;
	
	    static createFrom(source: any = {}) {
	        return new StatusEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.x = source["x"];
	        this.y = source["y"];
	        this.path = source["path"];
	        this.orig = source["orig"];
	        this.staged = source["staged"];
	    }
	}
	export class StatusResult {
	    branch: string;
	    upstream?: string;
	    ahead: number;
	    behind: number;
	    entries: StatusEntry[];
	    clean: boolean;
	
	    static createFrom(source: any = {}) {
	        return new StatusResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.branch = source["branch"];
	        this.upstream = source["upstream"];
	        this.ahead = source["ahead"];
	        this.behind = source["behind"];
	        this.entries = this.convertValues(source["entries"], StatusEntry);
	        this.clean = source["clean"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace plugins {
	
	export class Plugin {
	    id: string;
	    name: string;
	    version: string;
	    description: string;
	    runtime: string;
	    entry: string;
	    permissions: string[];
	    enabled: boolean;
	    path: string;
	    running: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Plugin(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.version = source["version"];
	        this.description = source["description"];
	        this.runtime = source["runtime"];
	        this.entry = source["entry"];
	        this.permissions = source["permissions"];
	        this.enabled = source["enabled"];
	        this.path = source["path"];
	        this.running = source["running"];
	    }
	}

}

export namespace procs {
	
	export class Proc {
	    id: string;
	    name: string;
	    command: string;
	    args: string[];
	    cwd: string;
	    projectId: string;
	
	    static createFrom(source: any = {}) {
	        return new Proc(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.command = source["command"];
	        this.args = source["args"];
	        this.cwd = source["cwd"];
	        this.projectId = source["projectId"];
	    }
	}
	export class Status {
	    id: string;
	    running: boolean;
	    pid?: number;
	    started?: string;
	    exitCode?: number;
	    err?: string;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.running = source["running"];
	        this.pid = source["pid"];
	        this.started = source["started"];
	        this.exitCode = source["exitCode"];
	        this.err = source["err"];
	    }
	}

}

export namespace projects {
	
	export class Project {
	    id: string;
	    path: string;
	    name: string;
	    group: string;
	    tags: string[];
	    favorite: boolean;
	    notes: string;
	    createdAt: string;
	    lastOpenedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new Project(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.path = source["path"];
	        this.name = source["name"];
	        this.group = source["group"];
	        this.tags = source["tags"];
	        this.favorite = source["favorite"];
	        this.notes = source["notes"];
	        this.createdAt = source["createdAt"];
	        this.lastOpenedAt = source["lastOpenedAt"];
	    }
	}

}

export namespace relaywire {
	
	export class ClientInfo {
	    client_id: string;
	    name: string;
	    approved: boolean;
	    last_seen?: string;
	
	    static createFrom(source: any = {}) {
	        return new ClientInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.client_id = source["client_id"];
	        this.name = source["name"];
	        this.approved = source["approved"];
	        this.last_seen = source["last_seen"];
	    }
	}

}

export namespace remote {
	
	export class Status {
	    enabled: boolean;
	    connected: boolean;
	    relayAddr: string;
	    deviceId: string;
	    deviceName: string;
	    fingerprint: string;
	    apiPort: number;
	    activeTunnels: number;
	    pending: string[];
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.connected = source["connected"];
	        this.relayAddr = source["relayAddr"];
	        this.deviceId = source["deviceId"];
	        this.deviceName = source["deviceName"];
	        this.fingerprint = source["fingerprint"];
	        this.apiPort = source["apiPort"];
	        this.activeTunnels = source["activeTunnels"];
	        this.pending = source["pending"];
	    }
	}

}

export namespace searchsvc {
	
	export class Hit {
	    type: string;
	    id: string;
	    title: string;
	    subtitle: string;
	
	    static createFrom(source: any = {}) {
	        return new Hit(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.id = source["id"];
	        this.title = source["title"];
	        this.subtitle = source["subtitle"];
	    }
	}

}

export namespace snippets {
	
	export class Note {
	    id: string;
	    title: string;
	    body: string;
	    tags: string[];
	    pinned: boolean;
	    createdAt: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new Note(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.body = source["body"];
	        this.tags = source["tags"];
	        this.pinned = source["pinned"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class Snippet {
	    id: string;
	    title: string;
	    language: string;
	    code: string;
	    tags: string[];
	    favorite: boolean;
	    createdAt: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new Snippet(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.language = source["language"];
	        this.code = source["code"];
	        this.tags = source["tags"];
	        this.favorite = source["favorite"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}

}

export namespace store {
	
	export class AuditEntry {
	    id: number;
	    ts: string;
	    actor: string;
	    action: string;
	    detail: string;
	
	    static createFrom(source: any = {}) {
	        return new AuditEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.ts = source["ts"];
	        this.actor = source["actor"];
	        this.action = source["action"];
	        this.detail = source["detail"];
	    }
	}

}

export namespace tasks {
	
	export class Task {
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
	
	    static createFrom(source: any = {}) {
	        return new Task(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.description = source["description"];
	        this.status = source["status"];
	        this.priority = source["priority"];
	        this.dueDate = source["dueDate"];
	        this.projectId = source["projectId"];
	        this.order = source["order"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}

}

