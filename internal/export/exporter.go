package export

import (
	"github.com/arinbalyan/scrappy/internal/types"
)

// Exporter is the contract all output targets satisfy.
type Exporter interface {
	Name() string
	Open(path string) error
	WriteJob(j types.JobPosting) error
	Close() error
}
