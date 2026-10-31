#!/usr/bin/env node
/**
 * DevVault example plugin — newline-delimited JSON-RPC 2.0 over stdio.
 *
 * Protocol (devvault-plugin-rpc/1):
 *   - Host -> plugin: {"jsonrpc":"2.0","id":"...","method":"initialize"|"ping"|...,"params":{...}}
 *   - Plugin -> host: responses with the same id.
 *   - Plugin -> host: requests with method "host/..." (permission-checked).
 *   - Plugin -> host: notifications {"method":"log","params":"..."}.
 */
const readline = require('readline');

const rl = readline.createInterface({ input: process.stdin, terminal: false });

function send(msg) {
  process.stdout.write(JSON.stringify(msg) + '\n');
}

function reply(id, result) {
  send({ jsonrpc: '2.0', id, result });
}

function replyError(id, code, message) {
  send({ jsonrpc: '2.0', id, error: { code, message } });
}

async function callHost(method, params) {
  return new Promise((resolve, reject) => {
    const id = 'host-req-' + Math.random().toString(36).slice(2);
    const handler = (line) => {
      try {
        const msg = JSON.parse(line);
        if (msg.id === id) {
          rl.removeListener('line', handler);
          if (msg.error) reject(new Error(msg.error.message));
          else resolve(msg.result);
        }
      } catch { /* ignore non-JSON lines */ }
    };
    rl.on('line', handler);
    send({ jsonrpc: '2.0', id, method, params: params || {} });
    setTimeout(() => {
      rl.removeListener('line', handler);
      reject(new Error('host request timed out'));
    }, 10000);
  });
}

rl.on('line', async (line) => {
  let msg;
  try { msg = JSON.parse(line); } catch { return; }
  if (!msg.id) return; // ignore notifications from host

  switch (msg.method) {
    case 'initialize':
      send({ jsonrpc: '2.0', method: 'log', params: 'Hello plugin initialised (protocol ' + (msg.params && msg.params.protocol || '?') + ')' });
      reply(msg.id, { ok: true, name: 'Hello Plugin', version: '1.0.0' });
      break;
    case 'ping':
      reply(msg.id, { message: 'pong from Hello Plugin ' + new Date().toISOString() });
      break;
    case 'greet':
      reply(msg.id, { message: 'Hello, ' + ((msg.params && msg.params.name) || 'developer') + '! Your vault is secure.' });
      break;
    case 'summarize': {
      // Demonstrates a permissioned host call.
      try {
        const projects = await callHost('host/projects/list');
        const tasks = await callHost('host/tasks/count');
        reply(msg.id, { projects: projects.length, openTasks: tasks.open });
      } catch (e) {
        replyError(msg.id, -32000, e.message);
      }
      break;
    }
    default:
      replyError(msg.id, -32601, 'unknown method: ' + msg.method);
  }
});

process.on('SIGTERM', () => process.exit(0));
