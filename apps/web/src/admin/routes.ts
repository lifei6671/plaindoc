export const ADMIN_LOGIN_ROUTE_PATH = "/admin/login";
export const ADMIN_ROUTE_BASE_PATH = "/admin";

// 统一规范化路径，去除末尾冗余斜杠。
export function normalizePath(pathname: string): string {
  const normalized = pathname.replace(/\/+$/, "");
  return normalized || "/";
}

export function isAdminPath(pathname: string): boolean {
  const normalizedPathname = normalizePath(pathname);
  return (
    normalizedPathname === ADMIN_LOGIN_ROUTE_PATH ||
    normalizedPathname === ADMIN_ROUTE_BASE_PATH ||
    normalizedPathname.startsWith(`${ADMIN_ROUTE_BASE_PATH}/`)
  );
}
