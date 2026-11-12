package notification

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/hibiken/asynq"
)

type mailerStub struct {
	payload BookingConfirmationPayload
	err     error
}

func (s *mailerStub) SendBookingConfirmation(
	_ context.Context,
	payload BookingConfirmationPayload,
) error {
	s.payload = payload
	return s.err
}

func TestBookingConfirmationHandlerSendsMockableMail(t *testing.T) {
	payload := BookingConfirmationPayload{
		BookingID:      42,
		RecipientEmail: "customer@example.com",
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	mailer := &mailerStub{}
	handler := NewBookingConfirmationHandler(mailer)

	if err := handler.ProcessTask(
		context.Background(),
		asynq.NewTask(BookingConfirmationTaskType, encoded),
	); err != nil {
		t.Fatalf("ProcessTask() error = %v", err)
	}
	if mailer.payload.BookingID != 42 || mailer.payload.RecipientEmail != "customer@example.com" {
		t.Fatalf("mailer payload = %+v", mailer.payload)
	}
}

func TestBookingConfirmationHandlerDoesNotRetryInvalidPayload(t *testing.T) {
	handler := NewBookingConfirmationHandler(&mailerStub{})
	err := handler.ProcessTask(
		context.Background(),
		asynq.NewTask(BookingConfirmationTaskType, []byte(`{"booking_id":0}`)),
	)
	if !errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("error = %v, want asynq.SkipRetry", err)
	}
}

func TestBookingConfirmationHandlerReturnsMailerError(t *testing.T) {
	wantErr := errors.New("mail provider unavailable")
	encoded, err := json.Marshal(BookingConfirmationPayload{
		BookingID:      42,
		RecipientEmail: "customer@example.com",
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	handler := NewBookingConfirmationHandler(&mailerStub{err: wantErr})

	err = handler.ProcessTask(
		context.Background(),
		asynq.NewTask(BookingConfirmationTaskType, encoded),
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
}
