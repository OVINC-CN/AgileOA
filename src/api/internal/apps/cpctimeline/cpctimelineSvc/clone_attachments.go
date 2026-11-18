package cpctimelineSvc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/attachment/attachmentConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/attachment/attachmentModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/attachment/attachmentUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpctimeline/cpctimelineModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/db"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/logger"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/storage"
)

func planCloneAttachments(
	tx *gorm.DB, workspaceID string, sourceID, targetID uint64,
) ([]attachmentModels.TimelineNodeCopy, error) {
	// 全量读取当前租户的节点附件，禁止静默跳过未完成或待删除文件。
	var files []attachmentModels.TimelineNodeAttachment
	if err := tx.Where("workspace_id = ? AND timeline_template_id = ?",
		workspaceID, sourceID).Order("id ASC").Find(&files).Error; err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, nil
	}
	counts := make(map[string]int)
	for _, file := range files {
		if file.State != attachmentConstants.Ready {
			return nil, &response.BusinessError{
				Status: http.StatusConflict, Message: "原模板存在未完成或待删除的附件，请先处理后再克隆",
			}
		}
		counts[file.NodeID]++
		if counts[file.NodeID] > config.Config.CPCTimeline.MaxAttachments {
			return nil, &response.BusinessError{
				Status: http.StatusConflict, Message: "原模板节点附件数量超过当前上限，无法克隆",
			}
		}
	}
	if !storage.Enabled() {
		return nil, &response.BusinessError{
			Status: http.StatusServiceUnavailable, Message: "附件服务暂时不可用",
		}
	}

	// 每个副本使用目标模板的独立路径、对象哈希、附件编号和初始版本。
	copies := make([]attachmentModels.TimelineNodeCopy, 0, len(files))
	for _, file := range files {
		prefix := "workspaces/" + workspaceID + "/cpc-timeline-templates/" +
			strconv.FormatUint(targetID, 10) + "/nodes/" + file.NodeID
		object, err := attachmentUtils.RandomObjectName(prefix, file.Filename)
		if err != nil {
			return nil, err
		}
		if len(object) > attachmentConstants.ObjectKeyMaxBytes {
			return nil, invalid("附件文件名过长，无法克隆")
		}
		digest := sha256.Sum256([]byte(object))
		copies = append(copies, attachmentModels.TimelineNodeCopy{
			Source: file,
			Target: attachmentModels.TimelineNodeAttachment{
				Record: attachmentModels.Record{
					WorkspaceID: workspaceID,
					ObjectKey:   object, ObjectHash: hex.EncodeToString(digest[:]),
					Filename: file.Filename, Size: file.Size, ContentType: file.ContentType,
					State: attachmentConstants.Ready, Revision: 1,
				},
				TimelineTemplateID: targetID,
				NodeID:             file.NodeID,
			},
		})
	}
	return copies, nil
}

func discardCloneAttachments(
	ctx context.Context, workspaceID string, templateID uint64,
	copies []attachmentModels.TimelineNodeCopy,
) {
	if len(copies) == 0 {
		return
	}

	// 提交结果不明确时先确认模板未落库，避免删除已经提交且仍被引用的对象。
	checkCtx, cancel := context.WithTimeout(
		context.WithoutCancel(ctx), config.Config.HTTP.RequestTimeout,
	)
	var count int64
	err := db.DB().WithContext(checkCtx).Model(&cpctimelineModels.Template{}).
		Where("workspace_id = ? AND id = ?", workspaceID, templateID).Count(&count).Error
	cancel()
	if err != nil {
		logger.Logger.WithContext(ctx).Errorf(
			"CPC timeline clone cleanup check failed; workspace: %s; template: %d; error: %v",
			workspaceID, templateID, err,
		)
		return
	}
	if count != 0 {
		logger.Logger.WithContext(ctx).Infof(
			"CPC timeline clone commit verified; workspace: %s; template: %d",
			workspaceID, templateID,
		)
		return
	}

	// 客户端取消也继续逐个补偿，单次清理沿用 YAML 中的对象请求超时。
	for _, fileCopy := range copies {
		cleanupCtx, cleanupCancel := context.WithTimeout(
			context.WithoutCancel(ctx), config.Config.COS.RequestTimeout,
		)
		err := storage.Delete(cleanupCtx, fileCopy.Target.ObjectKey)
		cleanupCancel()
		if err != nil {
			logger.Logger.WithContext(ctx).Errorf(
				"CPC timeline clone object cleanup failed; workspace: %s; template: %d; "+
					"source attachment: %d; error: %v",
				workspaceID, templateID, fileCopy.Source.ID, err,
			)
		}
	}
}
