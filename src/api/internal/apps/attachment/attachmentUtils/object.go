package attachmentUtils

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/attachment/attachmentConstants"
)

// RandomObjectName 生成独立随机目录，保留原始文件名且不覆盖同名文件。
func RandomObjectName(prefix, filename string) (string, error) {
	value := make([]byte, attachmentConstants.DirectoryRandomBytes)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return prefix + "/" + hex.EncodeToString(value) + "/" + filename, nil
}
