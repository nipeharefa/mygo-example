//go:build e2e

// The automation plugin, so a test runner can drive a built app from
// outside (see the README's smoke test): build with -tags e2e and name a
// socket in MYGO_AUTOMATION. It does nothing when the variable is unset,
// and the tagged-by-default absence keeps it out of every release build.
package main

import _ "github.com/egoist/mygo/plugins/automation"
