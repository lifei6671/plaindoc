import { monotonicFactory } from "ulid";

const DEFAULT_MAX_RETRY_COUNT = 8;
const createMonotonicUlid = monotonicFactory();

export interface GenerateLowercaseUlidOptions {
  maxRetryCount?: number;
  exists?: (candidate: string) => Promise<boolean>;
}

// 统一生成小写 ULID，并在必要时通过 exists 回调做去重重试。
export async function generateLowercaseUlid(
  options: GenerateLowercaseUlidOptions = {}
): Promise<string> {
  const maxRetryCount = options.maxRetryCount ?? DEFAULT_MAX_RETRY_COUNT;

  for (let attempt = 0; attempt < maxRetryCount; attempt += 1) {
    const candidate = createMonotonicUlid().toLowerCase();
    if (!options.exists) {
      return candidate;
    }
    const duplicated = await options.exists(candidate);
    if (!duplicated) {
      return candidate;
    }
  }

  throw new Error(`ULID 生成冲突，已重试 ${maxRetryCount} 次。`);
}
