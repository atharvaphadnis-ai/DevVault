import { useEffect, useState } from "react";
import { Satellite, Power, QrCode, ShieldCheck, ShieldAlert, Copy, UserCheck, UserX, RefreshCw } from "lucide-react";
import { api, RemoteStatus, ClientInfo } from "@/lib/ipc";
import { Card, PageHeader, Spinner, ErrorNote, EmptyState, Badge, ConfirmDialog, Modal } from "@/components/ui";
import { onEvent } from "@/lib/ipc";
import { useApp, errMessage } from "@/lib/store";

export default function RemotePage() {
  const [status, setStatus] = useState<RemoteStatus | null>(null);
  const [clients, setClients] = useState<ClientInfo[]>([]);
  const [pending, setPending] = useState<{ clientId: string; name: string } | null>(null);
  const [pairCode, setPairCode] = useState("");
  const [pairExpiry, setPairExpiry] = useState(0);
  const [relayAddr, setRelayAddr] = useState("");
  const [fingerprint, setFingerprint] = useState("");
  const [deviceName, setDeviceName] = useState("");
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [cliHelp, setCliHelp] = useState(false);
  const toast = useApp((s) => s.toast);

  const load = async () => {
    try {
      const st = await api.remoteStatus();
      setStatus(st);
      setRelayAddr((prev) => prev || st.relayAddr);
      setFingerprint((prev) => prev || st.fingerprint);
      setDeviceName((prev) => prev || st.deviceName);
      if (st.enabled && st.connected) {
        try { setClients(await api.remoteClients()); } catch { setClients([]); }
      }
      setError("");
    } catch (e) {
      setError(errMessage(e));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, []); // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => {
    const offs = [
      onEvent("remote:client-pending", (p: any) => {
        setPending({ clientId: p.clientId, name: p.name });
        load();
      }),
      onEvent("remote:status", () => load()),
      onEvent("remote:error", (msg: any) => toast(String(msg), "error")),
    ];
    const t = setInterval(load, 8000);
    return () => { offs.forEach((f) => f()); clearInterval(t); };
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  // pairing code countdown
  useEffect(() => {
    if (!pairCode) return;
    const t = setInterval(() => {
      setPairExpiry((s) => {
        if (s <= 1) { setPairCode(""); return 0; }
        return s - 1;
      });
    }, 1000);
    return () => clearInterval(t);
  }, [pairCode]);

  const toggle = async () => {
    setBusy(true);
    try {
      if (status?.enabled) {
        await api.remoteStop();
        toast("Remote access disabled (kill switch)", "success");
      } else {
        await api.remoteConfigure(relayAddr.trim(), fingerprint.trim(), deviceName.trim());
        await api.remoteStart();
        toast("Remote access enabled — agent connecting to relay", "success");
      }
      await load();
    } catch (e) {
      toast(errMessage(e), "error");
    } finally {
      setBusy(false);
    }
  };

  return (
    <div>
      <PageHeader
        title="DevVault Anywhere"
        subtitle="Secure remote access via a managed relay. The desktop agent makes outbound-only connections — no port forwarding, no public IP."
        actions={
          status && (
            <button className={status.enabled ? "btn-danger" : "btn-primary"} disabled={busy} onClick={toggle}>
              <Power className="w-4 h-4" /> {status.enabled ? "Disable remote access" : "Enable remote access"}
            </button>
          )
        }
      />
      {error && <ErrorNote message={error} />}
      {loading && <Spinner />}
      {status && (
        <div className="space-y-4">
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <Card>
              <div className="flex items-center gap-2 mb-2">
                <Satellite className="w-4 h-4 text-muted" />
                <span className="section-title !mb-0">Agent status</span>
              </div>
              {status.enabled ? (
                status.connected ? <Badge tone="ok">Connected to relay</Badge> : <Badge tone="warn">Enabled — connecting…</Badge>
              ) : (
                <Badge tone="info">Disabled</Badge>
              )}
              <div className="text-xs text-muted mt-3 space-y-1 font-mono break-all">
                <div>device: {status.deviceName || "—"}</div>
                <div>device id: {status.deviceId.slice(0, 8)}…</div>
                {status.enabled && <div>active tunnels: {status.activeTunnels}</div>}
              </div>
            </Card>

            <Card className="md:col-span-2">
              <div className="section-title mb-3">Relay configuration</div>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                <div>
                  <label className="text-xs text-muted">Relay address (host:port)</label>
                  <input className="input w-full mt-1 font-mono" placeholder="relay.example.com:7921" value={relayAddr} onChange={(e) => setRelayAddr(e.target.value)} />
                </div>
                <div>
                  <label className="text-xs text-muted">This device name</label>
                  <input className="input w-full mt-1" placeholder="devvault-desktop" value={deviceName} onChange={(e) => setDeviceName(e.target.value)} />
                </div>
                <div className="md:col-span-2">
                  <label className="text-xs text-muted">Relay TLS fingerprint (SHA-256, printed by the relay on startup)</label>
                  <input className="input w-full mt-1 font-mono text-xs" placeholder="ab12cd34…" value={fingerprint} onChange={(e) => setFingerprint(e.target.value)} />
                </div>
              </div>
              <div className="mt-3 text-xs text-muted leading-relaxed">
                Development: run <code className="chip">devvault-relay --addr :7921</code> locally (see docs/DEVVAULT_ANYWHERE.md).
                Production: deploy the same binary on a VPS with a domain and pin its fingerprint here.
                A hosted relay operated by the DevVault project is not available yet — this is documented honestly in the README.
              </div>
            </Card>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <Card>
              <div className="flex items-center justify-between mb-3">
                <span className="section-title !mb-0">Pair a new remote client</span>
                {pairCode ? <Badge tone="warn">expires in {pairExpiry}s</Badge> : null}
              </div>
              {pairCode ? (
                <div className="text-center py-4">
                  <div className="text-3xl font-mono font-bold tracking-[0.35em] gradient-text">{pairCode}</div>
                  <div className="text-xs text-muted mt-2">Enter this code in devvault-cli within {pairExpiry}s, then approve below.</div>
                </div>
              ) : (
                <button
                  className="btn-primary w-full justify-center"
                  disabled={!status.enabled || !status.connected}
                  onClick={async () => {
                    try {
                      const code = await api.remotePairingCode();
                      setPairCode(code);
                      setPairExpiry(300);
                    } catch (e) {
                      toast(errMessage(e), "error");
                    }
                  }}
                >
                  <QrCode className="w-4 h-4" /> Generate pairing code
                </button>
              )}
              {!status.enabled && <div className="text-xs text-muted mt-2">Enable remote access first.</div>}
              <button className="btn-ghost w-full justify-center mt-3" onClick={() => setCliHelp(true)}>
                How do I connect from another machine?
              </button>
            </Card>

            <Card>
              <div className="flex items-center justify-between mb-3">
                <span className="section-title !mb-0">Paired clients</span>
                <button className="btn-ghost !py-1 !px-2 text-xs" onClick={load}><RefreshCw className="w-3 h-3" /></button>
              </div>
              {clients.length === 0 ? (
                <div className="text-sm text-muted py-3">No clients paired yet.</div>
              ) : (
                <div className="space-y-2 max-h-60 overflow-y-auto">
                  {clients.map((c) => (
                    <div key={c.client_id} className="flex items-center justify-between px-3 py-2 rounded-lg bg-surface2 text-sm">
                      <div>
                        <div className="font-medium">{c.name}</div>
                        <div className="text-xs text-muted font-mono">{c.client_id.slice(0, 12)}… {c.last_seen ? `· last seen ${c.last_seen}` : ""}</div>
                      </div>
                      {c.approved ? <Badge tone="ok">approved</Badge> : (
                        <div className="flex gap-1.5">
                          <button className="btn-primary !py-1 !px-2 text-xs" onClick={async () => {
                            await api.remoteApproveClient(c.client_id, true);
                            toast("Client approved", "success");
                            load();
                          }}><UserCheck className="w-3 h-3" /> Approve</button>
                          <button className="btn-ghost !py-1 !px-2 text-xs hover:!text-danger" onClick={async () => {
                            await api.remoteApproveClient(c.client_id, false);
                            load();
                          }}><UserX className="w-3 h-3" /></button>
                        </div>
                      )}
                    </div>
                  ))}
                </div>
              )}
              {pending && (
                <div className="mt-3 rounded-lg border border-warn bg-warn/10 p-3 text-sm flex items-center justify-between" style={{ borderColor: "var(--warn)", background: "color-mix(in srgb, var(--warn) 10%, transparent)" }}>
                  <span><ShieldAlert className="w-4 h-4 inline mr-1.5 -mt-0.5" />"{pending.name}" wants to pair</span>
                  <div className="flex gap-1.5">
                    <button className="btn-primary !py-1 !px-2 text-xs" onClick={async () => {
                      await api.remoteApproveClient(pending.clientId, true);
                      toast(`Approved "${pending.name}"`, "success");
                      setPending(null);
                      load();
                    }}>Approve</button>
                    <button className="btn-ghost !py-1 !px-2 text-xs" onClick={async () => {
                      await api.remoteApproveClient(pending.clientId, false);
                      setPending(null);
                      load();
                    }}>Deny</button>
                  </div>
                </div>
              )}
            </Card>
          </div>

          <Card>
            <div className="flex items-center gap-2 mb-2">
              <ShieldCheck className="w-4 h-4 text-ok" />
              <span className="section-title !mb-0">How your connection stays safe</span>
            </div>
            <ul className="text-sm text-muted space-y-1.5 list-disc pl-5">
              <li>All traffic runs over TLS 1.2+ with certificate pinning — the relay fingerprint above must match, or connections are refused.</li>
              <li>Device authentication uses HMAC-signed handshakes with a replay window.</li>
              <li>Remote clients are approved manually per device; pairing codes expire in 5 minutes and are rate-limited.</li>
              <li>The one-click kill switch (Disable remote access) tears down the agent instantly and is recorded in the audit log.</li>
              <li>Every pairing, approval and tunnel open/close is audited on both the relay and this device.</li>
            </ul>
          </Card>
        </div>
      )}

      <Modal open={cliHelp} onClose={() => setCliHelp(false)} title="Connect from another machine" wide>
        <ol className="text-sm space-y-3 list-decimal pl-5">
          <li>Install <code className="chip">devvault-cli</code> on the remote machine (build it from this repo: <code className="chip">go build ./cmd/devvault-cli</code>).</li>
          <li>Generate a pairing code here, then run:
            <pre className="mt-1 text-xs font-mono bg-surface2 rounded-lg p-3 overflow-auto">{`devvault-cli pair --relay ${relayAddr || "relay.example.com:7921"} \\\n  --fingerprint ${fingerprint ? fingerprint.slice(0, 16) + "…" : "<relay fingerprint>"} \\\n  --code ${pairCode || "XXXXXXXX"} --name work-laptop`}</pre>
          </li>
          <li>Approve the client in this page when the request appears.</li>
          <li>Open a tunnel and call the agent API:
            <pre className="mt-1 text-xs font-mono bg-surface2 rounded-lg p-3 overflow-auto">{`devvault-cli tunnel --identity ~/.devvault/client.json --device ${status?.deviceId.slice(0, 8) || "<device-id>"}… --port 7922\ncurl -H "X-DevVault-Client: cli" http://127.0.0.1:7922/api/health`}</pre>
          </li>
        </ol>
        <div className="text-xs text-muted mt-3">Copy tip: select the command and press Ctrl+Shift+C in most terminals.</div>
      </Modal>
      <button className="hidden"><Copy className="w-4 h-4" /></button>
    </div>
  );
}
