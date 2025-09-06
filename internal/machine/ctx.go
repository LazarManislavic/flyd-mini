package machine

import (
	"database/sql"

	"github.com/manuelinfosec/flyd/internal/s3"
)

// AppContext holds shared dependencies for FSM state functions.
type AppContext struct {
	DB *sql.DB
	S3 *s3.S3Client
}