package riders

import (
	"context"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/yourusername/ghartak-backend/internal/modules/auth"
	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
	"github.com/yourusername/ghartak-backend/internal/platform/domain"
	"github.com/yourusername/ghartak-backend/internal/platform/httpserver"
	"github.com/yourusername/ghartak-backend/internal/platform/httpx"
)

type riderAPI interface {
	Register(ctx context.Context, in RegisterInput) (Profile, error)
	Apply(ctx context.Context, in ApplyInput) (auth.OTPResult, error)
	Me(ctx context.Context, id uuid.UUID) (Profile, error)
	Onboarding(ctx context.Context, id uuid.UUID) (OnboardingStatus, error)
	SaveDetails(ctx context.Context, id uuid.UUID, in DetailsInput) (Profile, error)
	SaveDocuments(ctx context.Context, id uuid.UUID, in DocumentsInput) (Profile, error)
	Submit(ctx context.Context, id uuid.UUID) (Profile, error)
	BookOrientation(ctx context.Context, id uuid.UUID, in OrientationInput) (Profile, error)
	SetOnline(ctx context.Context, id uuid.UUID, online bool) (Profile, error)
	Position(ctx context.Context, id uuid.UUID, lat, lng float64) error
	List(ctx context.Context, status domain.VerificationStatus) ([]Profile, error)
	Approve(ctx context.Context, id, zoneID uuid.UUID) (Profile, error)
	Reject(ctx context.Context, id uuid.UUID) (Profile, error)
	Suspend(ctx context.Context, id uuid.UUID) (Profile, error)
}

type Handler struct {
	riders riderAPI
	log    zerolog.Logger
}

func NewHandler(riders riderAPI, log zerolog.Logger) *Handler {
	return &Handler{riders: riders, log: log}
}

type registerBody struct {
	Phone            string `json:"phone"`
	Name             string `json:"name"`
	CNIC             string `json:"cnic"`
	CNICObjectKey    string `json:"cnic_object_key"`
	SelfieObjectKey  string `json:"selfie_object_key"`
	LicenseObjectKey string `json:"license_object_key"`
	VehicleReg       string `json:"vehicle_reg"`
	ZoneID           string `json:"zone_id"`
}

type applyBody struct {
	Phone   string `json:"phone"`
	ZoneID  string `json:"zone_id"`
	Channel string `json:"channel"`
}

type detailsBody struct {
	Name          string `json:"name"`
	CNIC          string `json:"cnic"`
	VehicleType   string `json:"vehicle_type"`
	VehicleReg    string `json:"vehicle_reg"`
	LicenseNumber string `json:"license_number"`
}

type documentsBody struct {
	CNICFrontObjectKey string `json:"cnic_front_object_key"`
	CNICBackObjectKey  string `json:"cnic_back_object_key"`
	LicenseObjectKey   string `json:"license_object_key"`
	SelfieObjectKey    string `json:"selfie_object_key"`
}

type orientationBody struct {
	PreferredSlot string `json:"preferred_slot"`
}

type onlineBody struct {
	IsOnline bool `json:"is_online"`
}

type approveBody struct {
	ZoneID string `json:"zone_id"`
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	in, err := decodeRegister(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	profile, err := h.riders.Register(r.Context(), in)
	if err != nil {
		h.log.Error().Err(err).Msg("register rider")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusCreated, profile)
}

func (h *Handler) Apply(w http.ResponseWriter, r *http.Request) {
	in, err := decodeApply(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	result, err := h.riders.Apply(r.Context(), in)
	if err != nil {
		h.log.Error().Err(err).Msg("rider apply")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, result)
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	id, err := callerID(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	profile, err := h.riders.Me(r.Context(), id)
	if err != nil {
		h.log.Error().Err(err).Str("rider_id", id.String()).Msg("rider profile")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, profile)
}

func (h *Handler) OnboardingStatus(w http.ResponseWriter, r *http.Request) {
	id, err := callerID(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	status, err := h.riders.Onboarding(r.Context(), id)
	if err != nil {
		h.log.Error().Err(err).Str("rider_id", id.String()).Msg("rider onboarding")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, status)
}

func (h *Handler) Details(w http.ResponseWriter, r *http.Request) {
	id, in, err := decodeDetails(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	profile, err := h.riders.SaveDetails(r.Context(), id, in)
	if err != nil {
		h.log.Error().Err(err).Str("rider_id", id.String()).Msg("rider details")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, profile)
}

func (h *Handler) Documents(w http.ResponseWriter, r *http.Request) {
	id, in, err := decodeDocuments(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	profile, err := h.riders.SaveDocuments(r.Context(), id, in)
	if err != nil {
		h.log.Error().Err(err).Str("rider_id", id.String()).Msg("rider documents")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, profile)
}

func (h *Handler) Submit(w http.ResponseWriter, r *http.Request) {
	id, err := callerID(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	profile, err := h.riders.Submit(r.Context(), id)
	if err != nil {
		h.log.Error().Err(err).Str("rider_id", id.String()).Msg("rider submit")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, profile)
}

func (h *Handler) Orientation(w http.ResponseWriter, r *http.Request) {
	id, in, err := decodeOrientation(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	profile, err := h.riders.BookOrientation(r.Context(), id, in)
	if err != nil {
		h.log.Error().Err(err).Str("rider_id", id.String()).Msg("rider orientation")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, profile)
}

func (h *Handler) Availability(w http.ResponseWriter, r *http.Request) {
	id, online, err := decodeOnline(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	profile, err := h.riders.SetOnline(r.Context(), id, online)
	if err != nil {
		h.log.Error().Err(err).Str("rider_id", id.String()).Bool("is_online", online).Msg("rider availability")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, profile)
}

func (h *Handler) Position(w http.ResponseWriter, r *http.Request) {
	id, lat, lng, err := decodePosition(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if err := h.riders.Position(r.Context(), id, lat, lng); err != nil {
		h.log.Error().Err(err).Str("rider_id", id.String()).Msg("rider position")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *Handler) Queue(w http.ResponseWriter, r *http.Request) {
	status, err := parseStatus(r.URL.Query().Get("status"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	list, err := h.riders.List(r.Context(), status)
	if err != nil {
		h.log.Error().Err(err).Msg("rider queue")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, list)
}

func (h *Handler) Approve(w http.ResponseWriter, r *http.Request) {
	id, zoneID, err := decodeApprove(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	profile, err := h.riders.Approve(r.Context(), id, zoneID)
	if err != nil {
		h.log.Error().Err(err).Str("rider_id", id.String()).Msg("approve rider")
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, profile)
}

func (h *Handler) Reject(w http.ResponseWriter, r *http.Request) {
	h.decide(w, r, h.riders.Reject, "reject rider")
}

func (h *Handler) Suspend(w http.ResponseWriter, r *http.Request) {
	h.decide(w, r, h.riders.Suspend, "suspend rider")
}

func (h *Handler) decide(w http.ResponseWriter, r *http.Request, fn func(context.Context, uuid.UUID) (Profile, error), action string) {
	id, err := pathID(r, "id")
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	profile, err := fn(r.Context(), id)
	if err != nil {
		h.log.Error().Err(err).Str("rider_id", id.String()).Msg(action)
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, profile)
}

func decodeRegister(r *http.Request) (RegisterInput, error) {
	body, zoneID, err := readRegister(r)
	if err != nil {
		return RegisterInput{}, err
	}
	if err := validRiderText(body); err != nil {
		return RegisterInput{}, err
	}
	return RegisterInput{
		Phone: body.Phone, Name: body.Name, CNIC: body.CNIC, CNICObjectKey: body.CNICObjectKey,
		SelfieObjectKey: body.SelfieObjectKey, LicenseObjectKey: body.LicenseObjectKey,
		VehicleReg: body.VehicleReg, ZoneID: zoneID,
	}, nil
}

func decodeApply(r *http.Request) (ApplyInput, error) {
	var body applyBody
	if err := httpx.Decode(r, &body); err != nil {
		return ApplyInput{}, err
	}
	zoneID, err := httpx.ParseUUID(body.ZoneID, "zone_id")
	if err != nil {
		return ApplyInput{}, err
	}
	return ApplyInput{
		Phone: body.Phone, ZoneID: zoneID, Channel: body.Channel,
		IP: httpserver.ClientIP(r.RemoteAddr),
	}, nil
}

func decodeDetails(r *http.Request) (uuid.UUID, DetailsInput, error) {
	id, err := callerID(r)
	if err != nil {
		return uuid.UUID{}, DetailsInput{}, err
	}
	var body detailsBody
	if err := httpx.Decode(r, &body); err != nil {
		return uuid.UUID{}, DetailsInput{}, err
	}
	return id, DetailsInput{
		Name: strings.TrimSpace(body.Name), CNIC: body.CNIC,
		VehicleType: domain.VehicleType(body.VehicleType),
		VehicleReg: strings.TrimSpace(body.VehicleReg),
		LicenseNumber: strings.TrimSpace(body.LicenseNumber),
	}, nil
}

func decodeDocuments(r *http.Request) (uuid.UUID, DocumentsInput, error) {
	id, err := callerID(r)
	if err != nil {
		return uuid.UUID{}, DocumentsInput{}, err
	}
	var body documentsBody
	if err := httpx.Decode(r, &body); err != nil {
		return uuid.UUID{}, DocumentsInput{}, err
	}
	return id, DocumentsInput{
		CNICFrontObjectKey: body.CNICFrontObjectKey,
		CNICBackObjectKey:  body.CNICBackObjectKey,
		LicenseObjectKey:   body.LicenseObjectKey,
		SelfieObjectKey:    body.SelfieObjectKey,
	}, nil
}

func decodeOrientation(r *http.Request) (uuid.UUID, OrientationInput, error) {
	id, err := callerID(r)
	if err != nil {
		return uuid.UUID{}, OrientationInput{}, err
	}
	var body orientationBody
	if err := httpx.Decode(r, &body); err != nil {
		return uuid.UUID{}, OrientationInput{}, err
	}
	return id, OrientationInput{PreferredSlot: body.PreferredSlot}, nil
}

func readRegister(r *http.Request) (registerBody, uuid.UUID, error) {
	var body registerBody
	if err := httpx.Decode(r, &body); err != nil {
		return registerBody{}, uuid.UUID{}, err
	}
	zoneID, err := httpx.ParseUUID(body.ZoneID, "zone_id")
	return body, zoneID, err
}

func validRiderText(body registerBody) error {
	if !between(len(body.Name), 1, 100) || !between(len(body.VehicleReg), 1, 30) {
		return apperror.Invalid("rider details are invalid")
	}
	return validKeys(body)
}

func validKeys(body registerBody) error {
	if !objectKey(body.CNICObjectKey) || !objectKey(body.SelfieObjectKey) || !objectKey(body.LicenseObjectKey) {
		return apperror.Invalid("document keys are invalid")
	}
	return nil
}

func objectKey(raw string) bool {
	return between(len(raw), 1, 200)
}

func between(n, min, max int) bool {
	return n >= min && n <= max
}

type positionBody struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

func decodePosition(r *http.Request) (uuid.UUID, float64, float64, error) {
	id, err := callerID(r)
	if err != nil {
		return uuid.UUID{}, 0, 0, err
	}
	var body positionBody
	if err := httpx.Decode(r, &body); err != nil {
		return uuid.UUID{}, 0, 0, err
	}
	if !validCoord(body.Lat, body.Lng) {
		return uuid.UUID{}, 0, 0, apperror.Invalid("position is invalid")
	}
	return id, body.Lat, body.Lng, nil
}

func validCoord(lat, lng float64) bool {
	return validLat(lat) && validLng(lng)
}

func validLat(lat float64) bool {
	return lat >= -90 && lat <= 90
}

func validLng(lng float64) bool {
	return lng >= -180 && lng <= 180
}

func decodeOnline(r *http.Request) (uuid.UUID, bool, error) {
	id, err := callerID(r)
	if err != nil {
		return uuid.UUID{}, false, err
	}
	var body onlineBody
	if err := httpx.Decode(r, &body); err != nil {
		return uuid.UUID{}, false, err
	}
	return id, body.IsOnline, nil
}

func decodeApprove(r *http.Request) (uuid.UUID, uuid.UUID, error) {
	id, err := pathID(r, "id")
	if err != nil {
		return uuid.UUID{}, uuid.UUID{}, err
	}
	var body approveBody
	if err := httpx.Decode(r, &body); err != nil {
		return uuid.UUID{}, uuid.UUID{}, err
	}
	zoneID, err := httpx.ParseUUID(body.ZoneID, "zone_id")
	return id, zoneID, err
}

func parseStatus(raw string) (domain.VerificationStatus, error) {
	if raw == "" {
		return domain.VerificationPending, nil
	}
	status := domain.VerificationStatus(raw)
	if _, ok := reviewStatuses[status]; !ok {
		return "", apperror.Invalid("status is invalid")
	}
	return status, nil
}

var reviewStatuses = map[domain.VerificationStatus]struct{}{
	domain.VerificationPending:   {},
	domain.VerificationApproved:  {},
	domain.VerificationRejected:  {},
	domain.VerificationSuspended: {},
}

func callerID(r *http.Request) (uuid.UUID, error) {
	principal, ok := auth.PrincipalFrom(r.Context())
	if !ok {
		return uuid.UUID{}, apperror.ErrUnauthorized
	}
	return principal.AccountID, nil
}

func pathID(r *http.Request, name string) (uuid.UUID, error) {
	return httpx.ParseUUID(chi.URLParam(r, name), name)
}
