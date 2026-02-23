import { stdin, stderr, stdout } from "process";
import { createInterface } from "readline";
import {
  isHandshakeRequest,
  isRenderRequest,
  type WorkerHandshakeResponse,
  type WorkerRenderError,
  type WorkerRenderResponse,
  type WorkerRequest
} from "./protocol";
import { renderSpaceReader } from "./render-space-reader";
import type { ReaderPagePayload } from "./ssr-types";

const SSR_ROUTE_SPACE_READER = "space-reader";
const PROTOCOL_VERSION = (process.env.SSR_PROTOCOL_VERSION ?? "v1").trim() || "v1";

function writeJSONLine(value: unknown): void {
  stdout.write(`${JSON.stringify(value)}\n`);
}

function toRenderError(code: string, message: string): WorkerRenderError {
  return {
    code,
    message: message.trim() || "unknown error"
  };
}

function writeRenderErrorResponse(id: string, code: string, message: string): void {
  const response: WorkerRenderResponse = {
    id,
    ok: false,
    error: toRenderError(code, message)
  };
  writeJSONLine(response);
}

function handleHandshakeRequest(request: WorkerRequest<ReaderPagePayload>): void {
  const response: WorkerHandshakeResponse = {
    type: "handshake",
    ok: request.version === PROTOCOL_VERSION,
    version: PROTOCOL_VERSION
  };
  if (!response.ok) {
    response.error = toRenderError(
      "PROTOCOL_VERSION_MISMATCH",
      `protocol version mismatch: expected=${PROTOCOL_VERSION} got=${request.version}`
    );
  }
  writeJSONLine(response);
}

function handleRenderRequest(request: WorkerRequest<ReaderPagePayload>): void {
  const requestID = isRenderRequest(request) ? request.id : "";
  if (!isRenderRequest(request)) {
    writeRenderErrorResponse("", "INVALID_REQUEST_TYPE", "request type must be render");
    return;
  }
  if ((request.id ?? "").trim() === "") {
    writeRenderErrorResponse("", "INVALID_REQUEST_ID", "request id is required");
    return;
  }
  if (request.version !== PROTOCOL_VERSION) {
    writeRenderErrorResponse(
      request.id,
      "PROTOCOL_VERSION_MISMATCH",
      `protocol version mismatch: expected=${PROTOCOL_VERSION} got=${request.version}`
    );
    return;
  }
  if ((request.route ?? "").trim() !== SSR_ROUTE_SPACE_READER) {
    writeRenderErrorResponse(
      request.id,
      "UNSUPPORTED_ROUTE",
      `unsupported route: ${request.route ?? "(empty)"}`
    );
    return;
  }

  try {
    const rendered = renderSpaceReader(request.payload);
    const response: WorkerRenderResponse = {
      id: request.id,
      ok: true,
      html: rendered.html,
      head: rendered.head,
      metrics: rendered.metrics
    };
    writeJSONLine(response);
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    stderr.write(`[ssr-worker] render failed id=${requestID} message=${message}\n`);
    writeRenderErrorResponse(request.id, "RENDER_FAILED", message);
  }
}

function handleRequestLine(rawLine: string): void {
  const line = rawLine.trim();
  if (!line) {
    return;
  }

  let request: WorkerRequest<ReaderPagePayload>;
  try {
    request = JSON.parse(line) as WorkerRequest<ReaderPagePayload>;
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    stderr.write(`[ssr-worker] decode request failed message=${message}\n`);
    writeRenderErrorResponse("", "INVALID_JSON", message);
    return;
  }

  if (isHandshakeRequest(request)) {
    handleHandshakeRequest(request);
    return;
  }
  handleRenderRequest(request);
}

const lineReader = createInterface({
  input: stdin,
  crlfDelay: Infinity
});

lineReader.on("line", (line: string) => {
  try {
    handleRequestLine(line);
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    stderr.write(`[ssr-worker] unhandled line error message=${message}\n`);
    writeRenderErrorResponse("", "UNHANDLED_WORKER_ERROR", message);
  }
});

lineReader.on("close", () => {
  stderr.write("[ssr-worker] stdin closed, worker exiting\n");
});
