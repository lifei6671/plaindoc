const EXTERNAL_LINK_REL_TOKENS = ["noopener", "noreferrer", "nofollow"] as const;
const HTTP_PROTOCOLS = new Set(["http:", "https:"]);

export function normalizeLinkRequestOrigin(value: string | undefined): string {
  const normalized = (value ?? "").trim();
  if (!normalized) {
    return "";
  }
  try {
    const parsedURL = new URL(normalized);
    if (!HTTP_PROTOCOLS.has(parsedURL.protocol)) {
      return "";
    }
    return parsedURL.origin;
  } catch {
    return "";
  }
}

export function isExternalHTTPLink(rawHref: string, requestOrigin: string): boolean {
  const normalizedHref = rawHref.trim();
  if (!normalizedHref || normalizedHref.startsWith("#") || !requestOrigin) {
    return false;
  }
  try {
    const resolvedURL = new URL(normalizedHref, requestOrigin);
    if (!HTTP_PROTOCOLS.has(resolvedURL.protocol)) {
      return false;
    }
    return resolvedURL.origin !== requestOrigin;
  } catch {
    return false;
  }
}

export function mergeExternalLinkRel(rawRel: string): string {
  const tokenSet = new Set<string>();
  for (const token of rawRel.split(/\s+/)) {
    const normalizedToken = token.trim().toLowerCase();
    if (normalizedToken) {
      tokenSet.add(normalizedToken);
    }
  }
  for (const requiredToken of EXTERNAL_LINK_REL_TOKENS) {
    tokenSet.add(requiredToken);
  }
  return Array.from(tokenSet).join(" ");
}
