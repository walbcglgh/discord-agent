package tools

import (
	"context"

	"discord-agent/internal/llm"
)

type imagesKey struct{}

type outputsKey struct{}

// Outputs collects side-effect files (e.g. generated mp3) for the caller to
// deliver alongside the text reply.
type Outputs struct {
	Files []string
}

func WithOutputs(ctx context.Context, o *Outputs) context.Context {
	return context.WithValue(ctx, outputsKey{}, o)
}

func AddFile(ctx context.Context, path string) {
	if o, ok := ctx.Value(outputsKey{}).(*Outputs); ok && o != nil {
		o.Files = append(o.Files, path)
	}
}

func WithImages(ctx context.Context, imgs *[]llm.Image) context.Context {
	return context.WithValue(ctx, imagesKey{}, imgs)
}

func AddImage(ctx context.Context, img llm.Image) {
	if p, ok := ctx.Value(imagesKey{}).(*[]llm.Image); ok && p != nil {
		*p = append(*p, img)
	}
}
