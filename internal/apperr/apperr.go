package apperr

import (
	"errors"
	"fmt"
	"strings"
)

// Stable error codes shared between the Go backend and the frontend i18n dictionary.
const (
	// Generic
	CodeInvalidRequest   = "INVALID_REQUEST"
	CodeNotFound         = "NOT_FOUND"
	CodeInternalError    = "INTERNAL_ERROR"
	CodeAPIRouteNotFound = "API_ROUTE_NOT_FOUND"
	CodeUnauthorized     = "UNAUTHORIZED"
	CodeForbidden        = "FORBIDDEN"

	// Auth & Users
	CodeSystemAlreadyInitialized    = "SYSTEM_ALREADY_INITIALIZED"
	CodeInvalidCredentials          = "INVALID_CREDENTIALS"
	CodeAccountDisabled             = "ACCOUNT_DISABLED"
	CodeRateLimited                 = "RATE_LIMITED"
	CodeAdminRoleRequired           = "ADMIN_ROLE_REQUIRED"
	CodeInsufficientPermission      = "INSUFFICIENT_PERMISSION"
	CodeInsufficientClusterPerm     = "INSUFFICIENT_CLUSTER_PERMISSION"
	CodePermissionLookupFailed      = "PERMISSION_LOOKUP_FAILED"
	CodeInvalidRole                 = "INVALID_ROLE"
	CodeInvalidUserID               = "INVALID_USER_ID"
	CodeUserAlreadyExists           = "USER_ALREADY_EXISTS"
	CodePasswordTooShort            = "PASSWORD_TOO_SHORT"
	CodePasswordHashFailed          = "PASSWORD_HASH_FAILED"
	CodeTokenIssueFailed            = "TOKEN_ISSUE_FAILED"
	CodeCannotDisableSelf           = "CANNOT_DISABLE_SELF"
	CodeCannotDeleteSelf            = "CANNOT_DELETE_SELF"

	// System settings & Self-upgrade
	CodeInvalidPortPoolRange     = "INVALID_PORT_POOL_RANGE"
	CodeInvalidProxyURL          = "INVALID_PROXY_URL"
	CodeProxyTestFailed          = "PROXY_TEST_FAILED"
	CodeVersionCheckFailed       = "VERSION_CHECK_FAILED"
	CodeUpgradeUnavailable       = "UPGRADE_UNAVAILABLE"
	CodeUpgradeInProgress        = "UPGRADE_IN_PROGRESS"
	CodeInvalidTargetImage       = "INVALID_TARGET_IMAGE"
	CodeUpgradeTriggerFailed     = "UPGRADE_TRIGGER_FAILED"

	// Clusters & Ports
	CodeClusterNotFound      = "CLUSTER_NOT_FOUND"
	CodeClusterAlreadyExists = "CLUSTER_ALREADY_EXISTS"
	CodeInvalidClusterID     = "INVALID_CLUSTER_ID"
	CodeClusterIDRequired    = "CLUSTER_ID_REQUIRED"
	CodeInvalidShard         = "INVALID_SHARD"
	CodeCavesDisabled        = "CAVES_DISABLED"
	CodeClusterTokenMissing  = "CLUSTER_TOKEN_MISSING"
	CodePortExhausted        = "PORT_EXHAUSTED"
	CodePortOutOfRange       = "PORT_OUT_OF_RANGE"
	CodePortDuplicate        = "PORT_DUPLICATE"
	CodePortConflict         = "PORT_CONFLICT"

	// Runtime & Console
	CodeDSTBinaryNotFound       = "DST_BINARY_NOT_FOUND"
	CodeShardAlreadyActive      = "SHARD_ALREADY_ACTIVE"
	CodeShardNotRunning         = "SHARD_NOT_RUNNING"
	CodeShardStartFailed        = "SHARD_START_FAILED"
	CodeConsoleOperatorRequired = "CONSOLE_OPERATOR_REQUIRED"

	// Players
	CodeInvalidPlayerKUID   = "INVALID_PLAYER_KUID"
	CodeUnknownPlayerAction = "UNKNOWN_PLAYER_ACTION"

	// World & Lua
	CodeLuaValidationFailed    = "LUA_VALIDATION_FAILED"
	CodeLevelDataMustBeTable   = "LEVELDATA_MUST_RETURN_TABLE"
	CodeLuaParseFailed         = "LUA_PARSE_FAILED"
	CodeModInfoParseFailed     = "MODINFO_PARSE_FAILED"
	CodeModOverridesParseFail  = "MODOVERRIDES_PARSE_FAILED"

	// Backups & Save import
	CodeInvalidBackupID          = "INVALID_BACKUP_ID"
	CodeBackupFileMissing        = "BACKUP_FILE_MISSING"
	CodeMissingZipFile           = "MISSING_ZIP_FILE"
	CodeSaveArchiveTooLarge      = "SAVE_ARCHIVE_TOO_LARGE"
	CodeInvalidZipArchive        = "INVALID_ZIP_ARCHIVE"
	CodeIllegalZipPath           = "ILLEGAL_ZIP_PATH"
	CodeImportClusterRootMissing = "IMPORT_CLUSTER_ROOT_NOT_FOUND"

	// File browser
	CodeClusterDirUnavailable = "CLUSTER_DIR_UNAVAILABLE"
	CodePathAccessDenied      = "PATH_ACCESS_DENIED"
	CodeCannotReadDir         = "CANNOT_READ_DIR"
	CodeCannotOverwriteRoot   = "CANNOT_OVERWRITE_ROOT"
	CodeFileNotEditable       = "FILE_NOT_EDITABLE"
	CodeCannotOverwriteDir    = "CANNOT_OVERWRITE_DIR"
	CodeCannotWriteSymlink    = "CANNOT_WRITE_SYMLINK"
	CodeProtectedPathDelete   = "PROTECTED_PATH_DELETE"
	CodeDirDeleteDisabled     = "DIR_DELETE_DISABLED"
	CodeCannotDownloadDir     = "CANNOT_DOWNLOAD_DIR"

	// Mods & Workshop
	CodeModArchiveTooLarge        = "MOD_ARCHIVE_TOO_LARGE"
	CodeModZipMissingModInfo      = "MOD_ZIP_MISSING_MODINFO"
	CodeNoWorkshopIDs             = "NO_WORKSHOP_IDS"
	CodeLocalModNotFound          = "LOCAL_MOD_NOT_FOUND"
	CodeOnlyWorkshopDownloadable  = "ONLY_WORKSHOP_DOWNLOADABLE"
	CodeOnlyLocalModDeletable     = "ONLY_LOCAL_MOD_DELETABLE"
	CodeInvalidWorkshopID         = "INVALID_WORKSHOP_ID"
	CodeWorkshopAPIFailed         = "WORKSHOP_API_FAILED"
	CodeWorkshopCollectionAPIFail = "WORKSHOP_COLLECTION_API_FAILED"
	CodeWorkshopCollectionEmpty   = "WORKSHOP_COLLECTION_EMPTY"
	CodeSteamAPIKeyRequired       = "STEAM_API_KEY_REQUIRED"
	CodeWorkshopSearchFailed      = "WORKSHOP_SEARCH_FAILED"
	CodeSteamAPIKeyRejected       = "STEAM_API_KEY_REJECTED"
	CodeInvalidModID              = "INVALID_MOD_ID"

	// SteamCMD
	CodeKleiBuildsFetchFailed   = "KLEI_BUILDS_FETCH_FAILED"
	CodeSteamCMDBusy            = "STEAMCMD_BUSY"
	CodeSteamCMDNotFound        = "STEAMCMD_NOT_FOUND"
	CodeWorkshopDownloadMissing = "WORKSHOP_DOWNLOAD_MISSING"

	// Snippets, Schedules, Webhooks
	CodeInvalidSnippetID         = "INVALID_SNIPPET_ID"
	CodeInvalidTaskID            = "INVALID_TASK_ID"
	CodeTaskNotFound             = "TASK_NOT_FOUND"
	CodeInvalidTaskType          = "INVALID_TASK_TYPE"
	CodeAdminRequiredForUpdate   = "ADMIN_REQUIRED_FOR_UPDATE_TASK"
	CodeInvalidCronExpr          = "INVALID_CRON_EXPR"
	CodeAnnounceMessageEmpty     = "ANNOUNCE_MESSAGE_EMPTY"
	CodeInvalidWebhookID         = "INVALID_WEBHOOK_ID"
	CodeWebhookNotFound          = "WEBHOOK_NOT_FOUND"
	CodeWebhookURLEmpty          = "WEBHOOK_URL_EMPTY"
)

// Error is a structured application error carrying a stable i18n code,
// optional interpolation parameters, and a fallback English message.
type Error struct {
	Code    string         `json:"code"`
	Params  map[string]any `json:"params,omitempty"`
	Message string         `json:"error"`
	Cause   error          `json:"-"`
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func New(code, message string) *Error {
	return &Error{Code: code, Message: message}
}

func Newf(code string, params map[string]any, format string, args ...any) *Error {
	return &Error{
		Code:    code,
		Params:  params,
		Message: fmt.Sprintf(format, args...),
	}
}

func Wrap(code string, cause error, params map[string]any, message string) *Error {
	if message == "" && cause != nil {
		message = cause.Error()
	}
	return &Error{
		Code:    code,
		Params:  params,
		Message: message,
		Cause:   cause,
	}
}

func WrapDetail(code string, cause error, format string, args ...any) *Error {
	var params map[string]any
	if cause != nil {
		params = map[string]any{"detail": cause.Error()}
	}
	return &Error{
		Code:    code,
		Params:  params,
		Message: fmt.Sprintf(format, args...),
		Cause:   cause,
	}
}

// Extract resolves an error into its structured code, optional params, and fallback message.
func Extract(err error, fallbackCode string) (code string, params map[string]any, msg string) {
	if err == nil {
		return "", nil, ""
	}
	var ae *Error
	if errors.As(err, &ae) {
		return ae.Code, ae.Params, ae.Message
	}
	msg = err.Error()
	if strings.Contains(msg, "UNIQUE constraint failed: users.username") {
		return CodeUserAlreadyExists, nil, msg
	}
	if fallbackCode != "" {
		return fallbackCode, nil, msg
	}
	return CodeInternalError, nil, msg
}

// Body builds a standard JSON error response map keeping `error` for backwards compatibility.
func Body(err error, fallbackCode string) map[string]any {
	code, params, msg := Extract(err, fallbackCode)
	out := map[string]any{
		"error": msg,
		"code":  code,
	}
	if len(params) > 0 {
		out["params"] = params
	}
	return out
}

// CodeBody builds a standard JSON error response directly from a code, fallback message, and optional params.
func CodeBody(code, message string, params map[string]any) map[string]any {
	out := map[string]any{
		"error": message,
		"code":  code,
	}
	if len(params) > 0 {
		out["params"] = params
	}
	return out
}
