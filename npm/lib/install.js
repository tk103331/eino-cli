'use strict';

const fs = require('fs');
const path = require('path');
const { execFileSync } = require('child_process');

const REPO = 'tk103331/eino-cli';
const BINARY = 'eino';

const OS_MAP = { darwin: 'darwin', linux: 'linux' };
const ARCH_MAP = { x64: 'amd64', arm64: 'arm64' };

function platformTarget() {
  const os = OS_MAP[process.platform];
  const arch = ARCH_MAP[process.arch];
  if (!os || !arch) {
    return null;
  }
  return { os, arch, name: `${BINARY}-${os}-${arch}` };
}

function version() {
  return require('../package.json').version;
}

function vendorDir() {
  return path.join(__dirname, '..', 'vendor');
}

function binaryPath() {
  const target = platformTarget();
  if (!target) {
    return null;
  }
  return path.join(vendorDir(), target.name);
}

function download(url, dest) {
  fs.mkdirSync(path.dirname(dest), { recursive: true });
  const tmp = `${dest}.tmp`;
  try {
    execFileSync('curl', ['-fsSL', url, '-o', tmp], { stdio: 'ignore' });
  } catch (curlErr) {
    try {
      execFileSync('wget', ['-qO', tmp, url], { stdio: 'ignore' });
    } catch (wgetErr) {
      fs.rmSync(tmp, { force: true });
      throw new Error(`failed to download ${url} (curl/wget unavailable)`);
    }
  }
  fs.chmodSync(tmp, 0o755);
  fs.renameSync(tmp, dest);
}

// ensureBinarySync returns the path to the platform binary, downloading it
// from the GitHub release on first use.
function ensureBinarySync() {
  const target = platformTarget();
  if (!target) {
    throw new Error(
      `unsupported platform ${process.platform}/${process.arch}; install with Homebrew or go install instead`
    );
  }
  const dest = binaryPath();
  if (fs.existsSync(dest)) {
    return dest;
  }
  const v = version();
  const url = `https://github.com/${REPO}/releases/download/v${v}/${target.name}`;
  download(url, dest);
  return dest;
}

module.exports = { ensureBinarySync, binaryPath, platformTarget, version };
