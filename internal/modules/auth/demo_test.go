package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
)

func TestDemoLoginDevOnly(t *testing.T) {
	svc := NewService(&fakeAccounts{customer: Account{ID: uuid.New(), Status: "active"}}, &fakeCodes{values: map[string]string{}}, allowAll{}, &fakeSessions{}, piiKey(), hashKey(), jwtKey(), false)
	_, err := svc.DemoLogin(context.Background(), "03001111001", RoleCustomer)
	if !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestDemoLoginIssuesSession(t *testing.T) {
	id := uuid.MustParse("44444444-4444-4444-8444-444444444401")
	svc := NewService(&fakeAccounts{customer: Account{ID: id, Status: "active"}}, &fakeCodes{values: map[string]string{}}, allowAll{}, &fakeSessions{}, piiKey(), hashKey(), jwtKey(), true)
	session, err := svc.DemoLogin(context.Background(), "03001111001", RoleCustomer)
	if err != nil {
		t.Fatal(err)
	}
	if session.AccountID != id || session.AccessToken == "" || session.Role != RoleCustomer {
		t.Fatalf("session = %+v", session)
	}
}

func TestListDemoAccounts(t *testing.T) {
	svc := NewService(&fakeAccounts{}, &fakeCodes{values: map[string]string{}}, allowAll{}, &fakeSessions{}, piiKey(), hashKey(), jwtKey(), true)
	list, err := svc.ListDemoAccounts()
	if err != nil || len(list) < 5 {
		t.Fatalf("list = %d err = %v", len(list), err)
	}
}
