import { afterEach, describe, expect, it, vi } from "vitest";
import type { AdminSpaceTransferEvent } from "../types";
import { createHttpAdapter } from "./adapter";

class FakeEventSource {
  static instances: FakeEventSource[] = [];

  onmessage: ((event: MessageEvent<string>) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;
  readonly listeners = new Map<string, EventListener[]>();
  readonly url: string;
  readonly withCredentials: boolean;

  constructor(url: string | URL, init?: EventSourceInit) {
    this.url = String(url);
    this.withCredentials = init?.withCredentials === true;
    FakeEventSource.instances.push(this);
  }

  addEventListener(type: string, listener: EventListener): void {
    const listeners = this.listeners.get(type) ?? [];
    listeners.push(listener);
    this.listeners.set(type, listeners);
  }

  close(): void {}

  emit(type: string, event: AdminSpaceTransferEvent): void {
    const message = new MessageEvent(type, { data: JSON.stringify(event) });
    if (type === "message") {
      this.onmessage?.(message);
    }
    for (const listener of this.listeners.get(type) ?? []) {
      listener(message);
    }
  }
}

describe("createHttpAdapter transfer subscriptions", () => {
  const originalEventSource = globalThis.EventSource;

  afterEach(() => {
    FakeEventSource.instances = [];
    globalThis.EventSource = originalEventSource;
    vi.restoreAllMocks();
  });

  it("normalizes transfer download URLs to the API origin", () => {
    globalThis.EventSource = FakeEventSource as unknown as typeof EventSource;
    const gateway = createHttpAdapter({ baseUrl: "https://api.example.test/api" });
    const onEvent = vi.fn();

    gateway.admin.subscribeSpaceExport({
      streamUrl: "/api/admin/spaces/space-a/exports/job-a/events?token=stream",
      onEvent
    });

    expect(FakeEventSource.instances[0]?.url).toBe("https://api.example.test/api/admin/spaces/space-a/exports/job-a/events?token=stream");

    FakeEventSource.instances[0]?.emit("completed", {
      type: "completed",
      progress: 100,
      downloadUrl: "/api/admin/space-exports/job-a/download?token=download"
    });

    expect(onEvent).toHaveBeenCalledWith(expect.objectContaining({
      downloadUrl: "https://api.example.test/api/admin/space-exports/job-a/download?token=download"
    }));
  });

  it("calls the global transfer task endpoints and normalizes returned URLs", async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url.endsWith("/api/admin/space-transfer-tasks?status=active")) {
        return new Response(JSON.stringify({
          code: 0,
          message: "success",
          data: {
            tasks: [
              {
                jobId: "job-a",
                kind: "space_export",
                status: "running",
                progress: 45,
                updatedAt: "2026-05-17T00:00:00Z"
              }
            ]
          }
        }), { status: 200, headers: { "Content-Type": "application/json" } });
      }
      if (url.endsWith("/api/admin/space-transfer-tasks/space_export/job-a/stream-token")) {
        return new Response(JSON.stringify({
          code: 0,
          message: "success",
          data: { streamUrl: "/api/admin/spaces/space-a/exports/job-a/events?token=stream" }
        }), { status: 200, headers: { "Content-Type": "application/json" } });
      }
      if (url.endsWith("/api/admin/space-transfer-tasks/space_export/job-a/download-token")) {
        return new Response(JSON.stringify({
          code: 0,
          message: "success",
          data: { downloadUrl: "/api/admin/space-exports/job-a/download?token=download" }
        }), { status: 200, headers: { "Content-Type": "application/json" } });
      }
      throw new Error(`unexpected request: ${url}`);
    });
    const originalFetch = globalThis.fetch;
    globalThis.fetch = fetchMock as unknown as typeof fetch;
    try {
      const gateway = createHttpAdapter({ baseUrl: "https://api.example.test/api" });

      await expect(gateway.admin.listSpaceTransferTasks({ status: "active" })).resolves.toEqual({
        tasks: [
          expect.objectContaining({
            jobId: "job-a",
            kind: "space_export",
            status: "running"
          })
        ]
      });
      await expect(gateway.admin.issueSpaceTransferStreamToken({
        kind: "space_export",
        jobId: "job-a"
      })).resolves.toEqual({
        streamUrl: "https://api.example.test/api/admin/spaces/space-a/exports/job-a/events?token=stream"
      });
      await expect(gateway.admin.issueSpaceTransferDownloadToken({
        kind: "space_export",
        jobId: "job-a"
      })).resolves.toEqual({
        downloadUrl: "https://api.example.test/api/admin/space-exports/job-a/download?token=download"
      });
    } finally {
      globalThis.fetch = originalFetch;
    }
  });
});
