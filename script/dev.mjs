import { spawn } from 'node:child_process';
import http from 'node:http';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(scriptDir, '..');
const appDir = path.join(root, 'app');
const serviceDir = path.join(root, 'service');
const serviceUrl = 'http://127.0.0.1:18080';
const viteUrl = 'http://127.0.0.1:5173/';
const children = [];

function waitHTTP(url, name) {
  return new Promise((resolve, reject) => {
    let attempts = 0;
    const tick = () => {
      attempts += 1;
      const req = http.get(url, (res) => {
        res.resume();
        if (res.statusCode >= 200 && res.statusCode < 500) {
          resolve();
        } else {
          retry();
        }
      });
      req.setTimeout(1000, () => {
        req.destroy();
        retry();
      });
      req.on('error', retry);
    };
    const retry = () => {
      if (attempts >= 50) {
        reject(new Error(`${name} did not become ready: ${url}`));
        return;
      }
      setTimeout(tick, 200);
    };
    tick();
  });
}

function start(command, args, options) {
  const child = spawn(command, args, {
    ...options,
    shell: true,
    stdio: 'inherit',
  });
  children.push(child);
  child.on('exit', (code) => {
    if (!shuttingDown && code !== null && code !== 0) {
      shutdown(code);
    }
  });
  return child;
}

let shuttingDown = false;
function shutdown(code = 0) {
  if (shuttingDown) {
    return;
  }
  shuttingDown = true;
  for (const child of children) {
    if (!child.killed) {
      child.kill();
    }
  }
  process.exitCode = code;
}

process.on('SIGINT', () => shutdown(0));
process.on('SIGTERM', () => shutdown(0));

try {
  start('go', ['run', '.', '--headless', '--port', '18080'], { cwd: serviceDir });
  await waitHTTP(`${serviceUrl}/healthz`, 'Go service');

  start('npm', ['run', 'dev'], {
    cwd: appDir,
    env: { ...process.env, VITE_SERVICE_URL: serviceUrl },
  });
  await waitHTTP(viteUrl, 'Vite');

  spawn('cmd', ['/c', 'start', '', viteUrl], { shell: false, detached: true, stdio: 'ignore' }).unref();
  console.log(`Dev UI: ${viteUrl}`);
} catch (error) {
  console.error(error instanceof Error ? error.message : error);
  shutdown(1);
}
