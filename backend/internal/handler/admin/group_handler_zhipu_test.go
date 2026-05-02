package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGroupHandler_AllowsZhipuPlatform(t *testing.T) {
	router, _ := setupAdminRouter()

	cases := []struct {
		name   string
		method string
		path   string
		body   map[string]any
	}{
		{
			name:   "create",
			method: http.MethodPost,
			path:   "/api/v1/admin/groups",
			body: map[string]any{
				"name":     "zhipu",
				"platform": "zhipu",
			},
		},
		{
			name:   "update",
			method: http.MethodPut,
			path:   "/api/v1/admin/groups/2",
			body: map[string]any{
				"platform": "zhipu",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body, err := json.Marshal(tc.body)
			require.NoError(t, err)

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, tc.path, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(rec, req)

			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		})
	}
}
