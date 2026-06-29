/**
 * Runs golden fixture tests for all QuarkGate drivers.
 */
const fs = require('fs');
const path = require('path');
const { spawnSync } = require('child_process');

const driversRoot = path.join(__dirname, '..');
const hostPath = path.join(__dirname, 'host.js');

function listDrivers() {
  return fs.readdirSync(driversRoot, { withFileTypes: true })
    .filter((d) => d.isDirectory() && !d.name.startsWith('_') && d.name !== 'fixtures' && d.name !== 'sdk')
    .map((d) => d.name);
}

function runHost(input) {
  const r = spawnSync('node', [hostPath], {
    input: JSON.stringify(input),
    encoding: 'utf8',
    cwd: path.join(__dirname, '..'),
  });
  if (r.status !== 0) {
    throw new Error(r.stderr || r.stdout || 'host failed');
  }
  return JSON.parse(r.stdout.trim());
}

function deepEqual(a, b) {
  return JSON.stringify(a) === JSON.stringify(b);
}

function runFixture(provider, action) {
  const dir = path.join(driversRoot, 'fixtures', provider);
  const inputPath = path.join(dir, `${action}-input.json`);
  const outputPath = path.join(dir, `${action}-output.json`);
  if (!fs.existsSync(inputPath) || !fs.existsSync(outputPath)) {
    return { skipped: true };
  }
  const input = JSON.parse(fs.readFileSync(inputPath, 'utf8'));
  input.ipc_version = '1';
  input.action = action;
  input.provider = provider;
  const expected = JSON.parse(fs.readFileSync(outputPath, 'utf8'));
  const actual = runHost(input);
  if (!deepEqual(actual, expected)) {
    console.error(`FAIL ${provider} ${action}`);
    console.error('expected:', JSON.stringify(expected));
    console.error('actual:  ', JSON.stringify(actual));
    return { failed: true };
  }
  console.log(`OK ${provider} ${action}`);
  return { passed: true };
}

function main() {
  const drivers = listDrivers();
  let failed = 0;
  let passed = 0;
  for (const provider of drivers) {
    for (const action of ['estimate', 'prepare', 'normalize', 'parseResponse']) {
      const r = runFixture(provider, action);
      if (r.failed) failed++;
      if (r.passed) passed++;
    }
  }
  console.log(`drivers: ${passed} passed, ${failed} failed`);
  if (failed > 0) process.exit(1);
}

main();
