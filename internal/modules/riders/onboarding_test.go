package riders

import (
	"testing"
	"time"

	"github.com/yourusername/ghartak-backend/internal/platform/domain"
)

func TestTimelineSubmittedPending(t *testing.T) {
	now := time.Now()
	status := timeline(Profile{
		VerificationStatus:  string(domain.VerificationPending),
		OnboardingStep:      string(domain.OnboardingSubmitted),
		ApplicationReceived: &now,
		OrientationStatus:   string(domain.OrientationNone),
	})
	if status[0].Status != "done" || status[1].Status != "current" {
		t.Fatalf("timeline = %+v", status)
	}
}

func TestTimelineApprovedNeedsOrientation(t *testing.T) {
	status := timeline(Profile{
		VerificationStatus: string(domain.VerificationApproved),
		OnboardingStep:     string(domain.OnboardingSubmitted),
		OrientationStatus:  string(domain.OrientationNone),
	})
	if status[1].Status != "done" || status[2].Status != "current" {
		t.Fatalf("timeline = %+v", status)
	}
}
