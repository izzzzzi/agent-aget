#!/usr/bin/env node
// Verify that agent-facing aget instruction surfaces preserve the core browser-work contract.
const fs = require('fs');
const path = require('path');

const root = path.join(__dirname, '..');

const REQUIRED_FILES = [
  'skills/aget/SKILL.md',
  'AGENTS.md',
  'AGENT_INSTRUCTIONS.md',
  'SYSTEM_PROMPT_snippet.md',
  '.cursor/rules/aget.mdc',
  '.clinerules/aget.md',
  '.github/copilot-instructions.md',
];

const OPTIONAL_FILES = [
  '.opencode/command/aget-open.md',
  '.opencode/command/aget-snapshot.md',
];

const GROUPS = [
  ['aget-only', ['aget', 'browser work']],
  ['no-playwright', ['Playwright']],
  ['no-puppeteer', ['Puppeteer']],
  ['no-selenium', ['Selenium']],
  ['no-python-js-automation', ['Python/JS browser automation']],
  ['no-cdp', ['raw CDP']],
  ['no-existing-browser', ['already-running browser']],
  ['probe-first', ['snapshot', 'read', 'find']],
  ['wait-not-sleep', ['wait', 'sleep']],
  ['close-session', ['session close']],
  ['cookies-profile', ['profile create', '--cookies']],
  ['untrusted-page-content', ['untrusted']],
];

const FORBIDDEN_UNSAFE_PATTERNS = [
  ['querySelector(...).click()', /querySelector\([^\n]*\.click\s*\(/i],
  ['Network.setCookies', /Network\.setCookies/i],
  ['numeric sleep', /\bsleep\s+\d+\b/i],
  ['unsafe aget page js', /\baget\s+page\s+js\b[^\n]*(click|navigation|navigate|form|cookie|keyboard)/i],
];

function readFile(rel) {
  return fs.readFileSync(path.join(root, rel), 'utf8');
}

function clearlyNegativeOrReadDebug(line) {
  return /\b(never|do not|don't|not for|forbid|forbidden|avoid)\b/i.test(line)
    || /read\/debug fallback|debug fallback|read-only|last-resort read\/debug/i.test(line);
}

function phrasesForFile(name, group, phrases) {
  if (name === '.github/copilot-instructions.md' && group === 'wait-not-sleep') return ['wait'];
  return phrases;
}

let failed = false;
const files = [];

for (const rel of REQUIRED_FILES) {
  const abs = path.join(root, rel);
  if (!fs.existsSync(abs)) {
    console.error(`${rel} is missing`);
    failed = true;
    continue;
  }
  files.push([rel, readFile(rel), true]);
}

for (const rel of OPTIONAL_FILES) {
  const abs = path.join(root, rel);
  if (fs.existsSync(abs)) files.push([rel, readFile(rel), false]);
}

for (const [name, content, checkGroups] of files) {
  if (checkGroups) {
    for (const [group, phrases] of GROUPS) {
      for (const phrase of phrasesForFile(name, group, phrases)) {
        if (!content.toLowerCase().includes(phrase.toLowerCase())) {
          console.error(`${name} is missing ${group} invariant phrase: "${phrase}"`);
          failed = true;
        }
      }
    }
  }

  for (const line of content.split('\n')) {
    for (const [label, pattern] of FORBIDDEN_UNSAFE_PATTERNS) {
      if (pattern.test(line) && !clearlyNegativeOrReadDebug(line)) {
        console.error(`${name} contains unsafe browser automation guidance (${label}): ${line.trim()}`);
        failed = true;
      }
    }
  }
}

const skill = readFile('skills/aget/SKILL.md');
for (const rel of [
  'references/open.md',
  'references/snapshot.md',
  'references/read.md',
  'references/find.md',
  'references/actions.md',
  'references/session.md',
  'references/doctor.md',
]) {
  if (!skill.includes(rel)) {
    console.error(`skills/aget/SKILL.md does not link ${rel}`);
    failed = true;
  }
}

if (failed) {
  console.error('aget skill copies drifted. Update the agent-facing surfaces from skills/aget/SKILL.md.');
  process.exit(1);
}

console.log('All aget skill copies preserve the core browser-work contract. OK');
