package errcode

import "errors"

var (
	ErrAdminSpaceTransferTaskNotFound             = errors.New("admin space transfer task not found")
	ErrAdminSpaceTransferTaskKindUnsupported      = errors.New("admin space transfer task kind is unsupported")
	ErrAdminSpaceTransferTaskStreamURLUnavailable = errors.New("admin space transfer task stream url is unavailable")
	ErrAdminSpaceTransferTaskDownloadUnavailable  = errors.New("admin space transfer task download is unavailable")
)
