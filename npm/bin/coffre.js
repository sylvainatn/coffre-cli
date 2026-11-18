#!/usr/bin/env node
// Point d'entrée npm : délègue au binaire Go téléchargé par install.js
"use strict";

const { spawnSync } = require("node:child_process");
const path = require("node:path");

const bin = path.join(__dirname, process.platform === "win32" ? "coffre.exe" : "coffre");
const result = spawnSync(bin, process.argv.slice(2), { stdio: "inherit" });

if (result.error) {
  console.error(`✗ Impossible de lancer coffre (${result.error.message}). Réinstallez le paquet.`);
  process.exit(1);
}
if (result.signal) process.kill(process.pid, result.signal);
process.exit(result.status ?? 1);
