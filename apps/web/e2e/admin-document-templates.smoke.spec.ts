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

async function installAdminTemplateApiMocks(page: Page): Promise<void> {
  const templates: MockTemplateRecord[] = [];

  await page.route("**/api/**", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const method = request.method().toUpperCase();
    const path = url.pathname;

    if (path === "/api/auth/session" && method === "GET") {
      await fulfillJsonResult(route, {
        user: {
          id: "admin-user-1",
          email: "admin@example.com",
          name: "Platform Admin"
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

    if (path === "/api/admin/me" && method === "GET") {
      await fulfillJsonResult(route, {
        userId: "admin-user-1",
        email: "admin@example.com",
        name: "Platform Admin",
        avatarUrl: "",
        roles: ["platform_admin"]
      });
      return;
    }

    if (path === "/api/admin/operation-tokens" && method === "POST") {
      await fulfillJsonResult(route, {
        token: "e2e-operation-token"
      });
      return;
    }

    if (path === "/api/admin/document-templates" && method === "GET") {
      const keyword = (url.searchParams.get("keyword") ?? "").trim().toLowerCase();
      const filtered = !keyword
        ? templates
        : templates.filter((item) => {
            return (
              item.templateId.toLowerCase().includes(keyword) ||
              item.name.toLowerCase().includes(keyword) ||
              item.sceneKey.toLowerCase().includes(keyword) ||
              item.sceneName.toLowerCase().includes(keyword)
            );
          });
      await fulfillJsonResult(route, {
        items: filtered.map((item) => ({
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
          pageSize: 200,
          total: filtered.length
        }
      });
      return;
    }

    if (path === "/api/admin/document-templates" && method === "POST") {
      const payload = (request.postDataJSON() ?? {}) as Record<string, unknown>;
      const templateId = String(payload.templateId ?? "").trim();
      const existing = templates.find((item) => item.templateId === templateId);
      if (existing) {
        await fulfillJsonResult(route, null, {
          status: 409,
          code: 4090,
          message: "模板 ID 已存在"
        });
        return;
      }

      const createdAt = nowIso();
      const created: MockTemplateRecord = {
        templateId,
        sceneKey: String(payload.sceneKey ?? "").trim(),
        sceneName: String(payload.sceneName ?? "").trim(),
        name: String(payload.name ?? "").trim(),
        description: String(payload.description ?? ""),
        defaultTitle: String(payload.defaultTitle ?? ""),
        contentMd: String(payload.contentMd ?? ""),
        sort: Number.isFinite(Number(payload.sort)) ? Math.trunc(Number(payload.sort)) : 0,
        builtin: false,
        enabled: Boolean(payload.enabled ?? true),
        createdAt,
        updatedAt: createdAt
      };
      templates.push(created);
      await fulfillJsonResult(route, created);
      return;
    }

    const templateDetailMatch = path.match(/^\/api\/admin\/document-templates\/([^/]+)$/);
    if (templateDetailMatch) {
      const templateId = decodeURIComponent(templateDetailMatch[1]);
      const targetIndex = templates.findIndex((item) => item.templateId === templateId);
      if (targetIndex < 0) {
        await fulfillJsonResult(route, null, {
          status: 404,
          code: 4040,
          message: "模板不存在"
        });
        return;
      }

      if (method === "GET") {
        await fulfillJsonResult(route, templates[targetIndex]);
        return;
      }

      if (method === "PUT") {
        const payload = (request.postDataJSON() ?? {}) as Record<string, unknown>;
        const current = templates[targetIndex];
        const updated: MockTemplateRecord = {
          ...current,
          sceneKey: typeof payload.sceneKey === "string" ? payload.sceneKey.trim() : current.sceneKey,
          sceneName: typeof payload.sceneName === "string" ? payload.sceneName.trim() : current.sceneName,
          name: typeof payload.name === "string" ? payload.name.trim() : current.name,
          description: typeof payload.description === "string" ? payload.description : current.description,
          defaultTitle: typeof payload.defaultTitle === "string" ? payload.defaultTitle : current.defaultTitle,
          contentMd: typeof payload.contentMd === "string" ? payload.contentMd : current.contentMd,
          sort: Number.isFinite(Number(payload.sort)) ? Math.trunc(Number(payload.sort)) : current.sort,
          enabled: typeof payload.enabled === "boolean" ? payload.enabled : current.enabled,
          updatedAt: nowIso()
        };
        templates[targetIndex] = updated;
        await fulfillJsonResult(route, updated);
        return;
      }

      if (method === "DELETE") {
        templates.splice(targetIndex, 1);
        await fulfillJsonResult(route, null);
        return;
      }
    }

    await fulfillJsonResult(route, null, {
      status: 404,
      code: 4040,
      message: `unhandled mock api: ${method} ${path}`
    });
  });
}

test.describe("Admin Document Templates Smoke", () => {
  test("creates, searches, edits and deletes template from admin page", async ({ page }) => {
    await installAdminTemplateApiMocks(page);
    await page.goto("/admin/document-templates");

    await expect(page.getByRole("button", { name: "新建模板" })).toBeVisible();
    await page.getByRole("button", { name: "新建模板" }).click();

    const createDialog = page.getByRole("dialog", { name: "新建文档模板" });
    await expect(createDialog).toBeVisible();
    await createDialog.getByLabel("模板 ID").fill("e2e-template");
    await createDialog.getByLabel("场景标识").fill("e2e-scene");
    await createDialog.getByLabel("场景名称").fill("E2E 场景");
    await createDialog.getByLabel("模板名称").fill("E2E 模板");
    await createDialog.getByLabel("模板描述").fill("created by playwright smoke");
    await createDialog.getByLabel("模板内容").fill("# E2E Template");
    await createDialog.getByRole("button", { name: "创建模板" }).click();

    const createdRow = page.locator("tr", { hasText: "e2e-template" });
    await expect(createdRow).toBeVisible();

    const searchInput = page.getByPlaceholder("按模板 ID / 名称 / 场景搜索");
    await searchInput.fill("e2e-template");
    await page.getByRole("button", { name: "搜索" }).click();
    await expect(page.locator("tr", { hasText: "e2e-template" })).toBeVisible();

    await createdRow.getByRole("button", { name: "编辑" }).click();
    const editDialog = page.getByRole("dialog", { name: /编辑模板/ });
    await expect(editDialog).toBeVisible();
    await editDialog.getByLabel("模板名称").fill("E2E 模板（更新）");
    await editDialog.getByLabel("模板描述").fill("updated by playwright smoke");
    await editDialog.getByLabel("模板内容").fill("# E2E Template Updated");
    await editDialog.getByRole("button", { name: "保存修改" }).click();

    const updatedRow = page.locator("tr", { hasText: "e2e-template" });
    await expect(updatedRow).toContainText("E2E 模板（更新）");

    await updatedRow.getByRole("button", { name: "删除" }).click();
    const deleteDialog = page.getByRole("dialog", { name: "删除文档模板" });
    await expect(deleteDialog).toBeVisible();
    await deleteDialog.getByRole("button", { name: "确认删除" }).click();

    await expect(page.locator("tr", { hasText: "e2e-template" })).toHaveCount(0);
    await expect(page.getByText("暂无模板")).toBeVisible();
  });
});
