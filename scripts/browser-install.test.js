'use strict';

const crypto = require('node:crypto');
const fs = require('node:fs');
const http = require('node:http');
const os = require('node:os');
const path = require('node:path');
const assert = require('node:assert/strict');
const { execFileSync } = require('node:child_process');
const installer = require('./browser-install');

async function main() {
  const originalCacheDir = process.env.AGET_BROWSER_CACHE_DIR;
  try {
    testCacheRootParity();
    testPathsFor();
    testPathsForCloakBrowser();
    await testInstallRejectsChecksumMismatch();
    await testInstallRejectsUnsafeArchiveName();
    await testInstallPreservesExistingInstallWhenStagedValidationFails();
    await testInstallExtractsTarGz();
  } finally {
    if (originalCacheDir === undefined) {
      delete process.env.AGET_BROWSER_CACHE_DIR;
    } else {
      process.env.AGET_BROWSER_CACHE_DIR = originalCacheDir;
    }
  }
}

function testCacheRootParity() {
  assert.equal(
    installer.cacheRoot({
      platform: 'linux',
      env: { AGET_BROWSER_CACHE_DIR: '/override', XDG_CACHE_HOME: '/xdg-cache' },
      homedir: '/home/user',
    }),
    '/override',
  );
  assert.equal(
    installer.cacheRoot({
      platform: 'linux',
      env: { XDG_CACHE_HOME: '/xdg-cache' },
      homedir: '/home/user',
    }),
    '/xdg-cache',
  );
  assert.equal(
    installer.cacheRoot({ platform: 'linux', env: {}, homedir: '/home/user' }),
    path.join('/home/user', '.cache'),
  );
  assert.equal(
    installer.cacheRoot({ platform: 'darwin', env: {}, homedir: '/Users/user' }),
    path.join('/Users/user', 'Library', 'Caches'),
  );
  assert.equal(
    installer.cacheRoot({
      platform: 'win32',
      env: { LOCALAPPDATA: 'C:\\Users\\user\\AppData\\Local' },
      homedir: 'C:\\Users\\user',
    }),
    'C:\\Users\\user\\AppData\\Local',
  );
  assert.equal(
    installer.cacheRoot({ platform: 'win32', env: {}, homedir: 'C:\\Users\\user' }),
    path.win32.join('C:\\Users\\user', 'AppData', 'Local'),
  );
}

function testPathsFor() {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'aget-browser-test-'));
  process.env.AGET_BROWSER_CACHE_DIR = root;

  try {
    const manifest = baseManifest();

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

    assert.throws(() => installer.pathsFor(manifest, 'plan9-amd64'), /unsupported managed browser platform/);

    const unsafeCases = [
      { version: '../148.0.7778.98' },
      { platform: '../linux-x64' },
      { executable_path: '/tmp/chrome' },
      { executable_path: '../chrome' },
      { executable_path: 'chrome-linux64/../chrome' },
      { executable_path: 'chrome-linux64//chrome' },
      { executable_path: 'chrome-linux64\\chrome' },
    ];
    for (const unsafe of unsafeCases) {
      const unsafeManifest = baseManifest(unsafe);
      assert.throws(() => installer.pathsFor(unsafeManifest, unsafe.platform || 'linux-x64'), /browser .*path|unsafe|relative/);
    }
  } finally {
    fs.rmSync(root, { recursive: true, force: true });
    delete process.env.AGET_BROWSER_CACHE_DIR;
  }
}

function testPathsForCloakBrowser() {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'aget-browser-test-'));
  process.env.AGET_BROWSER_CACHE_DIR = root;

  try {
    const manifest = baseManifest({
      browser: 'cloakbrowser',
      version: '146.0.7680.177.4',
      platform: 'darwin-arm64',
      platformVersion: '145.0.7632.109.2',
      archive: 'cloakbrowser-darwin-arm64.tar.gz',
      executable_path: 'Chromium.app/Contents/MacOS/Chromium',
    });

    const info = installer.pathsFor(manifest, 'darwin-arm64');
    assert.equal(
      info.installDir,
      path.join(root, 'agent-aget', 'cloakbrowser', '145.0.7632.109.2', 'darwin-arm64'),
    );
    assert.equal(
      info.executable,
      path.join(info.installDir, 'Chromium.app', 'Contents', 'MacOS', 'Chromium'),
    );
  } finally {
    fs.rmSync(root, { recursive: true, force: true });
    delete process.env.AGET_BROWSER_CACHE_DIR;
  }
}

async function testInstallRejectsChecksumMismatch() {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'aget-browser-test-'));
  process.env.AGET_BROWSER_CACHE_DIR = root;
  const archivePath = makeZip([{ name: 'chrome-linux64/chrome', body: '#!/bin/sh\n', mode: 0o755 }]);
  const server = await serveFile(archivePath);

  try {
    const manifest = baseManifest({
      url: server.url,
      sha256: '0'.repeat(64),
    });

    await assert.rejects(
      () => installer.installFromManifest(manifest, 'linux-x64'),
      /checksum|sha256/i,
    );
  } finally {
    await server.close();
    fs.rmSync(root, { recursive: true, force: true });
    delete process.env.AGET_BROWSER_CACHE_DIR;
  }
}

async function testInstallRejectsUnsafeArchiveName() {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'aget-browser-test-'));
  process.env.AGET_BROWSER_CACHE_DIR = root;

  try {
    for (const archive of ['../chrome.zip', '/tmp/chrome.zip', 'nested/chrome.zip', 'nested\\chrome.zip']) {
      await assert.rejects(
        () => installer.installFromManifest(baseManifest({ archive }), 'linux-x64'),
        /archive name must be a safe basename/,
      );
    }
  } finally {
    fs.rmSync(root, { recursive: true, force: true });
    delete process.env.AGET_BROWSER_CACHE_DIR;
  }
}

async function testInstallPreservesExistingInstallWhenStagedValidationFails() {
  if (process.platform === 'win32') {
    return;
  }

  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'aget-browser-test-'));
  process.env.AGET_BROWSER_CACHE_DIR = root;
  const existingManifest = baseManifest();
  const info = installer.pathsFor(existingManifest, 'linux-x64');
  fs.mkdirSync(path.dirname(info.executable), { recursive: true });
  fs.writeFileSync(info.executable, 'old browser\n', { mode: 0o644 });
  const archivePath = makeZip([{ name: 'chrome-linux64/not-chrome', body: 'new browser\n', mode: 0o755 }]);
  const server = await serveFile(archivePath);

  try {
    const manifest = baseManifest({
      url: server.url,
      sha256: sha256(archivePath),
    });

    await assert.rejects(
      () => installer.installFromManifest(manifest, 'linux-x64'),
      /executable validation failed|executable not found/,
    );
    assert.equal(fs.readFileSync(info.executable, 'utf8'), 'old browser\n');
  } finally {
    await server.close();
    fs.rmSync(root, { recursive: true, force: true });
    delete process.env.AGET_BROWSER_CACHE_DIR;
  }
}

async function testInstallExtractsTarGz() {
  if (process.platform === 'win32') {
    return;
  }

  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'aget-browser-test-'));
  process.env.AGET_BROWSER_CACHE_DIR = root;
  const archivePath = makeTarGz([{ name: 'chrome', body: '#!/bin/sh\n', mode: 0o755 }]);
  const server = await serveFile(archivePath);

  try {
    const manifest = baseManifest({
      browser: 'cloakbrowser',
      version: '146.0.7680.177.4',
      archive: 'cloakbrowser-linux-x64.tar.gz',
      url: server.url,
      sha256: sha256(archivePath),
      executable_path: 'chrome',
    });

    const info = await installer.installFromManifest(manifest, 'linux-x64');
    assert.equal(info.executable, path.join(root, 'agent-aget', 'cloakbrowser', '146.0.7680.177.4', 'linux-x64', 'chrome'));
    assert.equal(installer.isExecutable(info.executable), true);
  } finally {
    await server.close();
    fs.rmSync(root, { recursive: true, force: true });
    delete process.env.AGET_BROWSER_CACHE_DIR;
  }
}

function baseManifest(overrides = {}) {
  const platform = overrides.platform || 'linux-x64';
  const entry = {
    archive: 'chrome-linux64.zip',
    url: 'https://example.invalid/chrome.zip',
    sha256: 'abc',
    executable_path: 'chrome-linux64/chrome',
  };
  if (Object.hasOwn(overrides, 'archive')) entry.archive = overrides.archive;
  if (Object.hasOwn(overrides, 'url')) entry.url = overrides.url;
  if (Object.hasOwn(overrides, 'sha256')) entry.sha256 = overrides.sha256;
  if (Object.hasOwn(overrides, 'executable_path')) entry.executable_path = overrides.executable_path;

  const manifest = {
    browser: overrides.browser,
    version: overrides.version || '148.0.7778.98',
    platforms: {
      [platform]: entry,
    },
  };
  if (overrides.platformVersion) {
    manifest.platforms[platform].version = overrides.platformVersion;
  }
  return manifest;
}

function makeZip(entries) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'aget-browser-zip-'));
  const archivePath = path.join(root, 'archive.zip');
  for (const entry of entries) {
    const filePath = path.join(root, entry.name);
    fs.mkdirSync(path.dirname(filePath), { recursive: true });
    fs.writeFileSync(filePath, entry.body, { mode: entry.mode });
    fs.chmodSync(filePath, entry.mode);
  }
  execFileSync('zip', ['-qr', archivePath, ...entries.map((entry) => entry.name)], { cwd: root });
  return archivePath;
}

function makeTarGz(entries) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'aget-browser-tar-'));
  const archivePath = path.join(root, 'archive.tar.gz');
  for (const entry of entries) {
    const filePath = path.join(root, entry.name);
    fs.mkdirSync(path.dirname(filePath), { recursive: true });
    fs.writeFileSync(filePath, entry.body, { mode: entry.mode });
    fs.chmodSync(filePath, entry.mode);
  }
  execFileSync('tar', ['-czf', archivePath, ...entries.map((entry) => entry.name)], { cwd: root });
  return archivePath;
}

function sha256(filePath) {
  const hash = crypto.createHash('sha256');
  hash.update(fs.readFileSync(filePath));
  return hash.digest('hex');
}

function serveFile(filePath) {
  const server = http.createServer((_, response) => {
    fs.createReadStream(filePath).pipe(response);
  });

  return new Promise((resolve, reject) => {
    server.on('error', reject);
    server.listen(0, '127.0.0.1', () => {
      resolve({
        url: `http://127.0.0.1:${server.address().port}/archive.zip`,
        close: () => new Promise((closeResolve, closeReject) => {
          server.close((error) => (error ? closeReject(error) : closeResolve()));
        }),
      });
    });
  });
}

if (require.main === module) {
  main().catch((error) => {
    console.error(error);
    process.exit(1);
  });
}
