package userUtils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strconv"

	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
)

// EncryptAPIToken 以账号编号作为附加认证数据，防止数据库中的密文被跨账号替换。
func EncryptAPIToken(userID uint64, token string) ([]byte, error) {
	block, err := apiTokenCipher()
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, block.NonceSize())
	_, _ = rand.Read(nonce)
	return block.Seal(nonce, nonce, []byte(token), apiTokenAAD(userID)), nil
}

// DecryptAPIToken 仅供管理员再次查看时使用，认证请求只比较摘要。
func DecryptAPIToken(userID uint64, ciphertext []byte) (string, error) {
	block, err := apiTokenCipher()
	if err != nil {
		return "", err
	}
	if len(ciphertext) < block.NonceSize()+block.Overhead() {
		return "", errors.New("invalid API token ciphertext")
	}
	nonce := ciphertext[:block.NonceSize()]
	value, err := block.Open(nil, nonce, ciphertext[block.NonceSize():], apiTokenAAD(userID))
	if err != nil {
		return "", errors.New("API token decryption failed")
	}
	return string(value), nil
}

func apiTokenCipher() (cipher.AEAD, error) {
	key, err := base64.StdEncoding.DecodeString(config.Config.User.APITokenKey)
	if err != nil || len(key) != 32 {
		return nil, errors.New("API token encryption key is unavailable")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func apiTokenAAD(userID uint64) []byte {
	return []byte("user_api_tokens:" + strconv.FormatUint(userID, 10))
}
