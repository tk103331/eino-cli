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
  return { os, arch };
}

function version() {
  return require('../package.json').version;
}

function vendorDir() {
  return path.join(__dirname, '..', 'vendor');
}

// assetName mirrors the goreleaser archive naming:
// eino_<version>_<os>_<arch>.tar.gz.
function assetName(v) {
  const target = platformTarget();
  if (!target) {
    return null;
  }
  return `${BINARY}_${v}_${target.os}_${target.arch}.tar.gz`;
}

function binaryPath() {
  if (!platformTarget()) {
    return null;
  }
  return path.join(vendorDir(), BINARY);
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
  fs.chmodSync(tmp, 0o644);
  fs.renameSync(tmp, dest);
}

function extract(archive, dir) {
  try {
    // Only unpack the binary; the archive also carries the docs.
    execFileSync('tar', ['-xzf', archive, '-C', dir, BINARY], { stdio: 'ignore' });
  } catch (err) {
    throw new Error(`failed to extract ${archive} (tar unavailable): ${err.message}`);
  }
}

// ensureBinarySync returns the path to the platform binary, downloading and
// unpacking the release archive on first use.
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
  const archiveName = assetName(v);
  const url = `https://github.com/${REPO}/releases/download/v${v}/${archiveName}`;
  const archive = path.join(vendorDir(), archiveName);
  download(url, archive);
  extract(archive, vendorDir());
  fs.rmSync(archive, { force: true });
  fs.chmodSync(dest, 0o755);
  return dest;
}

module.exports = { ensureBinarySync, binaryPath, platformTarget, assetName, version };
