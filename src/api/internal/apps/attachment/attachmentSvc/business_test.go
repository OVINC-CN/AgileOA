package attachmentSvc

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/attachment/attachmentConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/attachment/attachmentModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
)

func TestWorkObjectUsesRecordDateAndIndependentRandomDirectory(t *testing.T) {
	pattern := regexp.MustCompile(
		`^workspaces/w7m2k9p4s6n8c3x5/work/2026/02/03/[0-9a-f]{10}/虚构材料 #1\?\.pdf$`,
	)
	first, err := workObjectName(
		"workspaces/w7m2k9p4s6n8c3x5",
		attachmentConstants.CategoryMonthlyWork,
		"2026-02-03",
		"虚构材料 #1?.pdf",
	)
	if err != nil || !pattern.MatchString(first) {
		t.Fatalf("日期目录、十位随机目录或原始文件名无效：%s，%v", first, err)
	}
	second, err := workObjectName(
		"workspaces/w7m2k9p4s6n8c3x5",
		attachmentConstants.CategoryMonthlyWork,
		"2026-02-03",
		"虚构材料 #1?.pdf",
	)
	if err != nil || first == second || !pattern.MatchString(second) {
		t.Fatal("同日同名文件复用了随机路径")
	}
	cpc, err := workObjectName(
		"workspaces/w7m2k9p4s6n8c3x5",
		attachmentConstants.CategoryCPCMonthlyWork,
		"2026-02-03",
		"虚构材料 #1?.pdf",
	)
	cpcPattern := regexp.MustCompile(
		`^workspaces/w7m2k9p4s6n8c3x5/cpc-work/2026/02/03/[0-9a-f]{10}/虚构材料 #1\?\.pdf$`,
	)
	if err != nil || !cpcPattern.MatchString(cpc) {
		t.Fatalf("党建附件未使用独立的 cpc-work 目录：%s，%v", cpc, err)
	}
}

func TestTimelineNodeAttachmentsKeepTheirOwnPolicyAndNodeIdentity(t *testing.T) {
	policy, err := policyFor(attachmentConstants.CategoryCPCTimelineNode)
	if err != nil {
		t.Fatal(err)
	}
	if policy.table != "cpc_timeline_node_attachments" ||
		policy.readAction != accessConstants.CPCTimelineAttachmentRead ||
		policy.uploadAction != accessConstants.CPCTimelineAttachmentUpload ||
		policy.deleteAction != accessConstants.CPCTimelineAttachmentDelete ||
		policy.maxAttachments != config.Config.CPCTimeline.MaxAttachments {
		t.Fatal("节点附件混用了其他业务的表、权限或数量限制")
	}
	record := attachmentModels.OwnedRecord{
		Record: attachmentModels.Record{
			ID:        11,
			ObjectKey: "private-object",
			Filename:  "虚构节点材料.pdf",
		},
		OwnerID: 7,
		NodeID:  "node_first",
	}
	metadata := info(attachmentConstants.CategoryCPCTimelineNode, record)
	if metadata.TimelineTemplateID != 7 || metadata.NodeID != "node_first" ||
		metadata.ProjectID != 0 || metadata.MonthlyWorkID != 0 {
		t.Fatal("节点附件响应丢失节点引用或被标记为其他业务附件")
	}
	audit := snapshot(
		attachmentConstants.CategoryCPCTimelineNode,
		record.OwnerID,
		record.NodeID,
		record.Record,
	)
	if audit["node_id"] != "node_first" || audit["timeline_template_id"] != uint64(7) {
		t.Fatal("节点附件审计未保留完整归属")
	}
	encoded, err := json.Marshal([]any{metadata, audit})
	if err != nil || strings.Contains(string(encoded), "private-object") {
		t.Fatal("节点附件响应或审计包含了私有对象键")
	}
}

func TestTimelineNodeRejectsInvalidIdentityBeforeDatabaseAccess(t *testing.T) {
	for _, nodeID := range []string{"", "../node", "node/first", "NODE", strings.Repeat("a", 65)} {
		if err := requireTimelineNode(nil, "w7m2k9p4s6n8c3x5", 7, nodeID); err == nil {
			t.Fatal("未保存或非法节点标识被接受")
		}
	}
	if err := requireTimelineNode(nil, "w7m2k9p4s6n8c3x5", 0, "node_first"); err == nil {
		t.Fatal("缺少模板编号时仍访问了节点")
	}
}

func TestTimelineNodeObjectKeepsStableIdentityAndRandomizesSameFilename(t *testing.T) {
	prefix := "workspaces/w7m2k9p4s6n8c3x5/cpc-timeline-templates/7/nodes/node_first"
	first, err := randomObjectName(prefix, "虚构节点材料.pdf")
	if err != nil {
		t.Fatal(err)
	}
	second, err := randomObjectName(prefix, "虚构节点材料.pdf")
	pattern := regexp.MustCompile("^" + regexp.QuoteMeta(prefix) + `/[0-9a-f]{10}/虚构节点材料\.pdf$`)
	if err != nil || first == second || !pattern.MatchString(first) ||
		!pattern.MatchString(second) {
		t.Fatal("节点对象键丢失稳定标识、原始文件名，或同名上传覆盖了原对象")
	}
}
