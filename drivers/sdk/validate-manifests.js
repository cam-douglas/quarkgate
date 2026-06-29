/**
 * Validates driver manifests against schemas/driver-manifest.v1.json
 */
const fs = require('fs');
const path = require('path');

const driversRoot = path.join(__dirname, '..');
const schemaPath = path.join(__dirname, '..', '..', 'schemas', 'driver-manifest.v1.json');

function listDriverDirs() {
  return fs.readdirSync(driversRoot, { withFileTypes: true })
    .filter((d) => d.isDirectory() && !d.name.startsWith('_') && d.name !== 'fixtures' && d.name !== 'sdk')
    .map((d) => d.name);
}

function validateManifest(folder, manifest, schema) {
  const errors = [];
  if (!manifest.id) errors.push('missing id');
  if (manifest.id && manifest.id !== folder) errors.push(`id ${manifest.id} != folder ${folder}`);
  if (!manifest.version) errors.push('missing version');
  if (!manifest.category) errors.push('missing category');
  if (!Array.isArray(manifest.operations) || manifest.operations.length === 0) {
    errors.push('operations required');
  }
  const categories = schema.properties.category.enum;
  if (manifest.category && !categories.includes(manifest.category)) {
    errors.push(`invalid category ${manifest.category}`);
  }
  for (const op of manifest.operations || []) {
    if (!op.id || !op.method || !op.path) errors.push(`invalid operation in ${folder}`);
  }
  return errors;
}

function main() {
  const schema = JSON.parse(fs.readFileSync(schemaPath, 'utf8'));
  let failed = 0;
  for (const name of listDriverDirs()) {
    const manifestPath = path.join(driversRoot, name, 'manifest.json');
    const manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf8'));
    const errors = validateManifest(name, manifest, schema);
    if (errors.length) {
      console.error(`FAIL ${name}:`, errors.join(', '));
      failed++;
    } else {
      console.log(`OK ${name}`);
    }
  }
  if (failed) process.exit(1);
}

main();
