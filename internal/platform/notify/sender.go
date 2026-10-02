package notify

import (
	"context"
	"strings"

	"github.com/rs/zerolog"

	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
)

// Channel is how an OTP is delivered. Meta WhatsApp / SMS / Resend wire in later.
type Channel string

const (
	ChannelWhatsApp Channel = "whatsapp"
	ChannelSMS      Channel = "sms"
	ChannelEmail    Channel = "email"
)

func ParseChannel(raw string) (Channel, error) {
	switch Channel(strings.ToLower(strings.TrimSpace(raw))) {
	case "", ChannelWhatsApp:
		return ChannelWhatsApp, nil
	case ChannelSMS:
		return ChannelSMS, nil
	case ChannelEmail:
		return ChannelEmail, nil
	default:
		return "", apperror.Invalid("channel is invalid")
	}
}

func ParsePhoneChannel(raw string) (Channel, error) {
	channel, err := ParseChannel(raw)
	if err != nil {
		return "", err
	}
	if channel == ChannelEmail {
		return "", apperror.Invalid("channel is invalid")
	}
	return channel, nil
}

// Sender delivers OTP codes. Development logs only; production plugs Meta/Resend.
type Sender interface {
	Send(ctx context.Context, channel Channel, destination, code string) error
}

type LogSender struct {
	log zerolog.Logger
}

func NewLogSender(log zerolog.Logger) *LogSender {
	return &LogSender{log: log}
}

func (s *LogSender) Send(_ context.Context, channel Channel, destination, code string) error {
	s.log.Info().
		Str("channel", string(channel)).
		Str("destination", mask(destination)).
		Msg("otp send stub")
	_ = code
	return nil
}

func mask(raw string) string {
	if len(raw) < 4 {
		return "***"
	}
	return raw[:2] + "***" + raw[len(raw)-2:]
}
