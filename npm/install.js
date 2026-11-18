#!/usr/bin/env node
// Télécharge le binaire Go correspondant à l'OS / l'architecture depuis
// les releases GitHub, vérifie son SHA-256, puis l'installe dans ./bin.
"use strict";

const { createHash } = require("node:crypto");
const { execFileSync } = require("node:child_process");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");

const REPO = process.env.COFFRE_REPO || "coffre-dev/coffre-cli";
const { version } = require("./package.json");

const PLATFORMS = { darwin: "darwin", linux: "linux", win32: "windows" };
const ARCHS = { x64: "amd64", arm64: "arm64" };

async function download(url) {
  const res = await fetch(url, { redirect: "follow" });
  if (!res.ok) throw new Error(`Téléchargement impossible (${res.status}) : ${url}`);
  return Buffer.from(await res.arrayBuffer());
}

async function main() {
  const goos = PLATFORMS[process.platform];
  const goarch = ARCHS[process.arch];
  if (!goos || !goarch) {
    throw new Error(`Plateforme non supportée : ${process.platform}/${process.arch}`);
  }

  const ext = goos === "windows" ? "zip" : "tar.gz";
  const archive = `coffre_${goos}_${goarch}.${ext}`;
  const base = `https://github.com/${REPO}/releases/download/v${version}`;

  const [data, checksums] = await Promise.all([
    download(`${base}/${archive}`),
    download(`${base}/checksums.txt`).then((b) => b.toString("utf8")),
  ]);

  const expected = checksums
    .split("\n")
    .map((l) => l.trim().split(/\s+/))
    .find(([, name]) => name === archive)?.[0];
  const actual = createHash("sha256").update(data).digest("hex");
  if (!expected || expected !== actual) {
    throw new Error(`Somme de contrôle invalide pour ${archive}`);
  }

  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), "coffre-"));
  const archivePath = path.join(tmp, archive);
  fs.writeFileSync(archivePath, data);
  // tar sait extraire les .zip sur Windows 10+ (bsdtar)
  execFileSync("tar", ["-xf", archivePath, "-C", tmp]);

  const binName = goos === "windows" ? "coffre.exe" : "coffre";
  const dest = path.join(__dirname, "bin", binName);
  fs.mkdirSync(path.dirname(dest), { recursive: true });
  fs.copyFileSync(path.join(tmp, binName), dest);
  fs.chmodSync(dest, 0o755);
  fs.rmSync(tmp, { recursive: true, force: true });
  console.log(`coffre ${version} installé (${goos}/${goarch})`);
}

main().catch((err) => {
  console.error(`✗ Installation de coffre échouée : ${err.message}`);
  process.exit(1);
});
