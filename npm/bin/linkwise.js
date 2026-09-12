#!/usr/bin/env node
// Hands control to the downloaded binary, so exit codes and signals pass
// through untouched. The exit code is the CLI's whole contract with a script.
const path = require("path");
const { spawnSync } = require("child_process");

const bin = path.join(__dirname, process.platform === "win32" ? "linkwise.exe" : "linkwise");
const result = spawnSync(bin, process.argv.slice(2), { stdio: "inherit" });

if (result.error) {
  console.error("linkwise: binary is missing. Try reinstalling @linkwise/cli.");
  process.exit(1);
}
process.exit(result.status === null ? 1 : result.status);
