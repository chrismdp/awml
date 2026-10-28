import "../build/wasm_exec.js";

let running;

export function createAWML(source) {
  if (!running) running = start(source);
  return running;
}

async function start(source) {
  const go = new globalThis.Go();
  let instantiated;
  if (source instanceof WebAssembly.Module) {
    instantiated = await WebAssembly.instantiate(source, go.importObject);
  } else if (source instanceof Response) {
    instantiated = await WebAssembly.instantiateStreaming(source, go.importObject);
  } else {
    instantiated = await WebAssembly.instantiate(source, go.importObject);
  }
  const instance = instantiated.instance || instantiated;
  const exited = go.run(instance);
  while (typeof globalThis.__awmlCheck !== "function") {
    await Promise.race([
      new Promise((resolve) => setTimeout(resolve, 0)),
      exited.then(() => {
        throw new Error("the AWML WebAssembly runtime exited before it was ready");
      }),
    ]);
  }

  return {
    check(sourceText) {
      return JSON.parse(globalThis.__awmlCheck(sourceText));
    },
    // unsound gives the mistakes that stop a file being drawn: every one check gives, except a line
    // that breaks a rule, which draw draws red.
    unsound(sourceText) {
      return JSON.parse(globalThis.__awmlUnsound(sourceText));
    },
    warnings(sourceText) {
      return JSON.parse(globalThis.__awmlWarnings(sourceText));
    },
    // checksJSON takes files as { file, source } and lists every check, as awml check --json does.
    checksJSON(files) {
      return answer(globalThis.__awmlChecks(JSON.stringify(given(files))));
    },
    checks(files) {
      return JSON.parse(this.checksJSON(files));
    },
    // summary, fit and draw take one file or several, each its text or { file, source }. Given
    // together, workflows follow each ref between them, a path ref from the file's name, which is
    // its path from one root. Each throws every file's own mistakes and those following the refs finds,
    // one to a line after the file's name, as awml prints them.
    summaryJSON(sourceTexts) {
      return answer(globalThis.__awmlSummary(JSON.stringify(given(sourceTexts))));
    },
    summary(sourceTexts) {
      return JSON.parse(this.summaryJSON(sourceTexts));
    },
    // fit judges each platform described as running every workflow given, telling the two apart by
    // what they hold. Given several workflows, it names each part after its file, as awml fit does,
    // and after where it comes among the workflows, as in 1#plan, for one given as text alone.
    fitJSON(sourceTexts, implementation = "") {
      return answer(globalThis.__awmlFit(JSON.stringify(given(sourceTexts)), implementation));
    },
    fit(sourceTexts, implementation = "") {
      return JSON.parse(this.fitJSON(sourceTexts, implementation));
    },
    // fitParts judges each part of the workflows given alone against each platform described, and
    // gives the fewest platforms that between them hold every part, as awml fit --parts does.
    fitPartsJSON(sourceTexts) {
      return answer(globalThis.__awmlFitParts(JSON.stringify(given(sourceTexts))));
    },
    fitParts(sourceTexts) {
      return JSON.parse(this.fitPartsJSON(sourceTexts));
    },
    // convert upgrades one file of an older version, or joins a legacy security view and its
    // platform view given as [security, platform], into one 0.8 file, as awml convert does. When it
    // notes something it couldn't carry over, or the current rules find mistakes in the file, it
    // throws them, one to a line, notes first. The error's notes and mistakes hold each, and its
    // converted holds the file, which awml convert still writes.
    convert(sourceTexts, implementation = "") {
      const sources = Array.isArray(sourceTexts) ? sourceTexts : [sourceTexts];
      const decoded = JSON.parse(globalThis.__awmlConvert(JSON.stringify(sources), implementation));
      if (decoded.error) throw new Error(decoded.error);
      if (decoded.notes.length || decoded.mistakes.length) {
        const error = new Error([...decoded.notes, ...decoded.mistakes].join("\n"));
        error.notes = decoded.notes;
        error.mistakes = decoded.mistakes;
        error.converted = decoded.file;
        throw error;
      }
      return decoded.file;
    },
    // draw draws one workflow's view, or the map of several workflows drawn together.
    draw(sourceText, implementation = "") {
      const output = globalThis.__awmlDraw(JSON.stringify(given(sourceText)), implementation);
      if (output.startsWith("{")) throw new Error(JSON.parse(output).error);
      return output;
    },
    // graph gives what draw draws, with nothing laid out, as awml graph does: one workflow's view,
    // or the map of several workflows given together. It takes what draw takes, and throws what draw
    // throws. Every label is given exactly as the file wrote it, and no text in it is escaped, so a
    // page that shows it must escape it itself, unlike draw, whose Mermaid escapes it.
    graphJSON(sourceText, implementation = "") {
      return answer(globalThis.__awmlGraph(JSON.stringify(given(sourceText)), implementation));
    },
    graph(sourceText, implementation = "") {
      return JSON.parse(this.graphJSON(sourceText, implementation));
    },
  };
}

// answer gives the JSON an entry point gave, or throws the error it gave in its place.
function answer(output) {
  const decoded = JSON.parse(output);
  if (decoded.error) throw new Error(decoded.error);
  return output;
}

function given(sources) {
  return Array.isArray(sources) ? sources : [sources];
}
