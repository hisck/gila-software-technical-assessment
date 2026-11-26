package orders

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"ecommerce/internal/contract"
)

type sagaState struct {
	status           string
	paymentRequested bool
	releaseRequested bool
}

type sagaPath struct {
	state sagaState
	trace []string
}

func TestOrderSagaModel(t *testing.T) {
	topics := []string{
		contract.InventoryReserved,
		contract.InventoryRejected,
		contract.PaymentApproved,
		contract.PaymentDeclined,
		contract.InventoryReleased,
	}
	waiting := sagaState{status: awaitingPayment}
	initial := sagaState{status: pendingStock}
	seen := map[sagaState]bool{waiting: true, initial: true}
	queue := []sagaPath{
		{state: waiting, trace: []string{awaitingPayment}},
		{state: initial, trace: []string{pendingStock}},
	}
	edges := 0
	for len(queue) > 0 {
		source := queue[0]
		queue = queue[1:]
		for _, topic := range topics {
			edges++
			next, command := orderTransition(source.state.status, topic)
			display := next
			if display == "" {
				display = source.state.status
			}
			edge := fmt.Sprintf("--%s--> %s", topic, display)
			if command != "" {
				edge += " [" + command + "]"
			}
			trace := append(slices.Clone(source.trace), edge)
			target, err := checkSagaStep(source.state, topic, next, command)
			if err != nil {
				t.Fatalf("%v\ncounterexample: %s", err, strings.Join(trace, " "))
			}
			if !seen[target] {
				seen[target] = true
				queue = append(queue, sagaPath{state: target, trace: trace})
			}
		}
	}

	for _, goal := range []struct {
		name  string
		state sagaState
	}{
		{"approved purchase", sagaState{status: paid, paymentRequested: true}},
		{"inventory rejection", sagaState{status: rejected}},
		{"compensated payment decline", sagaState{status: rejected, paymentRequested: true, releaseRequested: true}},
	} {
		if !seen[goal.state] {
			t.Errorf("required path is unreachable: %s", goal.name)
		}
	}
	t.Logf("explored %d reachable states and %d delivery edges across %d inbound topics", len(seen), edges, len(topics))
}

func checkSagaStep(from sagaState, event, next, command string) (sagaState, error) {
	if from.status == awaitingPayment && (next != "" || command != "") {
		return from, fmt.Errorf("order started before a payment outcome was selected")
	}
	if (from.status == paid || from.status == rejected) && (next != "" || command != "") {
		return from, fmt.Errorf("terminal order changed state or emitted a command")
	}
	switch next {
	case "":
		if command != "" {
			return from, fmt.Errorf("ignored delivery emitted %s", command)
		}
		return from, nil
	case pendingPayment:
		if event != contract.InventoryReserved || from.status != pendingStock || from.paymentRequested ||
			command != contract.PaymentRequested {
			return from, fmt.Errorf("payment must be requested once, after stock is reserved")
		}
		from.paymentRequested = true
	case paid:
		if event != contract.PaymentApproved || from.status != pendingPayment || !from.paymentRequested ||
			command != contract.InventoryCommit {
			return from, fmt.Errorf("stock commit requires an approved, requested payment")
		}
	case releasingStock:
		if event != contract.PaymentDeclined || from.status != pendingPayment || !from.paymentRequested ||
			from.releaseRequested || command != contract.InventoryRelease {
			return from, fmt.Errorf("a declined payment must request a stock release once")
		}
		from.releaseRequested = true
	case rejected:
		if command != "" {
			return from, fmt.Errorf("a rejected order emitted %s", command)
		}
		if from.paymentRequested {
			if event != contract.InventoryReleased || from.status != releasingStock || !from.releaseRequested {
				return from, fmt.Errorf("a declined payment became final before stock release was confirmed")
			}
		} else if event != contract.InventoryRejected || from.status != pendingStock {
			return from, fmt.Errorf("an order without a payment may be rejected only when inventory is rejected")
		}
	default:
		return from, fmt.Errorf("invalid state %q", next)
	}
	from.status = next
	return from, nil
}
