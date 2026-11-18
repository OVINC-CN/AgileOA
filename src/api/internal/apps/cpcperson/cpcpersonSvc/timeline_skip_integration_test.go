package cpcpersonSvc_test

import (
	"fmt"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpcperson/cpcpersonModels"
)

func assertTimelineNodeSkipping(
	t *testing.T, server *echo.Echo, actor, manager fixtureActor,
	workspaceID, otherWorkspace string, otherPersonID uint64, data cpcpersonModels.TimelineData,
) cpcpersonModels.TimelineData {
	t.Helper()
	path := fmt.Sprintf("/v1/workspaces/%s/cpc-people/%d/timeline", workspaceID, data.PersonID)
	node := data.Nodes[1]
	request := cpcpersonModels.SkipTimelineNodeRequest{
		Revision: node.Revision, TemplateRevision: data.TemplateRevision,
	}
	personRequest[any](t, server, actor, "POST", fmt.Sprintf(
		"/v1/workspaces/%s/cpc-people/%d/timeline/nodes/%s/skip",
		workspaceID, otherPersonID, node.ID,
	), request, 404)
	personRequest[any](t, server, manager, "POST", fmt.Sprintf(
		"/v1/workspaces/%s/cpc-people/%d/timeline/nodes/%s/skip",
		otherWorkspace, data.PersonID, node.ID,
	), request, 404)
	stale := request
	stale.Revision++
	personRequest[any](t, server, actor, "POST", path+"/nodes/"+node.ID+"/skip", stale, 409)
	stale = request
	stale.TemplateRevision++
	personRequest[any](t, server, actor, "POST", path+"/nodes/"+node.ID+"/skip", stale, 409)
	personRequest[any](t, server, actor, "POST", path+"/nodes/node_missing/skip", request, 400)

	// 跳过保留已有填写值，排除自身异常，并在名单中单独统计状态。
	data = personRequest[cpcpersonModels.TimelineData](
		t, server, actor, "POST", path+"/nodes/"+node.ID+"/skip", request, 200,
	)
	if data.Nodes[1].Status != "skipped" || data.Nodes[1].StatusLabel != "已跳过" ||
		data.Nodes[1].Values[node.TimeFieldID] != node.Values[node.TimeFieldID] ||
		data.Nodes[1].Revision != node.Revision+1 || data.Status != "normal" ||
		len(data.Nodes[1].Issues) != 0 || len(data.Nodes[1].Checks) != 0 {
		t.Fatal("跳过未保留已保存数据、更新状态及版本或清除自身异常")
	}
	progress := personRequest[cpcpersonModels.Info](t, server, actor, "GET", fmt.Sprintf(
		"/v1/workspaces/%s/cpc-people/%d", workspaceID, data.PersonID,
	), nil, 200)
	if progress.TimelineSkipped != 1 || progress.TimelineCompleted != 1 ||
		progress.TimelinePending != 0 {
		t.Fatal("名单未区分已跳过、已完成和待处理数量")
	}
	personRequest[any](t, server, actor, "POST", path+"/nodes/"+node.ID+"/skip", request, 409)
	request.Revision = data.Nodes[1].Revision
	repeated := personRequest[cpcpersonModels.TimelineData](
		t, server, actor, "POST", path+"/nodes/"+node.ID+"/skip", request, 200,
	)
	if repeated.Nodes[1].Revision != request.Revision {
		t.Fatal("重复跳过不应修改版本")
	}
	data = personRequest[cpcpersonModels.TimelineData](
		t, server, actor, "PUT", path+"/nodes/"+node.ID,
		cpcpersonModels.TimelineNodeRequest{
			Values: node.Values, Revision: repeated.Nodes[1].Revision,
			TemplateRevision: data.TemplateRevision,
		}, 200,
	)
	if data.Nodes[1].Status != "completed" || data.Status != "abnormal" {
		t.Fatal("保存并完成未恢复已跳过节点及其时间校验")
	}

	// 跳过参考节点同样停用依赖校验，恢复保存后重新显示异常。
	reference := data.Nodes[0]
	data = personRequest[cpcpersonModels.TimelineData](
		t, server, actor, "POST", path+"/nodes/"+reference.ID+"/skip",
		cpcpersonModels.SkipTimelineNodeRequest{
			Revision: reference.Revision, TemplateRevision: data.TemplateRevision,
		}, 200,
	)
	if data.Status != "normal" || len(data.Nodes[1].Checks) != 0 ||
		data.Nodes[0].Values[reference.TimeFieldID] != reference.Values[reference.TimeFieldID] {
		t.Fatal("跳过参考节点未停用依赖校验，或错误清空了参考值")
	}
	data = personRequest[cpcpersonModels.TimelineData](
		t, server, actor, "POST", path+"/nodes/"+reference.ID+"/reset",
		cpcpersonModels.ResetTimelineNodeRequest{
			Revision: data.Nodes[0].Revision, TemplateRevision: data.TemplateRevision,
		}, 200,
	)
	if data.Nodes[0].Status != "pending" || len(data.Nodes[0].Values) != 0 {
		t.Fatal("重置未将已跳过节点恢复为待处理")
	}
	return personRequest[cpcpersonModels.TimelineData](
		t, server, actor, "PUT", path+"/nodes/"+reference.ID,
		cpcpersonModels.TimelineNodeRequest{
			Values: reference.Values, Revision: data.Nodes[0].Revision,
			TemplateRevision: data.TemplateRevision,
		}, 200,
	)
}
