import { expect, test, type Page, type Route } from "@playwright/test";

interface MockTemplateRecord {
  templateId: string;
  sceneKey: string;
  sceneName: string;
  name: string;
  description: string;
  defaultTitle: string;
  contentMd: string;
  sort: number;
  builtin: boolean;
  enabled: boolean;
  createdAt: string;
  updatedAt: string;
}

interface MockTreeNode {
  id: string;
  documentId?: string;
  documentIdentifier?: string;
  documentRouteKey?: string;
  spaceId: string;
  parentId: string | null;
  type: "doc" | "folder";
  title: string;
  sort: number;
  visibility?: "public" | "authenticated" | "member";
  children: MockTreeNode[];
}

interface MockDocumentRecord {
  id: string;
  nodeId: string;
  themeId: string;
  title: string;
  contentMd: string;
  version: number;
  updatedAt: string;
  visibility: "public" | "authenticated" | "member";
}

function nowIso(): string {
  return new Date().toISOString();
}

async function fulfillJsonResult(
  route: Route,
  data: unknown,
  options?: { status?: number; code?: number; message?: string }
): Promise<void> {
  await route.fulfill({
    status: options?.status ?? 200,
    headers: {
      "content-type": "application/json; charset=utf-8"
    },
    body: JSON.stringify({
      code: options?.code ?? 0,
      message: options?.message ?? "ok",
      data
    })
  });
}

async function installWorkspaceTemplateCreateMocks(page: Page): Promise<{
  getLastCreateNodePayload: () => Record<string, unknown> | null;
}> {
  const createdAt = nowIso();
  const templates: MockTemplateRecord[] = [
    {
      templateId: "tpl-meeting",
      sceneKey: "meeting",
      sceneName: "会议",
      name: "会议纪要模板",
      description: "用于 E2E 冒烟",
      defaultTitle: "会议纪要",
      contentMd: "# 会议模板\n\n模板内容初始化成功",
      sort: 10,
      builtin: false,
      enabled: true,
      createdAt,
      updatedAt: createdAt
    }
  ];

  let treeNodes: MockTreeNode[] = [
    {
      id: "node-initial-1",
      documentId: "doc-initial-1",
      documentIdentifier: "intro",
      documentRouteKey: "intro",
      spaceId: "e2e-space",
      parentId: null,
      type: "doc",
      title: "介绍文档",
      sort: 1,
      visibility: "member",
      children: []
    }
  ];
  const documents = new Map<string, MockDocumentRecord>([
    [
      "doc-initial-1",
      {
        id: "doc-initial-1",
        nodeId: "node-initial-1",
        themeId: "default",
        title: "介绍文档",
        contentMd: "# intro",
        version: 1,
        updatedAt: createdAt,
        visibility: "member"
      }
    ]
  ]);
  let createNodeCount = 0;
  let lastCreateNodePayload: Record<string, unknown> | null = null;

  await page.route("**/api/**", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const method = request.method().toUpperCase();
    const path = url.pathname;

    if (path === "/api/auth/session" && method === "GET") {
      await fulfillJsonResult(route, {
        user: {
          id: "editor-user-1",
          email: "editor@example.com",
          name: "Editor User"
        },
        token: "e2e-token"
      });
      return;
    }

    if (path === "/api/auth/options" && method === "GET") {
      await fulfillJsonResult(route, {
        loginMode: "local_only",
        defaultProviderId: "local",
        allowUserRegister: true,
        providers: []
      });
      return;
    }

    if (path === "/api/themes" && method === "GET") {
      await fulfillJsonResult(route, []);
      return;
    }

    if (path === "/api/image-hosting" && method === "GET") {
      await fulfillJsonResult(route, {});
      return;
    }

    if (path === "/api/spaces" && method === "GET") {
      await fulfillJsonResult(route, [
        {
          id: "e2e-space",
          name: "E2E 空间",
          createdAt,
          updatedAt: createdAt
        }
      ]);
      return;
    }

    if (path === "/api/spaces/e2e-space" && method === "GET") {
      await fulfillJsonResult(route, {
        id: "e2e-space",
        name: "E2E 空间",
        createdAt,
        updatedAt: createdAt
      });
      return;
    }

    if (path === "/api/spaces/e2e-space/tree" && method === "GET") {
      await fulfillJsonResult(route, treeNodes);
      return;
    }

    if (path === "/api/spaces/e2e-space/nodes" && method === "POST") {
      const payload = (request.postDataJSON() ?? {}) as Record<string, unknown>;
      lastCreateNodePayload = payload;
      createNodeCount += 1;

      const nodeId = `node-e2e-${createNodeCount}`;
      const docId = `doc-e2e-${createNodeCount}`;
      const title =
        typeof payload.title === "string" && payload.title.trim() ? payload.title.trim() : "未命名文档";
      const documentIdentifier =
        typeof payload.documentIdentifier === "string" ? payload.documentIdentifier.trim() : "";
      const templateId = typeof payload.templateId === "string" ? payload.templateId.trim() : "";
      const matchedTemplate = templates.find((item) => item.templateId === templateId) ?? null;

      const updatedAt = nowIso();
      treeNodes = [
        ...treeNodes,
        {
          id: nodeId,
          documentId: docId,
          documentIdentifier: documentIdentifier || undefined,
          documentRouteKey: documentIdentifier || docId,
          spaceId: "e2e-space",
          parentId: null,
          type: "doc",
          title,
          sort: treeNodes.length + 1,
          visibility: "member",
          children: []
        }
      ];
      documents.set(docId, {
        id: docId,
        nodeId,
        themeId: "default",
        title,
        contentMd: matchedTemplate?.contentMd ?? "",
        version: 1,
        updatedAt,
        visibility: "member"
      });
      await fulfillJsonResult(route, {
        nodeId,
        docId
      });
      return;
    }

    const docAttachmentMatch = path.match(/^\/api\/docs\/([^/]+)\/attachments$/);
    if (docAttachmentMatch && method === "GET") {
      await fulfillJsonResult(route, {
        items: []
      });
      return;
    }

    const docMatch = path.match(/^\/api\/docs\/([^/]+)$/);
    if (docMatch) {
      const docId = decodeURIComponent(docMatch[1]);
      const document = documents.get(docId);
      if (!document) {
        await fulfillJsonResult(route, null, {
          status: 404,
          code: 4040,
          message: "文档不存在"
        });
        return;
      }
      if (method === "GET") {
        await fulfillJsonResult(route, document);
        return;
      }
      if (method === "PUT") {
        const payload = (request.postDataJSON() ?? {}) as Record<string, unknown>;
        const nextContentMd =
          typeof payload.contentMd === "string" ? payload.contentMd : document.contentMd;
        const nextVersion =
          typeof payload.baseVersion === "number"
            ? Math.max(Math.trunc(payload.baseVersion) + 1, document.version + 1)
            : document.version + 1;
        const updated: MockDocumentRecord = {
          ...document,
          contentMd: nextContentMd,
          version: nextVersion,
          updatedAt: nowIso()
        };
        documents.set(docId, updated);
        await fulfillJsonResult(route, { document: updated });
        return;
      }
    }

    if (path === "/api/document-templates" && method === "GET") {
      await fulfillJsonResult(route, {
        items: templates.map((item) => ({
          templateId: item.templateId,
          sceneKey: item.sceneKey,
          sceneName: item.sceneName,
          name: item.name,
          description: item.description,
          defaultTitle: item.defaultTitle,
          sort: item.sort,
          builtin: item.builtin,
          enabled: item.enabled,
          updatedAt: item.updatedAt
        })),
        pagination: {
          page: 1,
          pageSize: 100,
          total: templates.length
        }
      });
      return;
    }

    const templateDetailMatch = path.match(/^\/api\/document-templates\/([^/]+)$/);
    if (templateDetailMatch && method === "GET") {
      const templateId = decodeURIComponent(templateDetailMatch[1]);
      const template = templates.find((item) => item.templateId === templateId);
      if (!template) {
        await fulfillJsonResult(route, null, {
          status: 404,
          code: 4040,
          message: "模板不存在"
        });
        return;
      }
      await fulfillJsonResult(route, template);
      return;
    }

    await fulfillJsonResult(route, null, {
      status: 404,
      code: 4040,
      message: `unhandled mock api: ${method} ${path}`
    });
  });

  return {
    getLastCreateNodePayload: () => lastCreateNodePayload
  };
}

test.describe("Workspace Template Create Smoke", () => {
  test("creates a document with template and custom identifier", async ({ page }) => {
    const mocks = await installWorkspaceTemplateCreateMocks(page);
    await page.goto("/editor/e2e-space");

    const quickMenuButton = page.getByLabel("打开目录快捷菜单");
    await expect(quickMenuButton).toBeVisible();
    await quickMenuButton.click();
    await page.getByRole("menuitem", { name: "新建文档" }).click();

    await expect(page.getByRole("heading", { name: "新建文档" })).toBeVisible();
    await page.getByLabel("文档标题").fill("会议纪要文档");
    await page.getByLabel("模板（可选）").selectOption("tpl-meeting");
    await expect(page.getByText("会议纪要模板")).toBeVisible();
    await expect(page.getByText("模板内容初始化成功")).toBeVisible();
    await page.getByLabel("文档标识（可空）").fill("meeting-seo");
    await page.getByRole("button", { name: "创建" }).click();

    await expect(page.getByRole("heading", { name: "新建文档" })).toHaveCount(0);
    await expect(page).toHaveURL(/\/editor\/e2e-space\/doc-e2e-1$/);
    await expect(page.locator("#workspace-tree-item-node-e2e-1")).toContainText("会议纪要文档");
    await expect(page.getByText("模板内容初始化成功")).toBeVisible();

    expect(mocks.getLastCreateNodePayload()).toMatchObject({
      parentId: null,
      type: "doc",
      title: "会议纪要文档",
      documentIdentifier: "meeting-seo",
      templateId: "tpl-meeting"
    });
  });
});
