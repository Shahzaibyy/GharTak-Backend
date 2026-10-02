package riders

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/yourusername/ghartak-backend/internal/modules/auth"
	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
	"github.com/yourusername/ghartak-backend/internal/platform/domain"
	"github.com/yourusername/ghartak-backend/internal/platform/pii"
)

type store interface {
	Insert(ctx context.Context, row sealedRider) (Profile, error)
	InsertShell(ctx context.Context, phoneCipher, phoneLookup string, zoneID uuid.UUID) (Profile, error)
	Get(ctx context.Context, id uuid.UUID) (Profile, error)
	List(ctx context.Context, status domain.VerificationStatus) ([]Profile, error)
	SetOnline(ctx context.Context, id uuid.UUID, online bool) (Profile, error)
	Approve(ctx context.Context, id, zoneID uuid.UUID) (Profile, error)
	Reject(ctx context.Context, id uuid.UUID) (Profile, error)
	Suspend(ctx context.Context, id uuid.UUID) (Profile, error)
	UpdateDetails(ctx context.Context, id uuid.UUID, row detailsRow) (Profile, error)
	UpdateDocuments(ctx context.Context, id uuid.UUID, in DocumentsInput) (Profile, error)
	Submit(ctx context.Context, id uuid.UUID) (Profile, error)
	BookOrientation(ctx context.Context, id uuid.UUID, slot *string) (Profile, error)
}

type zoneGate interface {
	Exists(ctx context.Context, id uuid.UUID) error
	Active(ctx context.Context, id uuid.UUID) error
}

type otpIssuer interface {
	RequestOTP(ctx context.Context, req auth.OTPRequest) (auth.OTPResult, error)
}

type Service struct {
	store    store
	zones    zoneGate
	apply    applyStore
	otp      otpIssuer
	piiKey   []byte
	hashKey  []byte
	presence presence
}

func NewService(store store, zones zoneGate, piiKey, hashKey []byte) *Service {
	return &Service{store: store, zones: zones, piiKey: piiKey, hashKey: hashKey}
}

func (s *Service) UseApply(store applyStore) { s.apply = store }

func (s *Service) UseOTP(otp otpIssuer) { s.otp = otp }

func (s *Service) Register(ctx context.Context, in RegisterInput) (Profile, error) {
	if err := s.zones.Exists(ctx, in.ZoneID); err != nil {
		return Profile{}, err
	}
	row, err := s.seal(in)
	if err != nil {
		return Profile{}, err
	}
	return s.store.Insert(ctx, row)
}

func (s *Service) Apply(ctx context.Context, in ApplyInput) (auth.OTPResult, error) {
	if s.apply == nil || s.otp == nil {
		return auth.OTPResult{}, apperror.ErrUnavailable
	}
	if err := s.zones.Exists(ctx, in.ZoneID); err != nil {
		return auth.OTPResult{}, err
	}
	_, lookup, err := pii.SealPhone(s.piiKey, s.hashKey, in.Phone)
	if err != nil {
		return auth.OTPResult{}, apperror.Invalid("phone is invalid")
	}
	if err := s.apply.SaveApply(ctx, lookup, in.ZoneID.String()); err != nil {
		return auth.OTPResult{}, err
	}
	return s.otp.RequestOTP(ctx, auth.OTPRequest{
		Phone: in.Phone, Role: auth.RoleRider, Channel: in.Channel, IP: in.IP,
	})
}

func (s *Service) EnsureFromApply(ctx context.Context, ciphertext, lookup string) (auth.Account, error) {
	if s.apply == nil {
		return auth.Account{}, apperror.Unauthorized("account is not registered")
	}
	zoneRaw, err := s.apply.TakeApply(ctx, lookup)
	if err != nil {
		return auth.Account{}, err
	}
	zoneID, err := ParseZoneID(zoneRaw)
	if err != nil {
		return auth.Account{}, err
	}
	profile, err := s.store.InsertShell(ctx, ciphertext, lookup, zoneID)
	if errors.Is(err, apperror.ErrConflict) {
		return auth.Account{}, apperror.Unauthorized("account is not registered")
	}
	if err != nil {
		return auth.Account{}, err
	}
	return auth.Account{ID: profile.ID, Status: profile.VerificationStatus}, nil
}

func (s *Service) Me(ctx context.Context, id uuid.UUID) (Profile, error) {
	return s.store.Get(ctx, id)
}

func (s *Service) Onboarding(ctx context.Context, id uuid.UUID) (OnboardingStatus, error) {
	profile, err := s.store.Get(ctx, id)
	if err != nil {
		return OnboardingStatus{}, err
	}
	return OnboardingStatus{Profile: profile, Timeline: timeline(profile)}, nil
}

func (s *Service) SaveDetails(ctx context.Context, id uuid.UUID, in DetailsInput) (Profile, error) {
	row, err := s.sealDetails(in)
	if err != nil {
		return Profile{}, err
	}
	profile, err := s.store.UpdateDetails(ctx, id, row)
	if errors.Is(err, apperror.ErrNotFound) {
		return Profile{}, apperror.Conflict("rider cannot update details now")
	}
	return profile, err
}

func (s *Service) SaveDocuments(ctx context.Context, id uuid.UUID, in DocumentsInput) (Profile, error) {
	if err := validDocuments(in); err != nil {
		return Profile{}, err
	}
	profile, err := s.store.UpdateDocuments(ctx, id, in)
	if errors.Is(err, apperror.ErrNotFound) {
		return Profile{}, apperror.Conflict("rider cannot update documents now")
	}
	return profile, err
}

func (s *Service) Submit(ctx context.Context, id uuid.UUID) (Profile, error) {
	profile, err := s.store.Submit(ctx, id)
	if errors.Is(err, apperror.ErrNotFound) {
		return Profile{}, apperror.Conflict("complete details and documents before submit")
	}
	return profile, err
}

func (s *Service) BookOrientation(ctx context.Context, id uuid.UUID, in OrientationInput) (Profile, error) {
	slot := strings.TrimSpace(in.PreferredSlot)
	if len(slot) > 120 {
		return Profile{}, apperror.Invalid("preferred_slot is invalid")
	}
	profile, err := s.store.BookOrientation(ctx, id, nullEmpty(slot))
	if errors.Is(err, apperror.ErrNotFound) {
		return Profile{}, apperror.Conflict("rider cannot book orientation now")
	}
	return profile, err
}

func (s *Service) SetOnline(ctx context.Context, id uuid.UUID, online bool) (Profile, error) {
	profile, err := s.store.SetOnline(ctx, id, online)
	if errors.Is(err, apperror.ErrNotFound) && online {
		return Profile{}, apperror.Forbidden("rider must be approved, oriented, and assigned to a zone")
	}
	if err != nil {
		return profile, err
	}
	s.syncPresence(ctx, profile)
	return profile, nil
}

func (s *Service) List(ctx context.Context, status domain.VerificationStatus) ([]Profile, error) {
	return s.store.List(ctx, status)
}

func (s *Service) Approve(ctx context.Context, id, zoneID uuid.UUID) (Profile, error) {
	if err := s.zones.Active(ctx, zoneID); err != nil {
		return Profile{}, err
	}
	profile, err := s.store.Approve(ctx, id, zoneID)
	return s.finish(profile, err, "rider is not awaiting verification")
}

func (s *Service) Reject(ctx context.Context, id uuid.UUID) (Profile, error) {
	profile, err := s.store.Reject(ctx, id)
	return s.finish(profile, err, "rider is not awaiting verification")
}

func (s *Service) Suspend(ctx context.Context, id uuid.UUID) (Profile, error) {
	profile, err := s.store.Suspend(ctx, id)
	return s.finish(profile, err, "rider is not approved")
}

func (s *Service) finish(profile Profile, err error, message string) (Profile, error) {
	if errors.Is(err, apperror.ErrNotFound) {
		return Profile{}, apperror.Conflict(message)
	}
	return profile, err
}

func (s *Service) seal(in RegisterInput) (sealedRider, error) {
	phone, phoneLookup, err := pii.SealPhone(s.piiKey, s.hashKey, in.Phone)
	if err != nil {
		return sealedRider{}, apperror.Invalid("phone is invalid")
	}
	cnic, cnicLookup, err := pii.SealCNIC(s.piiKey, s.hashKey, in.CNIC)
	if err != nil {
		return sealedRider{}, apperror.Invalid("cnic is invalid")
	}
	return sealedRider{
		PhoneCipher: phone, PhoneLookup: phoneLookup, Name: in.Name,
		CNICCipher: cnic, CNICLookup: cnicLookup,
		CNICObjectKey: in.CNICObjectKey, SelfieObjectKey: in.SelfieObjectKey,
		LicenseObjectKey: in.LicenseObjectKey, VehicleReg: in.VehicleReg, ZoneID: in.ZoneID,
	}, nil
}

func (s *Service) sealDetails(in DetailsInput) (detailsRow, error) {
	if len(in.Name) < 1 || len(in.Name) > 100 {
		return detailsRow{}, apperror.Invalid("name is invalid")
	}
	if _, ok := vehicleTypes[in.VehicleType]; !ok {
		return detailsRow{}, apperror.Invalid("vehicle_type is invalid")
	}
	if in.VehicleType == domain.VehicleMotorcycle && (len(in.VehicleReg) < 1 || len(in.VehicleReg) > 30) {
		return detailsRow{}, apperror.Invalid("vehicle_reg is invalid")
	}
	cnic, cnicLookup, err := pii.SealCNIC(s.piiKey, s.hashKey, in.CNIC)
	if err != nil {
		return detailsRow{}, apperror.Invalid("cnic is invalid")
	}
	return detailsRow{
		Name: in.Name, CNICCipher: cnic, CNICLookup: cnicLookup,
		VehicleType: string(in.VehicleType), VehicleReg: in.VehicleReg, LicenseNumber: in.LicenseNumber,
	}, nil
}

func validDocuments(in DocumentsInput) error {
	if !objectKey(in.CNICFrontObjectKey) || !objectKey(in.CNICBackObjectKey) {
		return apperror.Invalid("document keys are invalid")
	}
	if !objectKey(in.LicenseObjectKey) || !objectKey(in.SelfieObjectKey) {
		return apperror.Invalid("document keys are invalid")
	}
	return nil
}

var vehicleTypes = map[domain.VehicleType]struct{}{
	domain.VehicleMotorcycle: {},
	domain.VehicleBicycle:    {},
}

func timeline(p Profile) []TimelineStep {
	received := stepStatus(p.ApplicationReceived != nil || p.OnboardingStep == string(domain.OnboardingSubmitted), false)
	docs := documentStep(p)
	orient := orientationStep(p)
	online := stepStatus(p.VerificationStatus == string(domain.VerificationApproved) &&
		(p.OrientationStatus == string(domain.OrientationBooked) || p.OrientationStatus == string(domain.OrientationCompleted)),
		false)
	return []TimelineStep{
		{Key: "application_received", Title: "Application received", Detail: "Documents received", Status: received},
		{Key: "document_check", Title: "Document check", Detail: docs.detail, Status: docs.status},
		{Key: "orientation", Title: "Orientation", Detail: orient.detail, Status: orient.status},
		{Key: "go_online", Title: "Go online", Detail: "Start receiving orders", Status: online},
	}
}

type stepInfo struct {
	status string
	detail string
}

func documentStep(p Profile) stepInfo {
	switch p.VerificationStatus {
	case string(domain.VerificationApproved):
		return stepInfo{status: "done", detail: "Approved"}
	case string(domain.VerificationRejected):
		return stepInfo{status: "done", detail: "Rejected"}
	case string(domain.VerificationPending):
		if p.OnboardingStep == string(domain.OnboardingSubmitted) {
			return stepInfo{status: "current", detail: "In progress"}
		}
		return stepInfo{status: "pending", detail: "Submit documents first"}
	default:
		return stepInfo{status: "pending", detail: "Pending"}
	}
}

func orientationStep(p Profile) stepInfo {
	switch p.OrientationStatus {
	case string(domain.OrientationCompleted):
		return stepInfo{status: "done", detail: "Completed"}
	case string(domain.OrientationBooked):
		return stepInfo{status: "current", detail: "Booked"}
	default:
		if p.VerificationStatus == string(domain.VerificationApproved) || p.OnboardingStep == string(domain.OnboardingSubmitted) {
			return stepInfo{status: "current", detail: "Short training, online or in person"}
		}
		return stepInfo{status: "pending", detail: "Short training, online or in person"}
	}
}

func stepStatus(done, current bool) string {
	if done {
		return "done"
	}
	if current {
		return "current"
	}
	return "pending"
}
