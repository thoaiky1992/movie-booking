// Package mail renders ticket emails. Delivery is a logging mock for now.
package mail

import (
	"fmt"
	"html"
	"strings"

	qrcode "github.com/skip2/go-qrcode"

	"movie_booking/internal/domain"
	"movie_booking/internal/usecase"
)

// TicketComposer renders booking events into emails. The QR code encodes only
// the ticket code, so no personal data is embedded in the image.
type TicketComposer struct {
	currency string
}

func NewTicketComposer(currency string) *TicketComposer { return &TicketComposer{currency: currency} }

var _ usecase.TicketComposer = (*TicketComposer)(nil)

const qrContentID = "ticket-qr@movie-booking"

func (c *TicketComposer) Compose(evt domain.BookingEvent) (usecase.EmailMessage, error) {
	switch evt.Type {
	case domain.EventBookingConfirmed:
		return c.confirmed(evt)
	case domain.EventBookingCancelled:
		return c.cancelled(evt), nil
	default:
		return usecase.EmailMessage{}, fmt.Errorf("no template for event type %q", evt.Type)
	}
}

func (c *TicketComposer) confirmed(evt domain.BookingEvent) (usecase.EmailMessage, error) {
	png, err := qrcode.Encode(evt.TicketCode, qrcode.Medium, 256)
	if err != nil {
		return usecase.EmailMessage{}, fmt.Errorf("encode qr: %w", err)
	}

	var b strings.Builder
	fmt.Fprintf(&b, `<html><body style="font-family:Arial,sans-serif;color:#222">`)
	fmt.Fprintf(&b, `<h2>Booking confirmed</h2><p>Hi %s, your tickets are ready.</p>`, html.EscapeString(evt.Recipient.Name))
	fmt.Fprintf(&b, `<table cellpadding="6" style="border-collapse:collapse">`)
	row(&b, "Movie", evt.MovieTitle)
	row(&b, "Cinema", evt.CinemaName+" — "+evt.HallName)
	row(&b, "Showtime", evt.StartsAt.Format("Mon 02 Jan 2006, 15:04 MST"))
	row(&b, "Seats", strings.Join(evt.Seats, ", "))
	row(&b, "Total", c.money(evt.TotalCents))
	row(&b, "Ticket code", evt.TicketCode)
	fmt.Fprintf(&b, `</table>`)
	fmt.Fprintf(&b, `<p>Show this QR code at the cinema entrance:</p>`)
	fmt.Fprintf(&b, `<img src="cid:%s" alt="Ticket QR" width="256" height="256"/>`, qrContentID)
	fmt.Fprintf(&b, `</body></html>`)

	return usecase.EmailMessage{
		To:       evt.Recipient.Email,
		Subject:  fmt.Sprintf("Your tickets for %s (%s)", evt.MovieTitle, evt.TicketCode),
		HTMLBody: b.String(),
		Inline: []usecase.InlineImage{{
			ContentID:   qrContentID,
			ContentType: "image/png",
			Filename:    "ticket-qr.png",
			Data:        png,
		}},
	}, nil
}

func (c *TicketComposer) cancelled(evt domain.BookingEvent) usecase.EmailMessage {
	refund := "No refund is due under the cancellation policy."
	if evt.RefundCents > 0 {
		refund = fmt.Sprintf("A refund of %s is being processed.", c.money(evt.RefundCents))
	}
	body := fmt.Sprintf(`<html><body style="font-family:Arial,sans-serif;color:#222">
<h2>Booking cancelled</h2>
<p>Your booking %s for <b>%s</b> (seats %s) has been cancelled.</p>
<p>%s</p></body></html>`,
		html.EscapeString(evt.TicketCode), html.EscapeString(evt.MovieTitle),
		html.EscapeString(strings.Join(evt.Seats, ", ")), html.EscapeString(refund))
	return usecase.EmailMessage{
		To:       evt.Recipient.Email,
		Subject:  fmt.Sprintf("Booking cancelled: %s", evt.MovieTitle),
		HTMLBody: body,
	}
}

func (c *TicketComposer) money(cents int64) string {
	return fmt.Sprintf("%d.%02d %s", cents/100, cents%100, c.currency)
}

func row(b *strings.Builder, label, value string) {
	fmt.Fprintf(b, `<tr><td style="color:#666">%s</td><td><b>%s</b></td></tr>`,
		html.EscapeString(label), html.EscapeString(value))
}
