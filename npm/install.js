#!/usr/bin/env node
// postinstall: download the platform-matching bifu-cli release binary from
// GitHub into ./bin, so the bin/bifu-cli.js launcher can exec it.
"use strict";
const fs = require("fs");
const os = require("os");
const path = require("path");
const https = require("https");
const { execFileSync } = require("child_process");

const REPO = "decodeex/bifu-cli";
const pkg = require("./package.json");

const OS = { darwin: "darwin", linux: "linux", win32: "windows" }[process.platform];
const ARCH = { x64: "amd64", arm64: "arm64" }[process.arch];

function fail(msg) {
  console.error("bifu-cli install: " + msg);
  console.error("  Download manually: https://github.com/" + REPO + "/releases");
  process.exit(0); // don't hard-fail npm install; the launcher will warn if missing
}

if (!OS || !ARCH) fail("unsupported platform " + process.platform + "/" + process.arch);
if (!pkg.version || pkg.version === "0.0.0") fail("no release version set");

const ext = OS === "windows" ? "zip" : "tar.gz";
const asset = `bifu-cli_${OS}_${ARCH}.${ext}`;
const url = `https://github.com/${REPO}/releases/download/v${pkg.version}/${asset}`;
const binDir = path.join(__dirname, "bin");
const tmp = path.join(os.tmpdir(), asset);

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

fs.mkdirSync(binDir, { recursive: true });
console.log("bifu-cli: downloading " + asset + " ...");
download(url, tmp, (err) => {
  if (err) return fail(err.message);
  try {
    // tar handles both .tar.gz and .zip on macOS/Linux/Windows 10+ (bsdtar).
    execFileSync("tar", ["-xf", tmp, "-C", binDir], { stdio: "ignore" });
    const bin = path.join(binDir, OS === "windows" ? "bifu-cli.exe" : "bifu-cli");
    if (!fs.existsSync(bin)) return fail("binary missing after extract");
    if (OS !== "windows") fs.chmodSync(bin, 0o755);
    fs.unlinkSync(tmp);
    console.log("bifu-cli: installed " + pkg.version + " (" + OS + "/" + ARCH + ")");
  } catch (e) {
    fail("extract failed: " + e.message);
  }
});
