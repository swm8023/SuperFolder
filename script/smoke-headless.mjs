import { spawn } from 'node:child_process';
import http from 'node:http';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(scriptDir, '..');
const exePath = process.argv[2] || path.join(root, 'bin', 'superfolder.exe');
const port = process.argv[3] || '18081';
const bootUrl = `http://127.0.0.1:${port}/boot`;

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

async function waitBoot(process) {
  for (let attempt = 0; attempt < 50; attempt += 1) {
    if (process.exitCode !== null) {
      throw new Error(`smoke process exited with code ${process.exitCode}`);
    }
    try {
      return await getJSON(bootUrl);
    } catch {
      await new Promise((resolve) => setTimeout(resolve, 200));
    }
  }
  throw new Error(`boot endpoint did not become ready: ${bootUrl}`);
}

const child = spawn(exePath, ['--headless', '--port', port], {
  stdio: 'ignore',
  windowsHide: true,
});

try {
  const boot = await waitBoot(child);
  if (boot.app !== 'superfolder') {
    throw new Error(`unexpected boot app: ${boot.app}`);
  }
  if (boot.headless !== true) {
    throw new Error(`unexpected boot headless: ${boot.headless}`);
  }
  if (typeof boot.rpcUrl !== 'string' || !boot.rpcUrl.startsWith('ws://127.0.0.1:') || !boot.rpcUrl.endsWith('/ws')) {
    throw new Error(`unexpected rpcUrl: ${boot.rpcUrl}`);
  }
  console.log(`Smoke headless passed: ${boot.rpcUrl}`);
} catch (error) {
  console.error(error instanceof Error ? error.message : error);
  process.exitCode = 1;
} finally {
  child.kill();
}
