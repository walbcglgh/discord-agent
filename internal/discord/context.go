package discord

import "context"

type messageRefKey struct{}
type pendingKey struct{}

// MessageRef binds a tool call to the message that triggered it.
type MessageRef struct {
	ChannelID string
	MessageID string
	GuildID   string
	UserID    string
	Username  string
	IsAdmin   bool
	Scope     string
}

func withMessageRef(ctx context.Context, ref MessageRef) context.Context {
	return context.WithValue(ctx, messageRefKey{}, ref)
}

func messageRefFrom(ctx context.Context) (MessageRef, bool) {
	ref, ok := ctx.Value(messageRefKey{}).(MessageRef)
	return ref, ok
}

type PendingAction struct {
	ID        string
	Type      string // ban | kick | mute
	UserID    string
	GuildID   string
	Reason    string
	Minutes   int
	ChannelID string
}

type PendingCollector struct{ Items []PendingAction }

func withPending(ctx context.Context, p *PendingCollector) context.Context {
	return context.WithValue(ctx, pendingKey{}, p)
}

func addPending(ctx context.Context, a PendingAction) {
	if p, ok := ctx.Value(pendingKey{}).(*PendingCollector); ok && p != nil {
		p.Items = append(p.Items, a)
	}
}
