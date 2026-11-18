package cpctimelineSvc

import (
	"context"
	"net/http"
	"strconv"
	"unicode/utf8"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/attachment/attachmentModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpctimeline/cpctimelineConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpctimeline/cpctimelineModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/logger"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
	"github.com/OVINC-CN/AgileOA/src/api/internal/storage"
)

// Clone 在空间锁内创建独立模板、节点和附件，成功审计与数据库写入共同提交。
func Clone(
	ctx context.Context, workspaceID string, id, revision uint64,
) (cpctimelineModels.Info, error) {
	var result cpctimelineModels.Info
	var copies []attachmentModels.TimelineNodeCopy
	var cloneID uint64
	auditEvent := event(workspaceID, id)
	auditEvent.Conditions["operation"] = "clone"
	auditEvent.Conditions["source_template_id"] = id
	auditEvent.Conditions["source_revision"] = revision
	err := accessUtils.Workspace(
		ctx, workspaceID, accessConstants.CPCTimelineCreate, auditEvent,
		func(tx *gorm.DB, principal accessModels.Principal, audit *auditModels.Event) error {
			// 校验原模板版本，名称冲突必须在任何对象复制之前拒绝。
			source, err := load(tx, principal.Workspace.ID, id)
			if err != nil {
				return err
			}
			if revision != source.Revision {
				return stale()
			}
			input, err := cloneInput(source)
			if err != nil {
				return err
			}
			record := cpctimelineModels.Template{
				WorkspaceID: principal.Workspace.ID,
				Name:        input.Name, Description: input.Description, Revision: 1,
			}
			if err := tx.Create(&record).Error; err != nil {
				return writeError(err)
			}
			cloneID = record.ID

			// 仅复制模板自有定义，不创建或改写任何人员时间线。
			nodes, err := nodeRecords(principal.Workspace.ID, record.ID, input.Definition.Nodes)
			if err != nil {
				return err
			}
			if err := tx.Create(&nodes).Error; err != nil {
				return err
			}
			if err := saveTransfers(tx, principal.Workspace.ID, record.ID, input); err != nil {
				return err
			}
			planned, err := planCloneAttachments(tx, principal.Workspace.ID, id, record.ID)
			if err != nil {
				return err
			}

			// 先记录尝试复制的目标，即使请求超时也能清理可能已落入 COS 的副本。
			attachments := make([]auditModels.Snapshot, 0, len(planned))
			for _, file := range planned {
				copies = append(copies, file)
				fileCopy := &copies[len(copies)-1]
				if err := storage.Copy(
					tx.Statement.Context, fileCopy.Source.ObjectKey, fileCopy.Target.ObjectKey,
				); err != nil {
					response.RecordError(tx.Statement.Context, err)
					return &response.BusinessError{
						Status: http.StatusServiceUnavailable, Message: "附件复制失败，请稍后重试",
					}
				}
				if err := tx.Create(&fileCopy.Target).Error; err != nil {
					return err
				}
				attachments = append(attachments, auditModels.Snapshot{
					"id": fileCopy.Target.ID, "source_attachment_id": fileCopy.Source.ID,
					"node_id": fileCopy.Target.NodeID, "filename": fileCopy.Target.Filename,
					"size": fileCopy.Target.Size, "state": fileCopy.Target.State,
					"revision": fileCopy.Target.Revision,
				})
			}

			// 在同一事务内读取数据库维护的时间，审计只保留业务元数据。
			result, err = load(tx, principal.Workspace.ID, record.ID)
			if err != nil {
				return err
			}
			audit.ObjectID = strconv.FormatUint(record.ID, 10)
			audit.After = snapshot(result)
			audit.After["attachments"] = attachments
			return nil
		},
	)

	// 数据库或审计失败时补偿清理；成功日志只在事务提交后写入。
	if err != nil {
		discardCloneAttachments(ctx, workspaceID, cloneID, copies)
		return result, err
	}
	attachmentIDs := make([]uint64, 0, len(copies))
	for _, fileCopy := range copies {
		attachmentIDs = append(attachmentIDs, fileCopy.Target.ID)
	}
	logger.Logger.WithContext(ctx).Infof(
		"CPC timeline template cloned; workspace: %s; source: %d; template: %d; attachments: %v",
		workspaceID, id, result.ID, attachmentIDs,
	)
	return result, nil
}

func cloneInput(source cpctimelineModels.Info) (cpctimelineModels.Input, error) {
	// 名称保持固定后缀，不截断原名或尝试其他编号。
	input := cpctimelineModels.Input{
		Name: source.Name + " (副本)", Description: source.Description,
		Definition: source.Definition, Transfers: source.Transfers,
	}
	if utf8.RuneCountInString(input.Name) > cpctimelineConstants.NameLength {
		return input, invalid("克隆后的模板名称超过 100 字符，请先缩短原模板名称")
	}
	return prepare(input)
}
