'use strict';

const fs = require('node:fs');
const path = require('node:path');
const { spawnSync } = require('node:child_process');
const pkg = require('../package.json');
const { target } = require('./platform');

const root = path.join(__dirname, '..');
const goreleaserVersion = 'v2.16.0';

function expectedArchives(version) {
  return [
    ['linux', 'x64'],
    ['linux', 'arm64'],
    ['darwin', 'x64'],
    ['darwin', 'arm64'],
    ['win32', 'x64'],
  ].map(([platform, arch]) => {
    const info = target(platform, arch);
    return `aget_${version}_${info.os}_${info.arch}${info.archiveExt}`;
  });
}

function verifyArtifactFiles(files, version = pkg.version) {
  for (const archive of expectedArchives(version)) {
    if (!files.includes(archive)) {
      throw new Error(`missing snapshot archive matching installer expectation: ${archive}`);
    }
  }

  if (!files.includes('checksums.txt')) {
    throw new Error('missing checksums.txt');
  }
}

function verifyPackageFiles() {
  const files = run('npm', ['pack', '--dry-run', '--json']);
  const parsed = JSON.parse(files);
  const names = parsed[0].files.map((file) => file.path);

  for (const required of ['browser-manifest.json', 'scripts/browser-install.js', 'AGENT_INSTRUCTIONS.md']) {
    if (!names.includes(required)) {
      throw new Error(`missing npm package file: ${required}`);
    }
  }

  const rootManifest = fs.readFileSync(path.join(root, 'browser-manifest.json'), 'utf8');
  const embeddedManifest = fs.readFileSync(
    path.join(root, 'internal/managedbrowser/browser-manifest.json'),
    'utf8',
  );
  if (rootManifest !== embeddedManifest) {
    throw new Error('root and embedded browser manifests differ');
  }
}

function run(command, args, options = {}) {
  const result = spawnSync(command, args, {
    cwd: root,
    stdio: options.stdio || 'pipe',
    encoding: 'utf8',
  });

  if (result.error) {
    throw result.error;
  }

  if (result.status !== 0) {
    const output = [result.stdout, result.stderr].filter(Boolean).join('\n').trim();
    throw new Error(`${command} ${args.join(' ')} failed${output ? `:\n${output}` : ''}`);
  }

  return (result.stdout || '').trim();
}

function git(args, options = {}) {
  return run('git', args, options);
}

function releaseTagForVersion(version = pkg.version) {
  return `v${version}`;
}

function gitRefExists(ref) {
  const result = spawnSync('git', ['rev-parse', '--quiet', '--verify', ref], {
    cwd: root,
    stdio: 'ignore',
  });
  return result.status === 0;
}

function gitOutput(args) {
  const result = spawnSync('git', args, { cwd: root, stdio: 'pipe', encoding: 'utf8' });
  return { status: result.status, stdout: (result.stdout || '').trim(), stderr: (result.stderr || '').trim() };
}

function currentGitState(version = pkg.version) {
  const tag = releaseTagForVersion(version);
  const head = git(['rev-parse', 'HEAD']);
  const tagExists = gitRefExists(`refs/tags/${tag}`);
  const tagCommit = tagExists ? git(['rev-list', '-n', '1', tag]) : '';
  const exact = gitOutput(['describe', '--tags', '--exact-match', 'HEAD']);
  return {
    version,
    tag,
    headCommit: head,
    tagExists,
    tagCommit,
    exactMatch: exact.status === 0 && exact.stdout === tag,
  };
}

function assertReleaseTagState(state = currentGitState()) {
  const tag = state.tag || releaseTagForVersion(state.version);
  if (!state.tagExists) {
    throw new Error(`missing release tag ${tag}; create it at HEAD before running release contract`);
  }
  if (state.tagCommit !== state.headCommit) {
    throw new Error(`release tag ${tag} does not point at HEAD (${state.tagCommit} != ${state.headCommit})`);
  }
  if (!state.exactMatch) {
    throw new Error(`HEAD is not exactly tagged ${tag}`);
  }
}

function ensureReleaseGitState(version = pkg.version) {
  const remote = spawnSync('git', ['remote', 'get-url', 'origin'], {
    cwd: root,
    stdio: 'ignore',
  });
  if (remote.status !== 0) {
    throw new Error('git remote origin is required for release artifact contract');
  }

  if (process.env.GITHUB_REF_TYPE === 'tag' || process.env.AGET_RELEASE_CONTRACT_STRICT_TAG === '1') {
    assertReleaseTagState(currentGitState(version));
  }
}

function goreleaserArgs() {
  if (spawnSync('goreleaser', ['--version'], { stdio: 'ignore' }).status === 0) {
    return ['goreleaser', ['release', '--snapshot', '--clean']];
  }

  return ['go', [
    'run',
    `github.com/goreleaser/goreleaser/v2@${goreleaserVersion}`,
    'release',
    '--snapshot',
    '--clean',
  ]];
}

function artifactFiles(directory = path.join(root, 'dist')) {
  const files = [];

  function walk(current) {
    for (const entry of fs.readdirSync(current, { withFileTypes: true })) {
      const full = path.join(current, entry.name);
      if (entry.isDirectory()) {
        walk(full);
      } else {
        files.push(path.basename(full));
      }
    }
  }

  walk(directory);
  return files;
}

function main() {
  ensureReleaseGitState();
  verifyPackageFiles();

  if (process.env.GITHUB_REF_TYPE !== 'tag' && process.env.AGET_RELEASE_CONTRACT_STRICT_TAG !== '1') {
    verifyArtifactFiles(expectedArchives(pkg.version).concat('checksums.txt'), pkg.version);
    console.log('release artifact contract ok (branch mode; tagged release build skipped)');
    return;
  }

  const [command, args] = goreleaserArgs();
  run(command, args, { stdio: 'inherit' });
  verifyArtifactFiles(artifactFiles());
  console.log('release artifact contract ok');
}

if (require.main === module) {
  try {
    main();
  } catch (error) {
    console.error(error.message);
    process.exit(1);
  }
}

module.exports = {
  expectedArchives,
  verifyArtifactFiles,
  verifyPackageFiles,
  releaseTagForVersion,
  assertReleaseTagState,
  currentGitState,
};
