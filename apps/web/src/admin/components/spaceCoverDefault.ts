const COVER_MAX_WIDTH = 1600;
const COVER_MAX_HEIGHT = 2560;
const COVER_MAX_TITLE_LINES = 3;
const COVER_MAX_TITLE_UNITS_PER_LINE = 10.8;
const SYSTEM_COVER_TEXT_MARGIN = 128;
const SYSTEM_COVER_FONT_FAMILY = "PingFang SC, Microsoft YaHei, Noto Sans SC, Source Han Sans SC, sans-serif";
const SYSTEM_COVER_LINE_HEIGHT_RATIO = 1.16;
const SYSTEM_COVER_BG_COLOR = "#d9e3f2";
const SYSTEM_COVER_TEXT_COLOR = "#2f3f5f";
const SYSTEM_COVER_TITLE_BASELINE_RATIO = 0.31;

function estimateCoverTitleUnits(text: string): number {
  let units = 0;
  for (const char of text) {
    if (char === " ") {
      units += 0.4;
      continue;
    }
    if (/[\x20-\x7E]/.test(char)) {
      units += 0.58;
      continue;
    }
    units += 1;
  }
  return units;
}

function trimCoverTitleWithEllipsis(text: string): string {
  const runes = [...text];
  if (runes.length <= 1) {
    return "…";
  }
  return `${runes.slice(0, -1).join("")}…`;
}

function splitSystemCoverTitleLines(rawTitle: string): string[] {
  const normalized = rawTitle.trim().replace(/\s+/g, " ");
  if (!normalized) {
    return ["未命名空间"];
  }

  const tokens = normalized.match(/[A-Za-z0-9._-]+|\s+|./g) ?? [normalized];
  const lines: string[] = [];
  let current = "";

  const pushCurrent = () => {
    const value = current.trim();
    if (value) {
      lines.push(value);
    }
  };

  for (let index = 0; index < tokens.length; index += 1) {
    const token = tokens[index];
    if (!token.trim() && (!current || current.endsWith(" "))) {
      continue;
    }

    const next = current + token;
    if (estimateCoverTitleUnits(next) <= COVER_MAX_TITLE_UNITS_PER_LINE) {
      current = next;
      continue;
    }

    if (!current.trim()) {
      current = token.trim();
    }

    if (lines.length >= COVER_MAX_TITLE_LINES - 1) {
      const rest = `${current}${tokens.slice(index).join("")}`.trim();
      let candidate = rest;
      while (estimateCoverTitleUnits(candidate) > COVER_MAX_TITLE_UNITS_PER_LINE && candidate.length > 1) {
        candidate = trimCoverTitleWithEllipsis(candidate);
      }
      if (!candidate.endsWith("…")) {
        candidate = `${candidate}…`;
      }
      lines.push(candidate);
      return lines;
    }

    pushCurrent();
    current = token.trimStart();
  }

  pushCurrent();
  if (lines.length === 0) {
    return ["未命名空间"];
  }
  return lines.slice(0, COVER_MAX_TITLE_LINES);
}

function escapeSVGText(value: string): string {
  return value
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&apos;");
}

let systemCoverMeasureContext: CanvasRenderingContext2D | null = null;

function getSystemCoverMeasureContext(): CanvasRenderingContext2D | null {
  if (typeof document === "undefined") {
    return null;
  }
  if (!systemCoverMeasureContext) {
    const canvas = document.createElement("canvas");
    systemCoverMeasureContext = canvas.getContext("2d");
  }
  return systemCoverMeasureContext;
}

function measureSystemCoverTextWidth(text: string, fontSize: number): number {
  const context = getSystemCoverMeasureContext();
  if (!context) {
    return estimateCoverTitleUnits(text) * fontSize;
  }
  context.font = `${fontSize}px ${SYSTEM_COVER_FONT_FAMILY}`;
  return context.measureText(text).width;
}

function buildSystemCoverSVG(spaceName: string): string {
  const lines = splitSystemCoverTitleLines(spaceName);
  const maxUnits = Math.max(...lines.map((line) => estimateCoverTitleUnits(line)));
  let fontSize = maxUnits > 11 ? 148 : maxUnits > 9 ? 164 : 178;
  const maxTextWidth = COVER_MAX_WIDTH - SYSTEM_COVER_TEXT_MARGIN * 2;
  const measuredMaxWidth = Math.max(...lines.map((line) => measureSystemCoverTextWidth(line, fontSize)));
  if (measuredMaxWidth > maxTextWidth) {
    const scale = maxTextWidth / measuredMaxWidth;
    fontSize = Math.max(1, Math.floor(fontSize * scale));
  }
  const lineHeight = Math.round(fontSize * SYSTEM_COVER_LINE_HEIGHT_RATIO);
  const startY = Math.round(COVER_MAX_HEIGHT * SYSTEM_COVER_TITLE_BASELINE_RATIO);
  const tspans = lines
    .map((line, index) => {
      const dy = index === 0 ? 0 : lineHeight;
      return `<tspan x="${SYSTEM_COVER_TEXT_MARGIN}" dy="${dy}">${escapeSVGText(line)}</tspan>`;
    })
    .join("");

  return `<svg xmlns="http://www.w3.org/2000/svg" width="${COVER_MAX_WIDTH}" height="${COVER_MAX_HEIGHT}" viewBox="0 0 ${COVER_MAX_WIDTH} ${COVER_MAX_HEIGHT}">
<rect width="100%" height="100%" fill="${SYSTEM_COVER_BG_COLOR}"/>
<g opacity="0.3">
  <ellipse cx="300" cy="2610" rx="880" ry="470" fill="#edf2fb"/>
</g>
<g opacity="0.52">
  <path d="M -260 2230 C 180 1880 670 1910 1070 2205 C 1230 2320 1360 2442 1470 2560 L -260 2560 Z" fill="#cad8ee"/>
</g>
<g opacity="0.64">
  <path d="M -230 1770 C 90 1600 420 1710 728 1988 C 938 2174 1098 2348 1222 2560 L 852 2560 C 733 2364 614 2226 472 2096 C 248 1888 18 1794 -230 1846 Z" fill="#b4caec"/>
</g>
<g opacity="0.68">
  <path d="M -320 1870 C -52 1722 190 1782 392 1944 C 584 2102 698 2280 768 2560 L 444 2560 C 382 2388 300 2260 185 2154 C 68 2042 -78 1972 -320 2000 Z" fill="#a2bcec"/>
</g>
<text x="${SYSTEM_COVER_TEXT_MARGIN}" y="${startY}" fill="${SYSTEM_COVER_TEXT_COLOR}" font-size="${fontSize}" font-weight="600" font-family="${SYSTEM_COVER_FONT_FAMILY}">
${tspans}
</text>
</svg>`;
}

export async function exportSystemGeneratedWebP(
  spaceName: string,
  quality = 0.9
): Promise<{ file: File; width: number; height: number }> {
  const svg = buildSystemCoverSVG(spaceName);
  const svgBlob = new Blob([svg], { type: "image/svg+xml;charset=utf-8" });
  const svgURL = URL.createObjectURL(svgBlob);

  try {
    const image = await new Promise<HTMLImageElement>((resolve, reject) => {
      const value = new Image();
      value.onload = () => resolve(value);
      value.onerror = () => reject(new Error("系统封面渲染失败"));
      value.src = svgURL;
    });

    const canvas = document.createElement("canvas");
    canvas.width = COVER_MAX_WIDTH;
    canvas.height = COVER_MAX_HEIGHT;
    const context = canvas.getContext("2d");
    if (!context) {
      throw new Error("浏览器不支持 Canvas 2D，无法生成系统封面");
    }
    context.imageSmoothingEnabled = true;
    context.imageSmoothingQuality = "high";
    context.drawImage(image, 0, 0, COVER_MAX_WIDTH, COVER_MAX_HEIGHT);

    const webpBlob = await new Promise<Blob>((resolve, reject) => {
      canvas.toBlob(
        (value) => {
          if (!value) {
            reject(new Error("系统封面导出失败"));
            return;
          }
          resolve(value);
        },
        "image/webp",
        quality
      );
    });

    return {
      file: new File([webpBlob], `space-cover-${Date.now()}.webp`, {
        type: "image/webp",
        lastModified: Date.now()
      }),
      width: COVER_MAX_WIDTH,
      height: COVER_MAX_HEIGHT
    };
  } finally {
    URL.revokeObjectURL(svgURL);
  }
}
