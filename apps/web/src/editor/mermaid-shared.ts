export interface MermaidRenderResult {
  errorMessage: string | null;
  svg: string;
}

export interface MermaidRenderCacheEntry extends MermaidRenderResult {
  pendingPromise?: Promise<MermaidRenderResult>;
}

export const MERMAID_RENDER_CACHE_MAX_ENTRIES = 200;

export function buildMermaidCacheKey(
  anchorDataAttributes: Record<string, string>,
  source: string
): string {
  const anchorIndex = anchorDataAttributes["data-anchor-index"]?.trim() || "mermaid";
  return `${anchorIndex}:${source}`;
}
