package main

// CLI entrypoint

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"
	"github.com/superfly/fsm"

	"github.com/manuelinfosec/flyd/internal/machine"
	"github.com/manuelinfosec/flyd/internal/s3"
	"github.com/manuelinfosec/flyd/internal/storage"
)

const AWS_BUCKET_NAME = "flyio-platform-hiring-challenge"
const AWS_REGION = "us-east-1"

func main() {
	// Root context with cancellation on interrupt (shared by FSM and S3)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	manager, err := fsm.New(
		fsm.Config{
			Logger: logrus.New(),                  // logging transitions/errors
			DBPath: "./db",                        // internal persistence
			Queues: map[string]int{"default": 10}, // concurrency control
		},
	)
	if err != nil {
		logrus.Fatalf("fatal error: %v", err)
	}

	logrus.Info("Internal database intiailzed")

	// Initialize domain database with schema
	db, err := storage.InitDB("internal/storage/schema.sql", "db/flyd.db")
	if err != nil {
		logrus.Fatalf("fatal error: %v", err)
	}
	defer db.Close()

	logrus.Info("Domain database initialized")

	// Initialize (Anonymous) S3 client
	s3Client, err := s3.NewS3Client(ctx, AWS_BUCKET_NAME, AWS_REGION)
	if err != nil {
		logrus.Fatalf("fatal error: %v", err)
	}

	logrus.Info("S3 Client intitialized")

	// Build shared app context
	appCtx := &machine.AppContext{
		DB: db,
		S3: s3Client,
	}

	logrus.Info("AppContext initialized")

	builder := fsm.Register[machine.FSMRequest, machine.FSMResponse](manager, "tasks").
		Start("fetch", machine.WithApp(appCtx, machine.FetchObject)).
		To("test_transition", machine.WithApp(appCtx, machine.UnpackLayers)).
		End("done")

	// Transitions
	startFn, _, err := builder.Build(ctx)
	if err != nil {
		logrus.Fatalf("fatal error: %v", err)
	}

	req := fsm.NewRequest(
		&machine.FSMRequest{
			ImageName: "golang", // logical blob family
			BucketName: AWS_BUCKET_NAME,
		},
		&machine.FSMResponse{},
	)

	runID, err := startFn(ctx, "unique-run-4", req)
	if err != nil {
		logrus.Fatalf("fatal error: %v", err)	
	}


	fmt.Println(runID)

	// logrus.Info("flyd closing...")
	// os.Exit(0)

	// Block until signal
	<-ctx.Done()
	logrus.Info("shutting down gracefully…")
}
