package attachmentUtils

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/attachment/attachmentConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
)

// WithObjectPrefix 为桶根相对路径加上当前配置的对象目录。
func WithObjectPrefix(object string) string {
	if prefix := config.Config.COS.Prefix; prefix != "" {
		return prefix + "/" + object
	}
	return object
}

// RandomObjectName 生成独立随机目录，保留原始文件名且不覆盖同名文件。
func RandomObjectName(directory, filename string) (string, error) {
	value := make([]byte, attachmentConstants.DirectoryRandomBytes)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return WithObjectPrefix(directory) + "/" + hex.EncodeToString(value) + "/" + filename, nil
}
