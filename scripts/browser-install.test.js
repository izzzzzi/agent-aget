'use strict';

const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const assert = require('node:assert/strict');
const installer = require('./browser-install');

function main() {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'aget-browser-test-'));
  process.env.AGET_BROWSER_CACHE_DIR = root;

  const manifest = {
    version: '148.0.7778.98',
    platforms: {
      'linux-x64': {
        archive: 'chrome-linux64.zip',
        url: 'https://example.invalid/chrome.zip',
        sha256: 'abc',
        executable_path: 'chrome-linux64/chrome',
      },
    },
  };

  const info = installer.pathsFor(manifest, 'linux-x64');
  assert.equal(info.cacheDir, root);
  assert.equal(
    info.installDir,
    path.join(root, 'agent-aget', 'chrome-for-testing', '148.0.7778.98', 'linux-x64'),
  );
  assert.equal(info.executable, path.join(info.installDir, 'chrome-linux64', 'chrome'));

  assert.equal(installer.isExecutable(info.executable), false);
  fs.mkdirSync(path.dirname(info.executable), { recursive: true });
  fs.writeFileSync(info.executable, '#!/bin/sh\n', { mode: 0o755 });
  assert.equal(installer.isExecutable(info.executable), true);

  fs.rmSync(root, { recursive: true, force: true });
  delete process.env.AGET_BROWSER_CACHE_DIR;
}

if (require.main === module) {
  main();
}
