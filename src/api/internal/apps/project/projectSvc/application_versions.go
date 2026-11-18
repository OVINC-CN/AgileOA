package projectSvc

import (
	"cmp"
	"context"
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/project/projectModels"
)

// ApplicationVersions 在原申报权限范围内从成功审计还原历史正文，读取审计提交后返回。
func ApplicationVersions(
	ctx context.Context,
	workspaceID, mode string,
	id uint64,
) (projectModels.ApplicationVersions, error) {
	result := projectModels.ApplicationVersions{Items: []projectModels.ApplicationVersion{}}
	err := applicationAccess(
		ctx,
		workspaceID,
		applicationAction(isStudentActor(ctx), mode, ""),
		event(workspaceID, "project_application", id),
		func(tx *gorm.DB, actor applicationActor, audit *auditModels.Event) error {
			// 先复核当前单据可见范围，历史对象编号不能替代租户及节点权限。
			var current struct {
				projectModels.Application
				SelfApplyDeadline *time.Time
			}
			if err := applicationScope(tx, workspaceID, mode, actor).
				Select("a.id, a.revision, p.self_apply_deadline").
				Where("a.id = ?", id).Take(&current).Error; err != nil {
				return lookupError(err)
			}

			// 原审计和业务读取共用事务与追踪，不查询敏感密文或存储地址。
			var err error
			result, err = applicationVersions(
				tx, workspaceID, current.Application, current.SelfApplyDeadline,
			)
			if err != nil {
				return err
			}
			audit.ReturnedCount = int64(len(result.Items))
			return nil
		},
	)
	return result, err
}

func applicationVersions(
	tx *gorm.DB,
	workspaceID string,
	current projectModels.Application,
	deadline *time.Time,
) (projectModels.ApplicationVersions, error) {
	result := projectModels.ApplicationVersions{Items: []projectModels.ApplicationVersion{}}
	// 仅按独立标量筛选成功的申报写入事件，读取行为不会产生新的内容版本。
	var records []auditModels.Record
	if err := tx.Model(&auditModels.Record{}).
		Select("id, instance_data, instance_origin_data, snapshot_user_info").
		Where("workspace_id = ? AND resource_type_id = ? AND instance_id = ? AND result_code = ?",
			workspaceID, "project_application", strconv.FormatUint(current.ID, 10), 0).
		Where("action_id IN ?", applicationVersionActions()).
		Order("id DESC").Find(&records).Error; err != nil {
		return result, err
	}

	// 同时读取前后快照，补全自动提交等后台变更之后首次留下的历史正文。
	seen := map[uint64]bool{}
	modifiers := map[uint64]string{}
	for _, record := range records {
		for index, content := range []json.RawMessage{record.InstanceData, record.InstanceOriginData} {
			version, err := applicationVersion(content, current)
			if err != nil {
				return result, err
			}
			if version == nil {
				continue
			}
			// 只有写入后的快照可以确认修改人，前快照不能借用下一次操作的身份。
			if index == 0 {
				if _, found := modifiers[version.Revision]; !found {
					name, err := applicationVersionModifier(record.SnapshotUserInfo)
					if err != nil {
						return result, err
					}
					modifiers[version.Revision] = name
				}
			}
			if seen[version.Revision] {
				continue
			}
			seen[version.Revision] = true
			result.Items = append(result.Items, *version)
		}
	}
	for index := range result.Items {
		result.Items[index].ActorName = modifiers[result.Items[index].Revision]
	}
	slices.SortFunc(result.Items, func(a, b projectModels.ApplicationVersion) int {
		return cmp.Compare(b.Revision, a.Revision)
	})
	result.Items = selectApplicationVersions(result.Items, deadline)
	return result, nil
}

func selectApplicationVersions(
	versions []projectModels.ApplicationVersion,
	deadline *time.Time,
) []projectModels.ApplicationVersion {
	if len(versions) == 0 {
		return versions
	}
	// 最近历史版与截止两侧的边界版本合并，响应最多三版，保持版本倒序。
	selected := map[uint64]bool{versions[0].Revision: true}
	if deadline != nil {
		var before, after *projectModels.ApplicationVersion
		for index := range versions {
			version := &versions[index]
			if version.UpdatedAt.Before(*deadline) {
				if before == nil {
					before = version
				}
			} else {
				after = version
			}
		}
		for _, version := range []*projectModels.ApplicationVersion{before, after} {
			if version != nil {
				selected[version.Revision] = true
			}
		}
	}
	result := make([]projectModels.ApplicationVersion, 0, len(selected))
	for _, version := range versions {
		if selected[version.Revision] {
			result = append(result, version)
		}
	}
	return result
}

func applicationVersionModifier(content json.RawMessage) (string, error) {
	if len(content) == 0 {
		return "", nil
	}
	var actor auditModels.Actor
	if err := json.Unmarshal(content, &actor); err != nil {
		return "", err
	}
	name, identifier := strings.TrimSpace(actor.DisplayName), strings.TrimSpace(actor.Username)
	if name == "" {
		return identifier, nil
	}
	if identifier == "" {
		return name, nil
	}
	return name + "(" + identifier + ")", nil
}

func applicationVersion(
	content json.RawMessage,
	current projectModels.Application,
) (*projectModels.ApplicationVersion, error) {
	if len(content) == 0 {
		return nil, nil
	}
	// 流程专用审计没有正文，不能把字段缺失还原成清空全部内容。
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(content, &fields); err != nil {
		return nil, err
	}
	for _, name := range []string{"statement", "rank", "score", "details"} {
		if _, exists := fields[name]; !exists {
			return nil, nil
		}
	}
	var snapshot struct {
		ID uint64 `json:"id"`
		projectModels.ApplicationVersion
	}
	if err := json.Unmarshal(content, &snapshot); err != nil {
		return nil, err
	}
	if snapshot.ID != current.ID || snapshot.Revision == 0 ||
		snapshot.Revision >= current.Revision {
		return nil, nil
	}
	// 使用固定响应模型投影快照，历史流程、原始身份快照及私有对象信息不随正文返回。
	if snapshot.Details == nil {
		snapshot.Details = []projectModels.ApplicationVersionDetail{}
	}
	for index := range snapshot.Details {
		if snapshot.Details[index].Attachments == nil {
			snapshot.Details[index].Attachments = []projectModels.ApplicationVersionAttachment{}
		}
	}
	return &snapshot.ApplicationVersion, nil
}

func applicationVersionActions() []string {
	return []string{
		accessConstants.ProjectUpdate,
		accessConstants.ProjectReviewUpdate,
		accessConstants.ProjectReviewApprove,
		accessConstants.ProjectReviewReject,
		accessConstants.ProjectApplicationAttachmentUpload,
		accessConstants.ProjectApplicationAttachmentDelete,
		accessConstants.StudentApplicationCreate,
		accessConstants.StudentApplicationUpdate,
		accessConstants.StudentApplicationSubmit,
		accessConstants.StudentApplicationAttachmentUpload,
		accessConstants.StudentApplicationAttachmentDelete,
		accessConstants.StudentReviewUpdate,
		accessConstants.StudentReviewApprove,
		accessConstants.StudentReviewReject,
		accessConstants.StudentReviewAttachmentUpload,
		accessConstants.StudentReviewAttachmentDelete,
	}
}
