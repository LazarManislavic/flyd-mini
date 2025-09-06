package main

// CLI entrypoint

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/superfly/fsm"

	"github.com/manuelinfosec/flyd/internal/storage"
)

func main() {
	_, err := storage.InitDB("internal/storage/schema.sql", "db/flyd.db")
	if err != nil {
		logrus.Fatalf("fatal error: %v", err)
	}

	_, err = fsm.New(
		fsm.Config{
			Logger: logrus.New(),	// logging transitions/errors
			DBPath: "./db",	// persistence
			Queues: map[string]int{"default": 10},	// concurrency control
		},
	)
	if err != nil {
		logrus.Fatalf("fatal error: %v", err)
	}

	fmt.Println("flyd starting...")
	os.Exit(0)
}