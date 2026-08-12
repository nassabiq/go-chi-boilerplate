package mail

import (
	"context"
	"errors"
	"fmt"

	natsclient "github.com/nassabiq/go-chi-boilerplate/internal/pkg/nats"
	"github.com/nats-io/nats.go/jetstream"
)

// Run starts the durable consumer for this worker's topic and blocks until ctx
// is cancelled or the iterator stops. ACK semantics:
//
//   - SMTP success        -> Ack
//   - SMTP failure        -> NakWithDelay (JetStream redelivery)
//   - malformed message   -> Term (no retry, poison message)
//   - MaxDeliver exceeded -> JetStream stops redelivering (no DLQ yet)
//
// Delivery is at-least-once: if SMTP succeeds but the ACK itself fails, the
// event may be redelivered and the email sent twice. EventID is stable so a
// future idempotency layer can key on it.
func (w *Worker) Run(ctx context.Context) error {
	consumer, err := w.nats.JetStream.CreateOrUpdateConsumer(ctx, natsclient.AppEventsStream, jetstream.ConsumerConfig{
		Durable:       w.consumerName,
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: w.filterSubject,
		AckWait:       w.ackWait,
		MaxDeliver:    w.maxDeliver,
	})
	if err != nil {
		return fmt.Errorf("create consumer %s: %w", w.consumerName, err)
	}

	msgs, err := consumer.Messages()
	if err != nil {
		return fmt.Errorf("subscribe messages: %w", err)
	}

	go func() {
		<-ctx.Done()
		msgs.Stop()
	}()

	w.logger.InfoContext(ctx, "mail worker consuming",
		"consumer", w.consumerName,
		"topic", w.filterSubject,
	)

	for {
		msg, err := msgs.Next()
		if err != nil {
			if errors.Is(err, jetstream.ErrMsgIteratorClosed) || ctx.Err() != nil {
				return nil
			}
			w.logger.ErrorContext(ctx, "read mail message", "error", err)
			continue
		}

		// Use a dedicated timeout context for in-flight processing so a SIGTERM
		// does not abruptly abort an ongoing SMTP transaction.
		processCtx, processCancel := context.WithTimeout(context.Background(), w.ackWait)
		w.Process(processCtx, msg)
		processCancel()
	}
}

// Process handles a single JetStream message: ACK on success, NAK-with-delay
// on transient failure, Term on poison messages.
func (w *Worker) Process(ctx context.Context, msg jetstream.Msg) {
	err := w.handler(ctx, msg)
	topic := msg.Subject()

	var poison *poisonError
	switch {
	case errors.As(err, &poison):
		// Malformed JSON / missing fields can never succeed on retry.
		w.metrics.RecordMailFailed(ctx, topic, "poison")
		w.logger.ErrorContext(ctx, "terminating malformed mail message",
			"consumer", w.consumerName,
			"topic", msg.Subject(),
			"error", err.Error(),
		)
		if termErr := msg.Term(); termErr != nil {
			w.logger.ErrorContext(ctx, "terminate mail message failed", "error", termErr)
		}
	case err != nil:
		// Transient failure (e.g. SMTP down): NAK with delay for redelivery.
		w.metrics.RecordMailFailed(ctx, topic, "transient")
		w.metrics.RecordMailRetry(ctx, topic)
		w.logger.ErrorContext(ctx, "mail message processing failed",
			"consumer", w.consumerName,
			"topic", msg.Subject(),
			"error", err.Error(),
		)
		if nakErr := msg.NakWithDelay(w.retryDelay); nakErr != nil {
			w.logger.ErrorContext(ctx, "nak mail message failed", "error", nakErr)
		}
	default:
		w.metrics.RecordMailSent(ctx, topic)
		if ackErr := msg.Ack(); ackErr != nil {
			// At-least-once: JetStream may redeliver after successful send.
			w.logger.ErrorContext(ctx, "ack mail message failed; email may be redelivered",
				"consumer", w.consumerName,
				"topic", msg.Subject(),
				"error", ackErr,
			)
		}
	}
}
