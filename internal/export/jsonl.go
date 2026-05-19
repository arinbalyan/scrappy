package export

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"github.com/arinbalyan/scrappy/internal/model"
)

func WriteJSONL(path string, jobs []model.JobPost) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create jsonl file: %w", err)
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	defer w.Flush()

	enc := json.NewEncoder(w)
	for _, job := range jobs {
		if err := enc.Encode(job); err != nil {
			return fmt.Errorf("encode job jsonl row: %w", err)
		}
	}

	return nil
}
