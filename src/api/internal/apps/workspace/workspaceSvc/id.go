package workspaceSvc

import (
	"crypto/rand"
	"math/big"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/workspace/workspaceConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/workspace/workspaceUtils"
)

func newID() (string, error) {
	// 通过密码学随机源均匀选择字符，拒绝取模偏差及纯字母或纯数字结果。
	limit := big.NewInt(int64(len(workspaceConstants.IDAlphabet)))
	for {
		value := make([]byte, workspaceConstants.IDLength)
		for index := range value {
			number, err := rand.Int(rand.Reader, limit)
			if err != nil {
				return "", err
			}
			value[index] = workspaceConstants.IDAlphabet[number.Int64()]
		}
		id := string(value)
		if workspaceUtils.ValidID(id) {
			return id, nil
		}
	}
}
