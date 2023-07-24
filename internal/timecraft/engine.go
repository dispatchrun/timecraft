package timecraft

import "context"

type Engine interface {
	Run(ctx context.Context, spec Spec)
}

type Spec struct {
	Path string
	Args []string
	Env  []string
}
