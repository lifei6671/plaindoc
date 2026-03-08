import TurndownService from "turndown";
import { gfm } from "turndown-plugin-gfm";

function writeError(message) {
  process.stderr.write(`${String(message || "html to markdown failed").trim()}\n`);
  process.exit(1);
}

async function readStdin() {
  const chunks = [];
  for await (const chunk of process.stdin) {
    chunks.push(chunk);
  }
  return Buffer.concat(chunks).toString("utf8");
}

function normalizeHtml(rawHtml) {
  const html = typeof rawHtml === "string" ? rawHtml.trim() : "";
  if (!html) {
    return "";
  }
  const bodyMatch = html.match(/<body\b[^>]*>([\s\S]*?)<\/body>/i);
  const bodyHTML = bodyMatch ? bodyMatch[1] : html;
  return bodyHTML
    .replace(/<script\b[^>]*>[\s\S]*?<\/script>/gi, "")
    .replace(/<style\b[^>]*>[\s\S]*?<\/style>/gi, "")
    .replace(/<noscript\b[^>]*>[\s\S]*?<\/noscript>/gi, "")
    .trim();
}

try {
  const rawInput = (await readStdin()).trim();
  if (!rawInput) {
    writeError("html to markdown input is empty");
  }

  const payload = JSON.parse(rawInput);
  const html = normalizeHtml(payload.html);
  if (!html) {
    process.stdout.write(JSON.stringify({ markdown: "" }));
    process.exit(0);
  }

  const service = new TurndownService({
    headingStyle: "atx",
    bulletListMarker: "-",
    codeBlockStyle: "fenced",
    emDelimiter: "_",
    strongDelimiter: "**"
  });
  service.use(gfm);

  service.addRule("preserveOfficeImages", {
    filter(node) {
      return node.nodeName === "IMG";
    },
    replacement(content, node) {
      const src = typeof node.getAttribute === "function" ? (node.getAttribute("src") || "").trim() : "";
      if (!src) {
        return "";
      }
      const alt = typeof node.getAttribute === "function" ? (node.getAttribute("alt") || "").trim() : "";
      const escapedAlt = alt.replace(/\]/g, "\\]");
      return `![${escapedAlt}](${src})`;
    }
  });

  service.addRule("dropHiddenButtons", {
    filter(node) {
      return node.nodeName === "BUTTON";
    },
    replacement() {
      return "";
    }
  });

  process.stdout.write(
    JSON.stringify({
      markdown: service.turndown(html).trim()
    })
  );
} catch (error) {
  writeError(error instanceof Error ? error.message : String(error));
}
