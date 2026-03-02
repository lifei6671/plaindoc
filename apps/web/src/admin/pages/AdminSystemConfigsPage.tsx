import {
  Database,
  Home,
  ImageIcon,
  Keyboard,
  LoaderCircle,
  Lock,
  Map,
  RefreshCw,
  Search,
  Save,
  type LucideIcon
} from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Button } from "../../components/ui/button";
import { Checkbox } from "../../components/ui/checkbox";
import { Input } from "../../components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../../components/ui/select";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "../../components/ui/tabs";
import { showToast } from "../../components/ui/toast";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "../../components/ui/tooltip";
import { type AdminSearchIndexStatusResult, type AdminSystemConfig, type DataGateway } from "../../data-access";
import { formatError } from "../../editor/status-utils";
import {
  cloneImageHostingConfig,
  DEFAULT_IMAGE_HOSTING_CONFIG,
  normalizeImageHostingConfig,
  type AliyunOssConfig,
  type CloudflareR2Config,
  type ImageHostingConfig,
  type ImageHostingImageProcessingMode,
  type ImageHostingImageQualityPreset,
  type ImageHostingProvider,
  type LocalImageHostingConfig
} from "../../settings/image-hosting";
import { AdminPageCard, AdminToolbarActions } from "../components/AdminPageLayout";

type SystemConfigKey = "site" | "editor" | "security" | "search" | "data-retention" | "auth" | "image-hosting" | "sitemap";
type SpaceVisibility = "public" | "authenticated" | "member";
type SitemapGenerationMode = "all_public" | "updated_within_days";
type AuthLoginMode = "local_only" | "ldap_only" | "mixed";
type SearchProvider = "bleve" | "meili" | "typesense" | "database";
type SearchFallbackPolicy = "degrade_to_database" | "return_error";
type SearchAnalyzer = "simple" | "jieba";
type SearchJiebaMode = "search";
type SearchJiebaDictSource = "db" | "file";
type DataRetentionCleanupTable =
  | "audit_logs"
  | "auth_captcha_challenges"
  | "auth_risk_states"
  | "user_sessions"
  | "document_attachments"
  | "document_image_assets";

interface SiteSystemConfigValue {
  allowRegistration: boolean;
  defaultSpaceVisibility: SpaceVisibility;
}

interface EditorSystemConfigValue {
  autosaveIntervalSeconds: number;
  maxDocumentSizeKB: number;
}

interface SecuritySystemConfigValue {
  accessTokenTTLMinutes: number;
  refreshTokenTTLMinutes: number;
}

interface SearchSystemConfigValue {
  enabled: boolean;
  activeProvider: SearchProvider;
  fallbackPolicy: SearchFallbackPolicy;
  analysis: {
    activeAnalyzer: SearchAnalyzer;
    analyzers: {
      simple: {
        enabled: boolean;
      };
      jieba: {
        enabled: boolean;
        mode: SearchJiebaMode;
        hmm: boolean;
        stopwordsEnabled: boolean;
        dictSource: SearchJiebaDictSource;
        dictVersion: string;
      };
    };
  };
}

interface DataRetentionSystemConfigValue {
  enabled: boolean;
  scheduleMinutes: number;
  cleanupBatchSize: number;
  cleanupTables: DataRetentionCleanupTable[];
  auditLogRetentionDays: number;
  authCaptchaRetentionHours: number;
  authRiskStateRetentionDays: number;
  userSessionRetentionDays: number;
}

interface AuthProviderLdapConfig {
  host: string;
  port: number;
  tlsMode: "ldaps" | "starttls" | "plain";
  baseDN: string;
  bindDN: string;
  bindPasswordCiphertext: string;
  userFilter: string;
  idAttribute: string;
  emailAttribute: string;
  nameAttribute: string;
  groupAttribute: string;
  connectTimeoutMs: number;
  readTimeoutMs: number;
}

interface AuthProviderMatchRules {
  emailDomains: string[];
  usernameRegex: string;
}

interface AuthProviderConfig {
  id: string;
  name: string;
  type: "ldap";
  enabled: boolean;
  priority: number;
  matchRules: AuthProviderMatchRules;
  ldap: AuthProviderLdapConfig;
}

interface AuthSystemConfigValue {
  loginMode: AuthLoginMode;
  defaultProviderId: string;
  allowUserRegister: boolean;
  providers: AuthProviderConfig[];
  breakGlass: {
    enabled: boolean;
    localAdminEmails: string[];
  };
}

interface SitemapSystemConfigValue {
  generationMode: SitemapGenerationMode;
  maxUpdatedWithinDays: number;
}

interface SystemConfigTabItem {
  key: SystemConfigKey;
  label: string;
  description: string;
  icon: LucideIcon;
}

const SYSTEM_CONFIG_TABS: SystemConfigTabItem[] = [
  {
    key: "site",
    label: "站点设置",
    description: "注册与默认可见性",
    icon: Home
  },
  {
    key: "editor",
    label: "编辑器设置",
    description: "自动保存与文档大小",
    icon: Keyboard
  },
  {
    key: "security",
    label: "安全设置",
    description: "Token 生命周期",
    icon: Lock
  },
  {
    key: "search",
    label: "全文检索",
    description: "引擎与分词策略",
    icon: Search
  },
  {
    key: "data-retention",
    label: "数据清理",
    description: "审计与临时数据保留策略",
    icon: Database
  },
  {
    key: "auth",
    label: "认证设置",
    description: "登录模式与 LDAP",
    icon: Lock
  },
  {
    key: "image-hosting",
    label: "图床设置",
    description: "本地 / R2 / OSS",
    icon: ImageIcon
  },
  {
    key: "sitemap",
    label: "Sitemap 设置",
    description: "生成规则与更新时间窗口",
    icon: Map
  }
];

const SPACE_VISIBILITY_OPTIONS: Array<{ value: SpaceVisibility; label: string }> = [
  { value: "public", label: "完全公开（未登录可见）" },
  { value: "authenticated", label: "登录可见（需登录）" },
  { value: "member", label: "成员可见（阅读者及以上）" }
];

const IMAGE_HOSTING_PROVIDER_OPTIONS: Array<{ value: ImageHostingProvider; label: string }> = [
  { value: "local", label: "本地存储" },
  { value: "cloudflare-r2", label: "Cloudflare R2" },
  { value: "aliyun-oss", label: "阿里云 OSS" }
];

const IMAGE_HOSTING_IMAGE_PROCESSING_MODE_OPTIONS: Array<{
  value: ImageHostingImageProcessingMode;
  label: string;
  description: string;
}> = [
    {
      value: "same_format",
      label: "原格式压缩",
      description: "尽量保持输入格式，仅对可重编码格式执行压缩。"
    },
    {
      value: "to_webp",
      label: "转为 WebP",
      description: "统一转为 WebP，再按质量档位压缩。"
    }
  ];

const IMAGE_HOSTING_IMAGE_QUALITY_PRESET_OPTIONS: Array<{
  value: ImageHostingImageQualityPreset;
  label: string;
  description: string;
}> = [
    {
      value: "original",
      label: "原图",
      description: "保持原始质量（原格式模式下会透传，不重编码）。"
    },
    {
      value: "high",
      label: "高清",
      description: "高质量压缩，优先保证视觉效果。"
    },
    {
      value: "standard",
      label: "标准",
      description: "默认档位，在质量和体积间平衡。"
    },
    {
      value: "saver",
      label: "省流",
      description: "更激进压缩，优先减小体积。"
    }
  ];

const IMAGE_HOSTING_IMAGE_MAX_WIDTH_MIN = 256;
const IMAGE_HOSTING_IMAGE_MAX_WIDTH_MAX = 20000;
const IMAGE_HOSTING_IMAGE_MAX_HEIGHT_MIN = 256;
const IMAGE_HOSTING_IMAGE_MAX_HEIGHT_MAX = 20000;

const IMAGE_HOSTING_UPLOAD_TEMPLATE_PLACEHOLDER =
  "images/{spaceId}/{docId}/{yyyy}/{mm}/{dd}/{assetId}.{ext}";
const IMAGE_HOSTING_UPLOAD_TEMPLATE_HINT =
  "可用变量：{spaceId} {docId} {yyyy} {mm} {dd} {hh} {assetId} {origName} {ext} {uploaderId} {Rand:N}；其中 N 取值 4-10，且必须包含 {assetId}。";
const IMAGE_HOSTING_UPLOAD_TEMPLATE_PRESETS: Array<{
  key: string;
  label: string;
  template: string;
}> = [
    {
      key: "by-document",
      label: "按文档归档",
      template: "images/{spaceId}/{docId}/{yyyy}/{mm}/{dd}/{assetId}_{origName}.{ext}"
    },
    {
      key: "by-day",
      label: "按年月日归档",
      template: "images/{spaceId}/{yyyy}/{mm}/{dd}/{assetId}.{ext}"
    },
    {
      key: "by-month",
      label: "按年月归档",
      template: "images/{spaceId}/{yyyy}/{mm}/{assetId}.{ext}"
    }
  ];
const IMAGE_HOSTING_UPLOAD_TEMPLATE_VARIABLES: Array<{
  token: string;
  label: string;
  description: string;
}> = [
  { token: "{spaceId}", label: "spaceId", description: "当前文档所属空间的 ID。" },
  { token: "{docId}", label: "docId", description: "当前文档的 ID。" },
  { token: "{yyyy}", label: "yyyy", description: "上传时间年份（四位，如 2026）。" },
  { token: "{mm}", label: "mm", description: "上传时间月份（两位，如 03）。" },
  { token: "{dd}", label: "dd", description: "上传时间日期（两位，如 01）。" },
  { token: "{hh}", label: "hh", description: "上传时间小时（24 小时制，两位）。" },
  { token: "{assetId}", label: "assetId", description: "后端生成的资源唯一 ID（必填，防冲突）。" },
  { token: "{origName}", label: "origName", description: "原始文件名（已清理非法字符，不含扩展名）。" },
  { token: "{ext}", label: "ext", description: "文件扩展名（不含点号，如 png/pdf）。" },
  { token: "{uploaderId}", label: "uploaderId", description: "上传用户 ID。" },
  { token: "{Rand:6}", label: "Rand:6", description: "随机字符串占位符。把 6 改成 4-10 可指定长度，如 {Rand:8}。" }
];

const SITEMAP_GENERATION_MODE_OPTIONS: Array<{
  value: SitemapGenerationMode;
  label: string;
  description: string;
}> = [
    {
      value: "all_public",
      label: "全部公开文档",
      description: "纳入所有公开且有内容的文档"
    },
    {
      value: "updated_within_days",
      label: "最近更新文档",
      description: "仅纳入最近 N 天更新的公开文档"
    }
  ];

const AUTH_LOGIN_MODE_OPTIONS: Array<{ value: AuthLoginMode; label: string }> = [
  { value: "local_only", label: "仅本地账号（local_only）" },
  { value: "ldap_only", label: "仅 LDAP（ldap_only）" },
  { value: "mixed", label: "本地 + LDAP（mixed）" }
];

const AUTH_LOCAL_PROVIDER_ID = "local";
const AUTH_SECRET_MASK = "********";

const SITE_TEMPLATE: SiteSystemConfigValue = {
  allowRegistration: true,
  defaultSpaceVisibility: "member"
};

const EDITOR_TEMPLATE: EditorSystemConfigValue = {
  autosaveIntervalSeconds: 15,
  maxDocumentSizeKB: 1024
};

const SECURITY_TEMPLATE: SecuritySystemConfigValue = {
  accessTokenTTLMinutes: 120,
  refreshTokenTTLMinutes: 10080
};

const SEARCH_PROVIDER_OPTIONS: Array<{ value: SearchProvider; label: string }> = [
  { value: "database", label: "数据库（简单搜索）" },
  { value: "bleve", label: "Bleve（内置）" },
  { value: "meili", label: "Meilisearch" },
  { value: "typesense", label: "Typesense" }
];

const SEARCH_FALLBACK_POLICY_OPTIONS: Array<{ value: SearchFallbackPolicy; label: string; description: string }> = [
  {
    value: "degrade_to_database",
    label: "自动降级到数据库查询",
    description: "检索引擎不可用时降级到数据库简单查询。"
  },
  {
    value: "return_error",
    label: "直接返回错误",
    description: "外部引擎不可用时直接返回错误。"
  }
];

const SEARCH_ANALYZER_OPTIONS: Array<{ value: SearchAnalyzer; label: string }> = [
  { value: "simple", label: "simple" },
  { value: "jieba", label: "jieba" }
];

const SEARCH_JIEBA_DICT_SOURCE_OPTIONS: Array<{ value: SearchJiebaDictSource; label: string }> = [
  { value: "db", label: "db（数据库词典）" },
  { value: "file", label: "file（文件词典）" }
];

const SEARCH_DICT_VERSION_PATTERN = /^[a-zA-Z0-9][a-zA-Z0-9._:-]{0,63}$/;

const SEARCH_TEMPLATE: SearchSystemConfigValue = {
  enabled: false,
  activeProvider: "bleve",
  fallbackPolicy: "degrade_to_database",
  analysis: {
    activeAnalyzer: "simple",
    analyzers: {
      simple: {
        enabled: true
      },
      jieba: {
        enabled: false,
        mode: "search",
        hmm: true,
        stopwordsEnabled: false,
        dictSource: "db",
        dictVersion: "default"
      }
    }
  }
};

const DATA_RETENTION_TEMPLATE: DataRetentionSystemConfigValue = {
  enabled: true,
  scheduleMinutes: 60,
  cleanupBatchSize: 500,
  cleanupTables: [
    "audit_logs",
    "auth_captcha_challenges",
    "auth_risk_states",
    "user_sessions",
    "document_attachments",
    "document_image_assets"
  ],
  auditLogRetentionDays: 180,
  authCaptchaRetentionHours: 72,
  authRiskStateRetentionDays: 30,
  userSessionRetentionDays: 30
};

const DATA_RETENTION_CLEANUP_TABLE_OPTIONS: Array<{
  value: DataRetentionCleanupTable;
  label: string;
  description: string;
}> = [
    {
      value: "audit_logs",
      label: "audit_logs",
      description: "系统审计日志"
    },
    {
      value: "auth_captcha_challenges",
      label: "auth_captcha_challenges",
      description: "验证码会话历史"
    },
    {
      value: "auth_risk_states",
      label: "auth_risk_states",
      description: "认证风控状态"
    },
    {
      value: "user_sessions",
      label: "user_sessions",
      description: "用户登录会话"
    },
    {
      value: "document_attachments",
      label: "document_attachments",
      description: "文档附件引用与孤儿文件实体"
    },
    {
      value: "document_image_assets",
      label: "document_image_assets",
      description: "文档图片资源（pending_cleanup）"
    }
  ];

const AUTH_PROVIDER_TEMPLATE: AuthProviderConfig = {
  id: "corp-ldap",
  name: "Corp LDAP",
  type: "ldap",
  enabled: true,
  priority: 100,
  matchRules: {
    emailDomains: ["corp.example.com"],
    usernameRegex: "^[a-z0-9._-]+$"
  },
  ldap: {
    host: "ldap.corp.example.com",
    port: 636,
    tlsMode: "ldaps",
    baseDN: "dc=corp,dc=example,dc=com",
    bindDN: "",
    bindPasswordCiphertext: "",
    userFilter: "(mail=%s)",
    idAttribute: "entryUUID",
    emailAttribute: "mail",
    nameAttribute: "cn",
    groupAttribute: "memberOf",
    connectTimeoutMs: 3000,
    readTimeoutMs: 3000
  }
};

const AUTH_TEMPLATE: AuthSystemConfigValue = {
  loginMode: "mixed",
  defaultProviderId: "corp-ldap",
  allowUserRegister: true,
  providers: [{ ...AUTH_PROVIDER_TEMPLATE }],
  breakGlass: {
    enabled: true,
    localAdminEmails: ["platform-admin@example.com"]
  }
};

const SITEMAP_TEMPLATE: SitemapSystemConfigValue = {
  generationMode: "all_public",
  maxUpdatedWithinDays: 180
};

const IMAGE_HOSTING_TEMPLATE = cloneImageHostingConfig(DEFAULT_IMAGE_HOSTING_CONFIG);

interface AdminSystemConfigsPageProps {
  dataGateway: DataGateway;
}

function formatDateTime(value: string | null): string {
  if (!value) {
    return "-";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "-";
  }
  return date.toLocaleString("zh-CN", { hour12: false });
}

function formatSearchRebuildSource(value: string): string {
  const source = value.trim().toLowerCase();
  if (source === "manual") {
    return "手动触发";
  }
  if (source === "bootstrap") {
    return "启动自检";
  }
  if (source === "sync_document") {
    return "文档增量同步";
  }
  if (source === "delete_document") {
    return "文档索引删除";
  }
  if (source === "sync_space") {
    return "空间增量同步";
  }
  if (source === "purge_space") {
    return "空间索引清理";
  }
  return source || "-";
}

function asRecord(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return null;
  }
  return value as Record<string, unknown>;
}

function parseInteger(value: unknown, fallback: number): number {
  if (typeof value === "number" && Number.isFinite(value)) {
    return Math.trunc(value);
  }
  return fallback;
}

function parseString(value: unknown, fallback: string): string {
  if (typeof value !== "string") {
    return fallback;
  }
  const trimmedValue = value.trim();
  return trimmedValue || fallback;
}

function parseSiteConfig(value: unknown): SiteSystemConfigValue {
  const payload = asRecord(value);
  if (!payload) {
    return { ...SITE_TEMPLATE };
  }

  const allowRegistration =
    typeof payload.allowRegistration === "boolean"
      ? payload.allowRegistration
      : SITE_TEMPLATE.allowRegistration;
  const defaultVisibility = payload.defaultSpaceVisibility;
  const defaultSpaceVisibility: SpaceVisibility =
    defaultVisibility === "public" || defaultVisibility === "authenticated" || defaultVisibility === "member"
      ? defaultVisibility
      : SITE_TEMPLATE.defaultSpaceVisibility;

  return {
    allowRegistration,
    defaultSpaceVisibility
  };
}

function parseEditorConfig(value: unknown): EditorSystemConfigValue {
  const payload = asRecord(value);
  if (!payload) {
    return { ...EDITOR_TEMPLATE };
  }

  return {
    autosaveIntervalSeconds: parseInteger(payload.autosaveIntervalSeconds, EDITOR_TEMPLATE.autosaveIntervalSeconds),
    maxDocumentSizeKB: parseInteger(payload.maxDocumentSizeKB, EDITOR_TEMPLATE.maxDocumentSizeKB)
  };
}

function parseSecurityConfig(value: unknown): SecuritySystemConfigValue {
  const payload = asRecord(value);
  if (!payload) {
    return { ...SECURITY_TEMPLATE };
  }

  return {
    accessTokenTTLMinutes: parseInteger(payload.accessTokenTTLMinutes, SECURITY_TEMPLATE.accessTokenTTLMinutes),
    refreshTokenTTLMinutes: parseInteger(payload.refreshTokenTTLMinutes, SECURITY_TEMPLATE.refreshTokenTTLMinutes)
  };
}

function cloneSearchConfig(value: SearchSystemConfigValue): SearchSystemConfigValue {
  return {
    enabled: value.enabled,
    activeProvider: value.activeProvider,
    fallbackPolicy: value.fallbackPolicy,
    analysis: {
      activeAnalyzer: value.analysis.activeAnalyzer,
      analyzers: {
        simple: {
          enabled: value.analysis.analyzers.simple.enabled
        },
        jieba: {
          enabled: value.analysis.analyzers.jieba.enabled,
          mode: value.analysis.analyzers.jieba.mode,
          hmm: value.analysis.analyzers.jieba.hmm,
          stopwordsEnabled: value.analysis.analyzers.jieba.stopwordsEnabled,
          dictSource: value.analysis.analyzers.jieba.dictSource,
          dictVersion: value.analysis.analyzers.jieba.dictVersion
        }
      }
    }
  };
}

function parseSearchProvider(value: unknown, fallbackValue: SearchProvider): SearchProvider {
  const normalizedValue = parseString(value, fallbackValue).toLowerCase();
  if (normalizedValue === "database") {
    return "database";
  }
  if (normalizedValue === "meili") {
    return "meili";
  }
  if (normalizedValue === "typesense") {
    return "typesense";
  }
  return "bleve";
}

function parseSearchFallbackPolicy(value: unknown, fallbackValue: SearchFallbackPolicy): SearchFallbackPolicy {
  const normalizedValue = parseString(value, fallbackValue).toLowerCase();
  if (normalizedValue === "return_error") {
    return "return_error";
  }
  return "degrade_to_database";
}

function parseSearchAnalyzer(value: unknown, fallbackValue: SearchAnalyzer): SearchAnalyzer {
  const normalizedValue = parseString(value, fallbackValue).toLowerCase();
  if (normalizedValue === "jieba") {
    return "jieba";
  }
  return "simple";
}

function parseSearchJiebaDictSource(value: unknown, fallbackValue: SearchJiebaDictSource): SearchJiebaDictSource {
  const normalizedValue = parseString(value, fallbackValue).toLowerCase();
  if (normalizedValue === "file") {
    return "file";
  }
  return "db";
}

function parseSearchDictVersion(value: unknown, fallbackValue: string): string {
  const dictVersion = parseString(value, fallbackValue);
  return SEARCH_DICT_VERSION_PATTERN.test(dictVersion) ? dictVersion : fallbackValue;
}

function normalizeSearchConfigForSave(value: SearchSystemConfigValue): SearchSystemConfigValue {
  const config = cloneSearchConfig(value);
  if (config.analysis.activeAnalyzer === "simple") {
    config.analysis.analyzers.simple.enabled = true;
  } else {
    config.analysis.analyzers.jieba.enabled = true;
  }
  config.analysis.analyzers.jieba.mode = "search";
  config.analysis.analyzers.jieba.dictVersion = parseSearchDictVersion(
    config.analysis.analyzers.jieba.dictVersion,
    SEARCH_TEMPLATE.analysis.analyzers.jieba.dictVersion
  );
  return config;
}

function parseSearchConfig(value: unknown): SearchSystemConfigValue {
  const payload = asRecord(value);
  if (!payload) {
    return cloneSearchConfig(SEARCH_TEMPLATE);
  }
  const analysis = asRecord(payload.analysis);
  const analyzers = asRecord(analysis?.analyzers);
  const simple = asRecord(analyzers?.simple);
  const jieba = asRecord(analyzers?.jieba);

  const activeAnalyzer = parseSearchAnalyzer(analysis?.activeAnalyzer, SEARCH_TEMPLATE.analysis.activeAnalyzer);

  const parsed: SearchSystemConfigValue = {
    enabled: typeof payload.enabled === "boolean" ? payload.enabled : SEARCH_TEMPLATE.enabled,
    activeProvider: parseSearchProvider(payload.activeProvider, SEARCH_TEMPLATE.activeProvider),
    fallbackPolicy: parseSearchFallbackPolicy(payload.fallbackPolicy, SEARCH_TEMPLATE.fallbackPolicy),
    analysis: {
      activeAnalyzer,
      analyzers: {
        simple: {
          enabled:
            typeof simple?.enabled === "boolean" ? simple.enabled : SEARCH_TEMPLATE.analysis.analyzers.simple.enabled
        },
        jieba: {
          enabled:
            typeof jieba?.enabled === "boolean" ? jieba.enabled : SEARCH_TEMPLATE.analysis.analyzers.jieba.enabled,
          mode: "search",
          hmm: typeof jieba?.hmm === "boolean" ? jieba.hmm : SEARCH_TEMPLATE.analysis.analyzers.jieba.hmm,
          stopwordsEnabled:
            typeof jieba?.stopwordsEnabled === "boolean"
              ? jieba.stopwordsEnabled
              : SEARCH_TEMPLATE.analysis.analyzers.jieba.stopwordsEnabled,
          dictSource: parseSearchJiebaDictSource(
            jieba?.dictSource,
            SEARCH_TEMPLATE.analysis.analyzers.jieba.dictSource
          ),
          dictVersion: parseSearchDictVersion(
            jieba?.dictVersion,
            SEARCH_TEMPLATE.analysis.analyzers.jieba.dictVersion
          )
        }
      }
    }
  };

  return normalizeSearchConfigForSave(parsed);
}

function cloneDataRetentionConfig(value: DataRetentionSystemConfigValue): DataRetentionSystemConfigValue {
  return {
    ...value,
    cleanupTables: [...value.cleanupTables]
  };
}

function parseDataRetentionConfig(value: unknown): DataRetentionSystemConfigValue {
  const payload = asRecord(value);
  if (!payload) {
    return cloneDataRetentionConfig(DATA_RETENTION_TEMPLATE);
  }
  const cleanupTablesRaw = Array.isArray(payload.cleanupTables) ? payload.cleanupTables : [];
  const parsedCleanupTables = cleanupTablesRaw
    .map((item) => (typeof item === "string" ? item.trim().toLowerCase() : ""))
    .filter((item): item is DataRetentionCleanupTable =>
      item === "audit_logs" ||
      item === "auth_captcha_challenges" ||
      item === "auth_risk_states" ||
      item === "user_sessions" ||
      item === "document_attachments" ||
      item === "document_image_assets"
    );
  const cleanupTables =
    parsedCleanupTables.length > 0
      ? Array.from(new Set(parsedCleanupTables))
      : [...DATA_RETENTION_TEMPLATE.cleanupTables];

  return {
    enabled: typeof payload.enabled === "boolean" ? payload.enabled : DATA_RETENTION_TEMPLATE.enabled,
    scheduleMinutes: Math.min(
      24 * 60,
      Math.max(5, parseInteger(payload.scheduleMinutes, DATA_RETENTION_TEMPLATE.scheduleMinutes))
    ),
    cleanupBatchSize: Math.min(
      20000,
      Math.max(100, parseInteger(payload.cleanupBatchSize, DATA_RETENTION_TEMPLATE.cleanupBatchSize))
    ),
    cleanupTables,
    auditLogRetentionDays: Math.min(
      3650,
      Math.max(1, parseInteger(payload.auditLogRetentionDays, DATA_RETENTION_TEMPLATE.auditLogRetentionDays))
    ),
    authCaptchaRetentionHours: Math.min(
      24 * 365,
      Math.max(1, parseInteger(payload.authCaptchaRetentionHours, DATA_RETENTION_TEMPLATE.authCaptchaRetentionHours))
    ),
    authRiskStateRetentionDays: Math.min(
      3650,
      Math.max(1, parseInteger(payload.authRiskStateRetentionDays, DATA_RETENTION_TEMPLATE.authRiskStateRetentionDays))
    ),
    userSessionRetentionDays: Math.min(
      3650,
      Math.max(1, parseInteger(payload.userSessionRetentionDays, DATA_RETENTION_TEMPLATE.userSessionRetentionDays))
    )
  };
}

function parseStringArray(value: unknown, fallbackValue: string[]): string[] {
  if (!Array.isArray(value)) {
    return [...fallbackValue];
  }
  return value
    .map((item) => (typeof item === "string" ? item.trim() : ""))
    .filter((item) => item.length > 0);
}

function parseAuthLDAPTLSMode(value: unknown, fallbackValue: AuthProviderLdapConfig["tlsMode"]): AuthProviderLdapConfig["tlsMode"] {
  const normalizedValue = parseString(value, fallbackValue);
  if (normalizedValue === "starttls") {
    return "starttls";
  }
  if (normalizedValue === "plain") {
    return "plain";
  }
  return "ldaps";
}

function resolvePreferredLDAPProviderID(
  providers: AuthProviderConfig[],
  fallbackProviderID: string,
): string {
  const enabledProvider = providers.find((provider) => provider.enabled && provider.id.trim().length > 0);
  if (enabledProvider) {
    return enabledProvider.id;
  }
  const firstProvider = providers.find((provider) => provider.id.trim().length > 0);
  if (firstProvider) {
    return firstProvider.id;
  }
  return fallbackProviderID;
}

function parseAuthConfig(value: unknown): AuthSystemConfigValue {
  const payload = asRecord(value);
  if (!payload) {
    return {
      ...AUTH_TEMPLATE,
      providers: AUTH_TEMPLATE.providers.map((provider) => ({
        ...provider,
        matchRules: { ...provider.matchRules },
        ldap: { ...provider.ldap }
      })),
      breakGlass: { ...AUTH_TEMPLATE.breakGlass, localAdminEmails: [...AUTH_TEMPLATE.breakGlass.localAdminEmails] }
    };
  }

  const loginModeRaw = parseString(payload.loginMode, AUTH_TEMPLATE.loginMode);
  const loginMode: AuthLoginMode =
    loginModeRaw === "local_only" || loginModeRaw === "ldap_only" || loginModeRaw === "mixed"
      ? loginModeRaw
      : AUTH_TEMPLATE.loginMode;
  const defaultProviderId = parseString(payload.defaultProviderId, AUTH_TEMPLATE.defaultProviderId);
  const allowUserRegister =
    typeof payload.allowUserRegister === "boolean" ? payload.allowUserRegister : AUTH_TEMPLATE.allowUserRegister;

  const providers = Array.isArray(payload.providers) ? payload.providers : [];
  const parsedProviders: AuthProviderConfig[] = providers
    .map((rawProvider) => asRecord(rawProvider))
    .filter((provider): provider is Record<string, unknown> => provider !== null)
    .map((provider) => {
      const matchRules = asRecord(provider.matchRules);
      const ldap = asRecord(provider.ldap);
      return {
        id: parseString(provider.id, AUTH_PROVIDER_TEMPLATE.id),
        name: parseString(provider.name, AUTH_PROVIDER_TEMPLATE.name),
        type: "ldap",
        enabled: typeof provider.enabled === "boolean" ? provider.enabled : AUTH_PROVIDER_TEMPLATE.enabled,
        priority: parseInteger(provider.priority, AUTH_PROVIDER_TEMPLATE.priority),
        matchRules: {
          emailDomains: parseStringArray(matchRules?.emailDomains, AUTH_PROVIDER_TEMPLATE.matchRules.emailDomains),
          usernameRegex: parseString(matchRules?.usernameRegex, AUTH_PROVIDER_TEMPLATE.matchRules.usernameRegex)
        },
        ldap: {
          host: parseString(ldap?.host, AUTH_PROVIDER_TEMPLATE.ldap.host),
          port: parseInteger(ldap?.port, AUTH_PROVIDER_TEMPLATE.ldap.port),
          tlsMode: parseAuthLDAPTLSMode(ldap?.tlsMode, AUTH_PROVIDER_TEMPLATE.ldap.tlsMode),
          baseDN: parseString(ldap?.baseDN, AUTH_PROVIDER_TEMPLATE.ldap.baseDN),
          bindDN: parseString(ldap?.bindDN, AUTH_PROVIDER_TEMPLATE.ldap.bindDN),
          bindPasswordCiphertext: parseString(ldap?.bindPasswordCiphertext, AUTH_PROVIDER_TEMPLATE.ldap.bindPasswordCiphertext),
          userFilter: parseString(ldap?.userFilter, AUTH_PROVIDER_TEMPLATE.ldap.userFilter),
          idAttribute: parseString(ldap?.idAttribute, AUTH_PROVIDER_TEMPLATE.ldap.idAttribute),
          emailAttribute: parseString(ldap?.emailAttribute, AUTH_PROVIDER_TEMPLATE.ldap.emailAttribute),
          nameAttribute: parseString(ldap?.nameAttribute, AUTH_PROVIDER_TEMPLATE.ldap.nameAttribute),
          groupAttribute: parseString(ldap?.groupAttribute, AUTH_PROVIDER_TEMPLATE.ldap.groupAttribute),
          connectTimeoutMs: parseInteger(ldap?.connectTimeoutMs, AUTH_PROVIDER_TEMPLATE.ldap.connectTimeoutMs),
          readTimeoutMs: parseInteger(ldap?.readTimeoutMs, AUTH_PROVIDER_TEMPLATE.ldap.readTimeoutMs)
        }
      };
    });

  const breakGlass = asRecord(payload.breakGlass);
  const parsedBreakGlass = {
    enabled: typeof breakGlass?.enabled === "boolean" ? breakGlass.enabled : AUTH_TEMPLATE.breakGlass.enabled,
    localAdminEmails: parseStringArray(breakGlass?.localAdminEmails, AUTH_TEMPLATE.breakGlass.localAdminEmails)
  };

  return {
    loginMode,
    defaultProviderId,
    allowUserRegister,
    providers: parsedProviders.length > 0 ? parsedProviders : [{ ...AUTH_PROVIDER_TEMPLATE, matchRules: { ...AUTH_PROVIDER_TEMPLATE.matchRules }, ldap: { ...AUTH_PROVIDER_TEMPLATE.ldap } }],
    breakGlass: parsedBreakGlass
  };
}

function cloneAuthConfig(value: AuthSystemConfigValue): AuthSystemConfigValue {
  return {
    ...value,
    providers: value.providers.map((provider) => ({
      ...provider,
      matchRules: {
        ...provider.matchRules,
        emailDomains: [...provider.matchRules.emailDomains]
      },
      ldap: { ...provider.ldap }
    })),
    breakGlass: {
      ...value.breakGlass,
      localAdminEmails: [...value.breakGlass.localAdminEmails]
    }
  };
}

function parseSitemapConfig(value: unknown): SitemapSystemConfigValue {
  const payload = asRecord(value);
  if (!payload) {
    return { ...SITEMAP_TEMPLATE };
  }

  const rawMode = parseString(payload.generationMode, SITEMAP_TEMPLATE.generationMode);
  const generationMode: SitemapGenerationMode =
    rawMode === "all_public" || rawMode === "updated_within_days" ? rawMode : SITEMAP_TEMPLATE.generationMode;
  const parsedMaxUpdatedWithinDays = parseInteger(payload.maxUpdatedWithinDays, SITEMAP_TEMPLATE.maxUpdatedWithinDays);
  const maxUpdatedWithinDays = Math.min(3650, Math.max(1, parsedMaxUpdatedWithinDays));

  return {
    generationMode,
    maxUpdatedWithinDays
  };
}

function parseImageHostingConfig(value: unknown): ImageHostingConfig {
  return normalizeImageHostingConfig(value);
}

function normalizeIntegerInput(rawValue: string, fallbackValue: number): number {
  const parsedValue = Number.parseInt(rawValue, 10);
  if (!Number.isFinite(parsedValue)) {
    return fallbackValue;
  }
  return Math.trunc(parsedValue);
}

function buildImageHostingTemplatePreview(template: string): string {
  const normalizedTemplate = template.trim();
  if (!normalizedTemplate) {
    return "-";
  }
  const replacements: Record<string, string> = {
    "{spaceId}": "space-demo",
    "{docId}": "doc-demo",
    "{yyyy}": "2026",
    "{mm}": "03",
    "{dd}": "01",
    "{hh}": "14",
    "{assetId}": "01HQ7GZ4M7P1ANR5TYA9D3K8CV",
    "{origName}": "roadmap",
    "{ext}": "png",
    "{uploaderId}": "user-demo"
  };
  let output = normalizedTemplate;
  Object.entries(replacements).forEach(([token, value]) => {
    output = output.split(token).join(value);
  });
  output = output.replace(/\{rand:(4|5|6|7|8|9|10)\}/gi, (_, rawLength: string) => {
    const length = Number.parseInt(rawLength, 10);
    if (!Number.isFinite(length) || length < 4 || length > 10) {
      return "Ab3D9k";
    }
    const seed = "aB3dE5fG7hI9JkLmN0pQ2rS4tU6vW8xYz1";
    return seed.slice(0, length);
  });
  return output;
}

export function AdminSystemConfigsPage({ dataGateway }: AdminSystemConfigsPageProps) {
  const [configs, setConfigs] = useState<AdminSystemConfig[]>([]);
  const [selectedKey, setSelectedKey] = useState<SystemConfigKey>("site");
  const [imageHostingProviderTab, setImageHostingProviderTab] = useState<ImageHostingProvider>("local");
  const [selectedAuthProviderID, setSelectedAuthProviderID] = useState<string>(AUTH_TEMPLATE.defaultProviderId);

  const [siteDraft, setSiteDraft] = useState<SiteSystemConfigValue>({ ...SITE_TEMPLATE });
  const [editorDraft, setEditorDraft] = useState<EditorSystemConfigValue>({ ...EDITOR_TEMPLATE });
  const [securityDraft, setSecurityDraft] = useState<SecuritySystemConfigValue>({ ...SECURITY_TEMPLATE });
  const [searchDraft, setSearchDraft] = useState<SearchSystemConfigValue>(cloneSearchConfig(SEARCH_TEMPLATE));
  const [dataRetentionDraft, setDataRetentionDraft] = useState<DataRetentionSystemConfigValue>({
    ...cloneDataRetentionConfig(DATA_RETENTION_TEMPLATE)
  });
  const [authDraft, setAuthDraft] = useState<AuthSystemConfigValue>(cloneAuthConfig(AUTH_TEMPLATE));
  const [sitemapDraft, setSitemapDraft] = useState<SitemapSystemConfigValue>({ ...SITEMAP_TEMPLATE });
  const [imageHostingDraft, setImageHostingDraft] = useState<ImageHostingConfig>(
    cloneImageHostingConfig(IMAGE_HOSTING_TEMPLATE)
  );

  const [dirtyKeys, setDirtyKeys] = useState<Record<SystemConfigKey, boolean>>({
    site: false,
    editor: false,
    security: false,
    search: false,
    "data-retention": false,
    auth: false,
    sitemap: false,
    "image-hosting": false
  });
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [testingLDAP, setTestingLDAP] = useState(false);
  const [runningCleanup, setRunningCleanup] = useState(false);
  const [runningSearchRebuild, setRunningSearchRebuild] = useState(false);
  const [searchIndexStatus, setSearchIndexStatus] = useState<AdminSearchIndexStatusResult | null>(null);
  const [searchStatusLoading, setSearchStatusLoading] = useState(false);
  const imageHostingTemplateInputRefs = useRef<Record<ImageHostingProvider, HTMLInputElement | null>>({
    local: null,
    "cloudflare-r2": null,
    "aliyun-oss": null
  });

  const openToast = useCallback((message: string, variant: "success" | "info" | "error" = "error") => {
    showToast(message, variant);
  }, []);

  const tabMap = useMemo(
    () =>
      SYSTEM_CONFIG_TABS.reduce<Record<SystemConfigKey, SystemConfigTabItem>>((accumulator, item) => {
        accumulator[item.key] = item;
        return accumulator;
      }, {} as Record<SystemConfigKey, SystemConfigTabItem>),
    []
  );
  const selectedTab = tabMap[selectedKey];

  const selectedConfig = useMemo(
    () => configs.find((item) => item.configKey === selectedKey) ?? null,
    [configs, selectedKey]
  );

  const findConfigValue = useCallback(
    (key: SystemConfigKey): Record<string, unknown> | null => {
      const item = configs.find((entry) => entry.configKey === key);
      return item?.value ?? null;
    },
    [configs]
  );

  const loadConfigs = useCallback(async () => {
    setLoading(true);
    try {
      const payload = await dataGateway.admin.listSystemConfigs();
      setConfigs(payload);
    } catch (error) {
      openToast(`加载系统配置失败：${formatError(error)}`);
      setConfigs([]);
    } finally {
      setLoading(false);
    }
  }, [dataGateway, openToast]);

  const loadSearchIndexStatus = useCallback(async (options?: { silent?: boolean }) => {
    const silent = options?.silent === true;
    if (!silent) {
      setSearchStatusLoading(true);
    }
    try {
      const statusResult = await dataGateway.admin.getSearchIndexStatus();
      setSearchIndexStatus(statusResult);
    } catch (error) {
      if (!silent) {
        openToast(`加载索引状态失败：${formatError(error)}`);
        setSearchIndexStatus(null);
      }
    } finally {
      if (!silent) {
        setSearchStatusLoading(false);
      }
    }
  }, [dataGateway.admin, openToast]);

  useEffect(() => {
    void loadConfigs();
  }, [loadConfigs]);

  useEffect(() => {
    if (selectedKey !== "search") {
      return;
    }
    void loadSearchIndexStatus();
    const timer = window.setInterval(() => {
      void loadSearchIndexStatus({ silent: true });
    }, 10000);
    return () => {
      window.clearInterval(timer);
    };
  }, [loadSearchIndexStatus, selectedKey]);

  useEffect(() => {
    if (!dirtyKeys.site) {
      setSiteDraft(parseSiteConfig(findConfigValue("site")));
    }
    if (!dirtyKeys.editor) {
      setEditorDraft(parseEditorConfig(findConfigValue("editor")));
    }
    if (!dirtyKeys.security) {
      setSecurityDraft(parseSecurityConfig(findConfigValue("security")));
    }
    if (!dirtyKeys.search) {
      setSearchDraft(parseSearchConfig(findConfigValue("search")));
    }
    if (!dirtyKeys["data-retention"]) {
      setDataRetentionDraft(parseDataRetentionConfig(findConfigValue("data-retention")));
    }
    if (!dirtyKeys.auth) {
      const parsedConfig = parseAuthConfig(findConfigValue("auth"));
      setAuthDraft(parsedConfig);
      setSelectedAuthProviderID(parsedConfig.providers[0]?.id ?? parsedConfig.defaultProviderId);
    }
    if (!dirtyKeys.sitemap) {
      setSitemapDraft(parseSitemapConfig(findConfigValue("sitemap")));
    }
    if (!dirtyKeys["image-hosting"]) {
      const parsedConfig = parseImageHostingConfig(findConfigValue("image-hosting"));
      setImageHostingDraft(parsedConfig);
      setImageHostingProviderTab(parsedConfig.defaultProvider);
    }
  }, [dirtyKeys, findConfigValue]);

  const markDirty = useCallback((key: SystemConfigKey) => {
    setDirtyKeys((previous) => ({
      ...previous,
      [key]: true
    }));
  }, []);

  const clearDirty = useCallback((key: SystemConfigKey) => {
    setDirtyKeys((previous) => ({
      ...previous,
      [key]: false
    }));
  }, []);

  const isSelectedDirty = dirtyKeys[selectedKey];

  const handleResetTemplate = useCallback(() => {
    switch (selectedKey) {
      case "site":
        setSiteDraft({ ...SITE_TEMPLATE });
        markDirty("site");
        return;
      case "editor":
        setEditorDraft({ ...EDITOR_TEMPLATE });
        markDirty("editor");
        return;
      case "security":
        setSecurityDraft({ ...SECURITY_TEMPLATE });
        markDirty("security");
        return;
      case "search":
        setSearchDraft((previous) => {
          const nextConfig = cloneSearchConfig(SEARCH_TEMPLATE);
          nextConfig.analysis.analyzers.jieba.dictSource = previous.analysis.analyzers.jieba.dictSource;
          nextConfig.analysis.analyzers.jieba.dictVersion = previous.analysis.analyzers.jieba.dictVersion;
          return nextConfig;
        });
        markDirty("search");
        return;
      case "data-retention":
        setDataRetentionDraft(cloneDataRetentionConfig(DATA_RETENTION_TEMPLATE));
        markDirty("data-retention");
        return;
      case "auth":
        setAuthDraft(cloneAuthConfig(AUTH_TEMPLATE));
        setSelectedAuthProviderID(AUTH_TEMPLATE.defaultProviderId);
        markDirty("auth");
        return;
      case "sitemap":
        setSitemapDraft({ ...SITEMAP_TEMPLATE });
        markDirty("sitemap");
        return;
      case "image-hosting":
        setImageHostingDraft(cloneImageHostingConfig(IMAGE_HOSTING_TEMPLATE));
        setImageHostingProviderTab(IMAGE_HOSTING_TEMPLATE.defaultProvider);
        markDirty("image-hosting");
        return;
      default:
        return;
    }
  }, [markDirty, selectedKey]);

  const handleLoadCurrent = useCallback(() => {
    switch (selectedKey) {
      case "site":
        setSiteDraft(parseSiteConfig(findConfigValue("site")));
        clearDirty("site");
        return;
      case "editor":
        setEditorDraft(parseEditorConfig(findConfigValue("editor")));
        clearDirty("editor");
        return;
      case "security":
        setSecurityDraft(parseSecurityConfig(findConfigValue("security")));
        clearDirty("security");
        return;
      case "search":
        setSearchDraft(parseSearchConfig(findConfigValue("search")));
        clearDirty("search");
        return;
      case "data-retention":
        setDataRetentionDraft(parseDataRetentionConfig(findConfigValue("data-retention")));
        clearDirty("data-retention");
        return;
      case "auth": {
        const parsedConfig = parseAuthConfig(findConfigValue("auth"));
        setAuthDraft(parsedConfig);
        setSelectedAuthProviderID(parsedConfig.providers[0]?.id ?? parsedConfig.defaultProviderId);
        clearDirty("auth");
        return;
      }
      case "sitemap":
        setSitemapDraft(parseSitemapConfig(findConfigValue("sitemap")));
        clearDirty("sitemap");
        return;
      case "image-hosting": {
        const parsedConfig = parseImageHostingConfig(findConfigValue("image-hosting"));
        setImageHostingDraft(parsedConfig);
        setImageHostingProviderTab(parsedConfig.defaultProvider);
        clearDirty("image-hosting");
        return;
      }
      default:
        return;
    }
  }, [clearDirty, findConfigValue, selectedKey]);

  const buildSelectedPayload = useCallback((): Record<string, unknown> => {
    switch (selectedKey) {
      case "site":
        return {
          allowRegistration: siteDraft.allowRegistration,
          defaultSpaceVisibility: siteDraft.defaultSpaceVisibility
        };
      case "editor":
        return {
          autosaveIntervalSeconds: editorDraft.autosaveIntervalSeconds,
          maxDocumentSizeKB: editorDraft.maxDocumentSizeKB
        };
      case "security":
        return {
          accessTokenTTLMinutes: securityDraft.accessTokenTTLMinutes,
          refreshTokenTTLMinutes: securityDraft.refreshTokenTTLMinutes
        };
      case "search":
        return normalizeSearchConfigForSave(searchDraft) as unknown as Record<string, unknown>;
      case "data-retention":
        return {
          enabled: dataRetentionDraft.enabled,
          scheduleMinutes: dataRetentionDraft.scheduleMinutes,
          cleanupBatchSize: dataRetentionDraft.cleanupBatchSize,
          cleanupTables: [...dataRetentionDraft.cleanupTables],
          auditLogRetentionDays: dataRetentionDraft.auditLogRetentionDays,
          authCaptchaRetentionHours: dataRetentionDraft.authCaptchaRetentionHours,
          authRiskStateRetentionDays: dataRetentionDraft.authRiskStateRetentionDays,
          userSessionRetentionDays: dataRetentionDraft.userSessionRetentionDays
        };
      case "auth":
        return cloneAuthConfig(authDraft) as unknown as Record<string, unknown>;
      case "sitemap":
        return {
          generationMode: sitemapDraft.generationMode,
          maxUpdatedWithinDays: sitemapDraft.maxUpdatedWithinDays
        };
      case "image-hosting":
        return cloneImageHostingConfig(imageHostingDraft) as unknown as Record<string, unknown>;
      default:
        return {};
    }
  }, [authDraft, dataRetentionDraft, editorDraft, imageHostingDraft, searchDraft, securityDraft, selectedKey, sitemapDraft, siteDraft]);

  const handleSave = useCallback(async () => {
    const payload = buildSelectedPayload();
    setSaving(true);
    try {
      const result = await dataGateway.admin.upsertSystemConfig({
        configKey: selectedKey,
        value: payload,
        expectedVersion: selectedConfig?.version ?? 0
      });
      clearDirty(selectedKey);
      openToast(`配置已保存：${selectedTab.label}（v${result.version}）`, "success");
      await loadConfigs();
      if (selectedKey === "search") {
        await loadSearchIndexStatus();
      }
    } catch (error) {
      openToast(`保存系统配置失败：${formatError(error)}`);
    } finally {
      setSaving(false);
    }
  }, [
    buildSelectedPayload,
    clearDirty,
    dataGateway.admin,
    loadConfigs,
    loadSearchIndexStatus,
    openToast,
    selectedConfig?.version,
    selectedKey,
    selectedTab.label
  ]);

  const setCloudflareField = useCallback((field: keyof CloudflareR2Config, value: string) => {
    setImageHostingDraft((previousConfig) => ({
      ...previousConfig,
      cloudflareR2: {
        ...previousConfig.cloudflareR2,
        [field]: parseString(value, "")
      }
    }));
    markDirty("image-hosting");
  }, [markDirty]);

  const setAliyunField = useCallback((field: keyof AliyunOssConfig, value: string) => {
    setImageHostingDraft((previousConfig) => ({
      ...previousConfig,
      aliyunOss: {
        ...previousConfig.aliyunOss,
        [field]: parseString(value, "")
      }
    }));
    markDirty("image-hosting");
  }, [markDirty]);

  const setLocalField = useCallback((field: keyof LocalImageHostingConfig, value: string) => {
    setImageHostingDraft((previousConfig) => ({
      ...previousConfig,
      local: {
        ...previousConfig.local,
        [field]: parseString(value, "")
      }
    }));
    markDirty("image-hosting");
  }, [markDirty]);

  const setImageProcessingMode = useCallback((mode: ImageHostingImageProcessingMode) => {
    setImageHostingDraft((previousConfig) => ({
      ...previousConfig,
      imageProcessing: {
        ...previousConfig.imageProcessing,
        mode
      }
    }));
    markDirty("image-hosting");
  }, [markDirty]);

  const setImageQualityPreset = useCallback((qualityPreset: ImageHostingImageQualityPreset) => {
    setImageHostingDraft((previousConfig) => ({
      ...previousConfig,
      imageProcessing: {
        ...previousConfig.imageProcessing,
        qualityPreset
      }
    }));
    markDirty("image-hosting");
  }, [markDirty]);

  const setImageProcessingMaxWidth = useCallback((rawValue: string) => {
    setImageHostingDraft((previousConfig) => {
      const parsed = normalizeIntegerInput(rawValue, previousConfig.imageProcessing.maxWidth);
      return {
        ...previousConfig,
        imageProcessing: {
          ...previousConfig.imageProcessing,
          maxWidth: Math.min(IMAGE_HOSTING_IMAGE_MAX_WIDTH_MAX, Math.max(IMAGE_HOSTING_IMAGE_MAX_WIDTH_MIN, parsed))
        }
      };
    });
    markDirty("image-hosting");
  }, [markDirty]);

  const setImageProcessingMaxHeight = useCallback((rawValue: string) => {
    setImageHostingDraft((previousConfig) => {
      const parsed = normalizeIntegerInput(rawValue, previousConfig.imageProcessing.maxHeight);
      return {
        ...previousConfig,
        imageProcessing: {
          ...previousConfig.imageProcessing,
          maxHeight: Math.min(IMAGE_HOSTING_IMAGE_MAX_HEIGHT_MAX, Math.max(IMAGE_HOSTING_IMAGE_MAX_HEIGHT_MIN, parsed))
        }
      };
    });
    markDirty("image-hosting");
  }, [markDirty]);

  const setImageProcessingSkipAnimated = useCallback((skipAnimated: boolean) => {
    setImageHostingDraft((previousConfig) => ({
      ...previousConfig,
      imageProcessing: {
        ...previousConfig.imageProcessing,
        skipAnimated
      }
    }));
    markDirty("image-hosting");
  }, [markDirty]);

  const bindImageHostingTemplateInputRef = useCallback(
    (provider: ImageHostingProvider) => (node: HTMLInputElement | null) => {
      imageHostingTemplateInputRefs.current[provider] = node;
    },
    []
  );

  const insertImageHostingTemplateVariable = useCallback(
    (
      provider: ImageHostingProvider,
      currentValue: string,
      token: string,
      onChange: (nextValue: string) => void
    ) => {
      const inputNode = imageHostingTemplateInputRefs.current[provider];
      const normalizedCurrentValue = currentValue ?? "";
      const insertFrom =
        inputNode && document.activeElement === inputNode
          ? inputNode.selectionStart ?? normalizedCurrentValue.length
          : normalizedCurrentValue.length;
      const insertTo =
        inputNode && document.activeElement === inputNode
          ? inputNode.selectionEnd ?? normalizedCurrentValue.length
          : normalizedCurrentValue.length;
      const nextValue =
        normalizedCurrentValue.slice(0, insertFrom) + token + normalizedCurrentValue.slice(insertTo);
      onChange(nextValue);
      window.requestAnimationFrame(() => {
        const nextCursor = insertFrom + token.length;
        const latestInputNode = imageHostingTemplateInputRefs.current[provider];
        if (!latestInputNode) {
          return;
        }
        latestInputNode.focus();
        latestInputNode.setSelectionRange(nextCursor, nextCursor);
      });
    },
    []
  );

  const selectedAuthProvider = useMemo(() => {
    return authDraft.providers.find((provider) => provider.id === selectedAuthProviderID) ?? authDraft.providers[0] ?? null;
  }, [authDraft.providers, selectedAuthProviderID]);

  const updateSelectedAuthProvider = useCallback(
    (updater: (provider: AuthProviderConfig) => AuthProviderConfig) => {
      setAuthDraft((previousConfig) => {
        const nextProviders = previousConfig.providers.map((provider) => {
          if (provider.id !== selectedAuthProviderID) {
            return provider;
          }
          return updater(provider);
        });
        return {
          ...previousConfig,
          providers: nextProviders
        };
      });
      markDirty("auth");
    },
    [markDirty, selectedAuthProviderID]
  );

  const handleTestLDAPConnection = useCallback(async () => {
    if (!selectedAuthProvider) {
      openToast("请先配置 LDAP Provider", "info");
      return;
    }
    setTestingLDAP(true);
    try {
      const payload = cloneAuthConfig(authDraft) as unknown as Record<string, unknown>;
      const result = await dataGateway.admin.testAuthLDAPConnection({
        value: payload,
        providerId: selectedAuthProvider.id
      });
      if (result.ok) {
        openToast(`LDAP 连接测试成功：${selectedAuthProvider.name}`, "success");
      } else {
        openToast(`LDAP 连接测试失败：${selectedAuthProvider.name}`);
      }
    } catch (error) {
      openToast(`LDAP 连接测试失败：${formatError(error)}`);
    } finally {
      setTestingLDAP(false);
    }
  }, [authDraft, dataGateway.admin, openToast, selectedAuthProvider]);

  const handleRunDataRetentionCleanup = useCallback(async () => {
    if (dirtyKeys["data-retention"]) {
      openToast("请先保存当前数据清理配置，再执行立即清理", "info");
      return;
    }
    setRunningCleanup(true);
    try {
      const result = await dataGateway.admin.runDataRetentionCleanup();
      if (result.totalDeleted > 0) {
        openToast(
          `清理完成：共删除 ${result.totalDeleted} 条（审计 ${result.deletedAuditLogs}、验证码 ${result.deletedAuthCaptchaChallenges}、风控 ${result.deletedAuthRiskStates}、会话 ${result.deletedUserSessions}、附件引用 ${result.deletedDocumentAttachments}、附件文件 ${result.deletedAttachmentBlobs}、图片 ${result.deletedDocumentImageAssets}）`,
          "success"
        );
      } else {
        openToast("清理完成：当前没有超过保留时长的数据", "info");
      }
    } catch (error) {
      openToast(`立即清理失败：${formatError(error)}`);
    } finally {
      setRunningCleanup(false);
    }
  }, [dataGateway.admin, dirtyKeys, openToast]);

  const handleRunSearchIndexRebuild = useCallback(async () => {
    if (dirtyKeys.search) {
      openToast("请先保存当前全文检索配置，再执行索引重建", "info");
      return;
    }
    setRunningSearchRebuild(true);
    try {
      const result = await dataGateway.admin.runSearchIndexRebuild();
      openToast(
        `索引重建完成：provider=${result.provider}，已写入 ${result.indexedDocuments} 条文档`,
        "success"
      );
      await loadSearchIndexStatus();
    } catch (error) {
      openToast(`全文索引重建失败：${formatError(error)}`);
    } finally {
      setRunningSearchRebuild(false);
    }
  }, [dataGateway.admin, dirtyKeys.search, loadSearchIndexStatus, openToast]);

  return (
    <section aria-label="系统配置管理">
      <AdminPageCard>
        <div className="overflow-hidden rounded-md border border-slate-200 bg-white">
          <div className="grid min-h-[560px] gap-0 md:grid-cols-[240px_minmax(0,1fr)]">
            <aside className="hidden bg-transparent p-2 md:block">
              <nav className="space-y-1" aria-label="配置分组">
                {SYSTEM_CONFIG_TABS.map((tab) => {
                  const isActive = tab.key === selectedKey;
                  const TabIcon = tab.icon;
                  return (
                    <button
                      key={tab.key}
                      type="button"
                      className={`w-full appearance-none rounded-lg border border-transparent px-3 py-2.5 text-left shadow-none outline-none transition focus-visible:ring-2 focus-visible:ring-sky-200 ${isActive
                          ? "bg-slate-200 text-slate-900"
                          : "text-slate-700 hover:bg-slate-200/70"
                        }`}
                      onClick={() => setSelectedKey(tab.key)}
                      disabled={saving}
                    >
                      <p className="flex items-center gap-2.5 text-sm font-medium">
                        <TabIcon size={16} />
                        <span>{tab.label}</span>
                      </p>
                      <p className="mt-0.5 pl-[26px] text-xs text-slate-500">{tab.description}</p>
                    </button>
                  );
                })}
              </nav>
            </aside>

            <main className="flex min-h-[560px] flex-col">
              <header className="flex h-16 shrink-0 items-center justify-between gap-3 border-b border-slate-200 px-4">
                <div>
                  <h3 className="text-base font-semibold text-slate-800">{selectedTab.label}</h3>
                </div>
                <AdminToolbarActions>
                  <Button type="button" variant="outline" disabled={loading || saving} onClick={handleResetTemplate}>
                    模板填充
                  </Button>
                  <Button type="button" variant="outline" disabled={loading || saving} onClick={handleLoadCurrent}>
                    载入线上值
                  </Button>
                  <Button type="button" variant="outline" disabled={loading || saving} onClick={() => void loadConfigs()}>
                    <RefreshCw size={14} />
                    <span>{loading ? "刷新中..." : "刷新"}</span>
                  </Button>
                  {selectedKey === "auth" ? (
                    <Button
                      type="button"
                      variant="outline"
                      disabled={loading || saving || testingLDAP}
                      onClick={() => void handleTestLDAPConnection()}
                    >
                      <RefreshCw size={14} />
                      <span>{testingLDAP ? "测试中..." : "测试 LDAP 连接"}</span>
                    </Button>
                  ) : null}
                  {selectedKey === "data-retention" ? (
                    <Button
                      type="button"
                      variant="outline"
                      disabled={loading || saving || runningCleanup}
                      onClick={() => void handleRunDataRetentionCleanup()}
                    >
                      <RefreshCw size={14} />
                      <span>{runningCleanup ? "清理中..." : "立即清理"}</span>
                    </Button>
                  ) : null}
                  {selectedKey === "search" ? (
                    <Button
                      type="button"
                      variant="outline"
                      disabled={
                        loading ||
                        saving ||
                        runningSearchRebuild ||
                        searchIndexStatus?.rebuildInProgress === true
                      }
                      onClick={() => void handleRunSearchIndexRebuild()}
                    >
                      <RefreshCw size={14} />
                      <span>
                        {runningSearchRebuild || searchIndexStatus?.rebuildInProgress === true ? "重建中..." : "重建索引"}
                      </span>
                    </Button>
                  ) : null}
                  {selectedKey === "search" ? (
                    <Button
                      type="button"
                      variant="outline"
                      disabled={loading || saving || searchStatusLoading}
                      onClick={() => void loadSearchIndexStatus()}
                    >
                      <RefreshCw size={14} />
                      <span>{searchStatusLoading ? "状态刷新中..." : "刷新索引状态"}</span>
                    </Button>
                  ) : null}
                  <Button type="button" disabled={loading || saving || !isSelectedDirty} onClick={() => void handleSave()}>
                    <Save size={14} />
                    <span>{saving ? "保存中..." : "保存配置"}</span>
                  </Button>
                </AdminToolbarActions>
              </header>

              {loading ? (
                <div className="mx-4 mt-4 flex items-center gap-2 rounded-md border border-slate-200 bg-slate-50 px-3 py-2 text-sm text-slate-600">
                  <LoaderCircle size={15} className="animate-spin" />
                  <span>正在加载系统配置...</span>
                </div>
              ) : null}

              <div className="mx-4 mt-4 rounded-md border border-slate-200 bg-slate-50 px-3 py-2 text-xs text-slate-600">
                配置键：<code>{selectedKey}</code>
                {selectedConfig ? `，版本：v${selectedConfig.version}` : "，版本：未创建"}
                {selectedConfig ? `，更新时间：${formatDateTime(selectedConfig.updatedAt)}` : ""}
                {selectedConfig?.updatedByUserId ? `，更新人：${selectedConfig.updatedByUserId}` : ""}
              </div>

              <div className="flex-1 overflow-y-auto p-4">

                {selectedKey === "site" ? (
                  <div className="rounded-md border border-slate-200 bg-white p-4">
                    <div className="grid gap-4 sm:grid-cols-2">
                      <label className="flex items-center gap-2.5 rounded-md border border-slate-200 bg-slate-50 px-3 py-3">
                        <Checkbox
                          checked={siteDraft.allowRegistration}
                          onCheckedChange={(checked) => {
                            setSiteDraft((previous) => ({
                              ...previous,
                              allowRegistration: checked === true
                            }));
                            markDirty("site");
                          }}
                          disabled={saving}
                        />
                        <div className="space-y-0.5">
                          <span className="text-sm font-medium text-slate-700">允许新用户注册</span>
                          <p className="text-xs text-slate-500">关闭后，前台注册接口会拒绝请求。</p>
                        </div>
                      </label>
                      <label className="space-y-1.5">
                        <span className="text-xs font-semibold tracking-wide text-slate-600">默认空间可见性</span>
                        <Select
                          value={siteDraft.defaultSpaceVisibility}
                          onValueChange={(value) => {
                            setSiteDraft((previous) => ({
                              ...previous,
                              defaultSpaceVisibility: value as SpaceVisibility
                            }));
                            markDirty("site");
                          }}
                          disabled={saving}
                        >
                          <SelectTrigger>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            {SPACE_VISIBILITY_OPTIONS.map((option) => (
                              <SelectItem key={option.value} value={option.value}>
                                {option.label}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      </label>
                    </div>
                  </div>
                ) : null}

                {selectedKey === "editor" ? (
                  <div className="rounded-md border border-slate-200 bg-white p-4">
                    <div className="grid gap-4 sm:grid-cols-2">
                      <label className="space-y-1.5">
                        <span className="text-xs font-semibold tracking-wide text-slate-600">自动保存间隔（秒）</span>
                        <Input
                          type="number"
                          min={5}
                          max={600}
                          value={String(editorDraft.autosaveIntervalSeconds)}
                          onChange={(event) => {
                            setEditorDraft((previous) => ({
                              ...previous,
                              autosaveIntervalSeconds: normalizeIntegerInput(
                                event.target.value,
                                previous.autosaveIntervalSeconds
                              )
                            }));
                            markDirty("editor");
                          }}
                          disabled={saving}
                        />
                      </label>
                      <label className="space-y-1.5">
                        <span className="text-xs font-semibold tracking-wide text-slate-600">文档最大体积（KB）</span>
                        <Input
                          type="number"
                          min={64}
                          max={4096}
                          value={String(editorDraft.maxDocumentSizeKB)}
                          onChange={(event) => {
                            setEditorDraft((previous) => ({
                              ...previous,
                              maxDocumentSizeKB: normalizeIntegerInput(event.target.value, previous.maxDocumentSizeKB)
                            }));
                            markDirty("editor");
                          }}
                          disabled={saving}
                        />
                      </label>
                    </div>
                  </div>
                ) : null}

                {selectedKey === "security" ? (
                  <div className="rounded-md border border-slate-200 bg-white p-4">
                    <div className="grid gap-4 sm:grid-cols-2">
                      <label className="space-y-1.5">
                        <span className="text-xs font-semibold tracking-wide text-slate-600">Access Token 时长（分钟）</span>
                        <Input
                          type="number"
                          min={5}
                          max={1440}
                          value={String(securityDraft.accessTokenTTLMinutes)}
                          onChange={(event) => {
                            setSecurityDraft((previous) => ({
                              ...previous,
                              accessTokenTTLMinutes: normalizeIntegerInput(
                                event.target.value,
                                previous.accessTokenTTLMinutes
                              )
                            }));
                            markDirty("security");
                          }}
                          disabled={saving}
                        />
                      </label>
                      <label className="space-y-1.5">
                        <span className="text-xs font-semibold tracking-wide text-slate-600">Refresh Token 时长（分钟）</span>
                        <Input
                          type="number"
                          min={60}
                          max={43200}
                          value={String(securityDraft.refreshTokenTTLMinutes)}
                          onChange={(event) => {
                            setSecurityDraft((previous) => ({
                              ...previous,
                              refreshTokenTTLMinutes: normalizeIntegerInput(
                                event.target.value,
                                previous.refreshTokenTTLMinutes
                              )
                            }));
                            markDirty("security");
                          }}
                          disabled={saving}
                        />
                      </label>
                    </div>
                  </div>
                ) : null}

                {selectedKey === "search" ? (
                  <div className="space-y-4 rounded-md border border-slate-200 bg-white p-4">
                    <div className="rounded-md border border-slate-200 bg-slate-50 p-3">
                      <div className="flex items-center justify-between gap-2">
                        <p className="text-xs font-semibold tracking-wide text-slate-700">索引运行状态</p>
                        {searchStatusLoading ? (
                          <span className="inline-flex items-center gap-1 text-[11px] text-slate-500">
                            <LoaderCircle size={12} className="animate-spin" />
                            同步中
                          </span>
                        ) : null}
                      </div>
                      <div className="mt-2 grid gap-2 text-xs text-slate-600 sm:grid-cols-2">
                        <p>
                          重建任务：
                          <span className={searchIndexStatus?.rebuildInProgress ? "text-amber-600" : "text-slate-600"}>
                            {searchIndexStatus?.rebuildInProgress ? "进行中" : "空闲"}
                          </span>
                        </p>
                        <p>
                          运行状态：
                          <span className={searchIndexStatus?.providerHealthy ? "text-emerald-600" : "text-amber-600"}>
                            {searchIndexStatus?.providerHealthy ? "健康" : "待检查/异常"}
                          </span>
                        </p>
                        <p>当前生效引擎：{searchIndexStatus?.effectiveProvider || "-"}</p>
                        <p>配置活跃引擎：{searchIndexStatus?.activeProvider || "-"}</p>
                        <p>活跃分词器：{searchIndexStatus?.activeAnalyzer || "-"}</p>
                        <p>
                          索引文档数：
                          {searchIndexStatus?.supportsDocCount
                            ? String(searchIndexStatus.indexedDocuments)
                            : "当前引擎不支持统计"}
                        </p>
                        <p>最近索引变更时间：{formatDateTime(searchIndexStatus?.lastRebuildAt ?? null)}</p>
                        <p>最近索引变更来源：{formatSearchRebuildSource(searchIndexStatus?.lastRebuildSource ?? "")}</p>
                        <p>最近索引变更文档数：{searchIndexStatus?.lastRebuildIndexedDocuments ?? 0}</p>
                      </div>
                      {searchIndexStatus?.providerMessage ? (
                        <p className="mt-2 rounded border border-amber-200 bg-amber-50 px-2 py-1 text-[11px] text-amber-700">
                          引擎提示：{searchIndexStatus.providerMessage}
                        </p>
                      ) : null}
                    </div>

                    <label className="flex items-center gap-2.5 rounded-md border border-slate-200 bg-slate-50 px-3 py-3">
                      <Checkbox
                        checked={searchDraft.enabled}
                        onCheckedChange={(checked) => {
                          setSearchDraft((previous) => ({
                            ...previous,
                            enabled: checked === true
                          }));
                          markDirty("search");
                        }}
                        disabled={saving}
                      />
                      <div className="space-y-0.5">
                        <span className="text-sm font-medium text-slate-700">启用全文检索</span>
                        <p className="text-xs text-slate-500">
                          关闭后前台不展示检索入口，且检索请求不会执行。
                        </p>
                      </div>
                    </label>

                    <div className="grid gap-4 sm:grid-cols-2">
                      <label className="space-y-1.5">
                        <span className="text-xs font-semibold tracking-wide text-slate-600">活跃检索引擎</span>
                        <Select
                          value={searchDraft.activeProvider}
                          onValueChange={(value) => {
                            setSearchDraft((previous) => ({
                              ...previous,
                              activeProvider: value as SearchProvider
                            }));
                            markDirty("search");
                          }}
                          disabled={saving}
                        >
                          <SelectTrigger>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            {SEARCH_PROVIDER_OPTIONS.map((option) => (
                              <SelectItem key={option.value} value={option.value}>
                                {option.label}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                        <p className="text-xs text-slate-500">
                          {searchDraft.activeProvider === "database"
                            ? "database 引擎为数据库 LIKE 检索，仅支持简单搜索（无倒排索引与高级排序）。"
                            : "外部引擎模式需对应 provider 已部署并联通。"}
                        </p>
                      </label>

                      <label className="space-y-1.5">
                        <span className="text-xs font-semibold tracking-wide text-slate-600">降级策略</span>
                        <Select
                          value={searchDraft.fallbackPolicy}
                          onValueChange={(value) => {
                            setSearchDraft((previous) => ({
                              ...previous,
                              fallbackPolicy: value as SearchFallbackPolicy
                            }));
                            markDirty("search");
                          }}
                          disabled={saving}
                        >
                          <SelectTrigger>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            {SEARCH_FALLBACK_POLICY_OPTIONS.map((option) => (
                              <SelectItem key={option.value} value={option.value}>
                                {option.label}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                        <p className="text-xs text-slate-500">
                          {
                            SEARCH_FALLBACK_POLICY_OPTIONS.find(
                              (option) => option.value === searchDraft.fallbackPolicy
                            )?.description
                          }
                        </p>
                      </label>

                      <label className="space-y-1.5 sm:col-span-2">
                        <span className="text-xs font-semibold tracking-wide text-slate-600">活跃分词器</span>
                        <Select
                          value={searchDraft.analysis.activeAnalyzer}
                          onValueChange={(value) => {
                            const nextAnalyzer = value as SearchAnalyzer;
                            setSearchDraft((previous) => {
                              const nextConfig = cloneSearchConfig(previous);
                              nextConfig.analysis.activeAnalyzer = nextAnalyzer;
                              if (nextAnalyzer === "simple") {
                                nextConfig.analysis.analyzers.simple.enabled = true;
                              } else {
                                nextConfig.analysis.analyzers.jieba.enabled = true;
                              }
                              return nextConfig;
                            });
                            markDirty("search");
                          }}
                          disabled={saving}
                        >
                          <SelectTrigger>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            {SEARCH_ANALYZER_OPTIONS.map((option) => (
                              <SelectItem key={option.value} value={option.value}>
                                {option.label}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      </label>
                    </div>

                    <div className="grid gap-3 sm:grid-cols-2">
                      <label className="flex items-center gap-2.5 rounded-md border border-slate-200 bg-slate-50 px-3 py-3">
                        <Checkbox
                          checked={searchDraft.analysis.analyzers.simple.enabled}
                          onCheckedChange={(checked) => {
                            setSearchDraft((previous) => ({
                              ...previous,
                              analysis: {
                                ...previous.analysis,
                                analyzers: {
                                  ...previous.analysis.analyzers,
                                  simple: {
                                    enabled: checked === true
                                  }
                                }
                              }
                            }));
                            markDirty("search");
                          }}
                          disabled={saving || searchDraft.analysis.activeAnalyzer === "simple"}
                        />
                        <div className="space-y-0.5">
                          <span className="text-sm font-medium text-slate-700">启用 simple 分词器</span>
                          <p className="text-xs text-slate-500">
                            活跃分词器必须保持启用，切换活跃分词器后才能关闭。
                          </p>
                        </div>
                      </label>

                      <label className="flex items-center gap-2.5 rounded-md border border-slate-200 bg-slate-50 px-3 py-3">
                        <Checkbox
                          checked={searchDraft.analysis.analyzers.jieba.enabled}
                          onCheckedChange={(checked) => {
                            setSearchDraft((previous) => ({
                              ...previous,
                              analysis: {
                                ...previous.analysis,
                                analyzers: {
                                  ...previous.analysis.analyzers,
                                  jieba: {
                                    ...previous.analysis.analyzers.jieba,
                                    enabled: checked === true
                                  }
                                }
                              }
                            }));
                            markDirty("search");
                          }}
                          disabled={saving || searchDraft.analysis.activeAnalyzer === "jieba"}
                        />
                        <div className="space-y-0.5">
                          <span className="text-sm font-medium text-slate-700">启用 jieba 分词器</span>
                          <p className="text-xs text-slate-500">
                            当前分词治理仅支持 jieba 词典，建议开启以支持中文搜索优化。
                          </p>
                        </div>
                      </label>
                    </div>

                    <div className="space-y-3 rounded-md border border-slate-200 bg-slate-50 p-3">
                      <p className="text-xs font-semibold tracking-wide text-slate-700">jieba 参数</p>
                      <div className="grid gap-4 sm:grid-cols-2">
                        <label className="space-y-1.5">
                          <span className="text-xs font-semibold tracking-wide text-slate-600">模式</span>
                          <Input value={searchDraft.analysis.analyzers.jieba.mode} disabled />
                          <p className="text-xs text-slate-500">当前版本固定为 `search`。</p>
                        </label>

                        <label className="space-y-1.5">
                          <span className="text-xs font-semibold tracking-wide text-slate-600">词典来源</span>
                          <Input
                            value={
                              SEARCH_JIEBA_DICT_SOURCE_OPTIONS.find(
                                (option) => option.value === searchDraft.analysis.analyzers.jieba.dictSource
                              )?.label ?? searchDraft.analysis.analyzers.jieba.dictSource
                            }
                            readOnly
                            disabled
                          />
                          <p className="text-xs text-slate-500">该值由运行时配置决定，此处仅展示。</p>
                        </label>

                        <label className="space-y-1.5 sm:col-span-2">
                          <span className="text-xs font-semibold tracking-wide text-slate-600">词典版本</span>
                          <Input
                            value={searchDraft.analysis.analyzers.jieba.dictVersion}
                            readOnly
                            disabled
                          />
                          <p className="text-xs text-slate-500">
                            该值由“分词治理”中的词典变更自动维护，此处仅展示当前生效版本。
                          </p>
                        </label>

                        <label className="flex items-center gap-2 rounded-md border border-slate-200 bg-white px-3 py-2.5">
                          <Checkbox
                            checked={searchDraft.analysis.analyzers.jieba.hmm}
                            onCheckedChange={(checked) => {
                              setSearchDraft((previous) => ({
                                ...previous,
                                analysis: {
                                  ...previous.analysis,
                                  analyzers: {
                                    ...previous.analysis.analyzers,
                                    jieba: {
                                      ...previous.analysis.analyzers.jieba,
                                      hmm: checked === true
                                    }
                                  }
                                }
                              }));
                              markDirty("search");
                            }}
                            disabled={saving}
                          />
                          <div className="space-y-0.5">
                            <span className="text-xs font-medium text-slate-700">开启 HMM</span>
                            <p className="text-xs text-slate-500">
                              仅在活跃分词器为 jieba 时生效；会额外生成连续中文二元词，提升未登录词命中率。
                            </p>
                          </div>
                        </label>

                        <label className="flex items-center gap-2 rounded-md border border-slate-200 bg-white px-3 py-2.5">
                          <Checkbox
                            checked={searchDraft.analysis.analyzers.jieba.stopwordsEnabled}
                            onCheckedChange={(checked) => {
                              setSearchDraft((previous) => ({
                                ...previous,
                                analysis: {
                                  ...previous.analysis,
                                  analyzers: {
                                    ...previous.analysis.analyzers,
                                    jieba: {
                                      ...previous.analysis.analyzers.jieba,
                                      stopwordsEnabled: checked === true
                                    }
                                  }
                                }
                              }));
                              markDirty("search");
                            }}
                            disabled={saving}
                          />
                          <div className="space-y-0.5">
                            <span className="text-xs font-medium text-slate-700">启用停用词过滤</span>
                            <p className="text-xs text-slate-500">
                              当前版本为预留配置，运行时暂未接入停用词过滤逻辑，开启后不会改变检索结果。
                            </p>
                          </div>
                        </label>
                      </div>
                    </div>
                  </div>
                ) : null}

                {selectedKey === "data-retention" ? (
                  <div className="space-y-4 rounded-md border border-slate-200 bg-white p-4">
                    <label className="flex items-center gap-2.5 rounded-md border border-slate-200 bg-slate-50 px-3 py-3">
                      <Checkbox
                        checked={dataRetentionDraft.enabled}
                        onCheckedChange={(checked) => {
                          setDataRetentionDraft((previous) => ({
                            ...previous,
                            enabled: checked === true
                          }));
                          markDirty("data-retention");
                        }}
                        disabled={saving}
                      />
                      <div className="space-y-0.5">
                        <span className="text-sm font-medium text-slate-700">启用自动清理</span>
                        <p className="text-xs text-slate-500">关闭后不执行任何自动删除，历史数据将持续累积。</p>
                      </div>
                    </label>
                    <div className="space-y-2 rounded-md border border-slate-200 bg-slate-50 px-3 py-3">
                      <p className="text-xs font-semibold tracking-wide text-slate-600">自动清理表（可多选）</p>
                      <div className="grid gap-2 sm:grid-cols-2">
                        {DATA_RETENTION_CLEANUP_TABLE_OPTIONS.map((option) => {
                          const checked = dataRetentionDraft.cleanupTables.includes(option.value);
                          return (
                            <label
                              key={option.value}
                              className="flex items-start gap-2 rounded-md border border-slate-200 bg-white px-2.5 py-2"
                            >
                              <Checkbox
                                checked={checked}
                                onCheckedChange={(nextChecked) => {
                                  setDataRetentionDraft((previous) => {
                                    const nextSet = new Set(previous.cleanupTables);
                                    if (nextChecked === true) {
                                      nextSet.add(option.value);
                                    } else {
                                      nextSet.delete(option.value);
                                    }
                                    const nextCleanupTables = Array.from(nextSet) as DataRetentionCleanupTable[];
                                    return {
                                      ...previous,
                                      cleanupTables: nextCleanupTables
                                    };
                                  });
                                  markDirty("data-retention");
                                }}
                                disabled={saving || (checked && dataRetentionDraft.cleanupTables.length <= 1)}
                              />
                              <div className="space-y-0.5">
                                <p className="text-xs font-medium text-slate-700">{option.label}</p>
                                <p className="text-[11px] text-slate-500">{option.description}</p>
                              </div>
                            </label>
                          );
                        })}
                      </div>
                    </div>
                    <div className="grid gap-4 sm:grid-cols-2">
                      <label className="space-y-1.5">
                        <span className="text-xs font-semibold tracking-wide text-slate-600">执行间隔（分钟）</span>
                        <Input
                          type="number"
                          min={5}
                          max={1440}
                          value={String(dataRetentionDraft.scheduleMinutes)}
                          onChange={(event) => {
                            setDataRetentionDraft((previous) => ({
                              ...previous,
                              scheduleMinutes: normalizeIntegerInput(event.target.value, previous.scheduleMinutes)
                            }));
                            markDirty("data-retention");
                          }}
                          disabled={saving}
                        />
                      </label>
                      <label className="space-y-1.5">
                        <span className="text-xs font-semibold tracking-wide text-slate-600">单轮批次大小</span>
                        <Input
                          type="number"
                          min={100}
                          max={20000}
                          value={String(dataRetentionDraft.cleanupBatchSize)}
                          onChange={(event) => {
                            setDataRetentionDraft((previous) => ({
                              ...previous,
                              cleanupBatchSize: normalizeIntegerInput(event.target.value, previous.cleanupBatchSize)
                            }));
                            markDirty("data-retention");
                          }}
                          disabled={saving}
                        />
                      </label>
                      <label className="space-y-1.5">
                        <span className="text-xs font-semibold tracking-wide text-slate-600">审计日志保留天数</span>
                        <Input
                          type="number"
                          min={1}
                          max={3650}
                          value={String(dataRetentionDraft.auditLogRetentionDays)}
                          onChange={(event) => {
                            setDataRetentionDraft((previous) => ({
                              ...previous,
                              auditLogRetentionDays: normalizeIntegerInput(event.target.value, previous.auditLogRetentionDays)
                            }));
                            markDirty("data-retention");
                          }}
                          disabled={saving}
                        />
                      </label>
                      <label className="space-y-1.5">
                        <span className="text-xs font-semibold tracking-wide text-slate-600">验证码会话保留小时</span>
                        <Input
                          type="number"
                          min={1}
                          max={8760}
                          value={String(dataRetentionDraft.authCaptchaRetentionHours)}
                          onChange={(event) => {
                            setDataRetentionDraft((previous) => ({
                              ...previous,
                              authCaptchaRetentionHours: normalizeIntegerInput(
                                event.target.value,
                                previous.authCaptchaRetentionHours
                              )
                            }));
                            markDirty("data-retention");
                          }}
                          disabled={saving}
                        />
                      </label>
                      <label className="space-y-1.5">
                        <span className="text-xs font-semibold tracking-wide text-slate-600">风控状态保留天数</span>
                        <Input
                          type="number"
                          min={1}
                          max={3650}
                          value={String(dataRetentionDraft.authRiskStateRetentionDays)}
                          onChange={(event) => {
                            setDataRetentionDraft((previous) => ({
                              ...previous,
                              authRiskStateRetentionDays: normalizeIntegerInput(
                                event.target.value,
                                previous.authRiskStateRetentionDays
                              )
                            }));
                            markDirty("data-retention");
                          }}
                          disabled={saving}
                        />
                      </label>
                      <label className="space-y-1.5">
                        <span className="text-xs font-semibold tracking-wide text-slate-600">会话保留天数</span>
                        <Input
                          type="number"
                          min={1}
                          max={3650}
                          value={String(dataRetentionDraft.userSessionRetentionDays)}
                          onChange={(event) => {
                            setDataRetentionDraft((previous) => ({
                              ...previous,
                              userSessionRetentionDays: normalizeIntegerInput(
                                event.target.value,
                                previous.userSessionRetentionDays
                              )
                            }));
                            markDirty("data-retention");
                          }}
                          disabled={saving}
                        />
                      </label>
                    </div>
                    <p className="rounded-md border border-slate-200 bg-slate-50 px-3 py-2 text-xs text-slate-600">
                      当前自动清理范围：{dataRetentionDraft.cleanupTables.map((item) => `\`${item}\``).join("、")}。
                    </p>
                  </div>
                ) : null}

                {selectedKey === "auth" ? (
                  <div className="space-y-4 rounded-md border border-slate-200 bg-white p-4">
                    <div className="grid gap-4 sm:grid-cols-2">
                      <label className="space-y-1.5">
                        <span className="text-xs font-semibold tracking-wide text-slate-600">登录模式</span>
                        <Select
                          value={authDraft.loginMode}
                          onValueChange={(value) => {
                            const nextLoginMode = value as AuthLoginMode;
                            setAuthDraft((previous) => ({
                              ...previous,
                              loginMode: nextLoginMode,
                              allowUserRegister: nextLoginMode === "ldap_only" ? false : previous.allowUserRegister,
                              defaultProviderId:
                                nextLoginMode === "ldap_only" &&
                                  previous.defaultProviderId === AUTH_LOCAL_PROVIDER_ID
                                  ? resolvePreferredLDAPProviderID(previous.providers, previous.defaultProviderId)
                                  : previous.defaultProviderId
                            }));
                            markDirty("auth");
                          }}
                          disabled={saving}
                        >
                          <SelectTrigger>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            {AUTH_LOGIN_MODE_OPTIONS.map((option) => (
                              <SelectItem key={option.value} value={option.value}>
                                {option.label}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      </label>
                      <label className="space-y-1.5">
                        <span className="text-xs font-semibold tracking-wide text-slate-600">默认 Provider ID</span>
                        <Input
                          value={authDraft.defaultProviderId}
                          onChange={(event) => {
                            setAuthDraft((previous) => ({
                              ...previous,
                              defaultProviderId: parseString(event.target.value, previous.defaultProviderId)
                            }));
                            markDirty("auth");
                          }}
                          disabled={saving}
                        />
                      </label>
                      <label className="flex items-center gap-2.5 rounded-md border border-slate-200 bg-slate-50 px-3 py-3">
                        <Checkbox
                          checked={authDraft.loginMode === "ldap_only" ? false : authDraft.allowUserRegister}
                          onCheckedChange={(checked) => {
                            setAuthDraft((previous) => ({
                              ...previous,
                              allowUserRegister: previous.loginMode === "ldap_only" ? false : checked === true
                            }));
                            markDirty("auth");
                          }}
                          disabled={saving || authDraft.loginMode === "ldap_only"}
                        />
                        <div className="space-y-0.5">
                          <span className="text-sm font-medium text-slate-700">允许用户注册</span>
                          <p className="text-xs text-slate-500">
                            {authDraft.loginMode === "ldap_only"
                              ? "`ldap_only` 模式下此项会自动关闭。"
                              : "`ldap_only` 场景建议关闭。"}
                          </p>
                        </div>
                      </label>
                      <label className="flex items-center gap-2.5 rounded-md border border-slate-200 bg-slate-50 px-3 py-3">
                        <Checkbox
                          checked={authDraft.breakGlass.enabled}
                          onCheckedChange={(checked) => {
                            setAuthDraft((previous) => ({
                              ...previous,
                              breakGlass: {
                                ...previous.breakGlass,
                                enabled: checked === true
                              }
                            }));
                            markDirty("auth");
                          }}
                          disabled={saving}
                        />
                        <div className="space-y-0.5">
                          <span className="text-sm font-medium text-slate-700">启用 break-glass</span>
                          <p className="text-xs text-slate-500">保留本地管理员应急登录能力。</p>
                        </div>
                      </label>
                      <label className="space-y-1.5 sm:col-span-2">
                        <span className="text-xs font-semibold tracking-wide text-slate-600">
                          break-glass 本地管理员邮箱（逗号分隔）
                        </span>
                        <Input
                          value={authDraft.breakGlass.localAdminEmails.join(",")}
                          onChange={(event) => {
                            const emails = event.target.value
                              .split(",")
                              .map((item) => item.trim())
                              .filter((item) => item.length > 0);
                            setAuthDraft((previous) => ({
                              ...previous,
                              breakGlass: {
                                ...previous.breakGlass,
                                localAdminEmails: emails
                              }
                            }));
                            markDirty("auth");
                          }}
                          disabled={saving}
                        />
                      </label>
                    </div>

                    <div className="rounded-md border border-slate-200 bg-slate-50 p-3">
                      <div className="grid gap-4 sm:grid-cols-2">
                        <label className="space-y-1.5">
                          <span className="text-xs font-semibold tracking-wide text-slate-600">LDAP Provider</span>
                          <Select
                            value={selectedAuthProvider?.id ?? ""}
                            onValueChange={(value) => setSelectedAuthProviderID(value)}
                            disabled={saving || authDraft.providers.length === 0}
                          >
                            <SelectTrigger>
                              <SelectValue placeholder="选择 Provider" />
                            </SelectTrigger>
                            <SelectContent>
                              {authDraft.providers.map((provider) => (
                                <SelectItem key={provider.id} value={provider.id}>
                                  {provider.name}（{provider.id}）
                                </SelectItem>
                              ))}
                            </SelectContent>
                          </Select>
                        </label>
                        <label className="flex items-center gap-2.5 rounded-md border border-slate-200 bg-white px-3 py-3">
                          <Checkbox
                            checked={selectedAuthProvider?.enabled ?? false}
                            onCheckedChange={(checked) => {
                              if (!selectedAuthProvider) {
                                return;
                              }
                              updateSelectedAuthProvider((provider) => ({
                                ...provider,
                                enabled: checked === true
                              }));
                            }}
                            disabled={saving || !selectedAuthProvider}
                          />
                          <span className="text-sm font-medium text-slate-700">启用当前 Provider</span>
                        </label>
                      </div>

                      {selectedAuthProvider ? (
                        <div className="mt-4 grid gap-4 sm:grid-cols-2">
                          <label className="space-y-1.5">
                            <span className="text-xs font-semibold tracking-wide text-slate-600">Provider ID</span>
                            <Input
                              value={selectedAuthProvider.id}
                              onChange={(event) => {
                                const nextProviderID = parseString(event.target.value, selectedAuthProvider.id);
                                setAuthDraft((previousConfig) => {
                                  const nextProviders = previousConfig.providers.map((provider) => {
                                    if (provider.id !== selectedAuthProviderID) {
                                      return provider;
                                    }
                                    return {
                                      ...provider,
                                      id: nextProviderID
                                    };
                                  });
                                  return {
                                    ...previousConfig,
                                    defaultProviderId:
                                      previousConfig.defaultProviderId === selectedAuthProviderID
                                        ? nextProviderID
                                        : previousConfig.defaultProviderId,
                                    providers: nextProviders
                                  };
                                });
                                setSelectedAuthProviderID(nextProviderID);
                                markDirty("auth");
                              }}
                              disabled={saving}
                            />
                          </label>
                          <label className="space-y-1.5">
                            <span className="text-xs font-semibold tracking-wide text-slate-600">Provider 名称</span>
                            <Input
                              value={selectedAuthProvider.name}
                              onChange={(event) => {
                                updateSelectedAuthProvider((provider) => ({
                                  ...provider,
                                  name: parseString(event.target.value, provider.name)
                                }));
                              }}
                              disabled={saving}
                            />
                          </label>
                          <label className="space-y-1.5">
                            <span className="text-xs font-semibold tracking-wide text-slate-600">优先级</span>
                            <Input
                              type="number"
                              min={0}
                              max={10000}
                              value={String(selectedAuthProvider.priority)}
                              onChange={(event) => {
                                updateSelectedAuthProvider((provider) => ({
                                  ...provider,
                                  priority: normalizeIntegerInput(event.target.value, provider.priority)
                                }));
                              }}
                              disabled={saving}
                            />
                          </label>
                          <label className="space-y-1.5">
                            <span className="text-xs font-semibold tracking-wide text-slate-600">匹配邮箱域名（逗号分隔）</span>
                            <Input
                              value={selectedAuthProvider.matchRules.emailDomains.join(",")}
                              onChange={(event) => {
                                const emailDomains = event.target.value
                                  .split(",")
                                  .map((item) => item.trim())
                                  .filter((item) => item.length > 0);
                                updateSelectedAuthProvider((provider) => ({
                                  ...provider,
                                  matchRules: {
                                    ...provider.matchRules,
                                    emailDomains
                                  }
                                }));
                              }}
                              disabled={saving}
                            />
                          </label>
                          <label className="space-y-1.5 sm:col-span-2">
                            <span className="text-xs font-semibold tracking-wide text-slate-600">用户名匹配正则</span>
                            <Input
                              value={selectedAuthProvider.matchRules.usernameRegex}
                              onChange={(event) => {
                                updateSelectedAuthProvider((provider) => ({
                                  ...provider,
                                  matchRules: {
                                    ...provider.matchRules,
                                    usernameRegex: parseString(event.target.value, provider.matchRules.usernameRegex)
                                  }
                                }));
                              }}
                              disabled={saving}
                            />
                          </label>
                          <label className="space-y-1.5">
                            <span className="text-xs font-semibold tracking-wide text-slate-600">Host</span>
                            <Input
                              value={selectedAuthProvider.ldap.host}
                              onChange={(event) => {
                                updateSelectedAuthProvider((provider) => ({
                                  ...provider,
                                  ldap: {
                                    ...provider.ldap,
                                    host: parseString(event.target.value, provider.ldap.host)
                                  }
                                }));
                              }}
                              disabled={saving}
                            />
                          </label>
                          <label className="space-y-1.5">
                            <span className="text-xs font-semibold tracking-wide text-slate-600">Port</span>
                            <Input
                              type="number"
                              min={1}
                              max={65535}
                              value={String(selectedAuthProvider.ldap.port)}
                              onChange={(event) => {
                                updateSelectedAuthProvider((provider) => ({
                                  ...provider,
                                  ldap: {
                                    ...provider.ldap,
                                    port: normalizeIntegerInput(event.target.value, provider.ldap.port)
                                  }
                                }));
                              }}
                              disabled={saving}
                            />
                          </label>
                          <label className="space-y-1.5">
                            <span className="text-xs font-semibold tracking-wide text-slate-600">TLS 模式</span>
                            <Select
                              value={selectedAuthProvider.ldap.tlsMode}
                              onValueChange={(value) => {
                                updateSelectedAuthProvider((provider) => ({
                                  ...provider,
                                  ldap: {
                                    ...provider.ldap,
                                    tlsMode: parseAuthLDAPTLSMode(value, provider.ldap.tlsMode)
                                  }
                                }));
                              }}
                              disabled={saving}
                            >
                              <SelectTrigger>
                                <SelectValue />
                              </SelectTrigger>
                              <SelectContent>
                                <SelectItem value="ldaps">LDAPS</SelectItem>
                                <SelectItem value="starttls">StartTLS</SelectItem>
                                <SelectItem value="plain">Plain（不加密）</SelectItem>
                              </SelectContent>
                            </Select>
                          </label>
                          <label className="space-y-1.5">
                            <span className="text-xs font-semibold tracking-wide text-slate-600">Base DN</span>
                            <Input
                              value={selectedAuthProvider.ldap.baseDN}
                              onChange={(event) => {
                                updateSelectedAuthProvider((provider) => ({
                                  ...provider,
                                  ldap: {
                                    ...provider.ldap,
                                    baseDN: parseString(event.target.value, provider.ldap.baseDN)
                                  }
                                }));
                              }}
                              disabled={saving}
                            />
                          </label>
                          <label className="space-y-1.5">
                            <span className="text-xs font-semibold tracking-wide text-slate-600">Bind DN</span>
                            <Input
                              value={selectedAuthProvider.ldap.bindDN}
                              onChange={(event) => {
                                updateSelectedAuthProvider((provider) => ({
                                  ...provider,
                                  ldap: {
                                    ...provider.ldap,
                                    bindDN: event.target.value.trim()
                                  }
                                }));
                              }}
                              disabled={saving}
                            />
                          </label>
                          <label className="space-y-1.5">
                            <span className="text-xs font-semibold tracking-wide text-slate-600">Bind Password</span>
                            <Input
                              type="password"
                              value={selectedAuthProvider.ldap.bindPasswordCiphertext}
                              placeholder={AUTH_SECRET_MASK}
                              onChange={(event) => {
                                updateSelectedAuthProvider((provider) => ({
                                  ...provider,
                                  ldap: {
                                    ...provider.ldap,
                                    bindPasswordCiphertext: event.target.value.trim()
                                  }
                                }));
                              }}
                              disabled={saving}
                            />
                          </label>
                          <label className="space-y-1.5 sm:col-span-2">
                            <span className="text-xs font-semibold tracking-wide text-slate-600">User Filter</span>
                            <Input
                              value={selectedAuthProvider.ldap.userFilter}
                              onChange={(event) => {
                                updateSelectedAuthProvider((provider) => ({
                                  ...provider,
                                  ldap: {
                                    ...provider.ldap,
                                    userFilter: parseString(event.target.value, provider.ldap.userFilter)
                                  }
                                }));
                              }}
                              disabled={saving}
                            />
                          </label>
                          <label className="space-y-1.5">
                            <span className="text-xs font-semibold tracking-wide text-slate-600">ID 属性</span>
                            <Input
                              value={selectedAuthProvider.ldap.idAttribute}
                              onChange={(event) => {
                                updateSelectedAuthProvider((provider) => ({
                                  ...provider,
                                  ldap: {
                                    ...provider.ldap,
                                    idAttribute: parseString(event.target.value, provider.ldap.idAttribute)
                                  }
                                }));
                              }}
                              disabled={saving}
                            />
                          </label>
                          <label className="space-y-1.5">
                            <span className="text-xs font-semibold tracking-wide text-slate-600">Email 属性</span>
                            <Input
                              value={selectedAuthProvider.ldap.emailAttribute}
                              onChange={(event) => {
                                updateSelectedAuthProvider((provider) => ({
                                  ...provider,
                                  ldap: {
                                    ...provider.ldap,
                                    emailAttribute: parseString(event.target.value, provider.ldap.emailAttribute)
                                  }
                                }));
                              }}
                              disabled={saving}
                            />
                          </label>
                          <label className="space-y-1.5">
                            <span className="text-xs font-semibold tracking-wide text-slate-600">Name 属性</span>
                            <Input
                              value={selectedAuthProvider.ldap.nameAttribute}
                              onChange={(event) => {
                                updateSelectedAuthProvider((provider) => ({
                                  ...provider,
                                  ldap: {
                                    ...provider.ldap,
                                    nameAttribute: parseString(event.target.value, provider.ldap.nameAttribute)
                                  }
                                }));
                              }}
                              disabled={saving}
                            />
                          </label>
                          <label className="space-y-1.5">
                            <span className="text-xs font-semibold tracking-wide text-slate-600">Group 属性</span>
                            <Input
                              value={selectedAuthProvider.ldap.groupAttribute}
                              onChange={(event) => {
                                updateSelectedAuthProvider((provider) => ({
                                  ...provider,
                                  ldap: {
                                    ...provider.ldap,
                                    groupAttribute: event.target.value.trim()
                                  }
                                }));
                              }}
                              disabled={saving}
                            />
                          </label>
                          <label className="space-y-1.5">
                            <span className="text-xs font-semibold tracking-wide text-slate-600">连接超时（ms）</span>
                            <Input
                              type="number"
                              min={100}
                              max={30000}
                              value={String(selectedAuthProvider.ldap.connectTimeoutMs)}
                              onChange={(event) => {
                                updateSelectedAuthProvider((provider) => ({
                                  ...provider,
                                  ldap: {
                                    ...provider.ldap,
                                    connectTimeoutMs: normalizeIntegerInput(event.target.value, provider.ldap.connectTimeoutMs)
                                  }
                                }));
                              }}
                              disabled={saving}
                            />
                          </label>
                          <label className="space-y-1.5">
                            <span className="text-xs font-semibold tracking-wide text-slate-600">读取超时（ms）</span>
                            <Input
                              type="number"
                              min={100}
                              max={30000}
                              value={String(selectedAuthProvider.ldap.readTimeoutMs)}
                              onChange={(event) => {
                                updateSelectedAuthProvider((provider) => ({
                                  ...provider,
                                  ldap: {
                                    ...provider.ldap,
                                    readTimeoutMs: normalizeIntegerInput(event.target.value, provider.ldap.readTimeoutMs)
                                  }
                                }));
                              }}
                              disabled={saving}
                            />
                          </label>
                        </div>
                      ) : (
                        <p className="mt-3 text-sm text-slate-500">当前没有可编辑的 LDAP Provider，请先在配置中补充 providers。</p>
                      )}
                    </div>
                  </div>
                ) : null}

                {selectedKey === "sitemap" ? (
                  <div className="rounded-md border border-slate-200 bg-white p-4">
                    <div className="grid gap-4 sm:grid-cols-2">
                      <label className="space-y-1.5 sm:col-span-2">
                        <span className="text-xs font-semibold tracking-wide text-slate-600">生成规则</span>
                        <Select
                          value={sitemapDraft.generationMode}
                          onValueChange={(value) => {
                            setSitemapDraft((previous) => ({
                              ...previous,
                              generationMode: value as SitemapGenerationMode
                            }));
                            markDirty("sitemap");
                          }}
                          disabled={saving}
                        >
                          <SelectTrigger>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            {SITEMAP_GENERATION_MODE_OPTIONS.map((option) => (
                              <SelectItem key={option.value} value={option.value}>
                                {option.label}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                        <p className="text-xs text-slate-500">
                          {
                            SITEMAP_GENERATION_MODE_OPTIONS.find((option) => option.value === sitemapDraft.generationMode)
                              ?.description
                          }
                        </p>
                      </label>
                      <label className="space-y-1.5">
                        <span className="text-xs font-semibold tracking-wide text-slate-600">最多纳入最近更新天数</span>
                        <Input
                          type="number"
                          min={1}
                          max={3650}
                          value={String(sitemapDraft.maxUpdatedWithinDays)}
                          onChange={(event) => {
                            setSitemapDraft((previous) => ({
                              ...previous,
                              maxUpdatedWithinDays: normalizeIntegerInput(event.target.value, previous.maxUpdatedWithinDays)
                            }));
                            markDirty("sitemap");
                          }}
                          disabled={saving}
                        />
                        <p className="text-xs text-slate-500">仅在“最近更新文档”规则下生效，范围 1-3650。</p>
                      </label>
                    </div>
                  </div>
                ) : null}

                {selectedKey === "image-hosting" ? (
                  <div className="rounded-md border border-slate-200 bg-white p-4">
                    <div className="grid gap-4 sm:grid-cols-2">
                      <label className="space-y-1.5">
                        <span className="text-xs font-semibold tracking-wide text-slate-600">默认图床</span>
                        <Select
                          value={imageHostingDraft.defaultProvider}
                          onValueChange={(value) => {
                            setImageHostingDraft((previousConfig) => ({
                              ...previousConfig,
                              defaultProvider: value as ImageHostingProvider
                            }));
                            markDirty("image-hosting");
                          }}
                          disabled={saving}
                        >
                          <SelectTrigger>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            {IMAGE_HOSTING_PROVIDER_OPTIONS.map((option) => (
                              <SelectItem key={option.value} value={option.value}>
                                {option.label}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      </label>
                      <div className="rounded-md border border-slate-200 bg-slate-50 px-3 py-3 text-xs text-slate-600">
                        当前默认图床：{
                          IMAGE_HOSTING_PROVIDER_OPTIONS.find((option) => option.value === imageHostingDraft.defaultProvider)
                            ?.label
                        }
                      </div>
                    </div>

                    <div className="mt-4 space-y-3 rounded-md border border-slate-200 bg-slate-50/60 p-3">
                      <p className="text-xs font-semibold tracking-wide text-slate-700">图片压缩</p>
                      <div className="grid gap-3 sm:grid-cols-2">
                        <label className="space-y-1.5">
                          <span className="text-xs font-semibold tracking-wide text-slate-600">压缩模式</span>
                          <Select
                            value={imageHostingDraft.imageProcessing.mode}
                            onValueChange={(value) => {
                              setImageProcessingMode(value as ImageHostingImageProcessingMode);
                            }}
                            disabled={saving}
                          >
                            <SelectTrigger>
                              <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                              {IMAGE_HOSTING_IMAGE_PROCESSING_MODE_OPTIONS.map((option) => (
                                <SelectItem key={option.value} value={option.value}>
                                  {option.label}
                                </SelectItem>
                              ))}
                            </SelectContent>
                          </Select>
                          <p className="text-xs text-slate-500">
                            {
                              IMAGE_HOSTING_IMAGE_PROCESSING_MODE_OPTIONS.find(
                                (option) => option.value === imageHostingDraft.imageProcessing.mode
                              )?.description
                            }
                          </p>
                        </label>
                        <label className="space-y-1.5">
                          <span className="text-xs font-semibold tracking-wide text-slate-600">质量档位</span>
                          <Select
                            value={imageHostingDraft.imageProcessing.qualityPreset}
                            onValueChange={(value) => {
                              setImageQualityPreset(value as ImageHostingImageQualityPreset);
                            }}
                            disabled={saving}
                          >
                            <SelectTrigger>
                              <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                              {IMAGE_HOSTING_IMAGE_QUALITY_PRESET_OPTIONS.map((option) => (
                                <SelectItem key={option.value} value={option.value}>
                                  {option.label}
                                </SelectItem>
                              ))}
                            </SelectContent>
                          </Select>
                          <p className="text-xs text-slate-500">
                            {
                              IMAGE_HOSTING_IMAGE_QUALITY_PRESET_OPTIONS.find(
                                (option) => option.value === imageHostingDraft.imageProcessing.qualityPreset
                              )?.description
                            }
                          </p>
                        </label>
                        <label className="space-y-1.5">
                          <span className="text-xs font-semibold tracking-wide text-slate-600">最大宽度（px）</span>
                          <Input
                            type="number"
                            min={IMAGE_HOSTING_IMAGE_MAX_WIDTH_MIN}
                            max={IMAGE_HOSTING_IMAGE_MAX_WIDTH_MAX}
                            value={String(imageHostingDraft.imageProcessing.maxWidth)}
                            onChange={(event) => setImageProcessingMaxWidth(event.target.value)}
                            disabled={saving}
                          />
                        </label>
                        <label className="space-y-1.5">
                          <span className="text-xs font-semibold tracking-wide text-slate-600">最大高度（px）</span>
                          <Input
                            type="number"
                            min={IMAGE_HOSTING_IMAGE_MAX_HEIGHT_MIN}
                            max={IMAGE_HOSTING_IMAGE_MAX_HEIGHT_MAX}
                            value={String(imageHostingDraft.imageProcessing.maxHeight)}
                            onChange={(event) => setImageProcessingMaxHeight(event.target.value)}
                            disabled={saving}
                          />
                          <p className="text-xs text-slate-500">
                            分别限制单张图片的宽度和高度，范围 {IMAGE_HOSTING_IMAGE_MAX_WIDTH_MIN.toLocaleString("en-US")} -{" "}
                            {IMAGE_HOSTING_IMAGE_MAX_WIDTH_MAX.toLocaleString("en-US")} 像素。
                          </p>
                        </label>
                        <label className="flex items-center gap-2 rounded-md border border-slate-200 bg-white px-3 py-2.5">
                          <Checkbox
                            checked={imageHostingDraft.imageProcessing.skipAnimated}
                            onCheckedChange={(checked) => {
                              setImageProcessingSkipAnimated(checked === true);
                            }}
                            disabled={saving}
                          />
                          <div className="space-y-0.5">
                            <span className="text-xs font-semibold tracking-wide text-slate-700">跳过动图压缩</span>
                            <p className="text-[11px] text-slate-500">启用后，GIF/WebP 动图将直接透传，避免动画丢失。</p>
                          </div>
                        </label>
                      </div>
                      <p className="text-[11px] text-slate-500">
                        作用说明：该宽高限制用于拦截超大图片，降低服务器在解码和压缩阶段的内存/CPU 峰值。若上传图片宽或高超过阈值，系统会直接拒绝上传并提示错误，不会自动缩放图片。
                      </p>
                    </div>

                    <Tabs
                      className="mt-4"
                      value={imageHostingProviderTab}
                      onValueChange={(value) => setImageHostingProviderTab(value as ImageHostingProvider)}
                    >
                      <p className="mb-2 text-xs font-semibold tracking-wide text-slate-600">图床配置项</p>
                      <TabsList className="h-auto w-full justify-start gap-2 overflow-x-auto border-0 bg-transparent p-0">
                        {IMAGE_HOSTING_PROVIDER_OPTIONS.map((option) => (
                          <TabsTrigger
                            key={option.value}
                            value={option.value}
                            disabled={saving}
                            className="rounded-md border border-slate-200 bg-white px-4 py-2 text-base font-medium text-slate-700 shadow-sm hover:bg-slate-50 data-[state=active]:border-sky-300 data-[state=active]:bg-sky-50 data-[state=active]:text-sky-700"
                          >
                            {option.label}
                          </TabsTrigger>
                        ))}
                      </TabsList>

                      <TabsContent value="local" className="grid gap-4 sm:grid-cols-2">
                        <label className="space-y-1.5">
                          <span className="text-xs font-semibold tracking-wide text-slate-600">上传接口地址</span>
                          <Input
                            placeholder="/api/uploads/images"
                            value={imageHostingDraft.local.uploadEndpoint}
                            onChange={(event) => setLocalField("uploadEndpoint", event.target.value)}
                            disabled={saving}
                          />
                        </label>
                        <label className="space-y-1.5">
                          <span className="text-xs font-semibold tracking-wide text-slate-600">公网访问前缀</span>
                          <Input
                            placeholder="/uploads"
                            value={imageHostingDraft.local.publicBaseUrl}
                            onChange={(event) => setLocalField("publicBaseUrl", event.target.value)}
                            disabled={saving}
                          />
                        </label>
                        <div className="space-y-2 rounded-md border border-slate-200 bg-slate-50/50 p-3 sm:col-span-2">
                          <div className="flex items-center justify-between gap-2">
                            <span className="text-xs font-semibold tracking-wide text-slate-700">上传路径模板</span>
                            <span className="text-[11px] text-slate-500">内置场景</span>
                          </div>
                          <div className="flex flex-wrap gap-2">
                            {IMAGE_HOSTING_UPLOAD_TEMPLATE_PRESETS.map((preset) => (
                              <button
                                key={`local-${preset.key}`}
                                type="button"
                                className={`rounded-md border px-2 py-1 text-xs transition ${
                                  imageHostingDraft.local.uploadPathTemplate.trim() === preset.template
                                    ? "border-sky-300 bg-sky-50 text-sky-700"
                                    : "border-slate-200 bg-white text-slate-600 hover:bg-slate-100"
                                }`}
                                disabled={saving}
                                onClick={() => setLocalField("uploadPathTemplate", preset.template)}
                              >
                                {preset.label}
                              </button>
                            ))}
                          </div>
                          <Input
                            ref={bindImageHostingTemplateInputRef("local")}
                            placeholder={IMAGE_HOSTING_UPLOAD_TEMPLATE_PLACEHOLDER}
                            value={imageHostingDraft.local.uploadPathTemplate}
                            onChange={(event) => setLocalField("uploadPathTemplate", event.target.value)}
                            disabled={saving}
                          />
                          <div className="flex flex-wrap gap-1.5">
                            <TooltipProvider delayDuration={120}>
                              {IMAGE_HOSTING_UPLOAD_TEMPLATE_VARIABLES.map((variable) => (
                                <Tooltip key={`local-${variable.token}`}>
                                  <TooltipTrigger asChild>
                                    <button
                                      type="button"
                                      className="rounded-md border border-slate-200 bg-white px-2 py-1 text-[11px] text-slate-600 transition hover:border-sky-300 hover:text-sky-700"
                                      disabled={saving}
                                      onClick={() =>
                                        insertImageHostingTemplateVariable(
                                          "local",
                                          imageHostingDraft.local.uploadPathTemplate,
                                          variable.token,
                                          (nextValue) => setLocalField("uploadPathTemplate", nextValue)
                                        )
                                      }
                                    >
                                      {variable.label}
                                    </button>
                                  </TooltipTrigger>
                                  <TooltipContent side="top" className="max-w-[280px] whitespace-pre-wrap break-words">
                                    {variable.description}
                                  </TooltipContent>
                                </Tooltip>
                              ))}
                            </TooltipProvider>
                          </div>
                          <p className="text-xs text-slate-500">{IMAGE_HOSTING_UPLOAD_TEMPLATE_HINT}</p>
                          <p className="text-xs text-slate-500">
                            预览：{" "}
                            <code className="rounded bg-slate-100 px-1 py-0.5 text-[11px] text-slate-700">
                              {buildImageHostingTemplatePreview(imageHostingDraft.local.uploadPathTemplate)}
                            </code>
                          </p>
                        </div>
                      </TabsContent>

                      <TabsContent value="cloudflare-r2" className="grid gap-4 sm:grid-cols-2">
                        <label className="space-y-1.5">
                          <span className="text-xs font-semibold tracking-wide text-slate-600">Account ID</span>
                          <Input
                            placeholder="4d2a1c..."
                            value={imageHostingDraft.cloudflareR2.accountId}
                            onChange={(event) => setCloudflareField("accountId", event.target.value)}
                            disabled={saving}
                          />
                        </label>
                        <label className="space-y-1.5">
                          <span className="text-xs font-semibold tracking-wide text-slate-600">Bucket</span>
                          <Input
                            placeholder="plaindoc-assets"
                            value={imageHostingDraft.cloudflareR2.bucket}
                            onChange={(event) => setCloudflareField("bucket", event.target.value)}
                            disabled={saving}
                          />
                        </label>
                        <label className="space-y-1.5">
                          <span className="text-xs font-semibold tracking-wide text-slate-600">Access Key ID</span>
                          <Input
                            placeholder="R2XXXX..."
                            value={imageHostingDraft.cloudflareR2.accessKeyId}
                            onChange={(event) => setCloudflareField("accessKeyId", event.target.value)}
                            disabled={saving}
                          />
                        </label>
                        <label className="space-y-1.5">
                          <span className="text-xs font-semibold tracking-wide text-slate-600">Secret Access Key</span>
                          <Input
                            type="password"
                            placeholder="输入 Secret Access Key"
                            value={imageHostingDraft.cloudflareR2.secretAccessKey}
                            onChange={(event) => setCloudflareField("secretAccessKey", event.target.value)}
                            disabled={saving}
                          />
                        </label>
                        <label className="space-y-1.5 sm:col-span-2">
                          <span className="text-xs font-semibold tracking-wide text-slate-600">公网访问域名</span>
                          <Input
                            placeholder="https://img.example.com"
                            value={imageHostingDraft.cloudflareR2.publicBaseUrl}
                            onChange={(event) => setCloudflareField("publicBaseUrl", event.target.value)}
                            disabled={saving}
                          />
                        </label>
                        <div className="space-y-2 rounded-md border border-slate-200 bg-slate-50/50 p-3 sm:col-span-2">
                          <div className="flex items-center justify-between gap-2">
                            <span className="text-xs font-semibold tracking-wide text-slate-700">上传路径模板</span>
                            <span className="text-[11px] text-slate-500">内置场景</span>
                          </div>
                          <div className="flex flex-wrap gap-2">
                            {IMAGE_HOSTING_UPLOAD_TEMPLATE_PRESETS.map((preset) => (
                              <button
                                key={`cloudflare-r2-${preset.key}`}
                                type="button"
                                className={`rounded-md border px-2 py-1 text-xs transition ${
                                  imageHostingDraft.cloudflareR2.uploadPathTemplate.trim() === preset.template
                                    ? "border-sky-300 bg-sky-50 text-sky-700"
                                    : "border-slate-200 bg-white text-slate-600 hover:bg-slate-100"
                                }`}
                                disabled={saving}
                                onClick={() => setCloudflareField("uploadPathTemplate", preset.template)}
                              >
                                {preset.label}
                              </button>
                            ))}
                          </div>
                          <Input
                            ref={bindImageHostingTemplateInputRef("cloudflare-r2")}
                            placeholder={IMAGE_HOSTING_UPLOAD_TEMPLATE_PLACEHOLDER}
                            value={imageHostingDraft.cloudflareR2.uploadPathTemplate}
                            onChange={(event) => setCloudflareField("uploadPathTemplate", event.target.value)}
                            disabled={saving}
                          />
                          <div className="flex flex-wrap gap-1.5">
                            <TooltipProvider delayDuration={120}>
                              {IMAGE_HOSTING_UPLOAD_TEMPLATE_VARIABLES.map((variable) => (
                                <Tooltip key={`cloudflare-r2-${variable.token}`}>
                                  <TooltipTrigger asChild>
                                    <button
                                      type="button"
                                      className="rounded-md border border-slate-200 bg-white px-2 py-1 text-[11px] text-slate-600 transition hover:border-sky-300 hover:text-sky-700"
                                      disabled={saving}
                                      onClick={() =>
                                        insertImageHostingTemplateVariable(
                                          "cloudflare-r2",
                                          imageHostingDraft.cloudflareR2.uploadPathTemplate,
                                          variable.token,
                                          (nextValue) => setCloudflareField("uploadPathTemplate", nextValue)
                                        )
                                      }
                                    >
                                      {variable.label}
                                    </button>
                                  </TooltipTrigger>
                                  <TooltipContent side="top" className="max-w-[280px] whitespace-pre-wrap break-words">
                                    {variable.description}
                                  </TooltipContent>
                                </Tooltip>
                              ))}
                            </TooltipProvider>
                          </div>
                          <p className="text-xs text-slate-500">{IMAGE_HOSTING_UPLOAD_TEMPLATE_HINT}</p>
                          <p className="text-xs text-slate-500">
                            预览：{" "}
                            <code className="rounded bg-slate-100 px-1 py-0.5 text-[11px] text-slate-700">
                              {buildImageHostingTemplatePreview(imageHostingDraft.cloudflareR2.uploadPathTemplate)}
                            </code>
                          </p>
                        </div>
                      </TabsContent>

                      <TabsContent value="aliyun-oss" className="grid gap-4 sm:grid-cols-2">
                        <label className="space-y-1.5">
                          <span className="text-xs font-semibold tracking-wide text-slate-600">Region（可选）</span>
                          <Input
                            placeholder="oss-cn-hangzhou"
                            value={imageHostingDraft.aliyunOss.region}
                            onChange={(event) => setAliyunField("region", event.target.value)}
                            disabled={saving}
                          />
                        </label>
                        <label className="space-y-1.5">
                          <span className="text-xs font-semibold tracking-wide text-slate-600">Bucket</span>
                          <Input
                            placeholder="plaindoc-assets"
                            value={imageHostingDraft.aliyunOss.bucket}
                            onChange={(event) => setAliyunField("bucket", event.target.value)}
                            disabled={saving}
                          />
                        </label>
                        <label className="space-y-1.5">
                          <span className="text-xs font-semibold tracking-wide text-slate-600">Endpoint（可选）</span>
                          <Input
                            placeholder="https://oss-cn-hangzhou.aliyuncs.com"
                            value={imageHostingDraft.aliyunOss.endpoint}
                            onChange={(event) => setAliyunField("endpoint", event.target.value)}
                            disabled={saving}
                          />
                        </label>
                        <label className="space-y-1.5">
                          <span className="text-xs font-semibold tracking-wide text-slate-600">Access Key ID</span>
                          <Input
                            placeholder="LTAI..."
                            value={imageHostingDraft.aliyunOss.accessKeyId}
                            onChange={(event) => setAliyunField("accessKeyId", event.target.value)}
                            disabled={saving}
                          />
                        </label>
                        <label className="space-y-1.5">
                          <span className="text-xs font-semibold tracking-wide text-slate-600">Access Key Secret</span>
                          <Input
                            type="password"
                            placeholder="输入 Access Key Secret"
                            value={imageHostingDraft.aliyunOss.accessKeySecret}
                            onChange={(event) => setAliyunField("accessKeySecret", event.target.value)}
                            disabled={saving}
                          />
                        </label>
                        <label className="space-y-1.5 sm:col-span-2">
                          <span className="text-xs font-semibold tracking-wide text-slate-600">公网访问域名</span>
                          <Input
                            placeholder="https://img.example.com"
                            value={imageHostingDraft.aliyunOss.publicBaseUrl}
                            onChange={(event) => setAliyunField("publicBaseUrl", event.target.value)}
                            disabled={saving}
                          />
                        </label>
                        <div className="space-y-2 rounded-md border border-slate-200 bg-slate-50/50 p-3 sm:col-span-2">
                          <div className="flex items-center justify-between gap-2">
                            <span className="text-xs font-semibold tracking-wide text-slate-700">上传路径模板</span>
                            <span className="text-[11px] text-slate-500">内置场景</span>
                          </div>
                          <div className="flex flex-wrap gap-2">
                            {IMAGE_HOSTING_UPLOAD_TEMPLATE_PRESETS.map((preset) => (
                              <button
                                key={`aliyun-oss-${preset.key}`}
                                type="button"
                                className={`rounded-md border px-2 py-1 text-xs transition ${
                                  imageHostingDraft.aliyunOss.uploadPathTemplate.trim() === preset.template
                                    ? "border-sky-300 bg-sky-50 text-sky-700"
                                    : "border-slate-200 bg-white text-slate-600 hover:bg-slate-100"
                                }`}
                                disabled={saving}
                                onClick={() => setAliyunField("uploadPathTemplate", preset.template)}
                              >
                                {preset.label}
                              </button>
                            ))}
                          </div>
                          <Input
                            ref={bindImageHostingTemplateInputRef("aliyun-oss")}
                            placeholder={IMAGE_HOSTING_UPLOAD_TEMPLATE_PLACEHOLDER}
                            value={imageHostingDraft.aliyunOss.uploadPathTemplate}
                            onChange={(event) => setAliyunField("uploadPathTemplate", event.target.value)}
                            disabled={saving}
                          />
                          <div className="flex flex-wrap gap-1.5">
                            <TooltipProvider delayDuration={120}>
                              {IMAGE_HOSTING_UPLOAD_TEMPLATE_VARIABLES.map((variable) => (
                                <Tooltip key={`aliyun-oss-${variable.token}`}>
                                  <TooltipTrigger asChild>
                                    <button
                                      type="button"
                                      className="rounded-md border border-slate-200 bg-white px-2 py-1 text-[11px] text-slate-600 transition hover:border-sky-300 hover:text-sky-700"
                                      disabled={saving}
                                      onClick={() =>
                                        insertImageHostingTemplateVariable(
                                          "aliyun-oss",
                                          imageHostingDraft.aliyunOss.uploadPathTemplate,
                                          variable.token,
                                          (nextValue) => setAliyunField("uploadPathTemplate", nextValue)
                                        )
                                      }
                                    >
                                      {variable.label}
                                    </button>
                                  </TooltipTrigger>
                                  <TooltipContent side="top" className="max-w-[280px] whitespace-pre-wrap break-words">
                                    {variable.description}
                                  </TooltipContent>
                                </Tooltip>
                              ))}
                            </TooltipProvider>
                          </div>
                          <p className="text-xs text-slate-500">{IMAGE_HOSTING_UPLOAD_TEMPLATE_HINT}</p>
                          <p className="text-xs text-slate-500">
                            预览：{" "}
                            <code className="rounded bg-slate-100 px-1 py-0.5 text-[11px] text-slate-700">
                              {buildImageHostingTemplatePreview(imageHostingDraft.aliyunOss.uploadPathTemplate)}
                            </code>
                          </p>
                        </div>
                      </TabsContent>
                    </Tabs>
                  </div>
                ) : null}
              </div>
            </main>
          </div>
        </div>
      </AdminPageCard>
    </section>
  );
}
