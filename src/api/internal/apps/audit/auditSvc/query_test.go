package auditSvc

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/pagination"
)

func TestFirstPageWithTotalReturnsRowsFromDatabaseJoin(t *testing.T) {
	for _, empty := range []bool{false, true} {
		connection, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = connection.Close() })
		database, err := gorm.Open(mysql.New(mysql.Config{
			Conn: connection, SkipInitializeWithVersion: true,
		}), &gorm.Config{DisableAutomaticPing: true})
		if err != nil {
			t.Fatal(err)
		}
		query := auditModels.Query{
			Query:          pagination.Query{Page: 1, PageSize: 5},
			From:           time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC),
			Until:          time.Date(2026, 11, 17, 0, 0, 0, 0, time.UTC),
			AccessSourceIP: "2001:db8::1",
		}

		// 复现 total=8217、第一页每页 5 条；日期以 MySQL 驱动的 time.Time 返回。
		mock.ExpectQuery("SELECT count\\(\\*\\) FROM `audit_events` .*access_source_ip = \\?").
			WithArgs("2026-11-03", "2026-11-16", query.From.UnixMilli(), query.Until.UnixMilli(),
				query.AccessSourceIP).
			WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(8217))
		rows := sqlmock.NewRows([]string{
			"id", "event_date", "event_id", "user_id", "snapshot_user_info",
		})
		if !empty {
			for id := 9005; id > 9000; id-- {
				rows.AddRow(id, query.From, "12f8f7ba823d4a8eac12b5db99db71b4", 7,
					`{"id":7,"username":"fixture"}`)
			}
		}
		mock.ExpectQuery("FROM \\(SELECT id,event_date FROM `audit_events` .*access_source_ip = \\? "+
			"ORDER BY id DESC LIMIT \\?\\) AS audit_page JOIN audit_events AS entries "+
			"ON entries.id = audit_page.id AND entries.event_date = audit_page.event_date").
			WithArgs("2026-11-03", "2026-11-16", query.From.UnixMilli(), query.Until.UnixMilli(),
				query.AccessSourceIP, 5, "2026-11-03", "2026-11-16").
			WillReturnRows(rows)
		result := auditModels.ListData{Items: []auditModels.Summary{}}
		err = readPage(database.WithContext(t.Context()), query, &result)
		if empty {
			if !errors.Is(err, ErrUnavailable) {
				t.Fatal("有总数的第一页静默返回了空结果")
			}
		} else if err != nil || result.Total != 8217 || len(result.Items) != 5 ||
			result.Items[0].ID != 9005 || result.Items[4].ID != 9001 ||
			result.Items[0].EventDate != "2026-11-03" ||
			!json.Valid(result.Items[0].SnapshotUserInfo) {
			t.Fatalf("第一页的总数、顺序或内容丢失：%+v，%v", result, err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	}
}
