import { spawnSync } from 'node:child_process';
import process from 'node:process';

const result = spawnSync(process.execPath, ['scripts/generate-i18n.mjs', '--check'], {
  cwd: new URL('..', import.meta.url),
  stdio: 'inherit',
});
process.exit(result.status ?? 1);

