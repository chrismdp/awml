import assert from "node:assert/strict";
import { readFile, readdir, mkdtemp, writeFile, rm } from "node:fs/promises";
import { spawnSync } from "node:child_process";
import { createAWML } from "./awml.js";

const native = new URL("../build/awml-native", import.meta.url).pathname;
const wasmBytes = await readFile(new URL("../build/awml.wasm", import.meta.url));
const api = await createAWML(wasmBytes);

const top = await yamlFiles(new URL("../examples/", import.meta.url));
const descriptions = (await yamlFiles(new URL("../examples/platforms/", import.meta.url))).sort();
const descriptionSources = await Promise.all(descriptions.map((path) => readFile(path, "utf8")));
let passed = 0;
let placements = 0;
let untrustedViews = 0;
for (const path of top.sort()) {
  const source = await readFile(path, "utf8");
  const relative = path.slice(new URL("../", import.meta.url).pathname.length);

  const nativeCheck = runNative(["check", relative]);
  const wasmErrors = api.check(source);
  const wasmWarnings = api.warnings(source);
  const wasmCheck = [
    ...wasmWarnings.map((message) => `${relative}: warning: ${message}\n`),
    ...(wasmErrors.length ? wasmErrors.map((message) => `${relative}: ${message}\n`) : [`${relative}: ok\n`]),
  ].join("");
  assert.equal(wasmCheck, nativeCheck, `${relative}: check output`);

  assert.equal(api.checksJSON([{ file: relative, source }]) + "\n", runNative(["check", "--json", relative]), `${relative}: checks JSON`);
  assert.equal(api.summaryJSON(source) + "\n", runNative(["summary", "--json", relative]), `${relative}: summary JSON`);
  assert.equal(api.draw(source) + "\n", runNative(["draw", relative]), `${relative}: Mermaid`);
  if (/untrusted: true/.test(source)) {
    assert.equal(api.drawUntrusted(source) + "\n", runNative(["draw", "--untrusted", relative]), `${relative}: untrusted Mermaid`);
    untrustedViews++;
  }
  if (/^awml: ["']?0\.4/m.test(source) && !/^platform:/m.test(source)) {
    const report = api.fit([source, ...descriptionSources]);
    assert.equal(api.fitJSON([source, ...descriptionSources]) + "\n",
      runNative(["fit", "--json", relative, ...descriptions], report.gaps.length ? 1 : 0), `${relative}: design fit`);
    for (const [, implementation] of (source.split("implementations:\n")[1] || "").matchAll(/^  (\w+):\s*$/gm)) {
      assert.equal(api.draw(source, implementation) + "\n",
        runNative(["draw", "--implementation", implementation, relative]), `${relative}: ${implementation} Mermaid`);
      const fit = api.fit([source, ...descriptionSources], implementation);
      assert.equal(api.fitJSON([source, ...descriptionSources], implementation) + "\n",
        runNative(["fit", "--json", "--implementation", implementation, relative, ...descriptions], fit.gaps.length ? 1 : 0), `${relative}: ${implementation} fit`);
      placements++;
    }
  }
  passed++;
}

const everyPath = [...top.sort(), ...descriptions].map((path) => path.slice(new URL("../", import.meta.url).pathname.length));
const everySource = await Promise.all(everyPath.map(async (file) => ({ file, source: await readFile(new URL(`../${file}`, import.meta.url), "utf8") })));
assert.equal(api.checksJSON(everySource) + "\n", runNative(["check", "--json", ...everyPath]), "checks JSON for every example at once");
assert.deepEqual(api.checks(everySource).files.map((file) => file.file), everyPath);

const canonicalPath = new URL("testdata/canonical.yaml", import.meta.url).pathname;
const canonicalSource = await readFile(canonicalPath, "utf8");
assert.deepEqual(api.check(canonicalSource), []);
assert.equal(api.draw(canonicalSource, "home") + "\n", runNative(["draw", "--implementation", "home", canonicalPath]));
assert.throws(() => api.draw(canonicalSource, "missing"), /unknown implementation/);
assert.throws(() => api.drawUntrusted(canonicalSource), /marks no part untrusted/);
assert(untrustedViews > 0, "no example marks a part untrusted");
assert.throws(() => api.fit([canonicalSource, ...descriptionSources], "missing"), /unknown implementation/);
assert.equal(api.fitJSON([canonicalSource], "home") + "\n",
  runNative(["fit", "--json", "--implementation", "home", canonicalPath]), "fit without descriptions");
assert.deepEqual(api.fit([canonicalSource], "home").results.map((result) => result.judged), [false]);
assert(api.check(canonicalSource.replace("parts: [plan, notes]", "parts: [plan]")).some((message) => message.includes('part "notes" is not placed')));

const temporary = await mkdtemp(new URL("../build/parity-", import.meta.url));
try {
  for (const [name, source] of [
    ["warning", canonicalSource.replace("label: Plan", "label: Plan, op: do")],
    ["unplaced", canonicalSource.replace("parts: [plan, notes]", "parts: [plan]")],
    ["duplicate", canonicalSource.replace("parts: [plan, notes]", "parts: [plan, notes, plan]")],
  ]) {
    const path = `${temporary}/${name}.yaml`;
    await writeFile(path, source);
    const errors = api.check(source);
    const output = [
      ...api.warnings(source).map((message) => `${path}: warning: ${message}\n`),
      ...(errors.length ? errors.map((message) => `${path}: ${message}\n`) : [`${path}: ok\n`]),
    ].join("");
    assert.equal(output, runNative(["check", path], errors.length ? 1 : 0), `0.4 ${name}`);
    assert.equal(api.checksJSON([{ file: path, source }]) + "\n", runNative(["check", "--json", path], errors.length ? 1 : 0), `0.4 ${name}: checks JSON`);
  }

  const security = "awml: \"0.3\"\ntitle: Pair\nview: security\nparts:\n  - { id: t, kind: trigger, label: T }\n  - { id: s, kind: ai, label: S }\nlines:\n  - { from: t, to: s, kind: start }\n";
  const pair = [
    { file: `${temporary}/security.yaml`, source: security },
    { file: `${temporary}/platform.yaml`, source: security.replace("view: security", "view: platform").replace("kind: ai", "kind: code") },
    { file: `${temporary}/broken.yaml`, source: "parts: [\n" },
  ];
  for (const { file, source } of pair) await writeFile(file, source);
  const pairReport = api.checks(pair);
  assert.equal(pairReport.checks.find((check) => check.id === "views.same-parts").result, "fail");
  assert.equal(api.checksJSON(pair) + "\n", runNative(["check", "--json", ...pair.map(({ file }) => file)], 1), "checks JSON for a legacy pair");
} finally {
  await rm(temporary, { recursive: true });
}

const badPath = new URL("testdata/bad.yaml", import.meta.url).pathname;
const badRelative = badPath.slice(new URL("../", import.meta.url).pathname.length);
const badSource = await readFile(badPath, "utf8");
const badErrors = api.check(badSource).map((message) => `${badRelative}: ${message}\n`).join("");
assert.equal(badErrors, runNative(["check", badRelative], 1), "invalid file: check messages");
assert.equal(api.checksJSON([{ file: badRelative, source: badSource }]) + "\n", runNative(["check", "--json", badRelative], 1), "invalid file: checks JSON");

const warningPath = new URL("testdata/warning.yaml", import.meta.url).pathname;
const warningRelative = warningPath.slice(new URL("../", import.meta.url).pathname.length);
const warningSource = await readFile(warningPath, "utf8");
const warning = api.warnings(warningSource).map((message) => `${warningRelative}: warning: ${message}\n`).join("");
assert.equal(
  warning + `${warningRelative}: ok\n`,
  runNative(["check", warningRelative]),
  "warning: check messages",
);

const redPath = new URL("testdata/red-line.yaml", import.meta.url).pathname;
const redRelative = redPath.slice(new URL("../", import.meta.url).pathname.length);
const redSource = await readFile(redPath, "utf8");
assert.equal([
  ...api.warnings(redSource).map((message) => `${redRelative}: warning: ${message}\n`),
  ...api.check(redSource).map((message) => `${redRelative}: ${message}\n`),
].join(""), runNative(["check", redRelative], 1), "red line: check messages");
assert.deepEqual(api.unsound(redSource), []);
assert.deepEqual(api.unsound(badSource), api.check(badSource));
assert.equal(api.draw(redSource) + "\n", runNative(["draw", redRelative]), "red line: Mermaid");
assert.match(api.draw(redSource), /linkStyle 1 stroke:#d0342c/);
assert.equal(api.draw(redSource, "home") + "\n", runNative(["draw", "--implementation", "home", redRelative]), "red line: home Mermaid");
assert.equal(api.drawUntrusted(redSource) + "\n", runNative(["draw", "--untrusted", redRelative]), "red line: untrusted Mermaid");
assert.equal(api.summaryJSON(redSource) + "\n", runNative(["summary", "--json", redRelative]), "red line: summary JSON");
assert.equal(api.summary(redSource).redLines, 1);

const fitPaths = [
  "examples/client-email.yaml",
  "examples/platforms/cowork-mac-mini.yaml",
  "examples/platforms/scripts-mac-mini.yaml",
  "examples/platforms/codex-laptop.yaml",
  "examples/platforms/worker.yaml",
];
const fitSources = await Promise.all(fitPaths.map((path) => readFile(new URL(`../${path}`, import.meta.url), "utf8")));
assert.equal(api.fitJSON(fitSources) + "\n", runNative(["fit", "--json", ...fitPaths]), "fit JSON");

console.log(`Node wasm parity: ${passed} examples passed (check, warnings, summary, Mermaid); ${placements} implementations; ${untrustedViews} untrusted views; invalid check messages and fit passed`);

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
