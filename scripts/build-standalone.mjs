import fs from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');

function stripModuleSyntax(code) {
  return code
    .replace(/^import\s+[\s\S]*?from\s+['"][^'"]+['"];\r?\n?/gm, '')
    .replace(/^export\s+/gm, '');
}

async function build() {
  const [html, css, engine, quotes, audio, main] = await Promise.all([
    fs.readFile(path.join(root, 'index.html'), 'utf8'),
    fs.readFile(path.join(root, 'styles.css'), 'utf8'),
    fs.readFile(path.join(root, 'src', 'engine.js'), 'utf8'),
    fs.readFile(path.join(root, 'src', 'quotes.js'), 'utf8'),
    fs.readFile(path.join(root, 'src', 'audio.js'), 'utf8'),
    fs.readFile(path.join(root, 'src', 'main.js'), 'utf8'),
  ]);

  const combinedJs = [engine, quotes, audio, main]
    .map(stripModuleSyntax)
    .join('\n\n');

  const standalone = html
    .replace('<link rel="stylesheet" href="./styles.css" />', `<style>\n${css}\n</style>`)
    .replace(
      '<script type="module" src="./src/main.js"></script>',
      `<script>\n(() => {\n${combinedJs}\n})();\n</script>`
    );

  await fs.writeFile(path.join(root, 'play.html'), standalone, 'utf8');
  console.log('Generated standalone play.html');
}

build();
