package errcode

import "errors"

var (
	ErrAdminSpaceImportFileRequired         = errors.New("admin space import file is required")
	ErrAdminSpaceImportZipInvalid           = errors.New("admin space import zip is invalid")
	ErrAdminSpaceImportManifestMissing      = errors.New("admin space import manifest is missing")
	ErrAdminSpaceImportTreeMissing          = errors.New("admin space import tree is missing")
	ErrAdminSpaceImportPackageUnsupported   = errors.New("admin space import package is unsupported")
	ErrAdminSpaceImportPackageNotImportable = errors.New("admin space import package is not importable")
	ErrAdminSpaceImportStagingNotFound      = errors.New("admin space import staging not found")
	ErrAdminSpaceImportStagingExpired       = errors.New("admin space import staging is expired")
	ErrAdminSpaceImportJobRunningLimit      = errors.New("admin space import running limit exceeded")
	ErrAdminSpaceImportJobTokenInvalid      = errors.New("admin space import job token is invalid")
	ErrAdminSpaceImportCommitForbidden      = errors.New("admin space import commit is forbidden")
)
