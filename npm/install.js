#!/usr/bin/env node
// postinstall: download the platform-matching bifu-cli release binary from
// GitHub into ./bin, so the bin/bifu-cli.js launcher can exec it.
"use strict";
const fs = require("fs");
const os = require("os");
const path = require("path");
const https = require("https");
const crypto = require("crypto");
const { execFileSync } = require("child_process");

const REPO = "decodeex/bifu-cli-releases"; // public repo hosting the binaries
const pkg = require("./package.json");

const OS = { darwin: "darwin", linux: "linux", win32: "windows" }[process.platform];
const ARCH = { x64: "amd64", arm64: "arm64" }[process.arch];

// Hard-fail on install problems (BIFU-CLI-202606-020): a silent exit(0) made a
// corrupted/tampered download look like a successful install, leaving the user
// on a stale or missing binary without warning.
function fail(msg) {
  console.error("WARNING: bifu-cli install failed: " + msg);
  console.error("  Download manually: https://github.com/" + REPO + "/releases");
  process.exit(1);
}

if (!OS || !ARCH) fail("unsupported platform " + process.platform + "/" + process.arch);
if (!pkg.version || pkg.version === "0.0.0") fail("no release version set");

const asset = `bifu-cli_${OS}_${ARCH}.tar.gz`;
const base = `https://github.com/${REPO}/releases/download/v${pkg.version}`;
const url = `${base}/${asset}`;
const sumsURL = `${base}/checksums.txt`;
const binDir = path.join(__dirname, "bin");
const tmp = path.join(os.tmpdir(), asset);
const tmpSums = path.join(os.tmpdir(), `bifu-cli_checksums_${pkg.version}.txt`);

function download(u, dest, cb, redirects) {
  redirects = redirects || 0;
  if (redirects > 10) return cb(new Error("too many redirects"));
  https
    .get(u, { headers: { "User-Agent": "bifu-cli-npm" } }, (res) => {
      if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
        res.resume();
        return download(res.headers.location, dest, cb, redirects + 1);
      }
      if (res.statusCode !== 200) {
        res.resume();
        return cb(new Error("HTTP " + res.statusCode + " for " + u));
      }
      const file = fs.createWriteStream(dest);
      res.pipe(file);
      file.on("finish", () => file.close(cb));
    })
    .on("error", cb);
}

// sha256 of a file, hex.
function sha256(file) {
  return crypto.createHash("sha256").update(fs.readFileSync(file)).digest("hex");
}

// verifyChecksum compares the downloaded archive against checksums.txt
// (BIFU-CLI-202606-008).
function verifyChecksum() {
  const text = fs.readFileSync(tmpSums, "utf8");
  let expected = null;
  for (const line of text.split("\n")) {
    const m = line.trim().match(/^([0-9a-f]{64})\s+(.+)$/i);
    if (m && m[2] === asset) { expected = m[1].toLowerCase(); break; }
  }
  if (!expected) fail("no checksum entry for " + asset);
  const actual = sha256(tmp).toLowerCase();
  if (actual !== expected) fail("checksum mismatch for " + asset + " (expected " + expected + ", got " + actual + ")");
}

// assertSafeArchive lists archive members and rejects path traversal / absolute
// paths before extraction (BIFU-CLI-202606-021).
function assertSafeArchive() {
  const list = execFileSync("tar", ["-tf", tmp], { encoding: "utf8" });
  for (const raw of list.split("\n")) {
    const name = raw.trim();
    if (!name) continue;
    if (name.startsWith("/") || name.startsWith("\\") || /(^|[\\/])\.\.([\\/]|$)/.test(name) || /^[A-Za-z]:/.test(name)) {
      fail("archive contains an unsafe path: " + name);
    }
  }
}

fs.mkdirSync(binDir, { recursive: true });
console.log("bifu-cli: downloading " + asset + " ...");
download(url, tmp, (err) => {
  if (err) return fail(err.message);
  download(sumsURL, tmpSums, (sErr) => {
    if (sErr) return fail("could not download checksums.txt: " + sErr.message);
    try {
      verifyChecksum();
      assertSafeArchive();
      // tar handles both .tar.gz and .zip on macOS/Linux/Windows 10+ (bsdtar).
      execFileSync("tar", ["-xf", tmp, "-C", binDir], { stdio: "ignore" });
      const bin = path.join(binDir, OS === "windows" ? "bifu-cli.exe" : "bifu-cli");
      if (!fs.existsSync(bin)) return fail("binary missing after extract");
      if (OS !== "windows") fs.chmodSync(bin, 0o755);
      fs.unlinkSync(tmp);
      try { fs.unlinkSync(tmpSums); } catch (_) {}
      console.log("bifu-cli: installed " + pkg.version + " (" + OS + "/" + ARCH + ")");
    } catch (e) {
      fail("extract failed: " + e.message);
    }
  });
});
