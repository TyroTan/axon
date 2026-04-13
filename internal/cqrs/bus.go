// Package cqrs provides lightweight in-process command and query buses.
// Handlers are registered at construction time; dispatch is type-safe via generics.
//
// Design intent: each Command/Query type is unique; the bus dispatches to exactly
// one registered handler. Panic on unregistered type to surface wiring errors early.
package cqrs

import (
	"context"
	"fmt"
	"reflect"
)

// ─── Command Bus ──────────────────────────────────────────────────────────────

// CommandHandler handles a specific command type C and returns error.
type CommandHandler[C any] interface {
	Handle(ctx context.Context, cmd C) error
}

// CommandBus dispatches commands to their registered handlers.
type CommandBus struct {
	handlers map[reflect.Type]func(context.Context, any) error
}

func NewCommandBus() *CommandBus {
	return &CommandBus{handlers: make(map[reflect.Type]func(context.Context, any) error)}
}

// Register adds a handler for command type C.
func Register[C any](bus *CommandBus, h CommandHandler[C]) {
	var zero C
	t := reflect.TypeOf(zero)
	bus.handlers[t] = func(ctx context.Context, cmd any) error {
		return h.Handle(ctx, cmd.(C))
	}
}

// Dispatch sends cmd to its handler. Panics if no handler is registered.
func (b *CommandBus) Dispatch(ctx context.Context, cmd any) error {
	t := reflect.TypeOf(cmd)
	h, ok := b.handlers[t]
	if !ok {
		panic(fmt.Sprintf("cqrs: no command handler registered for %s", t))
	}
	return h(ctx, cmd)
}

// ─── Query Bus ────────────────────────────────────────────────────────────────

// QueryHandler handles query type Q and returns result type R.
type QueryHandler[Q, R any] interface {
	Handle(ctx context.Context, q Q) (R, error)
}

// QueryBus dispatches queries to their registered handlers.
type QueryBus struct {
	handlers map[reflect.Type]func(context.Context, any) (any, error)
}

func NewQueryBus() *QueryBus {
	return &QueryBus{handlers: make(map[reflect.Type]func(context.Context, any) (any, error))}
}

// RegisterQuery adds a handler for query type Q returning R.
func RegisterQuery[Q, R any](bus *QueryBus, h QueryHandler[Q, R]) {
	var zero Q
	t := reflect.TypeOf(zero)
	bus.handlers[t] = func(ctx context.Context, q any) (any, error) {
		return h.Handle(ctx, q.(Q))
	}
}

// Ask dispatches q and returns the result. Panics if no handler is registered.
// Callers type-assert the returned any to the expected result type.
func (b *QueryBus) Ask(ctx context.Context, q any) (any, error) {
	t := reflect.TypeOf(q)
	h, ok := b.handlers[t]
	if !ok {
		panic(fmt.Sprintf("cqrs: no query handler registered for %s", t))
	}
	return h(ctx, q)
}

// Ask is a typed helper that infers the result type R.
func Ask[Q, R any](ctx context.Context, bus *QueryBus, q Q) (R, error) {
	raw, err := bus.Ask(ctx, q)
	if err != nil {
		var zero R
		return zero, err
	}
	return raw.(R), nil
}
