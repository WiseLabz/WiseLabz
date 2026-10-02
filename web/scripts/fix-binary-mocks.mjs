// Orval 8.37 emits `{}` for top-level binary mock responses even with data
// overrides. Keep regenerated download mocks valid until upstream fixes it.
import { readdir, readFile, writeFile } from 'node:fs/promises';
import { spawnSync } from 'node:child_process';
for (const file of await readdir('src/api/generated', { recursive: true })) {
  if (!file.endsWith('.faker.ts')) continue;
  const path = `src/api/generated/${file}`;
  const source = await readFile(path, 'utf8');
  const fixed = source.replace(
    /(export const \w+\s*=\s*\(\)\s*:\s*Blob\s*=>)[^\n]*/g,
    '$1 new Blob();'
  );
  if (source !== fixed) await writeFile(path, fixed);
}

const result = spawnSync('prettier', ['--write', 'src/api/generated', 'src/api/model'], {
  stdio: 'inherit',
});
if (result.error) throw result.error;
process.exitCode = result.status ?? 1;
