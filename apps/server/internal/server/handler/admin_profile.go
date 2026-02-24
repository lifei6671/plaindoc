package handler

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/middleware"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
	"github.com/lifei6671/plaindoc/apps/server/internal/service"
)

const (
	defaultAdminProfileLocalImageRoot = "uploads/local"
	maxAdminProfileAvatarSizeBytes    = 10 << 20
)

type adminProfileHandler struct {
	adminProfileService *service.AdminProfileService
	imageHostingService *service.ImageHostingService
	localImageRootDir   string
}

type adminProfileResponse struct {
	UserID    string    `json:"userId"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	AvatarURL string    `json:"avatarUrl"`
	Roles     []string  `json:"roles"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type updateAdminProfileRequest struct {
	Name      *string `json:"name"`
	AvatarURL *string `json:"avatarUrl"`
}

type updateAdminProfilePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
	ConfirmPassword string `json:"confirmPassword"`
}

// NewAdminProfileHandler 创建后台个人信息处理器。
func NewAdminProfileHandler(
	adminProfileService *service.AdminProfileService,
	imageHostingService *service.ImageHostingService,
) *adminProfileHandler {
	return &adminProfileHandler{
		adminProfileService: adminProfileService,
		imageHostingService: imageHostingService,
		localImageRootDir:   defaultAdminProfileLocalImageRoot,
	}
}

// GetProfile 返回当前管理员个人信息。
func (h *adminProfileHandler) GetProfile(c *gin.Context) {
	if h == nil || h.adminProfileService == nil {
		response.InternalError(c)
		return
	}
	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		response.AdminProfileErrAdminActorMissing.Write(c)
		return
	}

	payload, err := h.adminProfileService.GetProfile(c.Request.Context(), actorUserID)
	if err != nil {
		response.FromError(c, err)
		return
	}
	response.JSON(c, http.StatusOK, mapAdminProfileResponse(payload))
}

// UpdateProfile 更新当前管理员昵称或头像地址。
func (h *adminProfileHandler) UpdateProfile(c *gin.Context) {
	if h == nil || h.adminProfileService == nil {
		response.InternalError(c)
		return
	}
	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		response.AdminProfileErrAdminActorMissing.Write(c)
		return
	}

	var req updateAdminProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AdminProfileErrRequestBody.Write(c)
		return
	}

	payload, err := h.adminProfileService.UpdateProfile(c.Request.Context(), service.UpdateAdminProfileInput{
		ActorUserID: actorUserID,
		RequestID:   response.RequestIDFromContext(c),
		Name:        req.Name,
		AvatarURL:   req.AvatarURL,
	})
	if err != nil {
		response.FromError(c, err)
		return
	}
	response.JSON(c, http.StatusOK, mapAdminProfileResponse(payload))
}

// UpdatePassword 更新当前管理员密码。
func (h *adminProfileHandler) UpdatePassword(c *gin.Context) {
	if h == nil || h.adminProfileService == nil {
		response.InternalError(c)
		return
	}
	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		response.AdminProfileErrAdminActorMissing.Write(c)
		return
	}

	var req updateAdminProfilePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AdminProfileErrRequestBody.Write(c)
		return
	}
	if req.NewPassword != req.ConfirmPassword {
		response.AdminProfileErrNewPasswordConfirmPasswordMismatch.Write(c)
		return
	}

	if err := h.adminProfileService.UpdatePassword(c.Request.Context(), service.UpdateAdminPasswordInput{
		ActorUserID:     actorUserID,
		RequestID:       response.RequestIDFromContext(c),
		CurrentPassword: req.CurrentPassword,
		NewPassword:     req.NewPassword,
		ConfirmPassword: req.ConfirmPassword,
	}); err != nil {
		response.FromError(c, err)
		return
	}

	response.JSON(c, http.StatusOK, struct{}{})
}

// UploadAvatar 上传头像并更新当前管理员头像地址。
func (h *adminProfileHandler) UploadAvatar(c *gin.Context) {
	if h == nil || h.adminProfileService == nil {
		response.InternalError(c)
		return
	}
	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		response.AdminProfileErrAdminActorMissing.Write(c)
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil || fileHeader == nil {
		response.AdminProfileErrFileRequired.Write(c)
		return
	}
	if fileHeader.Size <= 0 {
		response.AdminProfileErrFileEmpty.Write(c)
		return
	}
	if fileHeader.Size > maxAdminProfileAvatarSizeBytes {
		response.AdminProfileErrAvatarFileExceeds10mbLimit.Write(c)
		return
	}

	contentType, err := detectAdminProfileUploadedFileContentType(fileHeader)
	if err != nil {
		response.AdminProfileErrCannotReadUploadedFile.Write(c)
		return
	}
	if !strings.HasPrefix(strings.ToLower(contentType), "image/") {
		response.AdminProfileErrOnlyImageFileAllowed.Write(c)
		return
	}

	objectKey, err := buildAdminProfileAvatarObjectKey(actorUserID, fileHeader.Filename, contentType, time.Now().UTC())
	if err != nil {
		response.InternalError(c)
		return
	}

	localImageRootDir := strings.TrimSpace(h.localImageRootDir)
	if localImageRootDir == "" {
		localImageRootDir = defaultAdminProfileLocalImageRoot
	}
	targetPath := filepath.Join(localImageRootDir, filepath.FromSlash(objectKey))
	targetDir := filepath.Dir(targetPath)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		response.InternalError(c)
		return
	}
	if err := c.SaveUploadedFile(fileHeader, targetPath); err != nil {
		response.InternalError(c)
		return
	}

	avatarURL := resolveAdminProfilePublicURL("/uploads", objectKey)
	if h.imageHostingService != nil {
		config, err := h.imageHostingService.GetConfig(c.Request.Context())
		if err != nil {
			response.InternalError(c)
			return
		}
		avatarURL = resolveAdminProfilePublicURL(config.Local.PublicBaseURL, objectKey)
	}

	payload, err := h.adminProfileService.UpdateProfile(c.Request.Context(), service.UpdateAdminProfileInput{
		ActorUserID: actorUserID,
		RequestID:   response.RequestIDFromContext(c),
		AvatarURL:   &avatarURL,
	})
	if err != nil {
		response.FromError(c, err)
		return
	}
	response.JSON(c, http.StatusOK, mapAdminProfileResponse(payload))
}

func mapAdminProfileResponse(payload service.AdminProfileRecord) adminProfileResponse {
	roles := make([]string, 0, len(payload.Roles))
	for _, role := range payload.Roles {
		roles = append(roles, string(role))
	}
	return adminProfileResponse{
		UserID:    payload.UserID,
		Email:     payload.Email,
		Name:      payload.Name,
		AvatarURL: payload.AvatarURL,
		Roles:     roles,
		CreatedAt: payload.CreatedAt,
		UpdatedAt: payload.UpdatedAt,
	}
}

func detectAdminProfileUploadedFileContentType(fileHeader *multipart.FileHeader) (string, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return "", err
	}
	defer file.Close()

	buffer := make([]byte, 512)
	readCount, readErr := file.Read(buffer)
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return "", readErr
	}
	return strings.TrimSpace(http.DetectContentType(buffer[:readCount])), nil
}

func buildAdminProfileAvatarObjectKey(
	userID string,
	fileName string,
	contentType string,
	now time.Time,
) (string, error) {
	extension := resolveAdminProfileFileExtension(fileName, contentType)
	randomSuffix, err := randomAdminProfileHex(4)
	if err != nil {
		return "", err
	}

	safeUserID := strings.ToLower(strings.TrimSpace(userID))
	if safeUserID == "" {
		safeUserID = "unknown"
	}
	return "avatars/" + safeUserID + "/" + now.Format("2006/01/02") + "/" +
		strconv.FormatInt(now.UnixMilli(), 10) + "-" + randomSuffix + "." + extension, nil
}

func resolveAdminProfileFileExtension(fileName string, contentType string) string {
	extension := strings.TrimSpace(strings.ToLower(path.Ext(fileName)))
	if extension != "" {
		extension = strings.TrimPrefix(extension, ".")
		if isSafeAdminProfileFileExtension(extension) {
			return extension
		}
	}

	extensions, err := mime.ExtensionsByType(contentType)
	if err == nil {
		for _, item := range extensions {
			candidate := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(item)), ".")
			if isSafeAdminProfileFileExtension(candidate) {
				return candidate
			}
		}
	}

	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "image/jpeg":
		return "jpg"
	case "image/png":
		return "png"
	case "image/gif":
		return "gif"
	case "image/webp":
		return "webp"
	case "image/svg+xml":
		return "svg"
	default:
		return "png"
	}
}

func isSafeAdminProfileFileExtension(extension string) bool {
	if extension == "" {
		return false
	}
	for _, char := range extension {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') {
			continue
		}
		return false
	}
	return true
}

func randomAdminProfileHex(lengthBytes int) (string, error) {
	if lengthBytes <= 0 {
		return "", errors.New("random length must be positive")
	}
	raw := make([]byte, lengthBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func resolveAdminProfilePublicURL(baseURL string, objectPath string) string {
	base := strings.TrimSpace(baseURL)
	if base == "" {
		base = "/uploads"
	}
	if strings.EqualFold(strings.TrimRight(base, "/"), "/api/uploads/local") {
		base = "/uploads"
	}
	if strings.EqualFold(strings.TrimRight(base, "/"), "/uploads/local") {
		base = "/uploads"
	}
	if strings.HasPrefix(base, "/api/uploads/") {
		base = strings.TrimPrefix(base, "/api")
	}
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(objectPath, "/")
}
