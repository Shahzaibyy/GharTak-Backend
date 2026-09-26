package merchants

import (
	"errors"
	"testing"

	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
	"github.com/yourusername/ghartak-backend/internal/platform/domain"
)

func TestAllowedDecision(t *testing.T) {
	tests := []struct {
		name    string
		status  domain.VerificationStatus
		wantErr error
	}{
		{name: "approve", status: domain.VerificationApproved},
		{name: "reject", status: domain.VerificationRejected},
		{name: "suspend", status: domain.VerificationSuspended, wantErr: apperror.ErrInvalidInput},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := allowedDecision(tt.status)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}
