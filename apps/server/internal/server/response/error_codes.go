package response

import "net/http"

const (
	// UnknownErrorCode 是未登记错误码的兜底值。
	UnknownErrorCode = 9999
)

var errorCodeRegistry = map[string]int{
	// 1000 段：认证与权限。
	"UNAUTHORIZED":          1001,
	"INVALID_CREDENTIALS":   1002,
	"FORBIDDEN":             1003,
	"ROLE_FORBIDDEN":        1004,
	"REGISTRATION_DISABLED": 1005,
	"USER_BANNED":           1006,
	"USER_DELETED":          1007,

	// 2000 段：请求参数与输入校验。
	"INVALID_REQUEST":          2001,
	"INVALID_EMAIL":            2002,
	"INVALID_PASSWORD":         2003,
	"INVALID_NAME":             2004,
	"INVALID_ROLE":             2005,
	"INVALID_STATUS":           2006,
	"INVALID_REASON":           2007,
	"INVALID_PAGE":             2008,
	"INVALID_PAGE_SIZE":        2009,
	"INVALID_USER_ID":          2010,
	"INVALID_SPACE_ID":         2011,
	"INVALID_DOCUMENT_ID":      2012,
	"INVALID_THEME_ID":         2013,
	"INVALID_VISIBILITY":       2014,
	"INVALID_SOURCE":           2015,
	"INVALID_UPLOAD_FILE":      2016,
	"INVALID_FILE_PATH":        2017,
	"INVALID_CONFIG_KEY":       2018,
	"INVALID_CONFIG_VALUE":     2019,
	"INVALID_EXPECTED_VERSION": 2020,
	"INVALID_OPERATION":        2021,
	"INVALID_MODULE":           2022,
	"INVALID_ACTION":           2023,
	"INVALID_FROM":             2024,
	"INVALID_TO":               2025,
	"INVALID_TIME_RANGE":       2026,
	"INVALID_MEMBER_ROLE":      2027,
	"INVALID_SPACE_NAME":       2028,
	"INVALID_COVER_IMAGE":      2029,
	"INVALID_SYNTAX_THEME":     2030,
	"INVALID_DESCRIPTION":      2031,
	"INVALID_SPACE_CATEGORY":   2032,

	// 3000 段：资源不存在。
	"ROUTE_NOT_FOUND":           3001,
	"METHOD_NOT_ALLOWED":        3002,
	"USER_NOT_FOUND":            3003,
	"SPACE_NOT_FOUND":           3004,
	"DOCUMENT_NOT_FOUND":        3005,
	"THEME_NOT_FOUND":           3006,
	"FILE_NOT_FOUND":            3007,
	"COVER_ASSET_NOT_FOUND":     3008,
	"MEMBER_NOT_FOUND":          3009,
	"MEMBER_TARGET_NOT_FOUND":   3010,
	"TRANSFER_TARGET_NOT_FOUND": 3011,
	"SPACE_CATEGORY_NOT_FOUND":  3012,

	// 4000 段：冲突与并发状态。
	"EMAIL_ALREADY_EXISTS":           4001,
	"CONFIG_VERSION_CONFLICT":        4002,
	"THEME_ALREADY_EXISTS":           4003,
	"THEME_IN_USE":                   4004,
	"OPERATION_TOKEN_REPLAYED":       4005,
	"OPERATION_TOKEN_EXPIRED":        4006,
	"OPERATION_TOKEN_SCOPE_MISMATCH": 4007,
	"OPERATION_TOKEN_INVALID":        4008,
	"SPACE_CATEGORY_NAME_EXISTS":     4009,

	// 5000 段：业务约束。
	"SPACE_DELETED":                    5001,
	"DOCUMENT_DELETED":                 5002,
	"MEMBER_TARGET_REQUIRED":           5003,
	"OWNER_MEMBER_IMMUTABLE":           5004,
	"SELF_OPERATION_FORBIDDEN":         5005,
	"TRANSFER_TARGET_REQUIRED":         5006,
	"TRANSFER_TARGET_NOT_MEMBER":       5007,
	"TRANSFER_TARGET_SELF":             5008,
	"THEME_BUILTIN_IMMUTABLE":          5009,
	"OPERATION_TOKEN_REQUIRED":         5010,
	"IMAGE_HOSTING_PROVIDER_DISABLED":  5011,
	"COVER_IMAGE_TOO_LARGE":            5012,
	"FILE_TOO_LARGE":                   5013,
	"COVER_FONT_UNAVAILABLE":           5014,
	"SPACE_CATEGORY_DEFAULT_IMMUTABLE": 5015,

	// 9000 段：系统级错误。
	"REQUEST_TIMEOUT": 9001,
	"INTERNAL_ERROR":  9002,
}

// ResolveErrorCode 将错误标识映射为整型错误码。
func ResolveErrorCode(code string) int {
	if value, ok := errorCodeRegistry[code]; ok {
		return value
	}
	return UnknownErrorCode
}

// NormalizeErrorHTTPStatus 根据统一协议将错误 HTTP 状态码收敛到 200/403。
func NormalizeErrorHTTPStatus(status int) int {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return http.StatusForbidden
	default:
		return http.StatusOK
	}
}
