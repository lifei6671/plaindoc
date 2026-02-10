import { createHttpAdapter } from "./http/adapter";
import { createLocalAdapter } from "./local/adapter";
import type { DataGateway } from "./types";

let singletonGateway: DataGateway | null = null;

function resolveDriver(): "local" | "http" {
  const raw = import.meta.env.VITE_DATA_DRIVER;
  if (raw === "http") {
    return "http";
  }
  return "local";
}

export function getDataGateway(): DataGateway {
  if (singletonGateway) {
    return singletonGateway;
  }

  const driver = resolveDriver();
  if (driver === "http") {
    const baseUrl = import.meta.env.VITE_API_BASE_URL ?? "/api";
    singletonGateway = createHttpAdapter({ baseUrl });
    return singletonGateway;
  }

  singletonGateway = createLocalAdapter();
  return singletonGateway;
}

export * from "./types";
