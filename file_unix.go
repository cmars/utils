// Copyright 2013 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

// +build !windows

package utils

import (
	"fmt"
	"os"
	"os/user"
	"strconv"
	"strings"

	"github.com/juju/errors"
)

func homeDir(userName string) (string, error) {
	u, err := user.Lookup(userName)
	if err != nil {
		return "", errors.NewUserNotFound(err, "no such user")
	}
	return u.HomeDir, nil
}

// ReplaceFile atomically replaces the destination file or directory
// with the source. The errors that are returned are identical to
// those returned by os.Rename.
func ReplaceFile(source, destination string) error {
	return os.Rename(source, destination)
}

// MakeFileURL returns a file URL if a directory is passed in else it does nothing
func MakeFileURL(in string) string {
	if strings.HasPrefix(in, "/") {
		return "file://" + in
	}
	return in
}

// ChownPath sets the uid and gid of path to match that of the user
// specified.
func ChownPath(path, username string) error {
	u, err := user.Lookup(username)
	if err != nil {
		return fmt.Errorf("cannot lookup %q user id: %v", username, err)
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return fmt.Errorf("invalid user id %q: %v", u.Uid, err)
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return fmt.Errorf("invalid group id %q: %v", u.Gid, err)
	}
	return os.Chown(path, uid, gid)
}
