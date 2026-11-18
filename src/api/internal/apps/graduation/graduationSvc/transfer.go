package graduationSvc

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/graduation/graduationConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/graduation/graduationModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/spreadsheet"
)

var headers = []string{"学号", "毕业年份", "类型", "毕业去向"}

// Template 仅生成空模板，不预置业务记录。
func Template(ctx context.Context, workspaceID string) (graduationModels.FileData, error) {
	var result graduationModels.FileData
	err := accessUtils.Workspace(
		ctx, workspaceID, accessConstants.GraduationImport, event(workspaceID, 0),
		func(_ *gorm.DB, _ accessModels.Principal, audit *auditModels.Event) error {
			content, err := spreadsheet.Encode([][]string{headers})
			if err != nil {
				return err
			}
			result = graduationModels.FileData{
				Filename: "graduation-destinations.xlsx", ContentType: spreadsheet.ContentType,
				Content: content,
			}
			audit.ReturnedCount = 1
			return nil
		},
	)
	return result, err
}

// Import 按当前空间学号整批新增绑定，已绑定、重复学号或审计失败全部回滚。
func Import(
	ctx context.Context,
	workspaceID string,
	request graduationModels.ImportRequest,
) (graduationModels.ImportData, error) {
	var result graduationModels.ImportData
	err := accessUtils.Workspace(
		ctx, workspaceID, accessConstants.GraduationImport, event(workspaceID, 0),
		func(tx *gorm.DB, _ accessModels.Principal, audit *auditModels.Event) error {
			rows, err := spreadsheet.Read(request.Content, spreadsheet.Limits{
				MaxBytes:   config.Config.Student.MaxFileBytes,
				MaxRows:    config.Config.Student.MaxImportRows,
				MaxColumns: len(headers),
			})
			if err != nil {
				return err
			}
			if len(rows) < 2 {
				return invalid("文件中没有毕业去向记录")
			}
			if strings.Join(rows[0], "\x00") != strings.Join(headers, "\x00") {
				return invalid("文件表头与模板不一致")
			}
			beforeSnapshots, afterSnapshots := []auditModels.Snapshot{}, []auditModels.Snapshot{}
			seen := map[uint64]bool{}
			for index, row := range rows[1:] {
				before, after, err := importRow(tx, workspaceID, row, seen)
				if err != nil {
					var business *response.BusinessError
					if errors.As(err, &business) {
						return &response.BusinessError{
							Status:  business.Status,
							Message: fmt.Sprintf("第 %d 行：%s", index+2, business.Message),
						}
					}
					return err
				}
				beforeSnapshots = append(beforeSnapshots, before)
				afterSnapshots = append(afterSnapshots, after)
				result.Created++
			}
			audit.Before = auditModels.Snapshot{"students": beforeSnapshots}
			audit.After = auditModels.Snapshot{"students": afterSnapshots}
			audit.ReturnedCount = int64(result.Created)
			return nil
		},
	)
	return result, err
}

func importRow(
	tx *gorm.DB,
	workspaceID string,
	row []string,
	seen map[uint64]bool,
) (auditModels.Snapshot, auditModels.Snapshot, error) {
	number := strings.TrimSpace(row[0])
	if number == "" || !validText(number, graduationConstants.StudentNoLength) {
		return nil, nil, invalid("学号无效")
	}
	var student struct{ ID uint64 }
	if err := tx.Table("students").Select("id").
		Where("workspace_id = ? AND student_no = ?", workspaceID, number).
		Take(&student).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, invalid("学生不存在或不可访问")
		}
		return nil, nil, err
	}
	if seen[student.ID] {
		return nil, nil, invalid("同一学生不能重复导入")
	}
	seen[student.ID] = true
	before, err := detail(tx, workspaceID, student.ID)
	if err != nil {
		return nil, nil, err
	}
	if before.Type != "" || before.Destination != "" {
		return nil, nil, invalid("学生已绑定毕业去向，不能重复导入")
	}
	input, err := cleanInput(
		graduationModels.Input{GraduationYear: row[1], TypeLabel: row[2], Destination: row[3]},
	)
	if err != nil {
		return nil, nil, err
	}
	if input.GraduationYear == "" {
		input.GraduationYear = before.GraduationYear
	} else if before.GraduationYear != "" && before.GraduationYear != input.GraduationYear {
		return nil, nil, invalid("毕业年份与学生已保存的年份不一致，请在列表中编辑")
	}
	if err := update(tx, workspaceID, student.ID, before.Revision, before, input); err != nil {
		return nil, nil, err
	}
	after := before
	after.Input, after.Revision = input, before.Revision+1
	return snapshot(before), snapshot(after), nil
}
