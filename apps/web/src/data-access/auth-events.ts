export const AUTH_UNAUTHORIZED_EVENT = "plaindoc.auth.unauthorized";

export interface AuthUnauthorizedEventDetail {
  status: number;
  code?: number;
  message?: string;
}

