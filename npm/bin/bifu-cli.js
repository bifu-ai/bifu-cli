#!/usr/bin/env node
// Thin launcher: exec the native bifu-cli binary fetched by install.js.
"use strict";
const path = require("path");
const fs = require("fs");
const { spawnSync } = require("child_process");

const bin = path.join(
  __dirname,
  process.platform === "win32" ? "bifu-cli.exe" : "bifu-cli"
);

if (!fs.existsSync(bin)) {
  console.error(
    "bifu-cli binary not found. Reinstall the package, or grab a build from " +
      "https://github.com/decodeex/bifu-cli/releases"
  );
  process.exit(1);
}

const res = spawnSync(bin, process.argv.slice(2), { stdio: "inherit" });
if (res.error) {
  console.error(res.error.message);
  process.exit(1);
}
process.exit(res.status === null ? 1 : res.status);
