package auth

import "context"

type actorKey struct{}

// WithActor returns a copy of ctx carrying a. The HTTP layer calls it once per request.
func WithActor(ctx context.Context, a Actor) context.Context {
	return context.WithValue(ctx, actorKey{}, a)
}

// ActorFrom returns the Actor stored in ctx, or the anonymous Actor.
func ActorFrom(ctx context.Context) Actor {
	a, ok := ctx.Value(actorKey{}).(Actor)
	if !ok {
		return Actor{Kind: Anonymous}
	}
	return a
}
