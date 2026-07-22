import { spawnSync } from 'node:child_process';
import { resolve } from 'node:path';
import process from 'node:process';

const root = resolve(import.meta.dirname, '..');

function run(command, args, cwd = root) {
  const result = spawnSync(command, args, { cwd, stdio: 'inherit', shell: false });
  if (result.error) throw result.error;
  if (result.status !== 0) process.exit(result.status ?? 1);
}

run(process.execPath, ['scripts/generate-product-config.mjs']);
run(process.execPath, ['scripts/generate-i18n.mjs']);
run('go', ['run', 'github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1', 'generate', '-f', 'sqlc.yaml'], resolve(root, 'apps/api'));
run('go', [
  'run',
  'github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0',
  '--config',
  'openapi/oapi-codegen.yaml',
  'openapi/openapi.yaml',
], resolve(root, 'apps/api'));

run(process.execPath, [
  'node_modules/openapi-typescript/bin/cli.js',
  'apps/api/openapi/openapi.yaml',
  '-o',
  'packages/contracts/src/api.generated.ts',
]);
