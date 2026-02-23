import { createHttpAdapter } from "./http/adapter";
import type { DataGateway } from "./types";

let singletonGateway: DataGateway | null = null;

export function getDataGateway(): DataGateway {
  if (singletonGateway) {
    return singletonGateway;
  }

  const baseUrl = import.meta.env.VITE_API_BASE_URL ?? "/api";
  singletonGateway = createHttpAdapter({ baseUrl });
  return singletonGateway;
}

export * from "./types";
export * from "./auth-events";
