package providers

import "os"

// lookupEnv is the default environment lookup. Defined in its own
// file so it can be overridden in tests via the package-level
// `getenv` variable.
func lookupEnv(key string) string {
	return os.Getenv(key)
}
