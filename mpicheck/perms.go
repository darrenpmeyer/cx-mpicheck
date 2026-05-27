// SPDX-License-Identifier: AGPL-3.0-only
package mpicheck

import "os"

// File permission policy for everything cx-mpicheck writes — JSON
// inventory / results / risks, fake lockfiles, backup files, and the
// directories that hold them.
//
// On POSIX (Linux, macOS, BSDs) these are honored verbatim: 0o600 is
// owner-only read/write, 0o700 is owner-only on directories. No group
// or other access.
//
// On Windows, Go's os package only honors the 0o200 (write) bit when
// translating a Go FileMode to a Windows attribute — specifically, a
// file with no write bits set gets the read-only attribute. There is
// no stdlib API to set ACLs explicitly. New files on NTFS inherit the
// parent directory's DACL, which on a typical user profile already
// restricts access to the owning user account. The constants below are
// therefore a no-op on Windows for confidentiality, but they remain
// the correct cross-platform expression of "owner-only" — the OS layer
// is what diverges, not the policy.
//
// Path validation (containment under cfg.RootDir, refusing symlinks
// for output targets) is intentionally NOT enforced: operators are
// trusted to supply sensible output locations via --out-* flags and
// CX_MPICHECK_OUT_* env vars. These permission bits constrain who can
// read the resulting files; the location of those files is operator
// responsibility.
const (
	// OwnerFileMode is the file mode applied to every output file
	// cx-mpicheck writes.
	OwnerFileMode os.FileMode = 0o600

	// OwnerDirMode is the directory mode for newly-created parent
	// directories that hold those output files.
	OwnerDirMode os.FileMode = 0o700
)
