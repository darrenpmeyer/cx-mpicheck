# Security considerations for `cx-mpicheck`

This document covers the residual operational risks an operator should weigh when running `cx-mpicheck` in CI. The codebase has been hardened against several issues a security review surfaced; what follows is information about accepted and inherent risks and other considerations that DevOps teams and other users may want to weigh when making deployment and configuration decisions.

## Threat model assumed

`cx-mpicheck` is intended to run in CI against a repository whose contents may be partially or fully attacker-controlled (e.g. PRs from external contributors). The CI environment carries the `CHECKMARX_MPIAPI_KEY` and may carry other secrets in env vars. The guidance below is calibrated to that setting; closed/internal repos need correspondingly less caution.

## Live package-manager invocation (`--resolve`, `--fake-lockfile`)

When `--resolve` is `smart`, `always`, or `demand` with `--fake-lockfile`, `cx-mpicheck` invokes external package managers (`npm`, `pnpm`, `pipenv`, `poetry`, `pip-compile`, `go mod tidy`) on the project files in the tree. `npm` and `pnpm` are passed `--ignore-scripts`, but the Python managers have no equivalent and resolving an sdist can execute `setup.py`. `npm`/`pnpm` will also fetch from `git+ssh://` and `git+https://` URLs listed in `dependencies`, which is its own arbitrary-fetch surface.

This is expected behavior — the feature exists because lockfiles aren't always committed, and may not always be current or accurate. The trust model is operator-explicit: **`cx-mpicheck` will not run an external package manager on a project file unless you have asked for it.**

The attack requires an adversary to trick `cx-mpicheck` into running a package manager in a context where we can expect that package manager to run later in the pipeline. As a result, realistically the net change in risk is likely zero in real-world pipelines or desktop deployments.

Recommendations:

- **Public-PR pipelines:** need to limit trust placed in manifest and lockfiles; maintainers should consider:
  - prefer `--resolve=never` and require contributors to commit lockfiles; never rely on manifests for installation, use lockfiles only
  - allow live resolution, but don't install packages or run scans until manifest files have been reviewed
  - environmental controls that restrict access to reduce risk that a compromised pipeline causes significant damage or exfiltrates secrets
- **Internal/trusted repos:** `--resolve=smart` is reasonable; the threat reduces to your own contributors. However, environmental controls are still recommended as a defensive layer.

## Output file paths (`--out-*`)

`--out-packages`, `--out-results`, `--out-risks`, the corresponding `CX_MPICHECK_OUT_*` env vars, and the `.bak` files written when a risk policy filters the report all create files at operator-supplied paths. The paths are **not** validated for containment under `--root` or refusal-to-follow-symlinks. An attacker who can influence the environment of the CI job (poisoned `.envrc`, workflow that propagates repo-controlled inputs into env, etc.) can cause writes anywhere the running user can write.

This is accepted risk: `cx-mpicheck` treats `--out-*` and `CX_MPICHECK_OUT_*` as part of the trusted control plane, the same way it trusts `CHECKMARX_MPIAPI_KEY`.

Recommendations:

- Avoid piping untrusted input into the `CX_MPICHECK_OUT_*` environment variables or onto the `--out-*` flags. Treat them like `PATH` or any other CI configuration.
- File and directory permissions for everything `cx-mpicheck` writes are owner-only on POSIX (0o600 / 0o700). On Windows the effective policy comes from NTFS DACL inheritance — see the comment block in `mpicheck/perms.go`.

## Notes / lower-severity items

- **`--print-risks`** writes MPIAPI's parsed risk records (package metadata, scores, titles, descriptions) to stdout. Default is off. Off is the right setting on shared CI runners where stdout lands in archived logs you don't fully control; persist to the `OutRisks` file and parse out-of-band instead. The MPIAPI key is never echoed on this path (responses are JSON, parsed and re-serialized; success responses do not include the auth header).
- **CI tool versions.** When `cx-mpicheck` invokes `npm`/`pnpm`/etc., it uses whatever version `PATH` provides. Older tool versions may have CVEs of their own. Pin tool versions in your CI image.

## Reporting

If you find an issue not covered here, please open a security advisory on the repository (or contact the maintainer directly) before disclosing publicly.
