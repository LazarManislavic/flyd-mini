package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/superfly/fsm"

	"github.com/manuelinfosec/flyd/internal/machine"
	"github.com/manuelinfosec/flyd/internal/s3"
	"github.com/manuelinfosec/flyd/internal/storage"
)

const (
	AWSBucket = "flyio-platform-hiring-challenge"
	AWSRegion = "us-east-1"
	DBPath    = "db/flyd.db"
	Schema    = "internal/storage/schema.sql"
)

func main() {
	// Setup root context with cancel on interrupt
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log := logrus.New()
	log.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})

	// Initialize domain database
	db, err := storage.InitDB(Schema, DBPath)
	if err != nil {
		log.Fatalf("Failed to initialize domain database: %v", err)
	}
	defer db.Close()

	// Initialize FSM manager
	manager, err := fsm.New(fsm.Config{
		Logger: log,
		DBPath: "./db",                        // FSM persistence
		Queues: map[string]int{"default": 10}, // concurrency control
	})
	if err != nil {
		log.Fatalf("Failed to initialize FSM manager: %v", err)
	}
	log.Info("FSM manager initialized")

	log.Infof("Domain database initialized at %s", DBPath)

	// Initialize S3 client
	s3Client, err := s3.NewS3Client(ctx, AWSBucket, AWSRegion)
	if err != nil {
		log.Fatalf("Failed to initialize S3 client: %v", err)
	}
	log.Infof("S3 client initialized for bucket=%s, region=%s", AWSBucket, AWSRegion)

	// Build shared app context
	appCtx := &machine.AppContext{
		DB: db,
		S3: s3Client,
	}
	log.Info("AppContext initialized")

	// FSM workflow
	builder := fsm.Register[machine.FSMRequest, machine.FSMResponse](manager, "tasks").
		Start("FetchObject", machine.WithApp(appCtx, machine.FetchObject)).
		To("UnpackLayers", machine.WithApp(appCtx, machine.UnpackLayers)).
		To("RegisterImage", machine.WithApp(appCtx, machine.RegisterImage)).
		To("ActivateSnapshot", machine.WithApp(appCtx, machine.ActivateSnapshot)).
		To("WriteResults", machine.WithApp(appCtx, machine.WriteResults)).
		End("done")

	startFn, resumeFn, err := builder.Build(ctx)
	if err != nil {
		logrus.Fatalf("Failed to build FSM: %v", err)
	}

	activeRuns, err := manager.Active(ctx, "tasks")
	if err == nil {
		for runType, runVersion := range activeRuns {
			logrus.Infof("Active run found: type=%s version=%s", runType, runVersion.String())
		}
	}

	// Resume unfinished runs
	logrus.Info("Resuming any unfinished runs…")
	if err := resumeFn(ctx); err != nil {
		logrus.Errorf("Failed to resume runs: %v", err)
	}

	// Start a new run with random UUID
	runID := uuid.New().String()
	req := fsm.NewRequest(
		&machine.FSMRequest{
			ImageName:  "golang",
			BucketName: AWSBucket,
		},
		&machine.FSMResponse{},
	)

	version, err := startFn(ctx, runID, req)
	if err != nil {
		logrus.Fatalf("FSM start failed: %v", err)
	}
	logrus.Infof("FSM started with runID=%s version=%s", runID, version)

	// Wait until the run finishes
	if err := manager.WaitByID(ctx, runID); err != nil {
		logrus.Errorf("FSM run %s failed: %v", runID, err)
	} else {
		logrus.Infof("FSM run %s completed successfully", runID)
	}

	// Graceful shutdown
	logrus.Info("Shutting down flyd gracefully…")
	manager.Shutdown(10 * time.Second)
}
