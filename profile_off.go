//go:build !memprofile

package main

import "flag"

// Profiling hooks are compiled out of release builds; see profile_on.go.
func profileFlags(*flag.FlagSet) {}

func profileAtExit(*int) {}
