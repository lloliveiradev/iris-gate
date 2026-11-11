#!/usr/bin/env node
import { readFileSync, writeFileSync } from 'fs';
import { renderMermaid } from '../node_modules/beautiful-mermaid/dist/index.js';

const args = process.argv.slice(2);
const get = (flag) => {
  const i = args.indexOf(flag);
  return i !== -1 ? args[i + 1] : null;
};

const input = get('--input');
const output = get('--output');
const theme = get('--theme') || 'tokyo-night';

if (!input) {
  console.error('Usage: render.mjs --input <file.mmd> --output <file.svg> [--theme <theme>]');
  process.exit(1);
}

const code = readFileSync(input, 'utf-8');
const svg = await renderMermaid(code, { theme });

if (output) {
  writeFileSync(output, svg);
  console.log(`✓ Rendered to ${output}`);
} else {
  process.stdout.write(svg);
}
