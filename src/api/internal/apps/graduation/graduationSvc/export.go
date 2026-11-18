package graduationSvc

import (
	"context"
	"encoding/base64"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/graduation/graduationConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/graduation/graduationModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/spreadsheet"
)

// Export 按列表筛选与排序导出全部毕业去向，在空间锁内复核独立权限并保存审计。
func Export(
	ctx context.Context,
	workspaceID string,
	query graduationModels.Query,
) (graduationModels.FileData, error) {
	// 分页不影响导出范围，沿用学生模块的传输上限。
	audit := queryEvent(workspaceID, query)
	delete(audit.Conditions, "page")
	delete(audit.Conditions, "page_size")
	var result graduationModels.FileData
	err := accessUtils.Workspace(
		ctx, workspaceID, accessConstants.GraduationExport, audit,
		func(tx *gorm.DB, principal accessModels.Principal, audit *auditModels.Event) error {
			// 固定字段与列表复用，不读取学生敏感资料。
			scope, err := filteredScope(tx, principal.Workspace.ID, query)
			if err != nil {
				return err
			}
			var items []graduationModels.Info
			if err := scope.Select(columns()).Order(graduationConstants.ListOrder).
				Limit(config.Config.Student.MaxExportRows + 1).Scan(&items).Error; err != nil {
				return err
			}
			if len(items) > config.Config.Student.MaxExportRows {
				return invalid("导出行数超过限制，请缩小筛选范围")
			}

			// 所有表头和数据按文本保存，待就业沿用列表的显示名称。
			records := make([][]string, 1, len(items)+1)
			records[0] = []string{
				"学号", "姓名", "学历", "专业", "年级", "毕业年份", "类型代码", "类型", "毕业去向",
			}
			for _, item := range items {
				label := item.TypeLabel
				if item.Type == "" {
					label = graduationConstants.PendingLabel
				}
				records = append(records, []string{
					item.StudentNo, item.StudentName, item.EducationLevel, item.Major, item.Grade,
					item.GraduationYear, item.Type, label, item.Destination,
				})
				audit.ReturnedIDs = append(
					audit.ReturnedIDs,
					strconv.FormatUint(item.StudentID, 10),
				)
			}
			content, err := spreadsheet.Encode(records)
			if err != nil {
				return err
			}
			fileSize := base64.RawStdEncoding.DecodedLen(len(strings.TrimRight(content, "=")))
			if fileSize > config.Config.Student.MaxFileBytes {
				return invalid("导出文件超过限制，请缩小筛选范围")
			}

			// 审计与读取共用事务，提交成功后才允许返回文件。
			audit.ReturnedCount = int64(len(items))
			result = graduationModels.FileData{
				Filename: "毕业去向.xlsx", ContentType: spreadsheet.ContentType, Content: content,
			}
			return nil
		},
	)
	return result, err
}
