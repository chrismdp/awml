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
    summaryJSON(sourceTexts) {
      const sources = Array.isArray(sourceTexts) ? sourceTexts : [sourceTexts];
      for (const sourceText of sources) {
        const errors = this.check(sourceText);
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
    fitJSON(sourceTexts, implementation = "") {
      const sources = Array.isArray(sourceTexts) ? sourceTexts : [sourceTexts];
      for (const sourceText of sources) {
        const errors = this.check(sourceText);
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
    draw(sourceText, implementation = "") {
      const errors = this.check(sourceText);
      if (errors.length) throw new Error(errors.join("\n"));
      const output = globalThis.__awmlDraw(sourceText, implementation);
      if (output.startsWith("{")) throw new Error(JSON.parse(output).error);
      return output;
    },
    // drawUntrusted draws where untrusted text reaches, as awml draw --untrusted does.
    drawUntrusted(sourceText) {
      const errors = this.check(sourceText);
      if (errors.length) throw new Error(errors.join("\n"));
      const output = globalThis.__awmlDrawUntrusted(sourceText);
      if (output.startsWith("{")) throw new Error(JSON.parse(output).error);
      return output;
    },
  };
}
