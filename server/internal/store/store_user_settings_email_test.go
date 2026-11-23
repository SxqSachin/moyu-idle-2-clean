package store

import (
	"strings"
	"testing"
)

func TestUpdateUnverifiedEmailSQLClearsPreviousVerification(t *testing.T) {
	for _, clause := range []string{
		"SET email = $2",
		"email_verified = false",
		"email_verified_at = NULL",
	} {
		if !strings.Contains(updateUnverifiedEmailSQL, clause) {
			t.Fatalf("email update SQL is missing %q", clause)
		}
	}
}
