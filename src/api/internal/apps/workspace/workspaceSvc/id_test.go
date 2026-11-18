package workspaceSvc

import (
	"testing"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/workspace/workspaceUtils"
)

func TestRandomWorkspaceIDsMatchThePublicContract(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for range 256 {
		id, err := newID()
		if err != nil || !workspaceUtils.ValidID(id) || seen[id] {
			t.Fatal("工作空间随机编号无效或发生重复")
		}
		seen[id] = true
	}
	for _, id := range []string{"", "1234567890123456", "abcdefghijklmnop", "W7m2k9p4s6n8c3x5", "w7m2k9p4s6n8c3x5-"} {
		if workspaceUtils.ValidID(id) {
			t.Fatalf("非法工作空间编号被接受：%s", id)
		}
	}
}
