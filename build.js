const esbuild = require('esbuild');
const fs = require('fs');
const path = require('path');

// Build the CLI application
esbuild.build({
  entryPoints: ['src/cli.js'],
  outfile: 'dist/cli.js',
  bundle: true,
  minify: true,
  platform: 'node',
  target: 'node14',
  format: 'cjs',
  sourcemap: false
}).then(() => {
  // Add shebang to the built file
  const distFile = path.join(__dirname, 'dist', 'cli.js');
  const content = fs.readFileSync(distFile, 'utf8');
  const withShebang = `#!/usr/bin/env node\n${content}`;
  fs.writeFileSync(distFile, withShebang);

  // Set executable permissions
  fs.chmodSync(distFile, 0o755);

  console.log('✓ Build complete: dist/cli.js');
  console.log('✓ Added shebang for executable');
  console.log('✓ Set executable permissions');
}).catch((error) => {
  console.error('Build failed:', error);
  process.exit(1);
});
