package service

import (
	"errors"
	"testing"
)

func TestAdminClientErrorClassification(t *testing.T) {
	for _, err := range []error{ErrPostNotFound, ErrAdminReviewInvalid, ErrAdminSelfBan, ErrAdminUserNotFound, ErrAdminProtectedUser, ErrAdminStatusInvalid} {
		if !IsAdminClientError(err) {
			t.Fatalf("%v should be safe for client mapping", err)
		}
	}
	if IsAdminClientError(errors.New("database unavailable")) {
		t.Fatal("database errors must be mapped to a generic internal response")
	}
}
