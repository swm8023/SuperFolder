import { spawn, spawnSync } from 'node:child_process';
import http from 'node:http';
import net from 'node:net';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(scriptDir, '..');
const appDir = path.join(root, 'app');
const serviceDir = path.join(root, 'service');
const serviceUrl = 'http://127.0.0.1:18080';
const viteUrl = 'http://127.0.0.1:5173/';
const servicePort = 18080;
const vitePort = 5173;
const children = [];

function assertPortFree(port, name) {
  return new Promise((resolve, reject) => {
    const server = net.createServer();
    server.once('error', (error) => {
      if (error?.code === 'EADDRINUSE') {
        reject(new Error(`${name} port ${port} is already in use. Stop the existing dev process and run script\\dev.bat again.`));
        return;
      }
      reject(error);
    });
    server.once('listening', () => {
      server.close(() => resolve());
    });
    server.listen(port, '127.0.0.1');
  });
}

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
      killProcessTree(child);
    }
  }
  process.exitCode = code;
}

function killProcessTree(child) {
  if (process.platform === 'win32' && child.pid) {
    spawnSync('taskkill', ['/pid', String(child.pid), '/t', '/f'], { stdio: 'ignore' });
    return;
  }
  child.kill();
}

process.on('SIGINT', () => shutdown(0));
process.on('SIGTERM', () => shutdown(0));

try {
  await assertPortFree(servicePort, 'Go service');
  await assertPortFree(vitePort, 'Vite');

  start('go', ['run', '.', '--headless', '--port', '18080'], { cwd: serviceDir });
  await waitHTTP(`${serviceUrl}/healthz`, 'Go service');

  start('npm', ['run', 'dev'], {
    cwd: appDir,
    env: { ...process.env },
  });
  await waitHTTP(viteUrl, 'Vite');

  spawn('cmd', ['/c', 'start', '', viteUrl], { shell: false, detached: true, stdio: 'ignore' }).unref();
  console.log(`Dev UI: ${viteUrl}`);
} catch (error) {
  console.error(error instanceof Error ? error.message : error);
  shutdown(1);
}
