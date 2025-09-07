package machine

import (
	"context"
	"database/sql"

	"github.com/superfly/fsm"

	"github.com/manuelinfosec/flyd/internal/s3"
)

// AppContext holds shared dependencies for FSM state functions.
type AppContext struct {
	DB *sql.DB
	S3 *s3.S3Client
}


type StepFn func(ctx context.Context, req *fsm.Request[FSMRequest, FSMResponse], app *AppContext) (*fsm.Response[FSMResponse], error)


// Adapter pattern to wrap domain-specific state functions into the 
// signature expected by the FSM library. The AppContext is provided
// via dependency injection, which keeps state logic clean.
func WithApp(app *AppContext, fn StepFn) func(context.Context, *fsm.Request[FSMRequest, FSMResponse]) (*fsm.Response[FSMResponse], error) {
	return func(ctx context.Context, req *fsm.Request[FSMRequest, FSMResponse]) (*fsm.Response[FSMResponse], error) {
		return fn(ctx, req, app)
	}
}
