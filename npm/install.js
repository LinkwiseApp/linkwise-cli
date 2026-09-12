// Downloads the release binary for this platform and verifies it against the
// published checksums file.
//
// A wrapper that skipped the checksum would be a package that installs
// whatever a compromised release page serves, which is worse than not
// shipping on npm at all.
const fs = require("fs");
const path = require("path");
const crypto = require("crypto");
const { version } = require("./package.json");

const TARGETS = {
  "darwin-arm64": "linkwise_darwin_arm64.tar.gz",
  "darwin-x64": "linkwise_darwin_amd64.tar.gz",
  "linux-arm64": "linkwise_linux_arm64.tar.gz",
  "linux-x64": "linkwise_linux_amd64.tar.gz",
  "win32-x64": "linkwise_windows_amd64.zip",
};

// Fetch follows the redirect GitHub answers a release download with, but a
// non-2xx still arrives as a resolved promise, so the status is checked here
// rather than discovered as a checksum mismatch three lines later.
async function get(url) {
  const res = await fetch(url);
  if (!res.ok) {
    throw new Error(`Could not download ${url}: HTTP ${res.status}`);
  }
  return res;
}

async function main() {
  const key = `${process.platform}-${process.arch}`;
  const asset = TARGETS[key];
  if (!asset) {
    throw new Error(
      `No Linkwise binary for ${key}. Install with Homebrew or build from source: ` +
        `https://github.com/LinkwiseApp/linkwise-cli`
    );
  }

  const base = `https://github.com/LinkwiseApp/linkwise-cli/releases/download/v${version}`;

  const sums = await (await get(`${base}/checksums.txt`)).text();
  const expected = sums
    .split("\n")
    .map((line) => line.trim().split(/\s+/))
    .find(([, name]) => name === asset)?.[0];

  if (!expected) {
    throw new Error(`No checksum published for ${asset}`);
  }

  const buf = Buffer.from(await (await get(`${base}/${asset}`)).arrayBuffer());
  const actual = crypto.createHash("sha256").update(buf).digest("hex");
  if (actual !== expected) {
    throw new Error(`Checksum mismatch for ${asset}. Refusing to install.`);
  }

  const dir = path.join(__dirname, "bin");
  fs.mkdirSync(dir, { recursive: true });
  fs.writeFileSync(path.join(dir, "archive"), buf);

  const { execFileSync } = require("child_process");
  if (asset.endsWith(".zip")) {
    execFileSync("unzip", ["-o", "archive"], { cwd: dir });
  } else {
    execFileSync("tar", ["-xzf", "archive"], { cwd: dir });
  }
  fs.unlinkSync(path.join(dir, "archive"));

  const bin = path.join(dir, process.platform === "win32" ? "linkwise.exe" : "linkwise");
  fs.chmodSync(bin, 0o755);
}

main().catch((err) => {
  console.error(err.message);
  process.exit(1);
});
