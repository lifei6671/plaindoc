export type WorkerMessageType = "handshake" | "render";

export interface WorkerHandshakeRequest {
  type: "handshake";
  version: string;
}

export interface WorkerHandshakeResponse {
  type: "handshake";
  ok: boolean;
  version: string;
  error?: WorkerRenderError;
}

export interface WorkerRenderRequest<TPayload = unknown> {
  id: string;
  type: "render";
  version: string;
  route: string;
  deadlineMs?: number;
  payload: TPayload;
}

export interface WorkerRenderHead {
  title?: string;
  description?: string;
  canonical?: string;
}

export interface WorkerRenderMetrics {
  renderMs?: number;
  payloadBytes?: number;
}

export interface WorkerRenderError {
  code: string;
  message: string;
}

export interface WorkerRenderResponse {
  id: string;
  ok: boolean;
  html?: string;
  head?: WorkerRenderHead;
  error?: WorkerRenderError;
  metrics?: WorkerRenderMetrics;
}

export type WorkerRequest<TPayload = unknown> =
  | WorkerHandshakeRequest
  | WorkerRenderRequest<TPayload>;

export type WorkerResponse = WorkerHandshakeResponse | WorkerRenderResponse;

export function isRenderRequest<TPayload = unknown>(
  value: WorkerRequest<TPayload>
): value is WorkerRenderRequest<TPayload> {
  return value.type === "render";
}

export function isHandshakeRequest<TPayload = unknown>(
  value: WorkerRequest<TPayload>
): value is WorkerHandshakeRequest {
  return value.type === "handshake";
}
