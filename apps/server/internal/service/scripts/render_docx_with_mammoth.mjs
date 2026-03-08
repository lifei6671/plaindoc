import mammoth from "mammoth";

const IMAGE_PLACEHOLDER_PREFIX = "mammoth-image:";

function writeError(message) {
  process.stderr.write(`${String(message || "mammoth render failed").trim()}\n`);
  process.exit(1);
}

async function readStdin() {
  const chunks = [];
  for await (const chunk of process.stdin) {
    chunks.push(chunk);
  }
  return Buffer.concat(chunks).toString("utf8");
}

try {
  const rawInput = (await readStdin()).trim();
  if (!rawInput) {
    writeError("mammoth render input is empty");
  }

  const payload = JSON.parse(rawInput);
  const docxBase64 = typeof payload.docxBase64 === "string" ? payload.docxBase64.trim() : "";
  if (!docxBase64) {
    writeError("docxBase64 is required");
  }

  const assets = [];
  const { value, messages } = await mammoth.convertToHtml(
    {
      buffer: Buffer.from(docxBase64, "base64")
    },
    {
      convertImage: mammoth.images.imgElement(async (image) => {
        const id = `asset-${assets.length + 1}`;
        assets.push({
          id,
          contentType: typeof image.contentType === "string" ? image.contentType.trim() : "application/octet-stream",
          dataBase64: await image.readAsBase64String()
        });
        return {
          src: `${IMAGE_PLACEHOLDER_PREFIX}${id}`
        };
      })
    }
  );

  process.stdout.write(
    JSON.stringify({
      html: typeof value === "string" ? value : "",
      assets,
      messages: Array.isArray(messages) ? messages : []
    })
  );
} catch (error) {
  writeError(error instanceof Error ? error.message : String(error));
}
