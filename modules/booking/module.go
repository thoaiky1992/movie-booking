package booking

import (
	"context"
	"database/sql"
	"log/slog"

	"go.uber.org/fx"

	"movie-booking/modules/booking/application"
	bookingcache "movie-booking/modules/booking/infrastructure/cache"
	"movie-booking/modules/booking/infrastructure/mailer"
	"movie-booking/modules/booking/infrastructure/persistence"
	"movie-booking/modules/booking/infrastructure/producer"
	bookingevent "movie-booking/modules/booking/presentation/event"
	bookinghttp "movie-booking/modules/booking/presentation/http_api"
	appconfig "movie-booking/shared/config"
)

var Module = fx.Module("booking",
	fx.Provide(
		newRepositoryPorts,
		newSeatLockerPorts,
		fx.Annotate(
			newConfirmationEnqueuer,
			fx.As(new(application.BookingConfirmationEnqueuer)),
		),
		fx.Annotate(mailer.NewMock, fx.As(new(application.Mailer))),
		newHoldSeatsCommand,
		newConfirmBookingCommand,
		newCancelBookingCommand,
		application.NewExpirePendingHoldsCommand,
		application.NewSendBookingConfirmationCommand,
		bookinghttp.NewHandler,
		bookingevent.NewBookingConfirmationHandler,
	),
)

type repositoryPorts struct {
	fx.Out

	Booking application.BookingRepository
	Pending application.PendingHoldRepository
}

func newRepositoryPorts(database *sql.DB) repositoryPorts {
	repository := persistence.NewBookingRepository(database)
	return repositoryPorts{Booking: repository, Pending: repository}
}

type seatLockerPorts struct {
	fx.Out

	Locker   application.SeatLocker
	Releaser application.ExpiredSeatLockReleaser
}

func newHoldSeatsCommand(
	repository application.BookingRepository,
	locker application.SeatLocker,
	logger *slog.Logger,
	config appconfig.Config,
) *application.HoldSeatsCommand {
	return application.NewHoldSeatsCommand(repository, locker, logger, config.Booking.SeatHoldTTL)
}

func newConfirmBookingCommand(
	repository application.BookingRepository,
	locker application.SeatLocker,
	enqueuer application.BookingConfirmationEnqueuer,
	logger *slog.Logger,
) *application.ConfirmBookingCommand {
	return application.NewConfirmBookingCommand(repository, locker, enqueuer, logger)
}

func newCancelBookingCommand(
	repository application.BookingRepository,
	locker application.SeatLocker,
	logger *slog.Logger,
) *application.CancelBookingCommand {
	return application.NewCancelBookingCommand(repository, locker, logger)
}

func newSeatLockerPorts(
	lifecycle fx.Lifecycle,
	config appconfig.Config,
) (seatLockerPorts, error) {
	client, err := bookingcache.NewClient(config.Redis.URL)
	if err != nil {
		return seatLockerPorts{}, err
	}
	lifecycle.Append(fx.Hook{OnStop: func(context.Context) error {
		return client.Close()
	}})
	locker := bookingcache.NewSeatLocker(client)
	return seatLockerPorts{Locker: locker, Releaser: locker}, nil
}

func newConfirmationEnqueuer(
	lifecycle fx.Lifecycle,
	config appconfig.Config,
) (*producer.BookingConfirmationEnqueuer, error) {
	enqueuer, err := producer.NewBookingConfirmationEnqueuer(config.Redis.URL)
	if err != nil {
		return nil, err
	}
	lifecycle.Append(fx.Hook{OnStop: func(context.Context) error {
		return enqueuer.Close()
	}})
	return enqueuer, nil
}
