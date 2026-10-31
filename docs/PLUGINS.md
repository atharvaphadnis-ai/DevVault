# Writing DevVault Plugins

Plugins are **out-of-process programs** that talk to DevVault over
**newline-delimited JSON-RPC 2.0 on stdio**. They can be written in any language
that can read stdin and write stdout (Node.js, Python, Go, Rust, …). A crashing
plugin can never take the app down.

## Manifest

Every plugin lives in its own folder under the plugins directory
(Settings → Plugins shows the exact path) with a `plugin.json`:

```json
{
  "id": "com.example.myplugin",
  "name": "My Plugin",
  "version": "1.0.0",
  "description": "What it does",
  "runtime": "node",            // node | python3 | binary
  "entry": "main.js",
  "permissions": ["host/projects/list", "host/tasks/count"]
}
```

## Protocol (`devvault-plugin-rpc/1`)

The host → plugin requests (all ndjson):

```jsonc
{"jsonrpc":"2.0","id":"1","method":"initialize","params":{"protocol":"devvault-plugin-rpc/1","permissions":[...]}}
{"jsonrpc":"2.0","id":"2","method":"ping","params":null}
```

Plugin → host:

```jsonc
// responses carry the same id:
{"jsonrpc":"2.0","id":"1","result":{"ok":true}}
// notifications (no id):
{"jsonrpc":"2.0","method":"log","params":"starting up"}
// requests to the host (permission-checked against the manifest):
{"jsonrpc":"2.0","id":"h1","method":"host/projects/list","params":{}}
```

Undeclared `host/*` calls fail with error code `-32001 permission denied`.

## Host methods available

| Method | Permission string | Returns |
| --- | --- | --- |
| List registered projects | `host/projects/list` | `[{id,name,path,group}]` |
| Open task count | `host/tasks/count` | `{"open": n}` |
| Search snippets | `host/snippets/search` | `[{id,title,language}]` |

## Lifecycle

1. **Discover** — DevVault scans the plugins directory on start and on Rescan.
2. **Enable** — the host spawns the process and performs the `initialize`
   handshake (10 s timeout). Failures disable the plugin with a clear error.
3. **Run** — the plugin stays alive; `log` notifications appear in the plugin's
   log stream; the Ping button sends a `ping` request.
4. **Disable/exit** — the host kills the process; state persists.

## Bundled example

`plugins/hello-plugin/` (Node.js) implements `initialize`, `ping`, `greet` and
`summarize` (which calls two host APIs). Copy it into your plugins directory,
hit **Rescan**, **Enable**, then **Ping**.

## Testing your plugin

```bash
# exercise the protocol manually:
echo '{"jsonrpc":"2.0","id":"1","method":"ping"}' | node main.js
```

## Publishing

Zip your plugin folder; users drop it into the plugins directory shown in the
Plugins page. Keep `id` stable across versions so settings persist.
