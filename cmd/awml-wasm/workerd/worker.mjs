import wasm from "../../../build/awml.wasm";
import { createAWML } from "../../../wasm/awml.js";

let api;

export default {
  async fetch(request) {
    const source = await request.text();
    api ||= createAWML(wasm);
    const awml = await api;
    return Response.json({ check: awml.check(source), summary: awml.summary(source) });
  },
};
