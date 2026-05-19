'use strict';

const crypto = require('node:crypto');
const fs = require('node:fs');
const https = require('node:https');
const os = require('node:os');
const path = require('node:path');
const { execFileSync } = require('node:child_process');

const root = path.join(__dirname, '..');

function cacheRoot() {
  if (process.env.AGET_BROWSER_CACHE_DIR) return process.env.AGET_BROWSER_CACHE_DIR;
  return path.join(
    os.homedir(),
    process.platform === 'darwin'
      ? 'Library/Caches'
      : process.platform === 'win32'
        ? 'AppData/Local'
        : '.cache',
  );
}

function platformKey(platform = process.platform, arch = process.arch) {
  if (platform === 'darwin' && arch === 'arm64') return 'darwin-arm64';
  if (platform === 'darwin' && arch === 'x64') return 'darwin-x64';
  if (platform === 'linux' && arch === 'x64') return 'linux-x64';
  if (platform === 'linux' && arch === 'arm64') return 'linux-arm64';
  if (platform === 'win32' && arch === 'x64') return 'win32-x64';
  return `${platform}-${arch}`;
}

function pathsFor(manifest, key = platformKey()) {
  const entry = manifest.platforms[key];
  if (!entry) throw new Error(`unsupported managed browser platform: ${key}`);
  const cacheDir = cacheRoot();
  const installDir = path.join(
    cacheDir,
    'agent-aget',
    'chrome-for-testing',
    manifest.version,
    key,
  );

  return {
    entry,
    cacheDir,
    installDir,
    executable: path.join(installDir, ...entry.executable_path.split('/')),
  };
}

function isExecutable(file) {
  try {
    const stat = fs.statSync(file);
    if (!stat.isFile()) return false;
    if (process.platform === 'win32') return true;
    return (stat.mode & 0o111) !== 0;
  } catch (_) {
    return false;
  }
}

function download(url, destination) {
  return new Promise((resolve, reject) => {
    const request = https.get(url, (response) => {
      if ([301, 302, 303, 307, 308].includes(response.statusCode)) {
        response.resume();
        download(response.headers.location, destination).then(resolve, reject);
        return;
      }

      if (response.statusCode !== 200) {
        response.resume();
        reject(new Error(`download failed (${response.statusCode}): ${url}`));
        return;
      }

      const file = fs.createWriteStream(destination);
      response.pipe(file);
      file.on('finish', () => file.close(resolve));
      file.on('error', reject);
    });

    request.on('error', reject);
  });
}

function sha256(filePath) {
  const hash = crypto.createHash('sha256');
  hash.update(fs.readFileSync(filePath));
  return hash.digest('hex');
}

function verifyChecksum(filePath, expected) {
  const actual = sha256(filePath);
  if (actual !== expected) {
    throw new Error(`checksum mismatch: expected ${expected}, got ${actual}`);
  }
}

function powershellCommand() {
  for (const candidate of ['powershell.exe', 'powershell', 'pwsh']) {
    try {
      execFileSync(candidate, ['-NoProfile', '-Command', '$PSVersionTable.PSVersion.ToString()'], {
        stdio: 'ignore',
      });
      return candidate;
    } catch (_) {
      // Try the next PowerShell executable name.
    }
  }

  throw new Error('PowerShell is required to extract zip archives');
}

function extractZip(archivePath, destination) {
  fs.mkdirSync(destination, { recursive: true });

  if (process.platform === 'win32') {
    const ps = powershellCommand();
    execFileSync(ps, [
      '-NoProfile',
      '-Command',
      'Expand-Archive -LiteralPath $args[0] -DestinationPath $args[1] -Force',
      archivePath,
      destination,
    ], { stdio: 'inherit' });
    return;
  }

  execFileSync('unzip', ['-q', archivePath, '-d', destination], { stdio: 'inherit' });
}

async function installFromManifest(manifest = null, key = platformKey()) {
  const loadedManifest = manifest || JSON.parse(
    fs.readFileSync(path.join(root, 'browser-manifest.json'), 'utf8'),
  );
  const info = pathsFor(loadedManifest, key);

  if (isExecutable(info.executable)) {
    return info;
  }

  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'agent-aget-browser-'));
  const archivePath = path.join(tmp, info.entry.archive);
  const extractDir = path.join(tmp, 'extract');

  try {
    await download(info.entry.url, archivePath);
    verifyChecksum(archivePath, info.entry.sha256);
    extractZip(archivePath, extractDir);

    fs.rmSync(info.installDir, { recursive: true, force: true });
    fs.mkdirSync(path.dirname(info.installDir), { recursive: true });
    fs.renameSync(extractDir, info.installDir);

    if (!isExecutable(info.executable)) {
      throw new Error(`managed browser executable not found: ${info.executable}`);
    }

    return info;
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
}

if (require.main === module) {
  installFromManifest().catch((error) => {
    console.error(error.message);
    process.exit(1);
  });
}

module.exports = { cacheRoot, platformKey, pathsFor, isExecutable, installFromManifest };
