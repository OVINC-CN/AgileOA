package projectConstants

import "time"

const (
	AutoSubmitDrafts     = "project:auto_submit_drafts"
	AutoSubmitDelay      = time.Minute
	RankApplications     = "project:rank_applications"
	NotifyRankingFailure = "project:notify_ranking_failure"
)

// RankingDimensions 固定排名分组目录，不包含学历或任意学生字段。
var RankingDimensions = []string{"grade", "major", "class_name"}
