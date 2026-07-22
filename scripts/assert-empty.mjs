import process from 'node:process';

let value = '';
process.stdin.setEncoding('utf8');
for await (const chunk of process.stdin) value += chunk;
if (value.trim() !== '') {
  process.stderr.write(value);
  process.stderr.write('\nExpected command output to be empty.\n');
  process.exit(1);
}
