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
      const output = globalThis.__awmlChecks(JSON.stringify(Array.isArray(files) ? files : [files]));
      const decoded = JSON.parse(output);
      if (decoded.error) throw new Error(decoded.error);
      return output;
    },
    checks(files) {
      return JSON.parse(this.checksJSON(files));
    },
    // summary, fit and draw take one file or several, each its text or { file, source }. Given
    // together, workflows follow each ref between them, a path ref from the file's name, which is
    // its path from one root. Each throws the mistakes following the refs finds, one to a line after
    // the file's name, as awml prints them.
    summaryJSON(sourceTexts) {
      const sources = given(sourceTexts);
      for (const sourceText of sources) {
        const errors = this.unsound(text(sourceText));
        if (errors.length) throw new Error(errors.join("\n"));
      }
      const output = globalThis.__awmlSummary(JSON.stringify(sources));
      const decoded = JSON.parse(output);
      if (decoded.error) throw new Error(decoded.error);
      return output;
    },
    summary(sourceTexts) {
      return JSON.parse(this.summaryJSON(sourceTexts));
    },
    // fit judges each platform described as running every workflow given, telling the two apart by
    // what they hold. Given several workflows, it names each part after its file, as awml fit does,
    // and after where it comes among the workflows, as in 1#plan, for one given as text alone.
    fitJSON(sourceTexts, implementation = "") {
      const sources = given(sourceTexts);
      for (const sourceText of sources) {
        const errors = this.check(text(sourceText));
        if (errors.length) throw new Error(errors.join("\n"));
      }
      const output = globalThis.__awmlFit(JSON.stringify(sources), implementation);
      const decoded = JSON.parse(output);
      if (decoded.error) throw new Error(decoded.error);
      return output;
    },
    fit(sourceTexts, implementation = "") {
      return JSON.parse(this.fitJSON(sourceTexts, implementation));
    },
    // convert upgrades one file of an older version, or joins a legacy security view and its
    // platform view given as [security, platform], into one 0.6 file, as awml convert does. When the
    // current rules find mistakes in the file, it throws them, one to a line, and the error's
    // converted holds the file, which awml convert still writes.
    convert(sourceTexts, implementation = "") {
      const sources = Array.isArray(sourceTexts) ? sourceTexts : [sourceTexts];
      const decoded = JSON.parse(globalThis.__awmlConvert(JSON.stringify(sources), implementation));
      if (decoded.error) throw new Error(decoded.error);
      if (decoded.mistakes.length) {
        const error = new Error(decoded.mistakes.join("\n"));
        error.converted = decoded.file;
        throw error;
      }
      return decoded.file;
    },
    // draw draws one workflow's view, or the map of several workflows drawn together.
    draw(sourceText, implementation = "") {
      const sources = given(sourceText);
      if (sources.length > 1) {
        if (implementation) throw new Error("an implementation is drawn for one workflow, not several");
        for (const source of sources) {
          const errors = this.unsound(text(source));
          if (errors.length) throw new Error(errors.join("\n"));
        }
        const output = globalThis.__awmlDrawMap(JSON.stringify(sources));
        if (output.startsWith("{")) throw new Error(JSON.parse(output).error);
        return output;
      }
      sourceText = text(sources[0]);
      const errors = this.unsound(sourceText);
      if (errors.length) throw new Error(errors.join("\n"));
      const output = typeof sources[0] === "string"
        ? globalThis.__awmlDraw(sourceText, implementation)
        : globalThis.__awmlDraw(sourceText, implementation, sources[0].file || "");
      if (output.startsWith("{")) throw new Error(JSON.parse(output).error);
      return output;
    },
    // drawUntrusted draws where untrusted text reaches, as awml draw --untrusted does.
    drawUntrusted(sourceText) {
      const errors = this.unsound(sourceText);
      if (errors.length) throw new Error(errors.join("\n"));
      const output = globalThis.__awmlDrawUntrusted(sourceText);
      if (output.startsWith("{")) throw new Error(JSON.parse(output).error);
      return output;
    },
  };
}

function given(sources) {
  return Array.isArray(sources) ? sources : [sources];
}

function text(source) {
  return typeof source === "string" ? source : source.source;
}
