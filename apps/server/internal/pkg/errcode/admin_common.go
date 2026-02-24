package errcode

import "errors"

var (
	// ErrAdminForbidden 表示当前用户无管理员权限。
	ErrAdminForbidden = errors.New("admin forbidden")
)
