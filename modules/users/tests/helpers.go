//go:build integration
// +build integration

package users_test

import (
	"testing"

	"josex/web/modules/core/testhelpers"
)

// accountLocale answers the language an account reads the app in; no admin endpoint answers it.
func accountLocale(t *testing.T, helper *testhelpers.ApiTestHelper, userID string) string {
	t.Helper()
	var locale string
	if err := helper.DB().QueryRow(t.Context(), `SELECT locale FROM auth.users WHERE id = $1`, userID).Scan(&locale); err != nil {
		t.Fatalf("read locale: %v", err)
	}
	return locale
}
