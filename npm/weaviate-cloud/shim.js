#!/usr/bin/env node
'use strict';

const path = require('path');
const { spawnSync } = require('child_process');

const PKG_NAME = require('./package.json').name;

const PLATFORM_PACKAGES = {
  'darwin-arm64': `${PKG_NAME}-darwin-arm64`,
  'darwin-x64': `${PKG_NAME}-darwin-x64`,
  'linux-arm64': `${PKG_NAME}-linux-arm64`,
  'linux-x64': `${PKG_NAME}-linux-x64`,
  'win32-x64': `${PKG_NAME}-win32-x64`,
};

const platformKey = `${process.platform}-${process.arch}`;
const platformPkg = PLATFORM_PACKAGES[platformKey];
if (!platformPkg) {
  process.stderr.write(`wcloud: unsupported platform: ${platformKey}\n`);
  process.exit(1);
}

let pkgDir;
try {
  pkgDir = path.dirname(require.resolve(`${platformPkg}/package.json`));
} catch {
  process.stderr.write(
    `wcloud: platform package ${platformPkg} is not installed.\n` +
    `  Try reinstalling: npm install -g ${PKG_NAME}\n`,
  );
  process.exit(1);
}

const binaryName = process.platform === 'win32' ? 'wcloud.exe' : 'wcloud';
const binaryPath = path.join(pkgDir, binaryName);

const result = spawnSync(binaryPath, process.argv.slice(2), { stdio: 'inherit' });
process.exit(result.status ?? 1);
