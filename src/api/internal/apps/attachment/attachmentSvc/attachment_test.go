package attachmentSvc

import "testing"

func TestFilenamePreservesChineseAndRejectsPaths(t *testing.T) {
	if !validFilename("虚构项目说明.pdf") {
		t.Fatal("中文原始文件名被拒绝")
	}
	for _, value := range []string{"", "   ", ".", "..", "../说明.pdf", `C:\说明.pdf`, "说明\n.pdf"} {
		if validFilename(value) {
			t.Fatalf("非法文件名被接受：%q", value)
		}
	}
}
