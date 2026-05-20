package util

import (
	"fmt"
	"io"
	"strings"

	"github.com/arinbalyan/scrappy/internal/model"
)

const DefaultMaxBodyBytes int64 = 4 * 1024 * 1024

func ReadBodyLimited(r io.Reader, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxBodyBytes
	}
	lr := io.LimitReader(r, maxBytes+1)
	b, err := io.ReadAll(lr)
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > maxBytes {
		return nil, fmt.Errorf("response body exceeds %d bytes", maxBytes)
	}
	return b, nil
}

func HasMeaningfulJobs(jobs []model.JobPost) bool {
	if len(jobs) == 0 {
		return false
	}
	for _, j := range jobs {
		t := strings.TrimSpace(j.Title)
		u := strings.TrimSpace(j.JobURL)
		c := strings.TrimSpace(j.CompanyName)
		if t != "" && (u != "" || c != "") {
			return true
		}
	}
	return false
}
