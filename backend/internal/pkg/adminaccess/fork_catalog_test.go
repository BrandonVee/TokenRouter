package adminaccess

import "testing"

// fork 管理动作必须逐条授权；新建相邻接口不能继承资源权限。
func TestForkPermissionCatalogBoundaries(t *testing.T) {
	for _, tc := range []struct{ permission, method, path string }{
		{"teams.read", "GET", "/teams/:id/members"},
		{"invoices.review", "POST", "/payment/invoices/:id/approve"},
		{"data_sharing.export", "POST", "/data-sharing/exports/:id/download-ticket"},
		{"storage.write", "PUT", "/settings/image-history-storage"},
		{"backups.download", "GET", "/backups/:id/download"},
	} {
		policy := Policy{GlobalPermissions: []string{tc.permission}}
		if !RouteAllowed(policy, tc.method, "/api/v1/admin"+tc.path) {
			t.Fatalf("授权动作不可访问: %s", tc.path)
		}
		if RouteAllowed(policy, tc.method, "/api/v1/admin"+tc.path+"/new-action") || RouteAllowed(Policy{}, tc.method, "/api/v1/admin"+tc.path) {
			t.Fatalf("未登记动作被隐式授权: %s", tc.path)
		}
	}
	if err := ValidatePolicy(Policy{Rules: []Rule{{Permissions: []string{"accounts.credentials"}, GroupIDs: []int64{1}, AccountTypes: []string{"cosy"}, Platforms: []string{"qoder"}}}}); err != nil {
		t.Fatal(err)
	}
}
