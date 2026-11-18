package attachmentUtils

import (
	"context"
	"errors"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apis/tencentcos"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/attachment/attachmentModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

// ValidateUploadRequest 校验直传元数据，实际文件内容不经过应用服务器。
func ValidateUploadRequest(request attachmentModels.UploadRequest) error {
	if !ValidFilename(request.Filename) || request.Size < 1 ||
		request.Size > config.Config.COS.MaxUploadBytes {
		return uploadError("文件名、文件大小无效或超过上传限制")
	}
	contentType, parameters, err := mime.ParseMediaType(request.ContentType)
	if err != nil || len(parameters) != 0 || contentType != request.ContentType ||
		len(contentType) > 255 || strings.Contains(contentType, "*") ||
		!ContentTypeAllowed(contentType, config.Config.COS.AllowedContentTypes) {
		return uploadError("文件类型不在允许上传的范围内")
	}
	return nil
}

// VerifyUploadedObject 按服务端记录核对 COS 实际长度和声明类型，不接收客户端对象键。
func VerifyUploadedObject(ctx context.Context, record attachmentModels.Record) error {
	metadata, err := tencentcos.Head(ctx, record.ObjectKey)
	if errors.Is(err, tencentcos.ErrObjectNotFound) {
		return &response.BusinessError{
			Status: http.StatusConflict, Message: "文件尚未上传完成，请重试",
		}
	}
	if err != nil {
		return err
	}
	if metadata.Size != record.Size || metadata.ContentType != record.ContentType {
		return uploadError("文件校验失败，请重新上传")
	}
	return nil
}

// ContentTypeAllowed 匹配标准媒体类型及 image/* 等类别通配符。
func ContentTypeAllowed(contentType string, allowed []string) bool {
	contentType = strings.ToLower(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0]))
	for _, item := range allowed {
		if item == contentType ||
			(strings.HasSuffix(item, "/*") &&
				strings.HasPrefix(contentType, strings.TrimSuffix(item, "*"))) {
			return true
		}
	}
	return false
}

func uploadError(message string) error {
	return &response.BusinessError{Status: http.StatusBadRequest, Message: message}
}

// UploadActorKey 仅从事务内复核的身份编号生成上传绑定快照，不接受客户端身份。
func UploadActorKey(userID, studentID uint64) string {
	if studentID != 0 {
		return "student:" + strconv.FormatUint(studentID, 10)
	}
	return "user:" + strconv.FormatUint(userID, 10)
}
