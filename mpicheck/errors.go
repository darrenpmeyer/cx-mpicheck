// SPDX-License-Identifier: AGPL-3.0-only
package mpicheck

import "fmt"

// Error wraps a failure with an exit code.
type Error struct {
	Code int
	Err  error
}

func (e *Error) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("mpicheck error (code %d)", e.Code)
	}
	return fmt.Sprintf("mpicheck error (code %d): %v", e.Code, e.Err)
}

func (e *Error) Unwrap() error { return e.Err }
