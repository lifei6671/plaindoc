package response

import "net/http"

const (
	// UnknownErrorCode 是未登记错误码的兜底值。
	UnknownErrorCode = 9999

	// 1000 段：认证与权限。
	CodeUnauthorized         = "UNAUTHORIZED"
	CodeForbidden            = "FORBIDDEN"
	CodeInvalidCredentials   = "INVALID_CREDENTIALS"
	CodeRegistrationDisabled = "REGISTRATION_DISABLED"
	CodeUserBanned           = "USER_BANNED"
	CodeUserDeleted          = "USER_DELETED"

	// 2000 段：请求参数与输入校验。
	CodeInvalidRequest          = "INVALID_REQUEST"
	CodeInvalidEmail            = "INVALID_EMAIL"
	CodeInvalidPassword         = "INVALID_PASSWORD"
	CodeInvalidName             = "INVALID_NAME"
	CodeInvalidPage             = "INVALID_PAGE"
	CodeInvalidPageSize         = "INVALID_PAGE_SIZE"
	CodeInvalidUserID           = "INVALID_USER_ID"
	CodeInvalidSpaceID          = "INVALID_SPACE_ID"
	CodeInvalidDocumentID       = "INVALID_DOCUMENT_ID"
	CodeInvalidThemeID          = "INVALID_THEME_ID"
	CodeInvalidVisibility       = "INVALID_VISIBILITY"
	CodeInvalidUploadFile       = "INVALID_UPLOAD_FILE"
	CodeInvalidFilePath         = "INVALID_FILE_PATH"
	CodeInvalidConfigKey        = "INVALID_CONFIG_KEY"
	CodeInvalidFrom             = "INVALID_FROM"
	CodeInvalidTo               = "INVALID_TO"
	CodeInvalidSpaceName        = "INVALID_SPACE_NAME"
	CodeInvalidSpaceCategory    = "INVALID_SPACE_CATEGORY"
	CodeInvalidNodeID           = "INVALID_NODE_ID"
	CodeInvalidNodeMove         = "INVALID_NODE_MOVE"
	CodePasswordConfirmMismatch = "PASSWORD_CONFIRM_MISMATCH"

	// 3000 段：资源不存在。
	CodeUserNotFound     = "USER_NOT_FOUND"
	CodeSpaceNotFound    = "SPACE_NOT_FOUND"
	CodeDocumentNotFound = "DOCUMENT_NOT_FOUND"
	CodeThemeNotFound    = "THEME_NOT_FOUND"
	CodeFileNotFound     = "FILE_NOT_FOUND"
	CodeNodeNotFound     = "NODE_NOT_FOUND"
	CodeRouteNotFound    = "ROUTE_NOT_FOUND"
	CodeMethodNotAllowed = "METHOD_NOT_ALLOWED"

	// 4000 段：冲突与并发状态。
	CodeEmailAlreadyExists = "EMAIL_ALREADY_EXISTS"

	// 5000 段：业务约束。
	CodeFileTooLarge                 = "FILE_TOO_LARGE"
	CodeImageHostingProviderDisabled = "IMAGE_HOSTING_PROVIDER_DISABLED"

	// 9000 段：系统级错误。
	CodeRequestTimeout = "REQUEST_TIMEOUT"
	CodeInternalError  = "INTERNAL_ERROR"
)

var errorCodeRegistry = map[string]int{
	// 1000 段：认证与权限。
	CodeUnauthorized:         1001,
	CodeInvalidCredentials:   1002,
	CodeForbidden:            1003,
	"ROLE_FORBIDDEN":         1004,
	CodeRegistrationDisabled: 1005,
	CodeUserBanned:           1006,
	CodeUserDeleted:          1007,

	// 2000 段：请求参数与输入校验。
	CodeInvalidRequest:          2001,
	CodeInvalidEmail:            2002,
	CodeInvalidPassword:         2003,
	CodeInvalidName:             2004,
	"INVALID_ROLE":              2005,
	"INVALID_STATUS":            2006,
	"INVALID_REASON":            2007,
	CodeInvalidPage:             2008,
	CodeInvalidPageSize:         2009,
	CodeInvalidUserID:           2010,
	CodeInvalidSpaceID:          2011,
	CodeInvalidDocumentID:       2012,
	CodeInvalidThemeID:          2013,
	CodeInvalidVisibility:       2014,
	"INVALID_SOURCE":            2015,
	CodeInvalidUploadFile:       2016,
	CodeInvalidFilePath:         2017,
	CodeInvalidConfigKey:        2018,
	"INVALID_CONFIG_VALUE":      2019,
	"INVALID_EXPECTED_VERSION":  2020,
	"INVALID_OPERATION":         2021,
	"INVALID_MODULE":            2022,
	"INVALID_ACTION":            2023,
	CodeInvalidFrom:             2024,
	CodeInvalidTo:               2025,
	"INVALID_TIME_RANGE":        2026,
	"INVALID_MEMBER_ROLE":       2027,
	CodeInvalidSpaceName:        2028,
	"INVALID_COVER_IMAGE":       2029,
	"INVALID_SYNTAX_THEME":      2030,
	"INVALID_DESCRIPTION":       2031,
	CodeInvalidSpaceCategory:    2032,
	CodeInvalidNodeID:           2033,
	"INVALID_AVATAR_URL":        2034,
	CodePasswordConfirmMismatch: 2035,
	"INVALID_CURRENT_PASSWORD":  2036,
	"INVALID_NEW_PASSWORD":      2037,
	CodeInvalidNodeMove:         2038,

	// 3000 段：资源不存在。
	CodeRouteNotFound:           3001,
	CodeMethodNotAllowed:        3002,
	CodeUserNotFound:            3003,
	CodeSpaceNotFound:           3004,
	CodeDocumentNotFound:        3005,
	CodeThemeNotFound:           3006,
	CodeFileNotFound:            3007,
	"COVER_ASSET_NOT_FOUND":     3008,
	"MEMBER_NOT_FOUND":          3009,
	"MEMBER_TARGET_NOT_FOUND":   3010,
	"TRANSFER_TARGET_NOT_FOUND": 3011,
	"SPACE_CATEGORY_NOT_FOUND":  3012,
	CodeNodeNotFound:            3013,

	// 4000 段：冲突与并发状态。
	CodeEmailAlreadyExists:           4001,
	"CONFIG_VERSION_CONFLICT":        4002,
	"THEME_ALREADY_EXISTS":           4003,
	"THEME_IN_USE":                   4004,
	"OPERATION_TOKEN_REPLAYED":       4005,
	"OPERATION_TOKEN_EXPIRED":        4006,
	"OPERATION_TOKEN_SCOPE_MISMATCH": 4007,
	"OPERATION_TOKEN_INVALID":        4008,
	"SPACE_CATEGORY_NAME_EXISTS":     4009,
	"DOCUMENT_VERSION_CONFLICT":      4010,
	"SPACE_ALREADY_EXISTS":           4011,

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
	CodeImageHostingProviderDisabled:   5011,
	"COVER_IMAGE_TOO_LARGE":            5012,
	CodeFileTooLarge:                   5013,
	"COVER_FONT_UNAVAILABLE":           5014,
	"SPACE_CATEGORY_DEFAULT_IMMUTABLE": 5015,
	"PASSWORD_UNCHANGED":               5016,

	// 9000 段：系统级错误。
	CodeRequestTimeout: 9001,
	CodeInternalError:  9002,
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
