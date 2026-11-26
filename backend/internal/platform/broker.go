package platform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"ecommerce/internal/contract"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	amqp "github.com/rabbitmq/amqp091-go"
)

const eventExchange = "ecommerce.events"
const deadExchange = "ecommerce.dead"

var subscriptions = map[string][]string{
	"catalog":  {contract.InventoryReserve, contract.InventoryRelease, contract.InventoryCommit},
	"search":   {contract.ProductChanged},
	"orders":   {contract.InventoryReserved, contract.InventoryRejected, contract.PaymentApproved, contract.PaymentDeclined, contract.InventoryReleased},
	"payments": {contract.PaymentRequested},
}

type brokerChannel = amqp.Channel

func Enqueue(ctx context.Context, tx pgx.Tx, topic string, event contract.Event) error {
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "INSERT INTO outbox(id, topic, body) VALUES ($1, $2, $3)", uuid.New(), topic, body)
	return err
}

func reconnect(ctx context.Context, brokerURL, role string, work func(*brokerChannel) error) {
	for ctx.Err() == nil {
		err := withBroker(brokerURL, work)
		if ctx.Err() == nil {
			slog.Warn("broker worker interrupted", "worker", role, "error", err)
			if !wait(ctx, 2*time.Second) {
				return
			}
		}
	}
}

func withBroker(brokerURL string, work func(*brokerChannel) error) error {
	connection, err := amqp.DialConfig(brokerURL, amqp.Config{
		Heartbeat: 10 * time.Second,
		Dial:      amqp.DefaultDial(5 * time.Second),
	})
	if err != nil {
		return err
	}
	defer connection.CloseDeadline(time.Now().Add(2 * time.Second))
	ch, err := connection.Channel()
	if err != nil {
		return err
	}
	if err := declareTopology(ch); err != nil {
		return err
	}
	return work(ch)
}

func declareTopology(ch *brokerChannel) error {
	for _, exchange := range []string{eventExchange, deadExchange} {
		if err := ch.ExchangeDeclare(exchange, "direct", true, false, false, false, nil); err != nil {
			return err
		}
	}
	for service, topics := range subscriptions {
		if _, err := ch.QueueDeclare(service+".dead", true, false, false, false, nil); err != nil {
			return err
		}
		if err := ch.QueueBind(service+".dead", service, deadExchange, false, nil); err != nil {
			return err
		}
		args := amqp.Table{
			"x-queue-type":              "quorum",
			"x-overflow":                "reject-publish",
			"x-delivery-limit":          int32(20),
			"x-dead-letter-exchange":    deadExchange,
			"x-dead-letter-routing-key": service,
			"x-dead-letter-strategy":    "at-least-once",
		}
		if _, err := ch.QueueDeclare(service, true, false, false, false, args); err != nil {
			return err
		}
		for _, topic := range topics {
			if err := ch.QueueBind(service, topic, eventExchange, false, nil); err != nil {
				return err
			}
		}
	}
	return nil
}

func consume(ctx context.Context, db *pgxpool.Pool, service string, ch *brokerChannel, handler Handler) error {
	if err := ch.Qos(1, 0, false); err != nil {
		return err
	}
	deliveries, err := ch.ConsumeWithContext(ctx, service, "", false, false, false, false, nil)
	if err != nil {
		return err
	}
	slog.Info("consumer ready", "queue", service)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case message, open := <-deliveries:
			if !open {
				return errors.New("delivery channel closed")
			}
			if err := handleDelivery(ctx, db, handler, message); err != nil {
				return err
			}
		}
	}
}

func handleDelivery(ctx context.Context, db *pgxpool.Pool, handler Handler, message amqp.Delivery) error {
	id, idErr := uuid.Parse(message.MessageId)
	var event contract.Event
	if jsonErr := json.Unmarshal(message.Body, &event); idErr != nil || id == uuid.Nil || jsonErr != nil {
		slog.Error("malformed event", "topic", message.RoutingKey, "message_id", message.MessageId)
		return message.Reject(false)
	}
	attempt, cancel := context.WithTimeout(ctx, 10*time.Second)
	err := pgx.BeginFunc(attempt, db, func(tx pgx.Tx) error {
		insert, err := tx.Exec(attempt, "INSERT INTO inbox(id) VALUES ($1) ON CONFLICT DO NOTHING", id)
		if err != nil || insert.RowsAffected() == 0 {
			return err
		}
		return handler(attempt, tx, message.RoutingKey, event)
	})
	cancel()
	if err != nil {
		slog.Error("event processing failed", "topic", message.RoutingKey, "message_id", id, "error", err)
		for wait(ctx, 3*time.Second) {
			probe, cancel := context.WithTimeout(ctx, time.Second)
			err := db.Ping(probe)
			cancel()
			if err == nil {
				return message.Reject(true)
			}
		}
		return ctx.Err()
	}
	return message.Ack(false)
}

func publishOutbox(ctx context.Context, db *pgxpool.Pool, ch *brokerChannel) error {
	if err := ch.Confirm(false); err != nil {
		return err
	}
	returns := ch.NotifyReturn(make(chan amqp.Return, 1))
	closed := ch.NotifyClose(make(chan *amqp.Error, 1))
	for ctx.Err() == nil {
		attempt, cancel := context.WithTimeout(ctx, 8*time.Second)
		sent, err := sendNext(attempt, db, ch, returns)
		cancel()
		if err != nil {
			return err
		}
		if !sent {
			timer := time.NewTimer(150 * time.Millisecond)
			select {
			case <-ctx.Done():
			case <-timer.C:
			case <-closed:
				timer.Stop()
				return errors.New("publish channel closed")
			}
			timer.Stop()
		}
	}
	return ctx.Err()
}

func sendNext(ctx context.Context, db *pgxpool.Pool, ch *brokerChannel, returns <-chan amqp.Return) (bool, error) {
	sent := false
	err := pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		var sequence int64
		var id string
		var topic string
		var body []byte
		err := tx.QueryRow(ctx, `
SELECT sequence, id::text, topic, body FROM outbox
ORDER BY sequence LIMIT 1 FOR UPDATE SKIP LOCKED`).Scan(&sequence, &id, &topic, &body)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}

		confirm, err := ch.PublishWithDeferredConfirmWithContext(ctx, eventExchange, topic, true, false, amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			MessageId:    id,
			Body:         body,
		})
		if err != nil {
			return err
		}
		acked, err := confirm.WaitContext(ctx)
		if err != nil {
			return err
		}
		select {
		case returned, open := <-returns:
			if !open {
				return errors.New("publish channel closed")
			}
			return fmt.Errorf("unroutable event %s: %s", returned.RoutingKey, returned.ReplyText)
		default:
		}
		if !acked {
			return fmt.Errorf("broker declined event %s", id)
		}
		_, err = tx.Exec(ctx, "DELETE FROM outbox WHERE sequence = $1", sequence)
		sent = err == nil
		return err
	})
	return sent, err
}

func wait(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}
