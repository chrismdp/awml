import assert from "node:assert/strict";
import { readFile, readdir } from "node:fs/promises";
import { spawnSync } from "node:child_process";
import { createAWML } from "./awml.js";

const native = new URL("../build/awml-native", import.meta.url).pathname;
const wasmBytes = await readFile(new URL("../build/awml.wasm", import.meta.url));
const api = await createAWML(wasmBytes);

const top = await yamlFiles(new URL("../examples/", import.meta.url));
let passed = 0;
for (const path of top.sort()) {
  const source = await readFile(path, "utf8");
  const relative = path.slice(new URL("../", import.meta.url).pathname.length);

  const nativeCheck = runNative(["check", relative]);
  const wasmErrors = api.check(source);
  const wasmCheck = wasmErrors.length
    ? wasmErrors.map((message) => `${relative}: ${message}\n`).join("")
    : `${relative}: ok\n`;
  assert.equal(wasmCheck, nativeCheck, `${relative}: check output`);

  assert.equal(api.summaryJSON(source) + "\n", runNative(["summary", "--json", relative]), `${relative}: summary JSON`);
  assert.equal(api.draw(source) + "\n", runNative(["draw", relative]), `${relative}: Mermaid`);
  passed++;
}

const badPath = new URL("testdata/bad.yaml", import.meta.url).pathname;
const badRelative = badPath.slice(new URL("../", import.meta.url).pathname.length);
const badSource = await readFile(badPath, "utf8");
const badErrors = api.check(badSource).map((message) => `${badRelative}: ${message}\n`).join("");
assert.equal(badErrors, runNative(["check", badRelative], 1), "invalid file: check messages");

const fitPaths = [
  "examples/client-email.yaml",
  "examples/platforms/cowork-mac-mini.yaml",
  "examples/platforms/scripts-mac-mini.yaml",
  "examples/platforms/codex-laptop.yaml",
  "examples/platforms/worker.yaml",
];
const fitSources = await Promise.all(fitPaths.map((path) => readFile(new URL(`../${path}`, import.meta.url), "utf8")));
assert.equal(api.fitJSON(fitSources) + "\n", runNative(["fit", "--json", ...fitPaths]), "fit JSON");

console.log(`Node wasm parity: ${passed} examples passed (check, summary, Mermaid); invalid check messages and fit passed`);

async function yamlFiles(directory) {
  const entries = await readdir(directory, { withFileTypes: true });
  const files = [];
  for (const entry of entries) {
    const url = new URL(entry.name + (entry.isDirectory() ? "/" : ""), directory);
    if (entry.isDirectory()) files.push(...(await yamlFiles(url)));
    else if (entry.name.endsWith(".yaml")) files.push(url.pathname);
  }
  return files;
}

function runNative(args, status = 0) {
  const result = spawnSync(native, args, { encoding: "utf8" });
  assert.equal(result.status, status, `${args.join(" ")}: ${result.stderr}`);
  return result.stdout;
}
