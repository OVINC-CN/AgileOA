package monthlyworkSvc

import (
	"encoding/json"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/attachment/attachmentConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/attachment/attachmentModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/monthlywork/monthlyworkModels"
)

func recordInfos(
	tx *gorm.DB,
	workspaceID string,
	records []monthlyworkModels.Record,
	includeFiles bool,
) ([]monthlyworkModels.Info, error) {
	result := make([]monthlyworkModels.Info, 0, len(records))
	if len(records) == 0 {
		return result, nil
	}

	// 批量读取当前空间内记录的负责人、参与人及已完成附件，保留明确的租户条件。
	ids := make([]uint64, 0, len(records))
	ownerIDs := make(map[uint64]bool, len(records))
	for _, record := range records {
		ids = append(ids, record.ID)
		ownerIDs[record.OwnerID] = true
	}
	var participants []monthlyworkModels.Participant
	if err := tx.Where("workspace_id = ? AND monthly_work_id IN ?", workspaceID, ids).
		Order("user_id ASC").Find(&participants).Error; err != nil {
		return nil, err
	}
	for _, participant := range participants {
		ownerIDs[participant.UserID] = true
	}
	ownerValues := make([]uint64, 0, len(ownerIDs))
	for id := range ownerIDs {
		ownerValues = append(ownerValues, id)
	}
	var owners []struct {
		ID          uint64
		DisplayName string
		Username    string
	}
	if err := tx.Table("users").Select("id", "display_name", "username").
		Where("id IN ?", ownerValues).Find(&owners).Error; err != nil {
		return nil, err
	}
	names := make(map[uint64][2]string, len(owners))
	for _, owner := range owners {
		names[owner.ID] = [2]string{owner.DisplayName, owner.Username}
	}
	participantsByRecord := make(map[uint64][]monthlyworkModels.ParticipantInfo)
	for _, participant := range participants {
		name := names[participant.UserID]
		participantsByRecord[participant.MonthlyWorkID] = append(
			participantsByRecord[participant.MonthlyWorkID],
			monthlyworkModels.ParticipantInfo{
				ID: participant.UserID, Name: name[0], Username: name[1],
			},
		)
	}
	// 仅在确认读取权限后投影文件名和大小，附件编号始终按业务记录返回。
	columns := []string{"id", "monthly_work_id"}
	if includeFiles {
		columns = append(columns, "filename", "size")
	}
	var attachments []attachmentModels.MonthlyWorkAttachment
	if err := tx.Select(columns).
		Where("workspace_id = ? AND monthly_work_id IN ? AND state = ?", workspaceID, ids, attachmentConstants.Ready).
		Order("id ASC").Find(&attachments).Error; err != nil {
		return nil, err
	}
	idsByRecord := make(map[uint64][]uint64)
	filesByRecord := make(map[uint64][]monthlyworkModels.File)
	for _, file := range attachments {
		id := file.MonthlyWorkID
		idsByRecord[id] = append(idsByRecord[id], file.ID)
		if includeFiles {
			filesByRecord[id] = append(filesByRecord[id], monthlyworkModels.File{
				ID:       file.ID,
				Filename: file.Filename,
				Size:     file.Size,
			})
		}
	}
	for _, record := range records {
		links := []monthlyworkModels.Link{}
		if err := json.Unmarshal([]byte(record.Links), &links); err != nil {
			return nil, err
		}
		ids := idsByRecord[record.ID]
		if ids == nil {
			ids = []uint64{}
		}
		files := filesByRecord[record.ID]
		if files == nil {
			files = []monthlyworkModels.File{}
		}
		owner := names[record.OwnerID]
		people := participantsByRecord[record.ID]
		if people == nil {
			people = []monthlyworkModels.ParticipantInfo{}
		}
		participantIDs := make([]uint64, 0, len(people))
		for _, person := range people {
			participantIDs = append(participantIDs, person.ID)
		}
		result = append(result, monthlyworkModels.Info{
			Input: monthlyworkModels.Input{
				Date: record.Date, WorkItem: record.WorkItem, OwnerID: record.OwnerID,
				ParticipantIDs: participantIDs,
				Description:    record.Description, Links: links,
			},
			AttachmentIDs: ids, Attachments: files,
			Participants: people,
			OwnerName:    owner[0], OwnerUsername: owner[1],
			ID: record.ID, Revision: record.Revision,
			CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt,
		})
	}
	return result, nil
}
