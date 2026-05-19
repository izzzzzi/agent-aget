'use strict';

const crypto = require('node:crypto');
const fs = require('node:fs');
const http = require('node:http');
const https = require('node:https');
const os = require('node:os');
const path = require('node:path');
const { execFileSync } = require('node:child_process');

const root = path.join(__dirname, '..');

function cacheRoot(options = {}) {
  const platform = options.platform || process.platform;
  const env = options.env || process.env;
  const homedir = options.homedir || os.homedir();

  if (env.AGET_BROWSER_CACHE_DIR) return env.AGET_BROWSER_CACHE_DIR;
  if (platform === 'linux' && env.XDG_CACHE_HOME) return env.XDG_CACHE_HOME;
  if (platform === 'win32' && env.LOCALAPPDATA) return env.LOCALAPPDATA;
  if (platform === 'darwin') return path.join(homedir, 'Library', 'Caches');
  if (platform === 'win32') return path.win32.join(homedir, 'AppData', 'Local');
  return path.join(homedir, '.cache');
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
  if (!safePathName(manifest.version)) {
    throw new Error('browser install path version must be a relative path name');
  }
  if (!safePathName(key)) {
    throw new Error('browser install path platform must be a relative path name');
  }
  const executablePath = safeExecutablePath(entry.executable_path);
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
    executable: path.join(installDir, executablePath),
  };
}

function safePathName(name) {
  return typeof name === 'string' &&
    name !== '' &&
    name !== '.' &&
    name !== '..' &&
    !name.includes('/') &&
    !name.includes('\\');
}

function safeArchiveName(name) {
  if (!safePathName(name) || path.isAbsolute(name) || path.win32.isAbsolute(name)) {
    throw new Error('browser archive name must be a safe basename');
  }
  return name;
}

function safeExecutablePath(name) {
  if (typeof name !== 'string' || name === '') {
    throw new Error('browser executable path must be relative');
  }
  if (path.isAbsolute(name) || path.win32.isAbsolute(name)) {
    throw new Error('browser executable path must be relative');
  }
  for (const segment of name.split('/')) {
    if (segment === '' || segment === '.' || segment === '..' || segment.includes('\\')) {
      throw new Error('browser executable path contains unsafe path segment');
    }
  }
  return path.join(...name.split('/'));
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
    const client = url.startsWith('http://') ? http : https;
    const request = client.get(url, (response) => {
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

function stagedExecutablePath(info, stagedDir) {
  const relative = path.relative(info.installDir, info.executable);
  if (
    relative === '' ||
    relative === '..' ||
    relative.startsWith(`..${path.sep}`) ||
    path.isAbsolute(relative)
  ) {
    throw new Error('browser executable path must stay within install dir');
  }
  return path.join(stagedDir, relative);
}

function validateExecutable(file) {
  if (!isExecutable(file)) {
    throw new Error(`browser executable validation failed: ${file}`);
  }
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

  const archiveName = safeArchiveName(info.entry.archive);
  fs.mkdirSync(path.dirname(info.installDir), { recursive: true });
  const tmp = fs.mkdtempSync(path.join(path.dirname(info.installDir), '.agent-aget-browser-'));
  const archivePath = path.join(tmp, archiveName);
  const extractDir = path.join(tmp, 'extract');
  const stagedDir = path.join(tmp, 'staged');
  const backupDir = path.join(tmp, 'previous');
  let hadPreviousInstall = false;

  try {
    await download(info.entry.url, archivePath);
    verifyChecksum(archivePath, info.entry.sha256);
    extractZip(archivePath, extractDir);
    fs.renameSync(extractDir, stagedDir);

    const stagedExecutable = stagedExecutablePath(info, stagedDir);
    if (process.platform !== 'win32' && fs.existsSync(stagedExecutable)) {
      fs.chmodSync(stagedExecutable, 0o755);
    }
    validateExecutable(stagedExecutable);

    try {
      fs.renameSync(info.installDir, backupDir);
      hadPreviousInstall = true;
    } catch (error) {
      if (error.code !== 'ENOENT') {
        throw error;
      }
    }

    try {
      fs.renameSync(stagedDir, info.installDir);
      validateExecutable(info.executable);
      if (hadPreviousInstall) {
        fs.rmSync(backupDir, { recursive: true, force: true });
      }
    } catch (error) {
      fs.rmSync(info.installDir, { recursive: true, force: true });
      if (hadPreviousInstall) {
        fs.renameSync(backupDir, info.installDir);
      }
      throw error;
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
