import assert from "node:assert/strict";
import { readFile, readdir, mkdtemp, mkdir, writeFile, rm } from "node:fs/promises";
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
let untrustedDrawings = 0;
let conversions = 0;
for (const path of top.sort()) {
  const source = await readFile(path, "utf8");
  const relative = path.slice(new URL("../", import.meta.url).pathname.length);

  // A drawing of a live system fails the check by design, for the lines it draws red.
  const wasmErrors = api.check(source);
  const failed = wasmErrors.length ? 1 : 0;
  const nativeCheck = runNative(["check", relative], failed);
  const wasmWarnings = api.warnings(source);
  const wasmCheck = [
    ...wasmWarnings.map((message) => `${relative}: warning: ${message}\n`),
    ...(wasmErrors.length ? wasmErrors.map((message) => `${relative}: ${message}\n`) : [`${relative}: ok\n`]),
  ].join("");
  assert.equal(wasmCheck, nativeCheck, `${relative}: check output`);

  assert.equal(api.checksJSON([{ file: relative, source }]) + "\n", runNative(["check", "--json", relative], failed), `${relative}: checks JSON`);
  assert.equal(api.summaryJSON(source) + "\n", runNative(["summary", "--json", relative]), `${relative}: summary JSON`);
  // A file of the current version comes back as it was, comments and all.
  if (!failed) {
    assert.equal(api.convert(source), source, `${relative}: convert`);
    assert.equal(runNative(["convert", relative]), source, `${relative}: awml convert`);
  }
  if (/^platform:/m.test(source)) {
    assert.throws(() => api.draw(source), /a platform description has no drawing/);
    assert.throws(() => api.graph(source), /a platform description has no drawing/);
    passed++;
    continue;
  }
  assert.equal(api.draw(source) + "\n", runNative(["draw", relative]), `${relative}: Mermaid`);
  assert.equal(api.graphJSON(source) + "\n", runNative(["graph", relative]), `${relative}: graph`);
  // The security view shows where untrusted text goes, in both.
  if (/untrusted: true/.test(source)) {
    assert(api.graph(source).lines.some((line) => line.untrusted), `${relative}: the graph says where untrusted text goes`);
    untrustedDrawings++;
  }
  if (!failed && !/^platform:/m.test(source)) {
    const report = api.fit([source, ...descriptionSources]);
    assert.equal(api.fitJSON([source, ...descriptionSources]) + "\n",
      runNative(["fit", "--json", relative, ...descriptions], report.gaps.length ? 1 : 0), `${relative}: design fit`);
    for (const [, implementation] of (source.split("implementations:\n")[1] || "").matchAll(/^  (\w+):\s*$/gm)) {
      assert.equal(api.draw(source, implementation) + "\n",
        runNative(["draw", "--implementation", implementation, relative]), `${relative}: ${implementation} Mermaid`);
      assert.equal(api.graphJSON(source, implementation) + "\n",
        runNative(["graph", "--implementation", implementation, relative]), `${relative}: ${implementation} graph`);
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
assert.equal(api.checksJSON(everySource) + "\n", runNative(["check", "--json", ...everyPath], 1), "checks JSON for every example at once");
// Each file named twice is read once, where it's first named, in both.
assert.deepEqual(api.checks(everySource).files.map((file) => file.file), [...new Set(everyPath)]);

const canonicalPath = new URL("testdata/canonical.yaml", import.meta.url).pathname;
const canonicalSource = await readFile(canonicalPath, "utf8");
assert.deepEqual(api.check(canonicalSource), []);
assert.equal(api.draw(canonicalSource, "home") + "\n", runNative(["draw", "--implementation", "home", canonicalPath]));
assert.throws(() => api.draw(canonicalSource, "missing"), /unknown implementation/);
assert.deepEqual(api.graph(canonicalSource, "home"), JSON.parse(runNative(["graph", "--implementation", "home", canonicalPath])));
assert.throws(() => api.graph(canonicalSource, "missing"), /unknown implementation/);
// There's no untrusted view, in the API or the WebAssembly beneath it.
for (const name of ["drawUntrusted", "graphUntrusted", "graphUntrustedJSON"]) assert.equal(api[name], undefined, `the API still has ${name}`);
for (const name of ["__awmlDrawUntrusted", "__awmlGraphUntrusted"]) assert.equal(globalThis[name], undefined, `the WebAssembly still has ${name}`);
assert(untrustedDrawings > 0, "no example marks a part untrusted");
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
    assert.equal(output, runNative(["check", path], errors.length ? 1 : 0), `canonical ${name}`);
    assert.equal(api.checksJSON([{ file: path, source }]) + "\n", runNative(["check", "--json", path], errors.length ? 1 : 0), `canonical ${name}: checks JSON`);
  }

  // An older file is refused, with a pointer to awml convert, in both engines.
  const older = [
    { file: `${temporary}/older.yaml`, source: canonicalSource.replace('awml: "0.8"', 'awml: "0.5"') },
    { file: `${temporary}/security.yaml`, source: "awml: \"0.3\"\ntitle: Pair\nview: security\nparts:\n  - { id: t, kind: trigger, label: T }\n" },
    { file: `${temporary}/broken.yaml`, source: "parts: [\n" },
  ];
  for (const { file, source } of older) await writeFile(file, source);
  assert.match(api.check(older[0].source)[0], /which this release no longer reads: upgrade the file with awml convert/);
  assert.equal(api.checks(older).checks.find((check) => check.id === "file.version").result, "fail");
  assert.equal(api.checksJSON(older) + "\n", runNative(["check", "--json", ...older.map(({ file }) => file)], 1), "checks JSON for older files");
  assert.throws(() => api.draw(older[0].source), /upgrade the file with awml convert/);

  // convert upgrades a file of each older version into one 0.8 file, workflow or platform
  // description, and gives the file awml convert writes, byte for byte.
  const dinners = (version) => `# The family's dinners.
awml: "${version}"
title: Dinners
view: security
groups:
  - {id: family, label: "family: parents, kids"}
parts:
  - {id: thu, kind: trigger, label: Thursday}
  - {id: plan, kind: ai, label: Plan, in: family}
  - {id: board, kind: surface, label: The board}
lines:
  - {from: thu, to: plan, kind: start}
  - {from: plan, to: board, kind: publish, label: posted}
`;
  const description = (version) => `awml: "${version}"
platform: {id: scripts, label: Scripts on a server}
off_for_up_to: PT0S
steps: code
agents: []
calls: [anthropic]
holds: [notes]
`;
  const upgrades = {
    "dinners-0.2.yaml": dinners("0.2"),
    "dinners-0.3.yaml": dinners("0.3"),
    "bread-0.4.yaml": canonicalSource.replace('awml: "0.8"', 'awml: "0.4"'),
    "renewals-0.5.yaml": `awml: "0.5"
title: Renewals
scopes:
  - {id: sam, label: "sam: Sam", processing: any}
  - {id: club, label: "club: Sam, members"}
parts:
  - {id: monday, kind: trigger, label: Monday}
  - {id: renew, kind: code, op: do, label: Renew the books, in: sam}
  - {id: standing, kind: approval, label: "Sam's standing approval: renewals only", in: sam}
  - {id: library, kind: outside, label: "Renew at the library: Ask", in: sam}
  - {id: phone, kind: surface, label: Sam's phone, in: sam}
  - {id: board, kind: surface, label: The club's board, in: club}
lines:
  - {from: monday, to: renew, kind: start}
  - {from: renew, to: standing, kind: data, breaks: false}
  - {from: standing, to: library, kind: data}
  - {from: standing, to: phone, kind: data}
  - {from: renew, to: board, kind: publish, label: what was renewed}
implementations:
  home:
    label: At home
    blocks:
      - {id: runner, label: Runner, platform: acme/platforms/ops-runner, parts: [renew, standing, library, phone, board]}
`,
    // Each comment moves to the top once, in the file's order.
    "comments-0.5.yaml": `# The family's dinners.
awml: "0.5"
title: Dinners
scopes:
  - {id: family, label: "family: parents, kids"}
parts:
  # The week starts on Thursday.
  - {id: thu, kind: trigger, label: Thursday} # After the school run.
  - {id: plan, kind: ai, label: Plan, in: family}
lines:
  # Thursday starts the plan.
  - {from: thu, to: plan, kind: start}
`,
    "description-0.3.yaml": description("0.3"),
    "description-0.4.yaml": description("0.4"),
    "description-0.5.yaml": description("0.5"),
    "description-0.6.yaml": description("0.6"),
    // A do step's line into another workflow's trigger becomes its outside action.
    "hand-off-0.6.yaml": `awml: "0.6"
title: Dinners
parts:
  - {id: tell, kind: code, op: do, label: Tell the shopping}
  - {id: ready, kind: trigger, label: The list is ready, wait: PT1H, ref: shopping.yaml#ready}
lines:
  - {from: tell, to: ready, kind: start}
`,
    "description-0.7.yaml": description("0.7"),
    // A 0.7 file changes only its version.
    "bread-0.7.yaml": canonicalSource.replace('awml: "0.8"', 'awml: "0.7"'),
  };
  for (const [name, source] of Object.entries(upgrades)) {
    await writeFile(`${temporary}/${name}`, source);
    assert.equal(api.convert(source), runNative(["convert", `${temporary}/${name}`]), `convert ${name}`);
    conversions++;
  }

  // A file already at 0.8 comes back unchanged.
  const current = api.convert(upgrades["bread-0.4.yaml"]);
  await writeFile(`${temporary}/current.yaml`, current);
  assert.equal(api.convert(current), current, "convert of a 0.8 file");
  assert.equal(runNative(["convert", `${temporary}/current.yaml`]), current, "awml convert of a 0.8 file");

  // A legacy security view joins with its platform view, under the implementation named or default.
  const security = `# A bakery plans tomorrow's loaves. The bakery is fictional.
awml: "0.3"
title: Tomorrow's loaves
view: security
groups:
  - { id: "bakery", label: "bakery: Jo, Kit", processing: { anthropic: any } }
parts:
  - { id: morning, kind: "trigger", label: "Tomorrow 07:00", wait: "PT1H" }
  - { id: "plan", kind: ai, op: "draft", label: "Plan a \\"special\\" loaf \\\\ batch", in: bakery }
  - { id: notes, kind: context, label: "Notes", at: "notes/bread", in: bakery }
lines:
  - { from: morning, to: "plan", kind: "start" }
  - { from: plan, to: notes, kind: data, label: "yes" }
`;
  const platform = `# It runs in the bakery office.
awml: "0.3"
title: Tomorrow's loaves
view: platform
groups:
  - { id: laptop, label: "Laptop: always on" }
parts:
  - { id: morning, kind: trigger, label: "Tomorrow 07:00", wait: PT1H }
  - { id: "plan", kind: ai, op: draft, label: "Plan a \\"special\\" loaf \\\\ batch", in: laptop }
  - { id: notes, kind: context, label: "Notes", at: notes/bread, in: laptop }
  - { id: key, kind: comment, label: "The key: read only", in: laptop }
lines:
  - { from: morning, to: plan, kind: start }
  - { from: plan, to: notes, kind: data, label: "yes" }
  - { from: key, to: plan, kind: tie }
`;
  const pair = [`${temporary}/security.yaml`, `${temporary}/platform.yaml`];
  await writeFile(pair[0], security);
  await writeFile(pair[1], platform);
  assert.equal(api.convert([security, platform]), runNative(["convert", ...pair]), "convert of a legacy pair");
  assert.equal(api.convert([security, platform], "office"), runNative(["convert", "--implementation", "office", ...pair]), "convert of a legacy pair, naming its implementation");
  conversions += 2;

  // When the current rules find mistakes in the converted file, convert throws them, one to a line,
  // and the error carries the file, which awml convert still writes.
  const mistaken = {
    "send-0.4.yaml": `awml: "0.4"
title: Send
parts:
  - { id: act, kind: code, op: do, label: "Act" }
  - { id: out, kind: outside, label: "Send it" }
lines:
  - { from: act, to: out, kind: data }
`,
    "red-line-0.5.yaml": `awml: "0.5"
title: The club's Friday message
scopes:
  - {id: ella, label: "ella: Ella"}
parts:
  - {id: fri, kind: trigger, label: "Friday 16:00"}
  - {id: write, kind: ai, op: draft, label: Draft the message, in: ella}
  - {id: ask, kind: code, op: do, label: Ask Ella to send it, in: ella}
  - {id: approve, kind: review, label: Ella's yes, in: ella}
  - {id: send, kind: outside, label: "Send it: Ask", in: ella}
lines:
  - {from: fri, to: write, kind: start}
  - {from: write, to: ask, kind: data, label: read the whole inbox, breaks: true}
  - {from: ask, to: approve, kind: data}
  - {from: approve, to: send, kind: data, label: "[yes]"}
  - {from: write, to: send, kind: data, label: sent without Ella's yes, breaks: true}
`,
  };
  for (const [name, source] of Object.entries(mistaken)) {
    await writeFile(`${temporary}/${name}`, source);
    const native = runNativeResult(["convert", `${temporary}/${name}`], 1);
    assert.throws(() => api.convert(source), (error) => {
      assert.equal(error.converted, native.stdout, `convert ${name}: the file`);
      assert.equal("awml convert: converted, but awml check finds these mistakes in the result:\n" +
        error.message.split("\n").map((message) => `  ${message}\n`).join(""), native.stderr, `convert ${name}: its mistakes`);
      return true;
    });
    conversions++;
  }

  // What convert couldn't carry over comes before the mistakes, in both engines.
  const noted = `awml: "0.6"
title: Dinners
parts:
  - {id: plan, kind: ai, op: draft, label: Plan}
  - {id: tell, kind: code, op: do, label: Tell the shopping}
  - {id: ready, kind: trigger, label: The list is ready, untrusted: true, ref: shopping.yaml#ready}
  - {id: done, kind: trigger, label: The plan is done, ref: notify.yaml#done}
lines:
  - {from: plan, to: tell, kind: start}
  - {from: tell, to: ready, kind: start}
  - {from: plan, to: done, kind: start}
`;
  await writeFile(`${temporary}/noted-0.6.yaml`, noted);
  const nativeNoted = runNativeResult(["convert", `${temporary}/noted-0.6.yaml`], 1);
  assert.throws(() => api.convert(noted), (error) => {
    assert.equal(error.converted, nativeNoted.stdout, "convert with a note: the file");
    assert.equal(error.notes.length, 1, "convert with a note: its note");
    assert.equal("awml convert: converted, but do this by hand:\n" + error.notes.map((note) => `  ${note}\n`).join("") +
      "awml convert: converted, but awml check finds these mistakes in the result:\n" + error.mistakes.map((mistake) => `  ${mistake}\n`).join(""),
      nativeNoted.stderr, "convert with a note: its notes and mistakes");
    assert.equal(error.message, [...error.notes, ...error.mistakes].join("\n"));
    return true;
  });
  conversions++;

  // A file that can't be converted throws why, as awml convert says it.
  const unconvertible = {
    "platform-view.yaml": platform,
    "too-old.yaml": "awml: \"0.1\"\ntitle: Bread\nparts: []\n",
    "not-a-mapping.yaml": "- a list\n",
    "not-yaml.yaml": "parts: [\n",
  };
  for (const [name, source] of Object.entries(unconvertible)) {
    await writeFile(`${temporary}/${name}`, source);
    const native = runNativeResult(["convert", `${temporary}/${name}`], 1);
    assert.throws(() => api.convert(source), (error) => {
      assert.equal(`${temporary}/${name}: ${error.message}\n`, native.stderr, `convert ${name}`);
      return true;
    });
  }
  const mismatched = platform.replace("kind: ai", "kind: code");
  await writeFile(`${temporary}/mismatched.yaml`, mismatched);
  const refused = runNativeResult(["convert", pair[0], `${temporary}/mismatched.yaml`], 1);
  assert.throws(() => api.convert([security, mismatched]), (error) => {
    assert.equal(`${error.message}\n`, refused.stderr, "convert of a pair that doesn't match");
    return true;
  });
  assert.throws(() => api.convert(upgrades["bread-0.4.yaml"], "office"), /^Error: only a legacy pair has an implementation to name$/);
  assert.throws(() => api.convert([security, platform, platform]), /^Error: convert takes one file, or a security view and its platform view$/);
  assert.throws(() => api.convert([]), /^Error: convert takes one file, or a security view and its platform view$/);

  // Workflows given together follow each ref between them, a path from the referring file's folder.
  const plan = `awml: "0.8"
title: Dinners
scopes:
  - { id: family, label: "family: parents, kids", processing: any }
parts:
  - { id: thu, kind: trigger, label: "Thursday 19:00", wait: PT1H }
  - { id: form, kind: connector, label: The order form, in: family }
  - { id: plan, kind: ai, op: draft, label: "Plan next week's dinners", in: family }
  - { id: list, kind: context, label: Shopping list, at: notes/list, in: family }
  - { id: tell, kind: code, op: do, label: Tell the shopping the list is ready, in: family }
  - { id: ready, kind: outside, label: "Start the shopping: Allow", in: family, ref: shop.yaml#ready }
lines:
  - { from: thu, to: plan, kind: start }
  - { from: form, to: plan, kind: data }
  - { from: plan, to: list, kind: data }
  - { from: plan, to: tell, kind: start }
  - { from: tell, to: ready, kind: start }
`;
  const shop = `awml: "0.8"
title: Shopping
scopes:
  - { id: home, label: "home: parents, kids", processing: any }
parts:
  - { id: ready, kind: trigger, label: The shopping list is ready, wait: PT1H }
  - { id: list, kind: context, label: Shopping list, at: notes/list, in: home, ref: plan.yaml#list }
  - { id: fill, kind: code, op: do, label: Fill the basket, in: home }
  - { id: basket, kind: outside, label: "Fill the basket: Allow", in: home }
lines:
  - { from: ready, to: fill, kind: start }
  - { from: list, to: fill, kind: data }
  - { from: fill, to: list, kind: data }
  - { from: fill, to: basket, kind: data }
`;
  const handOff = [
    { file: `${temporary}/plan.yaml`, source: plan },
    { file: `${temporary}/shop.yaml`, source: shop },
  ];
  for (const { file, source } of handOff) await writeFile(file, source);
  const handOffPaths = handOff.map(({ file }) => file);
  assert.equal(api.checksJSON(handOff) + "\n", runNative(["check", "--json", ...handOffPaths]), "checks JSON together");
  assert.equal(api.draw(handOff) + "\n", runNative(["draw", ...handOffPaths]), "map Mermaid");
  assert.throws(() => api.draw(handOff, "home"), /one workflow, not several/);
  assert.equal(api.graphJSON(handOff) + "\n", runNative(["graph", ...handOffPaths]), "map graph");
  assert.throws(() => api.graph(handOff, "home"), /one workflow, not several/);
  assert.equal(api.summaryJSON(handOff) + "\n", runNative(["summary", "--json", ...handOffPaths]), "summary JSON together");
  const handOffFit = api.fit([...handOff, ...descriptionSources]);
  assert.equal(api.fitJSON([...handOff, ...descriptionSources]) + "\n",
    runNative(["fit", "--json", ...descriptions, ...handOffPaths], handOffFit.gaps.length ? 1 : 0), "fit together");
  assert(Object.keys(handOffFit.results.find((result) => result.fits || result.judged).models).every((step) => step.includes("#")));
  // A ref is followed to where a file is: by awml from the folder it runs in, and by WebAssembly from
  // one root, so files named from the same folder link the same way in both.
  await mkdir(`${temporary}/family`);
  await mkdir(`${temporary}/work`);
  const located = [
    { file: "family/plan.yaml", source: plan.replace("shop.yaml#ready", "../work/shop.yaml#readied") },
    { file: "work/shop.yaml", source: shop.replace("plan.yaml#list", "family/plan#lists") },
  ];
  for (const { file, source } of located) await writeFile(`${temporary}/${file}`, source);
  assert.equal(api.checksJSON(located) + "\n", runNative(["check", "--json", ...located.map(({ file }) => file)], 1, temporary), "checks JSON of files named from one folder");
  assert.deepEqual(api.checks(located).checks.find((check) => check.id === "refs.known").mistakes.map(({ file }) => file), ["family/plan.yaml", "work/shop.yaml"]);
  const broken = [handOff[0], { file: handOff[1].file, source: shop.replace("plan.yaml#list", "plan.yaml#lists") }];
  await writeFile(broken[1].file, broken[1].source);
  assert.equal(api.checksJSON(broken) + "\n", runNative(["check", "--json", ...handOffPaths], 1), "checks JSON together, with a ref to a part that isn't there");
  assert.equal(api.checks(broken).checks.find((check) => check.id === "refs.known").result, "fail");

  // The mistakes following the refs finds stop draw, graph, summary and fit, which say them as awml
  // does.
  for (const [name, call, args] of [
    ["draw", () => api.draw(broken), ["draw", ...handOffPaths]],
    ["graph", () => api.graph(broken), ["graph", ...handOffPaths]],
    ["summary", () => api.summary(broken), ["summary", "--json", ...handOffPaths]],
    ["fit", () => api.fit([...broken, ...descriptionSources]), ["fit", "--json", ...handOffPaths, ...descriptions]],
  ]) {
    const refused = runNativeResult(args, 1);
    assert.equal(refused.stdout, "", `${name} of files with a ref to a part that isn't there: awml prints nothing`);
    assert.throws(call, (error) => {
      assert.equal(`${error.message}\n`, refused.stderr, `${name} of files with a ref to a part that isn't there`);
      return true;
    });
  }
  // Untrusted text follows an outside action into the workflow it starts. Here the order form's text
  // reaches the basket with nothing to stop it, a line that breaks a rule: check fails and fit
  // refuses, while the map, its graph and the summary are still given, the same from both.
  const taintedPlan = plan.replace("label: The order form, in: family", "label: The order form, untrusted: true, in: family");
  const tainted = [{ file: handOff[0].file, source: taintedPlan }, handOff[1]];
  for (const { file, source } of tainted) await writeFile(file, source);
  assert.equal(api.checksJSON(tainted) + "\n", runNative(["check", "--json", ...handOffPaths], 1), "checks JSON of untrusted text across a hand-off");
  assert.equal(api.checks(tainted).checks.find((check) => check.id === "untrusted.outside").result, "fail");
  assert.equal(api.draw(tainted) + "\n", runNative(["draw", ...handOffPaths]), "map of untrusted text across a hand-off");
  assert.equal(api.graphJSON(tainted) + "\n", runNative(["graph", ...handOffPaths]), "map graph of untrusted text across a hand-off");
  assert.equal(api.summaryJSON(tainted) + "\n", runNative(["summary", "--json", ...handOffPaths]), "summary of untrusted text across a hand-off");
  {
    const refused = runNativeResult(["fit", "--json", ...handOffPaths, ...descriptions], 1);
    assert.equal(refused.stdout, "", "fit of untrusted text across a hand-off: awml prints nothing");
    assert.throws(() => api.fit([...tainted, ...descriptionSources]), (error) => {
      assert.equal(`${error.message}\n`, refused.stderr, "fit of untrusted text across a hand-off");
      return true;
    });
  }
  for (const { file, source } of handOff) await writeFile(file, source);
  // A file that isn't a mapping is refused by fit as awml refuses it, and the module carries on.
  for (const [name, odd] of [["empty", ""], ["list", "- a\n"]]) {
    const oddFile = `${temporary}/odd-${name}.yaml`;
    await writeFile(oddFile, odd);
    const refused = runNativeResult(["fit", "--json", oddFile, ...descriptions], 1);
    assert.throws(() => api.fit([{ file: oddFile, source: odd }, ...descriptionSources]), (error) => {
      assert.equal(`${error.message}\n`, refused.stderr, `fit of a file that's ${name}`);
      return true;
    });
  }
  assert.deepEqual(api.check(plan), [], "the module still answers after refusing those files");
  // Every file's own mistakes are said with the shared ones, each after its file's name, by summary
  // and the map as by awml.
  const odd = [{ file: handOff[0].file, source: plan.replace("  - { id: thu,", "  - { id: odd, kind: bogus, label: Odd }\n  - { id: thu,") }, handOff[1]];
  for (const { file, source } of odd) await writeFile(file, source);
  for (const [name, call, args] of [
    ["summary", () => api.summary(odd), ["summary", "--json", ...handOffPaths]],
    ["draw", () => api.draw(odd), ["draw", ...handOffPaths]],
  ]) {
    const refused = runNativeResult(args, 1);
    assert.throws(call, (error) => {
      assert.equal(`${error.message}\n`, refused.stderr, `${name} of files with a mistake of their own`);
      return true;
    });
  }
  for (const { file, source } of handOff) await writeFile(file, source);

  // A part they share is untrusted if any workflow's copy of it is marked untrusted. Its text reaches
  // the replier's send with nothing to stop it, a line that breaks a rule, so the check fails, and the
  // map and the summary are still given, as awml gives them.
  const sorter = `awml: "0.8"
title: Inbox sorter
scopes:
  - { id: office, label: "office: Ana", processing: any }
parts:
  - { id: arrive, kind: trigger, label: "Mail arrives", wait: PT1H }
  - { id: mail, kind: connector, label: The shop inbox, in: office }
  - { id: sort, kind: code, op: sort, label: Sort the mail, in: office }
  - { id: queue, kind: context, label: Sorted mail, at: notes/queue, in: office }
lines:
  - { from: arrive, to: sort, kind: start }
  - { from: mail, to: sort, kind: data }
  - { from: sort, to: queue, kind: data }
`;
  const replier = `awml: "0.8"
title: Replier
scopes:
  - { id: office, label: "office: Ana", processing: any }
parts:
  - { id: hourly, kind: trigger, label: "Every hour", wait: PT1H }
  - { id: mail, kind: connector, label: The shop inbox, in: office, untrusted: true, ref: sorter.yaml#mail }
  - { id: queue, kind: context, label: Sorted mail, at: notes/queue, in: office, ref: sorter.yaml#queue }
  - { id: draft, kind: ai, op: draft, label: Draft a reply, in: office }
  - { id: go, kind: code, op: do, label: Send it, in: office }
  - { id: send, kind: outside, label: "Send the reply: Allow", in: office }
lines:
  - { from: hourly, to: draft, kind: start }
  - { from: queue, to: draft, kind: data }
  - { from: draft, to: go, kind: data }
  - { from: go, to: send, kind: data }
`;
  const inbox = [{ file: `${temporary}/sorter.yaml`, source: sorter }, { file: `${temporary}/replier.yaml`, source: replier }];
  for (const { file, source } of inbox) await writeFile(file, source);
  const inboxPaths = inbox.map(({ file }) => file);
  assert.equal(api.checksJSON(inbox) + "\n", runNative(["check", "--json", ...inboxPaths], 1), "checks JSON of a part marked untrusted by a copy");
  assert.equal(api.checks(inbox).checks.find((check) => check.id === "untrusted.outside").result, "fail");
  assert.match(api.draw(inbox), /class s1 unstopped/);
  assert.equal(api.summaryJSON(inbox) + "\n", runNative(["summary", "--json", ...inboxPaths]), "summary JSON of a part marked untrusted by a copy");
  assert.equal(api.draw(inbox) + "\n", runNative(["draw", ...inboxPaths]), "map of a part marked untrusted by a copy");
  assert.deepEqual(api.summary(inbox).untrusted.map(({ outsideActions }) => outsideActions), [["Send the reply: Allow"]]);

  // A box holds an outside action only where it starts a workflow given, so not the plan's here.
  const starting = [handOff[0], inbox[0]];
  assert.equal(api.draw(starting) + "\n", runNative(["draw", ...starting.map(({ file }) => file)]), "map of a workflow that starts one not given");
  assert(!api.draw(starting).includes("Start the shopping"), "the map draws an action that starts a workflow not given");
  assert(api.draw(handOff).includes("w1_ready --> w2_ready"), "the map draws no line from the action to the trigger it starts");

  // fit names each workflow by the shortest end of its path that's its own, and one given as text
  // alone by where it comes.
  await mkdir(`${temporary}/x`);
  await mkdir(`${temporary}/y`);
  const alike = ["x/sorter.yaml", "y/sorter.yaml"].map((file) => ({ file, source: sorter.replace("kind: code, op: sort", "kind: ai, op: sort") }));
  for (const { file, source } of alike) await writeFile(`${temporary}/${file}`, source);
  const alikeFit = api.fit([...alike, ...descriptionSources]);
  assert.equal(api.fitJSON([...alike, ...descriptionSources]) + "\n",
    runNative(["fit", "--json", ...alike.map(({ file }) => file), ...descriptions], alikeFit.gaps.length ? 1 : 0, temporary), "fit of two files of the same name");
  assert.deepEqual(Object.keys(alikeFit.results.find((result) => result.fits).models), ["x/sorter#sort", "y/sorter#sort"]);
  assert.deepEqual(Object.keys(api.fit([...alike.map(({ source }) => source), ...descriptionSources]).results.find((result) => result.fits).models), ["1#sort", "2#sort"]);

  // An outside action's ref names the trigger of the workflow it starts.
  const misnamed = [
    { file: handOff[0].file, source: plan.replace("shop.yaml#ready", "shop.yaml#fill") },
    handOff[1],
  ];
  for (const { file, source } of misnamed) await writeFile(file, source);
  assert.equal(api.checksJSON(misnamed) + "\n", runNative(["check", "--json", ...handOffPaths], 1), "checks JSON of an outside action whose ref names a step");
  assert.equal(api.checks(misnamed).checks.find((check) => check.id === "refs.starts").result, "fail");
  for (const { file, source } of broken) await writeFile(file, source);

  // Starting the shopping waits for nothing in the plan, so the plan alone can be fitted.
  const firing = api.fit([handOff[0], ...descriptionSources]);
  assert.deepEqual(firing.gaps, [], "fit asks for a wait on an outside action");
  assert.equal(api.fitJSON([handOff[0], ...descriptionSources]) + "\n", runNative(["fit", "--json", handOff[0].file, ...descriptions]), "fit of a workflow that starts another");

  // One file given by name is held to a ref it makes to itself.
  const self = { file: `${temporary}/self.yaml`, source: plan.replace("shop.yaml#ready", "self.yaml#thu") };
  await writeFile(self.file, self.source);
  for (const [name, call, args] of [
    ["draw", () => api.draw(self), ["draw", self.file]],
    ["graph", () => api.graph(self), ["graph", self.file]],
    ["summary", () => api.summary(self), ["summary", self.file]],
    ["fit", () => api.fit([self, ...descriptionSources]), ["fit", self.file, ...descriptions]],
  ]) {
    const refused = runNativeResult(args, 1);
    assert.throws(call, (error) => {
      assert.equal(`${error.message}\n`, refused.stderr, `${name} of a file with a ref to itself`);
      return true;
    });
  }

  // Each kind of line break in a label folds into one space in the Mermaid, in both engines, while
  // the graph gives the label as written.
  for (const [name, escape] of [["lf", "\\n"], ["crlf", "\\r\\n"], ["cr", "\\r"], ["u2028", "\\u2028"], ["u2029", "\\u2029"]]) {
    const source = `awml: "0.8"
title: "Bakery${escape}replies"
scopes:
  - { id: shop, label: "${escape}end${escape}", processing: any }
parts:
  - { id: mail, kind: trigger, label: "A customer${escape}${escape}emails", wait: PT1H }
  - id: draft
    kind: ai
    op: draft
    in: shop
    label: |
      Draft a reply
  - { id: board, kind: surface, label: "The shop's ${escape} board", in: shop }
  - { id: gap, kind: comment, label: "Nobody${escape}reads it" }
lines:
  - { from: mail, to: draft, kind: start }
  - { from: draft, to: board, kind: data, label: "posted${escape}to the board" }
  - { from: gap, to: board, kind: tie }
`;
    const path = `${temporary}/breaks-${name}.yaml`;
    await writeFile(path, source);
    const drawn = api.draw(source);
    assert.equal(drawn + "\n", runNative(["draw", path]), `line breaks, ${name}: Mermaid`);
    assert.equal(api.graphJSON(source) + "\n", runNative(["graph", path]), `line breaks, ${name}: graph`);
    assert(!["\r", String.fromCharCode(0x2028), String.fromCharCode(0x2029)].some((mark) => drawn.includes(mark)), `line breaks, ${name}: the Mermaid keeps a line break`);
    assert(drawn.includes('subgraph shop["end"]') && drawn.includes('label: "Nobody reads it"'), `line breaks, ${name}: the Mermaid doesn't fold them:\n${drawn}`);
    const rows = drawn.split("\n").map((row) => row.trim());
    assert.equal(rows.filter((row) => row.startsWith("subgraph ")).length, 1, `line breaks, ${name}: subgraphs`);
    assert.equal(rows.filter((row) => row === "end").length, 1, `line breaks, ${name}: ends`);
    assert.equal(api.graph(source).comments[0].label, JSON.parse(`"Nobody${escape}reads it"`), `line breaks, ${name}: the graph's label`);
  }
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
assert.match(api.draw(redSource), /linkStyle 0,1 stroke:#d0342c/);
assert.equal(api.draw(redSource, "home") + "\n", runNative(["draw", "--implementation", "home", redRelative]), "red line: home Mermaid");
assert.equal(api.graphJSON(redSource) + "\n", runNative(["graph", redRelative]), "red line: graph");
assert(api.graph(redSource).lines.some((line) => line.breaks), "red line: the graph says which lines break a rule");
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

// The examples that share a part are drawn together as their map.
const familyPaths = ["examples/dinners.yaml", "examples/shopping.yaml", "examples/bills.yaml", "examples/pickups.yaml", "examples/question.yaml"];
const family = await Promise.all(familyPaths.map(async (file) => ({ file, source: await readFile(new URL(`../${file}`, import.meta.url), "utf8") })));
assert.equal(api.draw(family) + "\n", await readFile(new URL("../examples/family-map.mmd", import.meta.url), "utf8"), "the family's map");
assert.equal(api.summaryJSON(family) + "\n", runNative(["summary", "--json", ...familyPaths]), "the family's summary JSON");

console.log(`Node wasm parity: ${passed} examples passed (check, warnings, summary, convert, Mermaid, graph); ${placements} implementations; ${untrustedDrawings} drawings of untrusted text; ${conversions} conversions; invalid check messages and fit passed`);

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

function runNative(args, status = 0, cwd = undefined) {
  return runNativeResult(args, status, cwd).stdout;
}

function runNativeResult(args, status = 0, cwd = undefined) {
  const result = spawnSync(native, args, { encoding: "utf8", cwd });
  assert.equal(result.status, status, `${args.join(" ")}: ${result.stderr}`);
  return result;
}
