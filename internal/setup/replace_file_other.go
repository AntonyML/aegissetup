//go:build !windows

// © Antony Monge López — Costa Rica — Céd. 604700548

package setup

import "os"

func replaceFileAtomic(source, destination string) error {
	return os.Rename(source, destination)
}
