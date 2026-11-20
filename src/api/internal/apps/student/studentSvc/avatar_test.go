package studentSvc

import (
	"strings"
	"testing"

	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
)

func TestAvatarKeyBelongsToCurrentWorkspaceAndAvatarDirectory(t *testing.T) {
	original := config.Config.COS.Prefix
	config.Config.COS.Prefix = "apps/agile"
	t.Cleanup(func() { config.Config.COS.Prefix = original })

	workspaceID := "w7m2k9p4s6n8c3x5"
	prefix := avatarPrefix(workspaceID)
	for _, key := range []string{
		prefix + "新生照片/虚构学生 1.JPG",
		"apps/agile/" + prefix + "头像.webp",
	} {
		if !validAvatarKey(workspaceID, key) {
			t.Fatalf("合法的原始 COS 对象键被拒绝：%q", key)
		}
	}
	for _, key := range []string{
		"https://cdn.example.com/头像.png?eo-sign=fixture",
		avatarPrefix("r4v8b2m6k9d3p5s7") + "头像.png",
		"apps/agile/" + avatarPrefix("r4v8b2m6k9d3p5s7") + "头像.png",
		"apps/other/" + prefix + "头像.png",
		"workspaces/" + workspaceID + "/projects/1/头像.png",
		prefix + "../头像.png",
		prefix + "%2e%2e/头像.png",
		prefix + "文件/../../头像.png",
		prefix + "头像.png?token=fixture",
		prefix + "头像.svg",
		prefix + strings.Repeat("图", 342) + ".png",
	} {
		if validAvatarKey(workspaceID, key) {
			t.Fatalf("越界或非法对象键被接受：%q", key)
		}
	}
}
