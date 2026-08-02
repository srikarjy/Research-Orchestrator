package agents

import (
	"context"
	"sync"
)

type MessageBus interface {
	Subscribe(agentID AgentID, handler MessageHandler) error
	Unsubscribe(agentID AgentID) error
	Publish(ctx context.Context, msg Message) error
	Request(ctx context.Context, msg Message) (Message, error)
}

type MessageHandler func(ctx context.Context, msg Message) (Message, error)

type InMemoryMessageBus struct {
	mu          sync.RWMutex
	subscribers map[AgentID]MessageHandler
	queues      map[AgentID]chan Message
}

func NewInMemoryMessageBus() *InMemoryMessageBus {
	return &InMemoryMessageBus{
		subscribers: make(map[AgentID]MessageHandler),
		queues:      make(map[AgentID]chan Message),
	}
}

func (b *InMemoryMessageBus) Subscribe(agentID AgentID, handler MessageHandler) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subscribers[agentID] = handler
	b.queues[agentID] = make(chan Message, 100)
	return nil
}

func (b *InMemoryMessageBus) Unsubscribe(agentID AgentID) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.subscribers, agentID)
	if ch, ok := b.queues[agentID]; ok {
		close(ch)
		delete(b.queues, agentID)
	}
	return nil
}

func (b *InMemoryMessageBus) Publish(ctx context.Context, msg Message) error {
	b.mu.RLock()
	handler, ok := b.subscribers[msg.To]
	queue, queueOk := b.queues[msg.To]
	b.mu.RUnlock()

	if !ok {
		return ErrAgentNotFound
	}

	if queueOk {
		select {
		case queue <- msg:
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}

	go func() {
		_, _ = handler(ctx, msg)
	}()

	return nil
}

func (b *InMemoryMessageBus) Request(ctx context.Context, msg Message) (Message, error) {
	b.mu.RLock()
	handler, ok := b.subscribers[msg.To]
	b.mu.RUnlock()

	if !ok {
		return Message{}, ErrAgentNotFound
	}

	return handler(ctx, msg)
}

func (b *InMemoryMessageBus) GetQueue(agentID AgentID) (chan Message, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	q, ok := b.queues[agentID]
	return q, ok
}