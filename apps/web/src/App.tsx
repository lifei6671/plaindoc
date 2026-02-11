import { markdown, markdownLanguage } from "@codemirror/lang-markdown";
import { languages } from "@codemirror/language-data";
import { EditorSelection } from "@codemirror/state";
import { EditorView } from "@codemirror/view";
import CodeMirror from "@uiw/react-codemirror";
import { AlertCircle, CheckCircle2, LoaderCircle, Monitor, Settings2, Smartphone } from "lucide-react";
import MarkdownIt from "markdown-it";
// KaTeX mhchem 扩展：支持 `\\ce{}` 化学公式语法。
import "katex/contrib/mhchem";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import ReactMarkdown from "react-markdown";
import rehypeKatex from "rehype-katex";
import rehypeRaw from "rehype-raw";
import rehypeSanitize from "rehype-sanitize";
import remarkGfm from "remark-gfm";
import remarkMath from "remark-math";
import { SettingsLayer } from "./components/SettingsLayer";
import { ThemeMenu } from "./components/ThemeMenu";
import { TocMenu } from "./components/TocMenu";
import { TopToast, type TopToastVariant } from "./components/TopToast";
import { ConflictError, getDataGateway } from "./data-access";
import {
  DEFAULT_PREVIEW_THEME_ID,
  FALLBACK_CONTENT,
  PREVIEW_BODY_CLASS,
  PREVIEW_BODY_ID,
  PREVIEW_CUSTOM_STYLE_EVENT,
  PREVIEW_CUSTOM_STYLE_STORAGE_KEY,
  PREVIEW_PANE_CLASS,
  PREVIEW_PANE_ID,
  PREVIEW_THEME_STORAGE_KEY,
  PREVIEW_VIEWPORT_MODE_STORAGE_KEY
} from "./editor/constants";
import { buildMarkdownComponents } from "./editor/markdown-components";
import {
  extractPlainTextFromMarkdown,
  parseTocFromMarkdown,
  remarkBlockAnchorPlugin
} from "./editor/markdown-utils";
import {
  PREVIEW_HTML_SANITIZE_SCHEMA,
  PREVIEW_MARKDOWN_REHYPE_OPTIONS
} from "./editor/markdown-sanitize";
import {
  buildPreviewThemeStyleText,
  getPreviewThemeClassName,
  normalizePreviewStyleText
} from "./editor/preview-style";
import {
  findFirstDocId,
  formatError,
  formatSavedTime,
  resolveSaveIndicatorVariant
} from "./editor/status-utils";
import type { PreviewViewportMode, SaveStatus } from "./editor/types";
import { useScrollSync } from "./editor/use-scroll-sync";
import { copyPreviewToWechat } from "./editor/wechat-export";
import { PREVIEW_THEME_TEMPLATES, resolvePreviewTheme } from "./preview-themes";
import {
  DEFAULT_IMAGE_HOSTING_CONFIG,
  cloneImageHostingConfig,
  normalizeImageHostingConfig,
  type ImageHostingConfig
} from "./settings/image-hosting";
import { uploadImageToDefaultHosting } from "./settings/image-hosting-upload";

// 扩展 window 类型，支持外部注入预览样式字符串。
declare global {
  interface Window {
    __PLAINDOC_PREVIEW_STYLE__?: string;
  }
}

interface AppToastState {
  isOpen: boolean;
  message: string;
  variant: TopToastVariant;
  triggerKey: number;
}

const TEMP_USER_ID = 1;
const IMAGE_HOSTING_CONFIG_KEY = "image_hosting";

// 从剪贴板事件中提取图片文件列表：优先 items，兜底 files。
function extractImageFilesFromClipboard(event: ClipboardEvent): File[] {
  const clipboardData = event.clipboardData;
  if (!clipboardData) {
    return [];
  }

  const imageFilesFromItems = Array.from(clipboardData.items)
    .filter((item) => item.kind === "file" && item.type.startsWith("image/"))
    .map((item) => item.getAsFile())
    .filter((file): file is File => file instanceof File);
  if (imageFilesFromItems.length) {
    return imageFilesFromItems;
  }

  return Array.from(clipboardData.files).filter((file) => file.type.startsWith("image/"));
}

// 生成 Markdown 图片文案：优先复用文件名，缺失时回退到 image-x。
function buildImageMarkdownLine(file: File, url: string, index: number): string {
  const rawName = file.name.replace(/\.[^.]+$/, "").trim();
  const altText = rawName || `image-${index + 1}`;
  return `![${altText}](${url})`;
}

// 在当前选区插入图片 Markdown，自动补齐前后换行避免粘连原文本。
function insertImageMarkdown(view: EditorView, markdownLines: string[]): void {
  const selectedRange = view.state.selection.main;
  const markdownBlock = markdownLines.join("\n");
  const docLength = view.state.doc.length;
  // 保证图片块前后都至少保留一个空行（即两侧至少有两个换行符）。
  const beforeContext = view.state.doc.sliceString(Math.max(0, selectedRange.from - 2), selectedRange.from);
  const afterContext = view.state.doc.sliceString(selectedRange.to, Math.min(docLength, selectedRange.to + 2));

  const prefix =
    selectedRange.from === 0 ? "" : beforeContext.endsWith("\n\n") ? "" : beforeContext.endsWith("\n") ? "\n" : "\n\n";
  const suffix =
    selectedRange.to === docLength
      ? ""
      : afterContext.startsWith("\n\n")
        ? ""
        : afterContext.startsWith("\n")
          ? "\n"
          : "\n\n";

  const insertText = `${prefix}${markdownBlock}${suffix}`;

  const cursor = selectedRange.from + insertText.length;
  view.dispatch({
    changes: {
      from: selectedRange.from,
      to: selectedRange.to,
      insert: insertText
    },
    selection: EditorSelection.cursor(cursor),
    scrollIntoView: true
  });
}

export default function App() {
  // 数据网关单例。
  const dataGateway = useMemo(() => getDataGateway(), []);
  // 当前编辑内容。
  const [content, setContent] = useState(FALLBACK_CONTENT);
  // 当前打开文档 ID。
  const [activeDocId, setActiveDocId] = useState<string | null>(null);
  // 当前保存基线版本。
  const [baseVersion, setBaseVersion] = useState(0);
  // 最近一次成功保存的内容。
  const [lastSavedContent, setLastSavedContent] = useState(FALLBACK_CONTENT);
  // 保存状态。
  const [saveStatus, setSaveStatus] = useState<SaveStatus>("loading");
  // 页头状态文案。
  const [statusMessage, setStatusMessage] = useState("初始化中...");
  // 当前文档所属空间名。
  const [activeSpaceName, setActiveSpaceName] = useState("未命名空间");
  // 当前文档名称。
  const [activeDocumentTitle, setActiveDocumentTitle] = useState("未命名文档");
  // 最近一次成功保存时间。
  const [lastSavedAt, setLastSavedAt] = useState<string | null>(null);
  // 当前生效的预览主题 ID。
  const [activePreviewThemeId, setActivePreviewThemeId] = useState(DEFAULT_PREVIEW_THEME_ID);
  // 外部注入的预览样式文本；为空时仅使用内置主题。
  const [customPreviewStyleText, setCustomPreviewStyleText] = useState("");
  // 预览视口模式：desktop 保持现状，mobile 模拟窄屏阅读。
  const [previewViewportMode, setPreviewViewportMode] = useState<PreviewViewportMode>("desktop");
  // 复制到公众号时的进行中状态：防止重复点击触发并发复制。
  const [isWechatCopying, setIsWechatCopying] = useState(false);
  // 粘贴图片上传状态：用于防止重复触发并展示状态文案。
  const [isImageUploading, setIsImageUploading] = useState(false);
  // 当前上传任务总数与已处理数量：用于展示实时上传进度。
  const [imageUploadTotalCount, setImageUploadTotalCount] = useState(0);
  const [imageUploadCompletedCount, setImageUploadCompletedCount] = useState(0);
  // 顶部提示状态：用于复制成功等短时反馈。
  const [appToast, setAppToast] = useState<AppToastState>({
    isOpen: false,
    message: "",
    variant: "success",
    triggerKey: 0
  });
  // 设置面板开关状态。
  const [isSettingsLayerOpen, setIsSettingsLayerOpen] = useState(false);
  // 图床配置读取状态。
  const [isImageHostingConfigLoading, setIsImageHostingConfigLoading] = useState(true);
  // 图床配置保存状态。
  const [isImageHostingConfigSaving, setIsImageHostingConfigSaving] = useState(false);
  // 图床配置错误文案。
  const [imageHostingConfigError, setImageHostingConfigError] = useState<string | null>(null);
  // 图床配置数据。
  const [imageHostingConfig, setImageHostingConfig] = useState<ImageHostingConfig>(
    DEFAULT_IMAGE_HOSTING_CONFIG
  );
  // 图床配置引用：供异步粘贴上传逻辑读取最新值，避免闭包拿到旧配置。
  const imageHostingConfigRef = useRef(imageHostingConfig);
  // 上传中引用：用于 paste 事件同步分支判断，避免并发上传。
  const isImageUploadingRef = useRef(isImageUploading);

  // 当前生效主题对象，用于渲染菜单高亮和生成样式。
  const activePreviewTheme = useMemo(
    () => resolvePreviewTheme(activePreviewThemeId),
    [activePreviewThemeId]
  );
  // 预览区主题类名：挂到正文 article 上参与主题变量匹配。
  const activePreviewThemeClassName = useMemo(
    () => getPreviewThemeClassName(activePreviewTheme.id),
    [activePreviewTheme.id]
  );

  // 滚动同步 Hook：封装编辑区/预览区双向同步与锚点重建逻辑。
  const { handleEditorPaneRef, handlePreviewScrollerRef, handleEditorCreate, handleTocNavigate } =
    useScrollSync({
      content,
      previewThemeClassName: activePreviewThemeClassName,
      customPreviewStyleText,
      previewViewportMode
    });

  // 加载并监听外部自定义样式：支持 window 变量、localStorage 与自定义事件三种入口。
  useEffect(() => {
    // 读取初始样式：window 注入优先，其次回退到 localStorage。
    const readInitialCustomStyleText = (): string => {
      const styleFromWindow = normalizePreviewStyleText(window.__PLAINDOC_PREVIEW_STYLE__);
      if (styleFromWindow) {
        return styleFromWindow;
      }
      try {
        return normalizePreviewStyleText(
          window.localStorage.getItem(PREVIEW_CUSTOM_STYLE_STORAGE_KEY)
        );
      } catch {
        return "";
      }
    };

    setCustomPreviewStyleText(readInitialCustomStyleText());

    // 响应外部样式更新事件，并同步持久化到 localStorage。
    const onCustomStyleChanged = (event: Event) => {
      const detail = (event as CustomEvent<string>).detail;
      const normalizedStyleText = normalizePreviewStyleText(detail);
      setCustomPreviewStyleText(normalizedStyleText);
      try {
        if (normalizedStyleText) {
          window.localStorage.setItem(PREVIEW_CUSTOM_STYLE_STORAGE_KEY, normalizedStyleText);
        } else {
          window.localStorage.removeItem(PREVIEW_CUSTOM_STYLE_STORAGE_KEY);
        }
      } catch {
        // localStorage 失败时仅忽略持久化，不影响当前会话样式。
      }
    };

    window.addEventListener(PREVIEW_CUSTOM_STYLE_EVENT, onCustomStyleChanged);
    return () => {
      window.removeEventListener(PREVIEW_CUSTOM_STYLE_EVENT, onCustomStyleChanged);
    };
  }, []);

  // 首次加载时恢复上次选择的主题模板。
  useEffect(() => {
    try {
      const storedThemeId = window.localStorage.getItem(PREVIEW_THEME_STORAGE_KEY);
      if (!storedThemeId) {
        return;
      }
      const restoredTheme = resolvePreviewTheme(storedThemeId);
      setActivePreviewThemeId(restoredTheme.id);
    } catch {
      // localStorage 不可用时保持默认主题。
    }
  }, []);

  // 首次加载时恢复上次选择的预览视口模式（PC / 移动端）。
  useEffect(() => {
    try {
      const storedPreviewViewportMode = window.localStorage.getItem(PREVIEW_VIEWPORT_MODE_STORAGE_KEY);
      if (storedPreviewViewportMode === "desktop" || storedPreviewViewportMode === "mobile") {
        setPreviewViewportMode(storedPreviewViewportMode);
      }
    } catch {
      // localStorage 不可用时保持默认 PC 预览模式。
    }
  }, []);

  // 主题变化时写入本地缓存，便于下次启动直接恢复。
  useEffect(() => {
    try {
      window.localStorage.setItem(PREVIEW_THEME_STORAGE_KEY, activePreviewThemeId);
    } catch {
      // localStorage 失败时忽略持久化，不影响当前显示。
    }
  }, [activePreviewThemeId]);

  // 预览模式变化时写入本地缓存，便于下次启动直接恢复。
  useEffect(() => {
    try {
      window.localStorage.setItem(PREVIEW_VIEWPORT_MODE_STORAGE_KEY, previewViewportMode);
    } catch {
      // localStorage 失败时忽略持久化，不影响当前显示。
    }
  }, [previewViewportMode]);

  // 同步配置引用，确保粘贴上传始终使用最新“默认图床 + 凭据”。
  useEffect(() => {
    imageHostingConfigRef.current = imageHostingConfig;
  }, [imageHostingConfig]);

  // 同步上传状态引用，避免在 paste 事件中读取到过期状态。
  useEffect(() => {
    isImageUploadingRef.current = isImageUploading;
  }, [isImageUploading]);

  // 首次加载图床配置：默认从 IndexedDB 的 user_config 表读取。
  useEffect(() => {
    let cancelled = false;

    const loadImageHostingConfig = async () => {
      setIsImageHostingConfigLoading(true);
      setImageHostingConfigError(null);
      try {
        const storedConfig = await dataGateway.userConfig.getValue<unknown>({
          userId: TEMP_USER_ID,
          key: IMAGE_HOSTING_CONFIG_KEY
        });
        if (cancelled) {
          return;
        }
        if (!storedConfig) {
          setImageHostingConfig(cloneImageHostingConfig(DEFAULT_IMAGE_HOSTING_CONFIG));
          return;
        }
        setImageHostingConfig(normalizeImageHostingConfig(storedConfig));
      } catch (error) {
        if (cancelled) {
          return;
        }
        console.error("[settings][image-hosting] 读取图床配置失败", error);
        setImageHostingConfigError(`读取图床配置失败：${formatError(error)}`);
      } finally {
        if (!cancelled) {
          setIsImageHostingConfigLoading(false);
        }
      }
    };

    void loadImageHostingConfig();

    return () => {
      cancelled = true;
    };
  }, [dataGateway]);

  const extensions = useMemo(
    () => [
      // 编辑器软换行，避免横向滚动影响同步体验。
      EditorView.lineWrapping,
      // 拦截粘贴图片：自动上传到默认图床并回填 Markdown 图片链接。
      EditorView.domEventHandlers({
        paste: (event, view) => {
          const imageFiles = extractImageFilesFromClipboard(event);
          if (!imageFiles.length) {
            return false;
          }

          event.preventDefault();
          void (async () => {
            if (isImageUploadingRef.current) {
              setStatusMessage("图片上传中，请稍候...");
              return;
            }

            isImageUploadingRef.current = true;
            setIsImageUploading(true);
            setImageUploadTotalCount(imageFiles.length);
            setImageUploadCompletedCount(0);
            setStatusMessage(`正在上传 ${imageFiles.length} 张图片...`);
            const successMarkdownLines: string[] = [];
            const failedMessages: string[] = [];

            try {
              for (const [index, imageFile] of imageFiles.entries()) {
                try {
                  const uploadedImage = await uploadImageToDefaultHosting(
                    imageHostingConfigRef.current,
                    imageFile
                  );
                  successMarkdownLines.push(buildImageMarkdownLine(imageFile, uploadedImage.url, index));
                } catch (error) {
                  console.error("[editor][paste-image] 单张图片上传失败", {
                    fileName: imageFile.name || "未命名图片",
                    provider: imageHostingConfigRef.current.defaultProvider,
                    error
                  });
                  failedMessages.push(`${imageFile.name || "未命名图片"}：${formatError(error)}`);
                } finally {
                  setImageUploadCompletedCount((previousCount) => previousCount + 1);
                }
              }

              if (successMarkdownLines.length) {
                insertImageMarkdown(view, successMarkdownLines);
                setStatusMessage(`已上传 ${successMarkdownLines.length} 张图片并插入链接`);
                setAppToast((previousToast) => ({
                  isOpen: true,
                  message: `图片上传成功（${successMarkdownLines.length}/${imageFiles.length}）`,
                  variant: "success",
                  triggerKey: previousToast.triggerKey + 1
                }));
              }

              if (failedMessages.length) {
                const firstError = failedMessages[0];
                console.error("[editor][paste-image] 部分图片上传失败", {
                  failedCount: failedMessages.length,
                  errors: failedMessages
                });
                setStatusMessage(`图片上传失败：${firstError}`);
                setAppToast((previousToast) => ({
                  isOpen: true,
                  message: `部分图片上传失败：${firstError}`,
                  variant: "error",
                  triggerKey: previousToast.triggerKey + 1
                }));
              }
            } catch (error) {
              console.error("[editor][paste-image] 粘贴图片上传流程异常", error);
              setStatusMessage(`图片上传异常：${formatError(error)}`);
              setAppToast((previousToast) => ({
                isOpen: true,
                message: `图片上传异常：${formatError(error)}`,
                variant: "error",
                triggerKey: previousToast.triggerKey + 1
              }));
            } finally {
              isImageUploadingRef.current = false;
              setIsImageUploading(false);
              setImageUploadTotalCount(0);
              setImageUploadCompletedCount(0);
            }
          })();
          return true;
        }
      }),
      markdown({
        // 启用 Markdown 语言与代码块语言支持。
        base: markdownLanguage,
        codeLanguages: languages
      })
    ],
    []
  );
  // remark 插件顺序：先 GFM，再解析数学公式，最后注入锚点属性。
  const remarkPlugins = useMemo(() => [remarkGfm, remarkMath, remarkBlockAnchorPlugin], []);
  // rehype 插件顺序：先解析内嵌 HTML，再做白名单清洗，最后渲染 KaTeX。
  const rehypePlugins = useMemo(() => {
    const sanitizePlugin: [typeof rehypeSanitize, typeof PREVIEW_HTML_SANITIZE_SCHEMA] = [
      rehypeSanitize,
      PREVIEW_HTML_SANITIZE_SCHEMA
    ];
    return [rehypeRaw, sanitizePlugin, rehypeKatex];
  }, []);
  // markdown-it 仅用于“去语法后的文字统计”和 TOC 语法解析。
  const markdownTextParser = useMemo(
    () =>
      new MarkdownIt({
        html: false,
        linkify: true,
        typographer: false
      }),
    []
  );
  // 解析文档标题与 [TOC] 标记，供目录菜单与语法渲染共用。
  const tocParseResult = useMemo(
    () => parseTocFromMarkdown(content, markdownTextParser),
    [content, markdownTextParser]
  );
  // TOC 标题列表。
  const tocItems = tocParseResult.items;
  // 当前文档是否声明了 [TOC] 语法标记。
  const hasTocMarker = tocParseResult.hasMarker;

  // 自定义 Markdown 渲染器。
  const markdownComponents = useMemo(
    () =>
      buildMarkdownComponents({
        activePreviewThemeId,
        tocItems,
        onTocNavigate: handleTocNavigate
      }),
    [activePreviewThemeId, handleTocNavigate, tocItems]
  );
  // 提取 Markdown 对应的纯文本内容。
  const plainTextContent = useMemo(
    () => extractPlainTextFromMarkdown(content, markdownTextParser),
    [content, markdownTextParser]
  );
  // 统计非空白字符数量，作为字数展示。
  const plainTextCount = useMemo(() => plainTextContent.replace(/\s+/g, "").length, [plainTextContent]);
  // 将最后保存时间格式化为状态栏文案。
  const lastSavedTimeLabel = useMemo(() => formatSavedTime(lastSavedAt), [lastSavedAt]);
  // 根据保存状态生成状态栏图标展示类型。
  const saveIndicatorVariant = useMemo(() => resolveSaveIndicatorVariant(saveStatus), [saveStatus]);
  // 当前主题对应的变量样式文本：通过 style 标签注入。
  const activePreviewThemeStyleText = useMemo(
    () => buildPreviewThemeStyleText(activePreviewTheme),
    [activePreviewTheme]
  );
  // 图片上传中的顶部提示文案：展示 x/y 进度以降低等待焦虑。
  const imageUploadLoadingMessage = useMemo(() => {
    if (!isImageUploading) {
      return "";
    }
    if (imageUploadTotalCount <= 0) {
      return "图片上传中...";
    }
    return `图片上传中（${Math.min(imageUploadCompletedCount, imageUploadTotalCount)}/${imageUploadTotalCount}）...`;
  }, [imageUploadCompletedCount, imageUploadTotalCount, isImageUploading]);

  // 首次启动：加载空间、文档树与默认文档内容。
  useEffect(() => {
    let cancelled = false;

    const bootstrap = async () => {
      try {
        const spaces = await dataGateway.workspace.listSpaces();
        const space =
          spaces[0] ??
          (await dataGateway.workspace.createSpace({
            name: "默认空间"
          }));
        const tree = await dataGateway.workspace.getTree(space.id);
        const existingDocId = findFirstDocId(tree);
        const docId =
          existingDocId ??
          (
            await dataGateway.workspace.createNode({
              spaceId: space.id,
              parentId: null,
              type: "doc",
              title: "未命名文档"
            })
          ).docId;

        if (!docId) {
          throw new Error("无法创建初始化文档");
        }

        const document = await dataGateway.document.getDocument(docId);
        if (cancelled) {
          return;
        }

        // 初始化编辑状态与保存基线。
        setActiveSpaceName(space.name);
        setActiveDocumentTitle(document.title || "未命名文档");
        setLastSavedAt(document.updatedAt);
        setActiveDocId(document.id);
        setBaseVersion(document.version);
        setContent(document.contentMd);
        setLastSavedContent(document.contentMd);
        setSaveStatus("ready");
        setStatusMessage(`已加载文档 v${document.version}`);
      } catch (error) {
        if (cancelled) {
          return;
        }
        setSaveStatus("error");
        setStatusMessage(`加载失败：${formatError(error)}`);
      }
    };

    void bootstrap();

    return () => {
      cancelled = true;
    };
  }, [dataGateway]);

  // 自动保存：内容变化后延迟提交，处理版本冲突与失败状态。
  useEffect(() => {
    if (
      !activeDocId ||
      content === lastSavedContent ||
      saveStatus === "loading" ||
      saveStatus === "saving"
    ) {
      return;
    }

    const timer = window.setTimeout(async () => {
      setSaveStatus("saving");
      setStatusMessage("保存中...");
      try {
        const result = await dataGateway.document.saveDocument({
          docId: activeDocId,
          contentMd: content,
          baseVersion
        });
        setBaseVersion(result.document.version);
        setActiveDocumentTitle(result.document.title || "未命名文档");
        setLastSavedAt(result.document.updatedAt);
        setLastSavedContent(result.document.contentMd);
        setSaveStatus("saved");
        setStatusMessage(`已保存 v${result.document.version}`);
      } catch (error) {
        if (error instanceof ConflictError) {
          setSaveStatus("conflict");
          setStatusMessage(
            `检测到冲突：当前基线 v${baseVersion}，最新版本 v${error.latestDocument.version}`
          );
          return;
        }
        setSaveStatus("error");
        setStatusMessage(`保存失败：${formatError(error)}`);
      }
    }, 800);

    // 输入持续变化时清理上一次保存定时器。
    return () => {
      window.clearTimeout(timer);
    };
  }, [activeDocId, baseVersion, content, dataGateway, lastSavedContent, saveStatus]);

  // 应用选中的主题：仅在主题真正变化时更新父组件状态。
  const handleThemeChange = useCallback((themeId: string) => {
    const targetTheme = resolvePreviewTheme(themeId);
    setActivePreviewThemeId((previousThemeId) =>
      previousThemeId === targetTheme.id ? previousThemeId : targetTheme.id
    );
  }, []);

  // 切换预览视口：desktop <-> mobile。
  const togglePreviewViewportMode = useCallback(() => {
    setPreviewViewportMode((previousMode) =>
      previousMode === "desktop" ? "mobile" : "desktop"
    );
  }, []);

  // 导出预览区为内联样式 HTML，并写入剪贴板供公众号编辑器粘贴。
  const handleCopyToWechat = useCallback(async () => {
    if (isWechatCopying) {
      return;
    }
    setIsWechatCopying(true);
    try {
      await copyPreviewToWechat();
      setStatusMessage("已复制预览内容，可直接粘贴到微信公众号编辑器");
      setAppToast((previousToast) => ({
        isOpen: true,
        message: "复制成功，可直接粘贴到微信公众号编辑器",
        variant: "success",
        triggerKey: previousToast.triggerKey + 1
      }));
    } catch (error) {
      setStatusMessage(`复制失败：${formatError(error)}`);
    } finally {
      setIsWechatCopying(false);
    }
  }, [isWechatCopying]);

  // 关闭顶部提示：供自动计时与后续手动关闭复用。
  const closeAppToast = useCallback(() => {
    setAppToast((previousToast) => {
      if (!previousToast.isOpen) {
        return previousToast;
      }
      return {
        ...previousToast,
        isOpen: false
      };
    });
  }, []);

  // 打开设置浮层。
  const openSettingsLayer = useCallback(() => {
    setIsSettingsLayerOpen(true);
  }, []);

  // 关闭设置浮层。
  const closeSettingsLayer = useCallback(() => {
    setIsSettingsLayerOpen(false);
  }, []);

  // 保存图床配置到数据抽象层。
  const saveImageHostingConfig = useCallback(
    async (nextConfig: ImageHostingConfig) => {
      setIsImageHostingConfigSaving(true);
      setImageHostingConfigError(null);
      try {
        const normalizedConfig = normalizeImageHostingConfig(nextConfig);
        await dataGateway.userConfig.setValue({
          userId: TEMP_USER_ID,
          key: IMAGE_HOSTING_CONFIG_KEY,
          value: normalizedConfig
        });
        setImageHostingConfig(normalizedConfig);
        setStatusMessage("图床配置已保存");
        setAppToast((previousToast) => ({
          isOpen: true,
          message: "图床配置已保存",
          variant: "success",
          triggerKey: previousToast.triggerKey + 1
        }));
        setIsSettingsLayerOpen(false);
      } catch (error) {
        console.error("[settings][image-hosting] 保存图床配置失败", error);
        setImageHostingConfigError(`保存图床配置失败：${formatError(error)}`);
      } finally {
        setIsImageHostingConfigSaving(false);
      }
    },
    [dataGateway]
  );

  // 手动同步到最新版本，用于冲突后的回拉。
  const syncLatestVersion = async () => {
    if (!activeDocId) {
      return;
    }
    try {
      const latestDocument = await dataGateway.document.getDocument(activeDocId);
      setActiveDocumentTitle(latestDocument.title || "未命名文档");
      setLastSavedAt(latestDocument.updatedAt);
      setContent(latestDocument.contentMd);
      setBaseVersion(latestDocument.version);
      setLastSavedContent(latestDocument.contentMd);
      setSaveStatus("ready");
      setStatusMessage(`已同步到最新版本 v${latestDocument.version}`);
    } catch (error) {
      setSaveStatus("error");
      setStatusMessage(`同步失败：${formatError(error)}`);
    }
  };

  return (
    // 主页面容器。
    <div className="page">
      <TopToast
        open={appToast.isOpen}
        message={appToast.message}
        variant={appToast.variant}
        triggerKey={appToast.triggerKey}
        durationMs={2600}
        onClose={closeAppToast}
        icon={<CheckCircle2 size={16} />}
      />
      <TopToast
        open={isImageUploading}
        message={imageUploadLoadingMessage}
        variant="info"
        icon={<LoaderCircle className="top-toast__loader" size={16} />}
      />
      {/* 当前主题样式：先注入内置模板变量，后续允许外部样式继续覆盖。 */}
      {activePreviewThemeStyleText ? (
        <style id="plaindoc-preview-theme-style">{activePreviewThemeStyleText}</style>
      ) : null}
      {/* 外部自定义预览样式：存在时插入到页面末端，确保覆盖内置主题。 */}
      {customPreviewStyleText ? (
        <style id="plaindoc-preview-custom-style">{customPreviewStyleText}</style>
      ) : null}
      <SettingsLayer
        open={isSettingsLayerOpen}
        initialImageHostingConfig={imageHostingConfig}
        isLoading={isImageHostingConfigLoading}
        isSaving={isImageHostingConfigSaving}
        errorMessage={imageHostingConfigError}
        onClose={closeSettingsLayer}
        onSaveImageHostingConfig={saveImageHostingConfig}
      />
      {/* 顶部状态栏。 */}
      <header className="header">
        <h1>PlainDoc</h1>
        <div className="header-actions">
          {/* 目录菜单：展示标题结构并支持快速跳转。 */}
          {hasTocMarker ? <TocMenu items={tocItems} onSelectItem={handleTocNavigate} /> : null}
          {/* 复制到公众号：将当前预览导出为内联样式 HTML。 */}
          <button
            type="button"
            className="wechat-copy-button"
            onClick={() => void handleCopyToWechat()}
            disabled={isWechatCopying}
            title="复制当前预览为公众号可粘贴内容"
            aria-label="复制当前预览为公众号可粘贴内容"
          >
            {isWechatCopying ? "复制中..." : "复制到公众号"}
          </button>
          {/* 预览模式切换：在 PC 与移动端窄屏模拟之间切换。 */}
          <button
            type="button"
            className={`preview-mode-toggle preview-mode-toggle--${previewViewportMode}`}
            onClick={togglePreviewViewportMode}
            title={previewViewportMode === "desktop" ? "切换到移动端预览" : "切换到 PC 预览"}
            aria-label={previewViewportMode === "desktop" ? "切换到移动端预览" : "切换到 PC 预览"}
          >
            {previewViewportMode === "desktop" ? <Monitor size={14} /> : <Smartphone size={14} />}
            <span className="preview-mode-toggle__label">
              {previewViewportMode === "desktop" ? "PC 预览" : "移动预览"}
            </span>
          </button>
          {/* 主题菜单：展开/收起只更新菜单组件自身。 */}
          <ThemeMenu
            themes={PREVIEW_THEME_TEMPLATES}
            activeThemeId={activePreviewTheme.id}
            onSelectTheme={handleThemeChange}
            customPreviewStyleText={customPreviewStyleText}
          />
          <button
            type="button"
            className="settings-trigger"
            onClick={openSettingsLayer}
            title="打开设置面板"
            aria-label="打开设置面板"
          >
            <Settings2 size={14} />
            <span>设置</span>
          </button>
        </div>
      </header>
      {/* 双栏工作区：左编辑、右预览。 */}
      <main className="workspace">
        <section className="pane editor-pane" ref={handleEditorPaneRef}>
          <CodeMirror
            value={content}
            extensions={extensions}
            height="100%"
            onCreateEditor={handleEditorCreate}
            onChange={(value) => {
              // 录入编辑内容，并将状态切回可保存。
              setContent(value);
              if (saveStatus !== "loading") {
                setSaveStatus("ready");
              }
            }}
            basicSetup={{
              lineNumbers: false,
              foldGutter: false
            }}
          />
        </section>
        <section
          id={PREVIEW_PANE_ID}
          className={`pane preview-pane preview-pane--${previewViewportMode} ${PREVIEW_PANE_CLASS}`}
          // 使用稳定 ref 回调，保证滚动监听不会被重复拆装。
          ref={handlePreviewScrollerRef}
        >
          <div className={`preview-viewport preview-viewport--${previewViewportMode}`}>
            <article
              id={PREVIEW_BODY_ID}
              className={`markdown-body ${PREVIEW_BODY_CLASS} preview-body--${previewViewportMode} ${activePreviewThemeClassName}`}
            >
              {/* 使用 remark 插件渲染 Markdown 并写入 block 锚点。 */}
              <ReactMarkdown
                remarkPlugins={remarkPlugins}
                // 开启 Markdown 内嵌 HTML 解析，安全边界由 rehype-sanitize 白名单控制。
                remarkRehypeOptions={PREVIEW_MARKDOWN_REHYPE_OPTIONS}
                rehypePlugins={rehypePlugins}
                components={markdownComponents}
              >
                {content}
              </ReactMarkdown>
            </article>
          </div>
        </section>
      </main>
      {/* 冲突提示与手动同步入口。 */}
      {saveStatus === "conflict" ? (
        <footer className="conflict-footer">
          <span>当前文档存在版本冲突，请先同步最新版本后再手动合并。</span>
          <button type="button" onClick={() => void syncLatestVersion()}>
            同步最新版本
          </button>
        </footer>
      ) : null}
      {/* 固定底部状态栏：左侧空间/文件，右侧保存时间/字数。 */}
      <footer className="status-bar">
        <div className="status-bar__side status-bar__side--left">
          <span style={{ fontWeight: 600 }}>文档位置：</span>
          <span className="status-pill" title={activeSpaceName}>
            {activeSpaceName}
          </span>
          <span className="status-separator">/</span>
          <span className="status-pill" title={activeDocumentTitle}>
            {activeDocumentTitle}
          </span>
        </div>
        <div className="status-bar__side status-bar__side--right">
          {/* 保存状态图标：未保存=黄色，保存中=旋转，已保存=绿色。 */}
          <span
            className={`status-save-indicator status-save-indicator--${saveIndicatorVariant}`}
            title={statusMessage}
            aria-label={statusMessage}
          >
            {saveIndicatorVariant === "saving" ? (
              <LoaderCircle className="status-save-icon status-save-icon--spin" size={14} />
            ) : null}
            {saveIndicatorVariant === "saved" ? (
              <CheckCircle2 className="status-save-icon" size={14} />
            ) : null}
            {saveIndicatorVariant === "unsaved" ? (
              <AlertCircle className="status-save-icon" size={14} />
            ) : null}
          </span>
          <span>
            <span style={{ fontWeight: 600 }}>最后保存时间：</span>
            {lastSavedTimeLabel}
          </span>
          <span>
            <span style={{ fontWeight: 600 }}>字数统计：</span>
            {plainTextCount}
          </span>
        </div>
      </footer>
    </div>
  );
}
