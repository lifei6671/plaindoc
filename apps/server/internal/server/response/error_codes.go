package response

import "net/http"

const (
	// UnknownErrorCode 是未登记错误码的兜底值。
	UnknownErrorCode = 9999

	// 1000 段：认证与权限。
	CodeUnauthorized               = "UNAUTHORIZED"
	CodeForbidden                  = "FORBIDDEN"
	CodeRoleForbidden              = "ROLE_FORBIDDEN"
	CodeInvalidCredentials         = "INVALID_CREDENTIALS"
	CodeRegistrationDisabled       = "REGISTRATION_DISABLED"
	CodeUserBanned                 = "USER_BANNED"
	CodeUserDeleted                = "USER_DELETED"
	CodeCaptchaRequired            = "CAPTCHA_REQUIRED"
	CodeCaptchaInvalid             = "CAPTCHA_INVALID"
	CodeAuthTemporarilyLocked      = "AUTH_TEMPORARILY_LOCKED"
	CodePasswordResetUnavailable   = "PASSWORD_RESET_UNAVAILABLE"
	CodePasswordResetRateLimited   = "PASSWORD_RESET_RATE_LIMITED"
	CodePasswordResetSendFailed    = "PASSWORD_RESET_SEND_FAILED"
	CodePasswordResetTokenInvalid  = "PASSWORD_RESET_TOKEN_INVALID"
	CodePasswordResetTokenExpired  = "PASSWORD_RESET_TOKEN_EXPIRED"
	CodePasswordResetTokenConsumed = "PASSWORD_RESET_TOKEN_CONSUMED"
	CodePasswordResetUnsupported   = "PASSWORD_RESET_UNSUPPORTED"

	// 2000 段：请求参数与输入校验。
	CodeInvalidRequest            = "INVALID_REQUEST"
	CodeInvalidEmail              = "INVALID_EMAIL"
	CodeInvalidPassword           = "INVALID_PASSWORD"
	CodeInvalidName               = "INVALID_NAME"
	CodeInvalidRole               = "INVALID_ROLE"
	CodeInvalidStatus             = "INVALID_STATUS"
	CodeInvalidReason             = "INVALID_REASON"
	CodeInvalidPage               = "INVALID_PAGE"
	CodeInvalidPageSize           = "INVALID_PAGE_SIZE"
	CodeInvalidUserID             = "INVALID_USER_ID"
	CodeInvalidSpaceID            = "INVALID_SPACE_ID"
	CodeInvalidDocumentID         = "INVALID_DOCUMENT_ID"
	CodeInvalidThemeID            = "INVALID_THEME_ID"
	CodeInvalidVisibility         = "INVALID_VISIBILITY"
	CodeInvalidSource             = "INVALID_SOURCE"
	CodeInvalidUploadFile         = "INVALID_UPLOAD_FILE"
	CodeInvalidFilePath           = "INVALID_FILE_PATH"
	CodeInvalidConfigKey          = "INVALID_CONFIG_KEY"
	CodeInvalidConfigValue        = "INVALID_CONFIG_VALUE"
	CodeInvalidExpectedVersion    = "INVALID_EXPECTED_VERSION"
	CodeInvalidOperation          = "INVALID_OPERATION"
	CodeInvalidModule             = "INVALID_MODULE"
	CodeInvalidAction             = "INVALID_ACTION"
	CodeInvalidFrom               = "INVALID_FROM"
	CodeInvalidTo                 = "INVALID_TO"
	CodeInvalidTimeRange          = "INVALID_TIME_RANGE"
	CodeInvalidMemberRole         = "INVALID_MEMBER_ROLE"
	CodeInvalidSpaceName          = "INVALID_SPACE_NAME"
	CodeInvalidCoverImage         = "INVALID_COVER_IMAGE"
	CodeInvalidSyntaxTheme        = "INVALID_SYNTAX_THEME"
	CodeInvalidDescription        = "INVALID_DESCRIPTION"
	CodeInvalidSpaceCategory      = "INVALID_SPACE_CATEGORY"
	CodeInvalidNodeID             = "INVALID_NODE_ID"
	CodeInvalidAvatarURL          = "INVALID_AVATAR_URL"
	CodeInvalidNodeMove           = "INVALID_NODE_MOVE"
	CodeInvalidDocumentIdentifier = "INVALID_DOCUMENT_IDENTIFIER"
	CodeInvalidTemplateID         = "INVALID_TEMPLATE_ID"
	CodePasswordConfirmMismatch   = "PASSWORD_CONFIRM_MISMATCH"
	CodeInvalidCurrentPassword    = "INVALID_CURRENT_PASSWORD"
	CodeInvalidNewPassword        = "INVALID_NEW_PASSWORD"

	// 3000 段：资源不存在。
	CodeRouteNotFound          = "ROUTE_NOT_FOUND"
	CodeMethodNotAllowed       = "METHOD_NOT_ALLOWED"
	CodeUserNotFound           = "USER_NOT_FOUND"
	CodeSpaceNotFound          = "SPACE_NOT_FOUND"
	CodeDocumentNotFound       = "DOCUMENT_NOT_FOUND"
	CodeThemeNotFound          = "THEME_NOT_FOUND"
	CodeFileNotFound           = "FILE_NOT_FOUND"
	CodeCoverAssetNotFound     = "COVER_ASSET_NOT_FOUND"
	CodeMemberNotFound         = "MEMBER_NOT_FOUND"
	CodeMemberTargetNotFound   = "MEMBER_TARGET_NOT_FOUND"
	CodeTransferTargetNotFound = "TRANSFER_TARGET_NOT_FOUND"
	CodeSpaceCategoryNotFound  = "SPACE_CATEGORY_NOT_FOUND"
	CodeNodeNotFound           = "NODE_NOT_FOUND"
	CodeTemplateNotFound       = "TEMPLATE_NOT_FOUND"

	// 4000 段：冲突与并发状态。
	CodeEmailAlreadyExists          = "EMAIL_ALREADY_EXISTS"
	CodeConfigVersionConflict       = "CONFIG_VERSION_CONFLICT"
	CodeThemeAlreadyExists          = "THEME_ALREADY_EXISTS"
	CodeThemeInUse                  = "THEME_IN_USE"
	CodeOperationTokenReplayed      = "OPERATION_TOKEN_REPLAYED"
	CodeOperationTokenExpired       = "OPERATION_TOKEN_EXPIRED"
	CodeOperationTokenScopeMismatch = "OPERATION_TOKEN_SCOPE_MISMATCH"
	CodeOperationTokenInvalid       = "OPERATION_TOKEN_INVALID"
	CodeSpaceCategoryNameExists     = "SPACE_CATEGORY_NAME_EXISTS"
	CodeDocumentVersionConflict     = "DOCUMENT_VERSION_CONFLICT"
	CodeSpaceAlreadyExists          = "SPACE_ALREADY_EXISTS"
	CodeDocumentIdentifierConflict  = "DOCUMENT_IDENTIFIER_CONFLICT"
	CodeTemplateAlreadyExists       = "TEMPLATE_ALREADY_EXISTS"

	// 5000 段：业务约束。
	CodeSpaceDeleted                  = "SPACE_DELETED"
	CodeDocumentDeleted               = "DOCUMENT_DELETED"
	CodeMemberTargetRequired          = "MEMBER_TARGET_REQUIRED"
	CodeOwnerMemberImmutable          = "OWNER_MEMBER_IMMUTABLE"
	CodeSelfOperationForbidden        = "SELF_OPERATION_FORBIDDEN"
	CodeTransferTargetRequired        = "TRANSFER_TARGET_REQUIRED"
	CodeTransferTargetNotMember       = "TRANSFER_TARGET_NOT_MEMBER"
	CodeTransferTargetSelf            = "TRANSFER_TARGET_SELF"
	CodeThemeBuiltinImmutable         = "THEME_BUILTIN_IMMUTABLE"
	CodeOperationTokenRequired        = "OPERATION_TOKEN_REQUIRED"
	CodeFileTooLarge                  = "FILE_TOO_LARGE"
	CodeImageHostingProviderDisabled  = "IMAGE_HOSTING_PROVIDER_DISABLED"
	CodeCoverImageTooLarge            = "COVER_IMAGE_TOO_LARGE"
	CodeCoverFontUnavailable          = "COVER_FONT_UNAVAILABLE"
	CodeSpaceCategoryDefaultImmutable = "SPACE_CATEGORY_DEFAULT_IMMUTABLE"
	CodePasswordUnchanged             = "PASSWORD_UNCHANGED"
	CodeDocumentIdentifierReserved    = "DOCUMENT_IDENTIFIER_RESERVED"
	CodeTemplateBuiltinImmutable      = "TEMPLATE_BUILTIN_IMMUTABLE"

	// 9000 段：系统级错误。
	CodeRequestTimeout = "REQUEST_TIMEOUT"
	CodeInternalError  = "INTERNAL_ERROR"
)

var errorCodeRegistry = map[string]int{
	// 1000 段：认证与权限。
	CodeUnauthorized:               1001,
	CodeInvalidCredentials:         1002,
	CodeForbidden:                  1003,
	CodeRoleForbidden:              1004,
	CodeRegistrationDisabled:       1005,
	CodeUserBanned:                 1006,
	CodeUserDeleted:                1007,
	CodeCaptchaRequired:            1008,
	CodeCaptchaInvalid:             1009,
	CodeAuthTemporarilyLocked:      1010,
	CodePasswordResetUnavailable:   1011,
	CodePasswordResetRateLimited:   1012,
	CodePasswordResetSendFailed:    1013,
	CodePasswordResetTokenInvalid:  1014,
	CodePasswordResetTokenExpired:  1015,
	CodePasswordResetTokenConsumed: 1016,
	CodePasswordResetUnsupported:   1017,

	// 2000 段：请求参数与输入校验。
	CodeInvalidRequest:            2001,
	CodeInvalidEmail:              2002,
	CodeInvalidPassword:           2003,
	CodeInvalidName:               2004,
	CodeInvalidRole:               2005,
	CodeInvalidStatus:             2006,
	CodeInvalidReason:             2007,
	CodeInvalidPage:               2008,
	CodeInvalidPageSize:           2009,
	CodeInvalidUserID:             2010,
	CodeInvalidSpaceID:            2011,
	CodeInvalidDocumentID:         2012,
	CodeInvalidThemeID:            2013,
	CodeInvalidVisibility:         2014,
	CodeInvalidSource:             2015,
	CodeInvalidUploadFile:         2016,
	CodeInvalidFilePath:           2017,
	CodeInvalidConfigKey:          2018,
	CodeInvalidConfigValue:        2019,
	CodeInvalidExpectedVersion:    2020,
	CodeInvalidOperation:          2021,
	CodeInvalidModule:             2022,
	CodeInvalidAction:             2023,
	CodeInvalidFrom:               2024,
	CodeInvalidTo:                 2025,
	CodeInvalidTimeRange:          2026,
	CodeInvalidMemberRole:         2027,
	CodeInvalidSpaceName:          2028,
	CodeInvalidCoverImage:         2029,
	CodeInvalidSyntaxTheme:        2030,
	CodeInvalidDescription:        2031,
	CodeInvalidSpaceCategory:      2032,
	CodeInvalidNodeID:             2033,
	CodeInvalidAvatarURL:          2034,
	CodePasswordConfirmMismatch:   2035,
	CodeInvalidCurrentPassword:    2036,
	CodeInvalidNewPassword:        2037,
	CodeInvalidNodeMove:           2038,
	CodeInvalidDocumentIdentifier: 2039,
	CodeInvalidTemplateID:         2040,

	// 3000 段：资源不存在。
	CodeRouteNotFound:          3001,
	CodeMethodNotAllowed:       3002,
	CodeUserNotFound:           3003,
	CodeSpaceNotFound:          3004,
	CodeDocumentNotFound:       3005,
	CodeThemeNotFound:          3006,
	CodeFileNotFound:           3007,
	CodeCoverAssetNotFound:     3008,
	CodeMemberNotFound:         3009,
	CodeMemberTargetNotFound:   3010,
	CodeTransferTargetNotFound: 3011,
	CodeSpaceCategoryNotFound:  3012,
	CodeNodeNotFound:           3013,
	CodeTemplateNotFound:       3014,

	// 4000 段：冲突与并发状态。
	CodeEmailAlreadyExists:          4001,
	CodeConfigVersionConflict:       4002,
	CodeThemeAlreadyExists:          4003,
	CodeThemeInUse:                  4004,
	CodeOperationTokenReplayed:      4005,
	CodeOperationTokenExpired:       4006,
	CodeOperationTokenScopeMismatch: 4007,
	CodeOperationTokenInvalid:       4008,
	CodeSpaceCategoryNameExists:     4009,
	CodeDocumentVersionConflict:     4010,
	CodeSpaceAlreadyExists:          4011,
	CodeDocumentIdentifierConflict:  4012,
	CodeTemplateAlreadyExists:       4013,

	// 5000 段：业务约束。
	CodeSpaceDeleted:                  5001,
	CodeDocumentDeleted:               5002,
	CodeMemberTargetRequired:          5003,
	CodeOwnerMemberImmutable:          5004,
	CodeSelfOperationForbidden:        5005,
	CodeTransferTargetRequired:        5006,
	CodeTransferTargetNotMember:       5007,
	CodeTransferTargetSelf:            5008,
	CodeThemeBuiltinImmutable:         5009,
	CodeOperationTokenRequired:        5010,
	CodeImageHostingProviderDisabled:  5011,
	CodeCoverImageTooLarge:            5012,
	CodeFileTooLarge:                  5013,
	CodeCoverFontUnavailable:          5014,
	CodeSpaceCategoryDefaultImmutable: 5015,
	CodePasswordUnchanged:             5016,
	CodeDocumentIdentifierReserved:    5017,
	CodeTemplateBuiltinImmutable:      5018,

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
