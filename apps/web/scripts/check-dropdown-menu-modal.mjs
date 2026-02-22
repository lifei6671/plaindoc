import fs from "node:fs";
import path from "node:path";

// 约束说明：
// 1) 业务代码只允许使用封装后的 DropdownMenu 组件；
// 2) 所有 DropdownMenu 禁止 modal=true；
// 3) 允许不写 modal（由封装默认 modal=false）。

const webRoot = process.cwd();
const srcRoot = path.join(webRoot, "src");
const wrapperFile = path.join(srcRoot, "components", "ui", "dropdown-menu.tsx");

/**
 * 递归收集目录下的代码文件。
 * 仅扫描 ts/tsx/js/jsx，避免处理构建产物。
 */
function collectSourceFiles(directoryPath) {
  const collectedFiles = [];
  const entries = fs.readdirSync(directoryPath, { withFileTypes: true });
  for (const entry of entries) {
    if (entry.name.startsWith(".")) {
      continue;
    }
    const fullPath = path.join(directoryPath, entry.name);
    if (entry.isDirectory()) {
      collectedFiles.push(...collectSourceFiles(fullPath));
      continue;
    }
    if (!/\.(tsx?|jsx?)$/i.test(entry.name)) {
      continue;
    }
    collectedFiles.push(fullPath);
  }
  return collectedFiles;
}

/**
 * 将字符偏移转换为行号（1-based），用于输出可读错误。
 */
function toLineNumber(fileText, offset) {
  return fileText.slice(0, offset).split("\n").length;
}

const violations = [];
const sourceFiles = collectSourceFiles(srcRoot);

for (const filePath of sourceFiles) {
  const fileText = fs.readFileSync(filePath, "utf8");
  const relativePath = path.relative(webRoot, filePath);

  // 防止绕过封装：仅 `components/ui/dropdown-menu.tsx` 可以直接依赖 Radix 原始包。
  if (
    filePath !== wrapperFile &&
    /from\s+["']@radix-ui\/react-dropdown-menu["']/.test(fileText)
  ) {
    violations.push(
      `${relativePath}: 检测到直接依赖 @radix-ui/react-dropdown-menu，请改为使用封装组件。`
    );
  }

  // 扫描 <DropdownMenu ...> 标签，拦截 modal=true（包括 modal、modal={true}、modal="true"）。
  const dropdownTagPattern = /<DropdownMenu(?![A-Za-z])[\s\S]*?>/g;
  const tags = fileText.matchAll(dropdownTagPattern);
  for (const tagMatch of tags) {
    const fullTag = tagMatch[0];
    const offset = tagMatch.index ?? 0;
    const lineNumber = toLineNumber(fileText, offset);

    if (!/\bmodal\b/.test(fullTag)) {
      continue;
    }
    if (/\bmodal\s*=\s*\{\s*false\s*\}/.test(fullTag)) {
      continue;
    }

    violations.push(
      `${relativePath}:${lineNumber} 检测到 <DropdownMenu> 使用 modal=true（或等价写法），必须改为 modal={false} 或省略。`
    );
  }
}

if (violations.length > 0) {
  console.error("[check:dropdown-menu] 发现不符合规范的 DropdownMenu 用法：");
  for (const violation of violations) {
    console.error(`- ${violation}`);
  }
  process.exit(1);
}

console.log("[check:dropdown-menu] 通过：未发现 modal=true 或绕过封装的用法。");
