package errcode

import "errors"

var (
	ErrAdminSpaceExportSpaceIDRequired   = errors.New("admin space export space id is required")
	ErrAdminSpaceExportRequestBody       = errors.New("admin space export request body is invalid")
	ErrAdminSpaceExportFormatUnsupported = errors.New("admin space export format is unsupported")
	ErrAdminSpaceExportJobNotFound       = errors.New("admin space export job not found")
	ErrAdminSpaceExportJobTokenInvalid   = errors.New("admin space export job token is invalid")
	ErrAdminSpaceExportJobRunningLimit   = errors.New("admin space export running limit exceeded")
	ErrAdminSpaceExportFileNotReady      = errors.New("admin space export file is not ready")
	ErrAdminSpaceExportFileExpired       = errors.New("admin space export file is expired")
	ErrAdminSpaceExportDownloadForbidden = errors.New("admin space export download is forbidden")
)
