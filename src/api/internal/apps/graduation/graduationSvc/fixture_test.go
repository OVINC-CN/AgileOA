package graduationSvc_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/user/userModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/db"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

func request(
	t *testing.T,
	server *echo.Echo,
	method, path string,
	body any,
	cookie *http.Cookie,
	status int,
) *httptest.ResponseRecorder {
	t.Helper()
	content, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequestWithContext(t.Context(), method, path, bytes.NewReader(content))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set("X-CSRF-Protection", "1")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, req)
	if recorder.Code != status {
		t.Fatalf(
			"%s %s：期望 %d，实际 %d：%s",
			method,
			path,
			status,
			recorder.Code,
			recorder.Body.String(),
		)
	}
	if status != 200 {
		var envelope response.Envelope[any]
		if err := json.Unmarshal(
			recorder.Body.Bytes(),
			&envelope,
		); err != nil ||
			envelope.Data != nil {
			t.Fatal("失败响应包含业务数据")
		}
	}
	return recorder
}

func decode[T any](t *testing.T, recorder *httptest.ResponseRecorder) T {
	t.Helper()
	var envelope response.Envelope[T]
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	return envelope.Data
}

func fixture(t *testing.T, username, token string, admin bool) (userModels.User, *http.Cookie) {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword(
		[]byte(strings.Repeat("0", 64)),
		config.Config.User.BcryptCost,
	)
	if err != nil {
		t.Fatal(err)
	}
	user := userModels.User{
		Username:       username,
		DisplayName:    "虚构验收账号",
		PasswordHash:   string(hash),
		IsAdmin:        admin,
		IsEnabled:      true,
		SessionVersion: 1,
	}
	connection := db.DB().WithContext(t.Context())
	if err := connection.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(token))
	session := userModels.Session{
		UserID:         user.ID,
		SessionVersion: 1,
		TokenHash:      hex.EncodeToString(digest[:]),
		ExpiresAt:      time.Now().Add(time.Hour).UTC(),
	}
	if err := connection.Create(&session).Error; err != nil {
		t.Fatal(err)
	}
	return user, &http.Cookie{
		Name:     config.Config.User.CookieName,
		Value:    token,
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
}

func grant(
	t *testing.T,
	server *echo.Echo,
	admin *http.Cookie,
	workspaceID string,
	userID uint64,
	actions []string,
) {
	t.Helper()
	path := fmt.Sprintf("/v1/system/access/workspaces/%s/users/%d/actions", workspaceID, userID)
	current := decode[accessModels.ActionsData](t, request(t, server, "GET", path, nil, admin, 200))
	request(
		t,
		server,
		"PUT",
		path,
		accessModels.UpdateActionsRequest{Actions: actions, Revision: current.Revision},
		admin,
		200,
	)
}
