'use strict';

const assert = require('node:assert/strict');
const contract = require('./release-contract-test');

function main() {
  assert.equal(contract.releaseTagForVersion('0.5.1'), 'v0.5.1');
  assert.throws(
    () => contract.assertReleaseTagState({ version: '0.5.1', tagExists: false, exactMatch: false, tagCommit: '', headCommit: 'abc' }),
    /missing release tag v0\.5\.1/,
  );
  assert.throws(
    () => contract.assertReleaseTagState({ version: '0.5.1', tagExists: true, exactMatch: false, tagCommit: 'abc', headCommit: 'def' }),
    /does not point at HEAD/,
  );
  assert.doesNotThrow(() => contract.assertReleaseTagState({ version: '0.5.1', tagExists: true, exactMatch: true, tagCommit: 'abc', headCommit: 'abc' }));
}

if (require.main === module) {
  main();
}

module.exports = main;
