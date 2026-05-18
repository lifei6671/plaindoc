import { createRequire } from "node:module";
import { dirname } from "node:path";
import type { AliasOptions } from "vite";

const requireFromConfig = createRequire(import.meta.url);

const codemirrorSingletonPackages = [
  "@codemirror/state",
  "@codemirror/view",
  "@codemirror/language",
  "@codemirror/commands",
  "@codemirror/search",
  "@codemirror/autocomplete",
  "@codemirror/lint",
  "@codemirror/theme-one-dark"
];

function resolvePackageRoot(packageName: string): string {
  return dirname(dirname(requireFromConfig.resolve(packageName)));
}

// CodeMirror 依赖 instanceof 判断。混用 npm/pnpm 或 workspace hoist 时，必须强制收敛 singleton 包。
export function createCodeMirrorResolve(): {
  alias: AliasOptions;
  dedupe: string[];
} {
  return {
    alias: codemirrorSingletonPackages.map((packageName) => ({
      find: packageName,
      replacement: resolvePackageRoot(packageName)
    })),
    dedupe: codemirrorSingletonPackages
  };
}
