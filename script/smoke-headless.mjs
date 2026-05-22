import { spawn } from 'node:child_process';
import fs from 'node:fs';
import http from 'node:http';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(scriptDir, '..');
const exePath = process.argv[2] || path.join(root, 'bin', 'superfolder.exe');
const port = process.argv[3] || '18081';
const webUrl = `http://127.0.0.1:${port}/`;
const healthUrl = `http://127.0.0.1:${port}/healthz`;
const rpcUrl = `ws://127.0.0.1:${port}/ws`;
const methodsPath = path.join(root, 'app', 'src', 'rpc', 'methods.json');
const appHelloMethod = JSON.parse(fs.readFileSync(methodsPath, 'utf8')).methods['app.hello'];

function getJSON(url) {
  return new Promise((resolve, reject) => {
    const req = http.get(url, (res) => {
      let data = '';
      res.setEncoding('utf8');
      res.on('data', (chunk) => {
        data += chunk;
      });
      res.on('end', () => {
        if (res.statusCode < 200 || res.statusCode >= 300) {
          reject(new Error(`HTTP ${res.statusCode}: ${url}`));
          return;
        }
        try {
          resolve(JSON.parse(data));
        } catch (error) {
          reject(error);
        }
      });
    });
    req.setTimeout(1000, () => {
      req.destroy(new Error(`timeout: ${url}`));
    });
    req.on('error', reject);
  });
}

function getText(url) {
  return new Promise((resolve, reject) => {
    const req = http.get(url, (res) => {
      let data = '';
      res.setEncoding('utf8');
      res.on('data', (chunk) => {
        data += chunk;
      });
      res.on('end', () => {
        if (res.statusCode < 200 || res.statusCode >= 300) {
          reject(new Error(`HTTP ${res.statusCode}: ${url}`));
          return;
        }
        resolve(data);
      });
    });
    req.setTimeout(1000, () => {
      req.destroy(new Error(`timeout: ${url}`));
    });
    req.on('error', reject);
  });
}

async function waitHealth(process) {
  for (let attempt = 0; attempt < 50; attempt += 1) {
    if (process.exitCode !== null) {
      throw new Error(`smoke process exited with code ${process.exitCode}`);
    }
    try {
      return await getJSON(healthUrl);
    } catch {
      await new Promise((resolve) => setTimeout(resolve, 200));
    }
  }
  throw new Error(`health endpoint did not become ready: ${healthUrl}`);
}

function callHello() {
  return new Promise((resolve, reject) => {
    const socket = new WebSocket(rpcUrl);
    const timer = setTimeout(() => {
      socket.close();
      reject(new Error(`rpc app.hello timed out: ${rpcUrl}`));
    }, 3000);

    socket.addEventListener('open', () => {
      socket.send(JSON.stringify({ id: 1, method: appHelloMethod, payload: {} }));
    });

    socket.addEventListener('message', (event) => {
      clearTimeout(timer);
      socket.close();
      try {
        resolve(JSON.parse(event.data));
      } catch (error) {
        reject(error);
      }
    });

    socket.addEventListener('error', () => {
      clearTimeout(timer);
      reject(new Error(`rpc websocket failed: ${rpcUrl}`));
    });
  });
}

const child = spawn(exePath, ['--headless', '--port', port], {
  stdio: 'ignore',
  windowsHide: true,
});

try {
  const health = await waitHealth(child);
  if (health.app !== 'superfolder' || health.ok !== true) {
    throw new Error(`unexpected health payload: ${JSON.stringify(health)}`);
  }
  const html = await getText(webUrl);
  if (!html.includes('<title>SuperFolder</title>')) {
    throw new Error(`unexpected web html from ${webUrl}`);
  }
  const hello = await callHello();
  if (hello.error) {
    throw new Error(`app.hello error: ${JSON.stringify(hello.error)}`);
  }
  if (hello.id !== 1 || hello.payload?.app !== 'superfolder' || hello.payload?.headless !== true) {
    throw new Error(`unexpected app.hello payload: ${JSON.stringify(hello)}`);
  }
  console.log(`Smoke headless passed: ${rpcUrl}`);
} catch (error) {
  console.error(error instanceof Error ? error.message : error);
  process.exitCode = 1;
} finally {
  child.kill();
}
