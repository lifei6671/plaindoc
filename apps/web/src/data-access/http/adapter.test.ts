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
});
