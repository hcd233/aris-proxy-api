import { VERTEX_SOURCE, type SceneId } from "./scenes/common";

/* Minimal WebGL2 renderer for the Ambient Stage: one fullscreen triangle,
 * one cached program per scene. Relative imports only — the shader check
 * harness bundles this module with esbuild, without the `@/` alias. */

export interface SceneProgram {
  set1f(name: string, v: number): void;
  set2f(name: string, x: number, y: number): void;
  set4fv(name: string, v: Float32Array): void;
}

export interface Renderer {
  /** Compile (once, cached) and bind a scene program. Throws on compile/link errors. */
  use(id: SceneId, fragment: string): SceneProgram;
  resize(width: number, height: number): void;
  draw(): void;
  dispose(): void;
}

function compile(gl: WebGL2RenderingContext, type: number, source: string): WebGLShader {
  const shader = gl.createShader(type);
  if (!shader) throw new Error("createShader failed");
  gl.shaderSource(shader, source);
  gl.compileShader(shader);
  if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) {
    const log = gl.getShaderInfoLog(shader) ?? "";
    gl.deleteShader(shader);
    throw new Error(`shader compile failed: ${log}`);
  }
  return shader;
}

function link(gl: WebGL2RenderingContext, fragment: string): WebGLProgram {
  const vs = compile(gl, gl.VERTEX_SHADER, VERTEX_SOURCE);
  const fs = compile(gl, gl.FRAGMENT_SHADER, fragment);
  const program = gl.createProgram();
  if (!program) throw new Error("createProgram failed");
  gl.attachShader(program, vs);
  gl.attachShader(program, fs);
  gl.linkProgram(program);
  gl.deleteShader(vs);
  gl.deleteShader(fs);
  if (!gl.getProgramParameter(program, gl.LINK_STATUS)) {
    const log = gl.getProgramInfoLog(program) ?? "";
    gl.deleteProgram(program);
    throw new Error(`program link failed: ${log}`);
  }
  return program;
}

function programApi(gl: WebGL2RenderingContext, program: WebGLProgram): SceneProgram {
  const locations = new Map<string, WebGLUniformLocation | null>();
  const loc = (name: string) => {
    if (!locations.has(name)) locations.set(name, gl.getUniformLocation(program, name));
    return locations.get(name) ?? null;
  };
  return {
    set1f: (name, v) => gl.uniform1f(loc(name), v),
    set2f: (name, x, y) => gl.uniform2f(loc(name), x, y),
    set4fv: (name, v) => gl.uniform4fv(loc(name), v),
  };
}

export function createRenderer(canvas: HTMLCanvasElement): Renderer | null {
  const gl = canvas.getContext("webgl2", {
    alpha: false,
    antialias: false,
    depth: false,
    stencil: false,
    powerPreference: "low-power",
    preserveDrawingBuffer: false,
  });
  if (!gl) return null;

  const vao = gl.createVertexArray();
  gl.bindVertexArray(vao);
  const programs = new Map<SceneId, { program: WebGLProgram; api: SceneProgram }>();

  return {
    use(id, fragment) {
      let entry = programs.get(id);
      if (!entry) {
        const program = link(gl, fragment);
        entry = { program, api: programApi(gl, program) };
        programs.set(id, entry);
      }
      gl.useProgram(entry.program);
      return entry.api;
    },
    resize(width, height) {
      canvas.width = width;
      canvas.height = height;
      gl.viewport(0, 0, width, height);
    },
    draw() {
      gl.drawArrays(gl.TRIANGLES, 0, 3);
    },
    dispose() {
      for (const { program } of programs.values()) gl.deleteProgram(program);
      programs.clear();
      gl.deleteVertexArray(vao);
      // Fires `webglcontextlost`: callers must detach their listener first.
      gl.getExtension("WEBGL_lose_context")?.loseContext();
    },
  };
}
