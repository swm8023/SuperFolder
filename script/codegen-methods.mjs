import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(scriptDir, '..');
const registryPath = path.join(root, 'app', 'src', 'rpc', 'methods.json');
const serviceDir = path.join(root, 'service');
const goOutput = path.join(root, 'service', 'backend', 'methods_gen.go');
const tsOutput = path.join(root, 'app', 'src', 'rpc', 'methods_gen.ts');

function goIdent(name) {
  return name.slice(0, 1).toUpperCase() + name.slice(1);
}

function assertMethodName(name) {
  if (!/^[a-z][a-z0-9]*(\.[a-z][a-z0-9]*)+$/.test(name)) {
    throw new Error(`Invalid RPC method name: ${name}`);
  }
}

function createTreeNode() {
  return { children: new Map(), method: null };
}

function addMethodToTree(root, name, id) {
  const parts = name.split('.');
  let node = root;
  for (const part of parts) {
    if (!node.children.has(part)) {
      node.children.set(part, createTreeNode());
    }
    node = node.children.get(part);
  }
  node.method = { name, id };
}

function sortedChildren(node) {
  return [...node.children.entries()].sort(([a], [b]) => a.localeCompare(b));
}

function isLeaf(node) {
  return node.method !== null && node.children.size === 0;
}

function goTypeLines(node, indent) {
  const lines = [];
  for (const [part, child] of sortedChildren(node)) {
    const field = goIdent(part);
    if (isLeaf(child)) {
      lines.push(`${indent}${field} Method`);
      continue;
    }
    lines.push(`${indent}${field} struct {`);
    lines.push(...goTypeLines(child, `${indent}\t`));
    lines.push(`${indent}}`);
  }
  return lines;
}

function goValueLines(node, indent) {
  const lines = [];
  for (const [part, child] of sortedChildren(node)) {
    const field = goIdent(part);
    if (isLeaf(child)) {
      lines.push(`${indent}${field}: ${child.method.id},`);
      continue;
    }
    lines.push(`${indent}${field}: struct {`);
    lines.push(...goTypeLines(child, `${indent}\t`));
    lines.push(`${indent}}{`);
    lines.push(...goValueLines(child, `${indent}\t`));
    lines.push(`${indent}},`);
  }
  return lines;
}

function tsObjectLines(node, indent) {
  const lines = [];
  for (const [part, child] of sortedChildren(node)) {
    if (isLeaf(child)) {
      lines.push(`${indent}${part}: ${child.method.id},`);
      continue;
    }
    lines.push(`${indent}${part}: {`);
    lines.push(...tsObjectLines(child, `${indent}  `));
    lines.push(`${indent}},`);
  }
  return lines;
}

function rangeKey(name) {
  return name.startsWith('app.') ? 'app' : 'default';
}

function getRange(registry, key) {
  const range = registry.ranges?.[key];
  if (!range) {
    throw new Error(`Missing RPC method range: ${key}`);
  }
  return range;
}

function main() {
  if (!fs.existsSync(registryPath)) {
    throw new Error(`RPC method registry not found: ${registryPath}`);
  }

  const registry = JSON.parse(fs.readFileSync(registryPath, 'utf8'));
  if (!registry.ranges) {
    throw new Error('RPC method registry is missing ranges.');
  }
  if (!registry.methods) {
    throw new Error('RPC method registry is missing methods.');
  }

  const methods = new Map();
  const ids = new Map();
  const pending = [];
  const entries = Object.entries(registry.methods).sort(([a], [b]) => a.localeCompare(b));
  if (entries.length === 0) {
    throw new Error('No RPC methods declared in app/src/rpc/methods.json.');
  }

  for (const [name, rawID] of entries) {
    assertMethodName(name);
    const id = Number(rawID);
    if (!Number.isInteger(id) || id < 0) {
      throw new Error(`Invalid RPC method id for ${name}: ${rawID}`);
    }
    if (id === 0) {
      pending.push(name);
      continue;
    }

    const key = rangeKey(name);
    const range = getRange(registry, key);
    if (id < Number(range.start) || id > Number(range.end)) {
      throw new Error(`RPC method id ${id} for ${name} is outside range ${key} [${range.start}, ${range.end}]`);
    }
    if (ids.has(id)) {
      throw new Error(`Duplicate RPC method id ${id} for ${name} and ${ids.get(id)}`);
    }

    methods.set(name, id);
    ids.set(id, name);
  }

  const nextByRange = new Map();
  for (const [key, range] of Object.entries(registry.ranges)) {
    const start = Number(range.start);
    const end = Number(range.end);
    let next = Number(range.next);
    if (start <= 0 || end < start) {
      throw new Error(`Invalid RPC method range: ${key}`);
    }
    if (next < start) {
      next = start;
    }
    nextByRange.set(key, next);
  }

  for (const [name, id] of methods) {
    const key = rangeKey(name);
    const next = nextByRange.get(key);
    if (id >= next) {
      nextByRange.set(key, id + 1);
    }
  }

  for (const name of pending.sort()) {
    const key = rangeKey(name);
    const range = getRange(registry, key);
    let next = nextByRange.get(key);
    while (ids.has(next)) {
      next += 1;
    }
    if (next > Number(range.end)) {
      throw new Error(`RPC method range exhausted: ${key}`);
    }
    methods.set(name, next);
    ids.set(next, name);
    nextByRange.set(key, next + 1);
  }

  for (const [key, next] of nextByRange) {
    registry.ranges[key].next = next;
  }

  const orderedMethods = {};
  for (const [name, id] of [...methods.entries()].sort(([a], [b]) => a.localeCompare(b))) {
    orderedMethods[name] = id;
  }

  fs.writeFileSync(
    registryPath,
    `${JSON.stringify({ ranges: registry.ranges, methods: orderedMethods }, null, 2)}\n`,
    'utf8',
  );

  const sortedMethods = [...methods.entries()].sort((a, b) => a[1] - b[1] || a[0].localeCompare(b[0]));
  const methodTree = createTreeNode();
  for (const [name, id] of sortedMethods) {
    addMethodToTree(methodTree, name, id);
  }

  const goLines = [
    '// Code generated by script/codegen-methods.bat; DO NOT EDIT.',
    '',
    'package backend',
    '',
    'import "strconv"',
    '',
    'type Method int',
    '',
  ];
  for (const [namespace, node] of sortedChildren(methodTree)) {
    goLines.push(`var ${goIdent(namespace)} = struct {`);
    goLines.push(...goTypeLines(node, '\t'));
    goLines.push('}{');
    goLines.push(...goValueLines(node, '\t'));
    goLines.push('}', '');
  }
  goLines.push('func MethodName(method Method) string {', '\tswitch method {');
  for (const [name] of sortedMethods) {
    const goPath = name.split('.').map(goIdent).join('.');
    goLines.push(`\tcase ${goPath}:`, `\t\treturn "${name}"`);
  }
  goLines.push('\tdefault:', '\t\treturn strconv.Itoa(int(method))', '\t}', '}', '');
  fs.writeFileSync(goOutput, goLines.join('\n'), 'utf8');

  const tsLines = ['// Code generated by script/codegen-methods.bat; DO NOT EDIT.', '', 'export const rpc = {'];
  tsLines.push(...tsObjectLines(methodTree, '  '));
  tsLines.push('} as const;', '', 'const METHOD_NAMES = new Map<number, string>([');
  for (const [name] of sortedMethods) {
    tsLines.push(`  [rpc.${name}, '${name}'],`);
  }
  tsLines.push(']);', '', 'export function methodName(method: number): string {', '  return METHOD_NAMES.get(method) ?? String(method);', '}', '');
  fs.writeFileSync(tsOutput, tsLines.join('\n'), 'utf8');

  execFileSync('gofmt', ['-w', path.join('backend', 'methods_gen.go')], {
    cwd: serviceDir,
    stdio: 'inherit',
  });

  console.log(`Generated RPC methods: ${sortedMethods.length}`);
}

try {
  main();
} catch (error) {
  console.error(error instanceof Error ? error.message : error);
  process.exit(1);
}
