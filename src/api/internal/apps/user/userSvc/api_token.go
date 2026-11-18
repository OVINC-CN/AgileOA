package userSvc

import (
	"context"
	"errors"
	"strconv"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/user/userConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/user/userModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/user/userUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/logger"
)

// APIToken 返回指定账号的完整凭据，读取审计提交后才允许响应。
func APIToken(ctx context.Context, id uint64) (userModels.APITokenData, error) {
	var result userModels.APITokenData
	event := apiTokenEvent(id)
	err := authenticated(ctx, accessConstants.UserAPITokenRead,
		func(tx *gorm.DB, _ userModels.User, _ userModels.Session) (auditModels.Event, error) {
			// 共享锁定账号，确保并行查看与重置、撤销遵循相同顺序。
			var user userModels.User
			if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).
				First(&user, id).Error; err != nil {
				return event, targetError(err)
			}
			var record userModels.APIToken
			err := tx.Where("user_id = ?", id).Take(&record).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return event, nil
			}
			if err != nil {
				return event, err
			}
			value, err := userUtils.DecryptAPIToken(id, record.Ciphertext)
			if err != nil {
				return event, err
			}
			result = apiTokenData(record, value)
			event.ReturnedIDs = []string{event.ObjectID}
			event.ReturnedCount = 1
			return event, nil
		}, event)
	return result, err
}

// ResetAPIToken 生成或替换唯一凭据，普通审计不包含原值、摘要或密文。
func ResetAPIToken(ctx context.Context, id uint64) (userModels.APITokenData, error) {
	var result userModels.APITokenData
	event := apiTokenEvent(id)
	err := authenticated(ctx, accessConstants.UserAPITokenWrite,
		func(tx *gorm.DB, _ userModels.User, _ userModels.Session) (auditModels.Event, error) {
			// 待改密或停用账号不能生成业务访问凭据。
			user, err := lockAPITokenUser(tx, id)
			if err != nil {
				return event, err
			}
			if !user.IsEnabled || (user.PasswordLoginEnabled && user.PasswordChangeRequired) {
				return event, conflict("请先启用账号并完成密码修改")
			}
			value := userConstants.APITokenPrefix + randomToken()
			ciphertext, err := userUtils.EncryptAPIToken(id, value)
			if err != nil {
				return event, err
			}
			var record userModels.APIToken
			err = tx.Where("user_id = ?", id).Take(&record).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return event, err
			}
			if err == nil {
				event.Before = apiTokenSnapshot(record)
				if err := tx.Model(&record).Updates(map[string]any{
					"token_hash": tokenHash(value), "ciphertext": ciphertext,
					"session_version": user.SessionVersion,
				}).Error; err != nil {
					return event, err
				}
			} else {
				record = userModels.APIToken{
					UserID: id, TokenHash: tokenHash(value), Ciphertext: ciphertext,
					SessionVersion: user.SessionVersion,
				}
				if err := tx.Create(&record).Error; err != nil {
					return event, err
				}
			}

			// 时间由数据库维护，在同一事务中读取后用于响应与审计。
			if err := tx.Where("user_id = ?", id).Take(&record).Error; err != nil {
				return event, err
			}
			event.After = apiTokenSnapshot(record)
			result = apiTokenData(record, value)
			return event, nil
		}, event)
	return result, err
}

// DeleteAPIToken 显式删除凭据，成功写入与审计共用事务。
func DeleteAPIToken(ctx context.Context, id uint64) error {
	event := apiTokenEvent(id)
	return authenticated(ctx, accessConstants.UserAPITokenDelete,
		func(tx *gorm.DB, _ userModels.User, _ userModels.Session) (auditModels.Event, error) {
			if _, err := lockAPITokenUser(tx, id); err != nil {
				return event, err
			}
			var record userModels.APIToken
			err := tx.Where("user_id = ?", id).Take(&record).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return event, nil
			}
			if err != nil {
				return event, err
			}
			event.Before = apiTokenSnapshot(record)
			return event, tx.Delete(&record).Error
		}, event)
}

func lockAPITokenUser(tx *gorm.DB, id uint64) (userModels.User, error) {
	var user userModels.User
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, id).Error
	return user, targetError(err)
}

func apiTokenData(record userModels.APIToken, value string) userModels.APITokenData {
	return userModels.APITokenData{
		Token: value, CreatedAt: &record.CreatedAt, UpdatedAt: &record.UpdatedAt,
	}
}

func apiTokenEvent(id uint64) auditModels.Event {
	return auditModels.Event{
		ObjectType: auditConstants.ObjectAPIToken,
		ObjectID:   strconv.FormatUint(id, 10),
		Conditions: auditModels.Snapshot{"user_id": id},
	}
}

func apiTokenSnapshot(record userModels.APIToken) auditModels.Snapshot {
	return auditModels.Snapshot{
		"user_id":    record.UserID,
		"created_at": record.CreatedAt,
		"updated_at": record.UpdatedAt,
	}
}

func removeAPIToken(tx *gorm.DB, operator userModels.User, action string, id uint64) (bool, error) {
	// 调用方已持有账号锁，删除与操作审计必须和账号变更一同提交。
	var record userModels.APIToken
	err := tx.Where("user_id = ?", id).Take(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	event := apiTokenEvent(id)
	event.Before = apiTokenSnapshot(record)
	if err := tx.Delete(&record).Error; err != nil {
		return false, err
	}
	return true, successful(tx, operator, action, event)
}

func logAPITokenRemoval(ctx context.Context, id uint64, removed bool) {
	// 仅在事务提交且实际删除后记录未直接返回给用户的凭据变更。
	if removed {
		logger.Logger.WithContext(ctx).Infof("API token deleted; user: %d", id)
	}
}
