package main

// https://stackoverflow.com/questions/77422213/how-to-hide-all-keys-when-using-slog-in-golang

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
)

type Handler struct {
	h   slog.Handler
	mu  *sync.Mutex
	out io.Writer
}

func NewHandler(o io.Writer, opts *slog.HandlerOptions) *Handler {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}
	return &Handler{
		out: o,
		h: slog.NewTextHandler(o, &slog.HandlerOptions{
			Level:       opts.Level,
			AddSource:   opts.AddSource,
			ReplaceAttr: nil,
		}),
		mu: &sync.Mutex{},
	}
}

func (h *Handler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.h.Enabled(ctx, level)
}

func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &Handler{h: h.h.WithAttrs(attrs), out: h.out, mu: h.mu}
}

func (h *Handler) WithGroup(name string) slog.Handler {
	return &Handler{h: h.h.WithGroup(name), out: h.out, mu: h.mu}
}

func (h *Handler) Handle(ctx context.Context, r slog.Record) error {

	formattedTime := r.Time.Format("[2006/01/02 15:04:05]")

	//add time and message to values
	strs := []string{formattedTime}

	if r.NumAttrs() != 0 {
		r.Attrs(func(a slog.Attr) bool {
			value := fmt.Sprintf("[%s]", a.Value.String())
			strs = append(strs, value)
			return true
		})
	}
	strs = append(strs, r.Message)
	strs = append(strs, "\n")

	result := strings.Join(strs, " ")
	b := []byte(result)

	h.mu.Lock()
	defer h.mu.Unlock()

	_, err := h.out.Write(b)

	return err

}
