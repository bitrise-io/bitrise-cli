# Git History Review — Pre-Open-Source Secret Audit

**Review date:** 2026-06-03

## Scope

Full audit of the entire `bitrise-cli` git history for secrets and other
sensitive data, performed before making the repository public. The audit
covered **all refs** — every local and remote-tracking branch, plus a check for
tags, stashes, and dangling/unreachable commits.

- Commits in history: **120** total (100 non-merge + 20 merge commits).
- Commit date range: **2026-05-06** → **2026-06-03**.
- Refs scanned (all branches; no tags or stashes exist):
  - `main`
  - `RDE-257-view-logs-feature`
  - `add-oauth-login-docs`
  - `claude/build-log-contiguous-sink`
  - `claude/build-log-contiguous-with-tui`
  - `claude/fix-watch-log-ordering`
  - `claude/sad-banzai-f14512`
  - `gate-non-ga-commands`
- `git stash list`: empty.
- `git fsck --unreachable --no-reflogs`: no unreachable commits.
- No binary blobs and no files larger than 50 KB were ever committed; the
  history is entirely small text source files (622 unique blobs, 266 distinct
  file paths ever added).

## Tools used and versions

| Tool | Version | Invocation |
|------|---------|------------|
| gitleaks | 8.30.1 | `gitleaks detect --source . --log-opts="--all" --report-format json --report-path /tmp/gitleaks-report.json` |
| trufflehog | 3.95.5 | `trufflehog git file://. --results=verified,unverified,unknown --json` |
| git | 2.54.0 | manual `git log --all` / `git cat-file` blob sweeps (see below) |

> Note on the trufflehog flag: the task's suggested `--only-verified=false` is
> not accepted by trufflehog 3.95.5 (the flag is now boolean). The equivalent
> behavior — emit **all** detections regardless of verification status — is
> `--results=verified,unverified,unknown`, which is what was run.

### Automated scan results

- **gitleaks:** `100 commits scanned`, **`no leaks found`** (report is an empty
  JSON array `[]`). The 100 vs. 120 difference is expected — gitleaks scans the
  100 content-bearing non-merge commits and skips the 20 merge commits, which
  introduce no new content.
- **trufflehog:** `1158 chunks` / `1.17 MB` scanned, **`verified_secrets: 0`,
  `unverified_secrets: 0`** — no findings of any kind.

### Manual sweeps performed

1. Commit messages and bodies across all refs (`git log --all --format`) for
   `password|secret|token|api_key|private_key|credential|bearer`.
2. Full diff history (`git log --all -p`) for credential **assignment** patterns
   (`password=`, `secret=`, `token=`, `api_key=`, `client_secret=`).
3. Full diff history for real secret **formats**: AWS keys (`AKIA…`), GitHub
   tokens (`ghp_`, `gho_`, `github_pat_`), Slack tokens (`xox…`), OpenAI keys
   (`sk-…`), Google API keys (`AIza…`), PEM private-key headers, and JWTs
   (`eyJ….….`). **Zero matches.**
4. Full diff history for internal/staging hostnames (`*.internal.*`,
   `staging.*`, `*.stg.*`, `*.corp.*`, `.bitrise.io`).
5. Full-content scan of **every one of the 622 unique blobs** in history
   (`git cat-file blob`) for credential assignments with non-placeholder values.
6. Filename sweep of all 266 paths ever added for sensitive names
   (`.env`, `.pem`, `.p12`, `.key`, `id_rsa`, `.netrc`, `credentials`,
   `.kubeconfig`, `serviceaccount`, etc.). **Zero matches.**
7. Author/committer email and Bitrise PAT (`bitpat_`) format checks.

---

## Section A — Findings requiring action before going public

**None.** No live credentials, private keys, internal-only hostnames, or other
sensitive data were found in any commit, on any branch, in commit messages, or
in any historical file blob.

---

## Section B — False positives / benign patterns

Every match surfaced by the automated tools and manual sweeps was reviewed and
determined to be safe. Grouped by category:

### B1. Test fixtures and placeholder values

These are obviously fake values used in unit tests and table-driven test cases —
short, non-conforming to any real credential format, and frequently named to
advertise that they are stubs:

- `Token: "tok"` / `slug: "tok"` — repeated test struct fixtures.
- `Token: "secret-pat-123"`, `"token":"bitpat_real"`, `"token":"bitpat_xyz"` —
  fake Bitrise personal-access-token placeholders in HTTP test servers.
- `const wantToken = "csrf-meta-value-abc"` — explicitly annotated in-source
  with `//nolint:gosec // G101: test fixture string, not a credential`.
- `VNCPassword: "hunter2"`, `VNCPassword: "p"`, `SSHPassword`/`VNCPassword`
  values in tests against `host.example` / `h:5900` (RFC-6761 reserved
  example hostnames).
- `[]string{"token=ghp_x"}` — `ghp_x` is a 5-char placeholder, not a real
  40-char GitHub PAT.
- Test/doc emails: `a@b.io`, `hunter2@host.example`, `p@host.example`,
  `pw%40with%3Aspecial@host.example` (a URL-encoding test case).

### B2. Public, documented Bitrise endpoints

The CLI talks to Bitrise's **public production** API/web endpoints. These are
published in Bitrise's own API docs and are not sensitive:

- `https://api.bitrise.io`, `https://api.bitrise.io/v0.1`,
  `https://api.bitrise.io/rde` — public REST API base URLs (defaults/constants).
- `https://app.bitrise.io`, `https://app.bitrise.io/oidc/token` — public web /
  OIDC token endpoints used by the login flow.
- `https://api.staging.example.com` — a **test fixture** base URL using the
  reserved `example.com` domain, not a real Bitrise staging host.

No `*.internal.*`, `*.corp.*`, or real `*.staging.*` infrastructure hostnames
appear anywhere in history.

### B3. Identifiers, env-var names, and field names (not values)

- `EnvToken = "BITRISE_TOKEN"`, `GITHUB_TOKEN: $GIT_BOT_USER_ACCESS_TOKEN`,
  `subject_token=<JWT>` (literal placeholder in docs) — these are environment
  variable **names** / references, not secret values. The release-workflow note
  in a commit message references a `GIT_BOT_USER_ACCESS_TOKEN` app secret by
  name; the value is stored in Bitrise, not in the repo.
- `IsSecret`, `SSHPassword`, `VNCPassword`, `BuildTriggerToken`, `r.Token`,
  `client_secret` — Go struct field names and config-read code
  (`firstNonEmpty(os.Getenv(EnvToken), …)`), not literals.
- `--secret-input api-key=VALUE`, `--saved-input gh-token=SAVED_INPUT_ID` —
  CLI help/usage text with uppercase placeholders.

### B4. Committer email addresses in commit metadata (expected, benign)

Some commits were authored with contributors' personal addresses
(`imre.kelenyi@gmail.com`, `rostas.balazs@gmail.com`, `trapacska@gmail.com`)
rather than `@bitrise.io` / GitHub `noreply` addresses. These appear **only in
git author/committer metadata, never in any file content**, and are inherent to
the commits that are already being published — this is normal and expected for
any git repository going public, and is **not a secret**.

> Optional, non-blocking: if the team prefers contributors not expose personal
> email addresses, this can be normalized with a `.mailmap` (cosmetic, no
> history rewrite) or scrubbed with `git filter-repo --mailmap` (history
> rewrite). This is a contributor-privacy preference, not a security finding —
> see the optional template in Section D.

---

## Section C — Overall recommendation

### ✅ Safe to make public

Two independent secret scanners (gitleaks 8.30.1, trufflehog 3.95.5) plus
targeted manual sweeps over the **complete** history — all branches, all 120
commits, all 622 blobs, commit messages, filenames, stash, and dangling
objects — produced **zero** findings of live credentials or sensitive
infrastructure data. Everything flagged was a test fixture, a placeholder, a
public endpoint, an env-var name, or a struct field name.

No history rewrite is required before open-sourcing.

---

## Section D — Cleanup commands (if cleanup needed)

**Not applicable** — no cleanup is required and **no history rewrite is
recommended**.

The templates below are provided **for reference only**, in case the team later
chooses to act on the *optional* contributor-email normalization noted in B4.
They are **not** required for the security posture of this repo.

> ⚠️ Any of these rewrite SHAs across the entire history. They must not be run
> until every contributor has been notified, all open PRs/branches are merged or
> closed, and a backup of the repo exists. **Do not force-push as part of this
> review** — that decision belongs to the repo owners. Run all commands on a
> fresh, full mirror clone.

### Prerequisite

```bash
# Install git-filter-repo (https://github.com/newren/git-filter-repo)
pip install git-filter-repo   # or: brew install git-filter-repo

# Work on a fresh full mirror, never your working clone:
git clone --mirror https://github.com/bitrise-io/bitrise-cli.git
cd bitrise-cli.git
```

### Option 1 (recommended if anything is ever needed): cosmetic `.mailmap` — NO history rewrite

```bash
# Add a .mailmap at the repo root mapping personal emails to canonical ones, e.g.:
#   Imre Kelényi <imre.kelenyi@bitrise.io> <imre.kelenyi@gmail.com>
# Tools (git shortlog, GitHub) then display the canonical identity without
# rewriting any commit. This is the safest option and changes no SHAs.
```

### Option 2 (only if a true rewrite is mandated): rewrite committer emails

```bash
# Map old author/committer emails to canonical ones across ALL history.
# This REWRITES every commit SHA.
git filter-repo --mailmap ../mailmap.txt
# where ../mailmap.txt contains lines like:
#   Imre Kelényi <imre.kelenyi@bitrise.io> <imre.kelenyi@gmail.com>
#   Balázs Rostás <balazs.rostas@bitrise.io> <rostas.balazs@gmail.com>
#   Tamas Papik   <tamaspapik@bitrise.io>    <trapacska@gmail.com>
```

### Generic template — purge a specific secret string from all history

```bash
# If a real secret were ever found (none was here), it would be scrubbed like so.
# Put one search==>replace rule per line in replacements.txt, e.g.:
#   literal:THE_LEAKED_SECRET==>REDACTED
git filter-repo --replace-text ../replacements.txt
```

### Generic template — remove a sensitive file from all history

```bash
# If a sensitive file had ever been committed (none was here):
git filter-repo --path path/to/secret-file --invert-paths
```

### After any rewrite

```bash
# Re-run BOTH scanners to confirm the rewrite worked and introduced nothing new:
gitleaks detect --source . --log-opts="--all"
trufflehog git file://. --results=verified,unverified,unknown
# Then coordinate the force-push with the repo owners (NOT part of this review).
```

---

## Section E: Content & Reputation Review

A second pass (added 2026-06-03) over the **same full scope** — all 8 branches,
all 120 commits, all 623 unique blobs, and every commit message — checking for
content that is not a credential but could be embarrassing, unprofessional, or
expose internal IP if published. Method: curated word-list grep across every
blob and every commit message (offensive language, code-smell markers,
disparaging/sarcastic prose, competitor names), plus targeted searches for
unreleased-feature signals, pricing/business metrics, internal
architecture/links, and legal/compliance notes.

**Headline:** No offensive language, no embarrassing comments, and no
business/financial data anywhere. The only substantive findings are **internal
architecture / IP disclosures** — none are secrets, but several reference an
internal repo name, internal links, and an unreleased-feature design. Crucially,
**the two highest-value disclosures live only on feature branches, not on
`main`.** The single most effective mitigation is therefore to **publish only
`main` and not push the working/feature branches to the public repo.**

### E1. Offensive / inappropriate language

| Commit SHA | Location | Finding | Severity |
|------------|----------|---------|----------|
| — | commit messages (all refs) | No profanity or slurs found | Info |
| — | all 623 blobs | No profanity or slurs found | Info |

**Result: clean.** A word-boundaried scan for common English curse words and
racial/ethnic/identity slurs (redaction would look like `f***` — none to
redact) returned zero matches in commit messages and in file content.

### E2. Embarrassing comments

| Commit SHA | Location | Finding | Severity |
|------------|----------|---------|----------|
| — | all blobs | No genuine `TODO`/`FIXME`/`HACK`/`XXX`/`WTF`/`KLUDGE` markers in any file | Info |
| (commit msg) | squash-merge commit body | Prose bullet "Drop the release-gating **TODO** from goreleaser config" — refers to a pending *task*, not a literal code marker; no `TODO` token was ever added or removed in any diff | Info |
| `2daf6e8` et al. | `cmd/purr_test.go` | `// Animation looks **ugly** if frames have different heights…` — legitimate technical comment about ASCII-animation rendering | Info (false positive) |
| `0227b4f` et al. | `bitriseapi/builds.go` | `// BuildLogChunk is one **piece of** build log output.` — benign; matched the "piece of" pattern | Info (false positive) |
| — | all refs | No disparaging remarks about customers, competitors, or colleagues; **no competitor product names** (CircleCI, Travis, Jenkins, GitLab CI, Buildkite, …) appear anywhere | Info |

**Result: clean.** Nothing unprofessional or disparaging. The few matches were
ordinary engineering prose.

> Aside — `bitrise-cli purr` (`cmd/purr.go`) is a deliberate cat-mascot ASCII
> animation easter-egg command. It is harmless and intentional, not an
> embarrassing leftover.

### E3. Intellectual property / sensitive business logic

No pricing, revenue, churn, or other business-metric data was found. No
legal/compliance notes hinting at suppressed issues (the only "liability" hit is
standard MIT-license boilerplate in `LICENSE`). The findings below are
internal-architecture / IP disclosures — **not secrets**, but worth a maintainer
decision before publishing.

| Commit SHA | Location | Finding | Severity |
|------------|----------|---------|----------|
| `2d7a0b5`, `d52a5a9` | `docs/oauth-login.md` — **branch `add-oauth-login-docs` only (not main)** | Internal OAuth/OIDC design doc. Exposes: an internal **Confluence** link (`atlassian.net/wiki/spaces/RD/pages/5026775055`, "ER-2774" security review); references to internal/likely-private repo PRs (`bitrise-mcp#62`, `bitrise-website#19554`); internal backend names (`bitrise-website` monolith, Ruby `OIDC::ExchangeOauthToken`, `TokenController`, `global_settings.yml`, `PAT_EXPIRY`); and detailed server-side token-exchange design incl. the `aud → description` client-allowlist mechanism. | **Medium** |
| `9d02e7c` | `cmd/root.go`, `CLAUDE.md` — **branch `gate-non-ga-commands` only (not main)** | Reveals **unreleased / non-GA command namespaces** (`build`, `app`, `user`, `stack`, `purr`, `step`, `yml`) and documents that release builds only *hide* them from `--help`/completion — they "still run if invoked explicitly." Discloses unshipped roadmap surface. | Low |
| `c81c76e` | `internal/app/create.go:205`, `internal/app/create_test.go:219` — **on `main`** | Code comment names the internal monolith repo and an internal config path: "Values come from **`bitrise-website/config/bitrise_ymls/custom_config.yml`**". | Low |
| `19d381c` | `internal/user/service.go:172`, `service_test.go` — **on `main`** | Comment exposes server-side (Ruby) validation internals: "`registration_type` must be one of `%w[manual login]` (UserAuthToken model)… trips inclusion validation → 422." Reveals backend model/validation behavior. | Low |
| `19d381c` | `internal/user/service.go:5,127,168,180` — **on `main`** | Hardcodes internal/undocumented backend endpoints (`POST /users/sign_in`, `POST /me/profile/security/user_auth_tokens`, `/me/profile/security`). Inherent to the CLI's operation (observable from traffic), so low sensitivity. | Info |
| `f3ac930` | `CLAUDE.md:9` — **on `main`** | "(Bitrise maintainers: it lives in the team **Confluence** space.)" — generic mention of an internal patterns-guide doc; no URL. | Info |
| `2d7a0b5` | `docs/cli/bitrise-cli_rde_template_create.md` — feature branch | Example feature flag `enable_beta_simulator` ("Boot the iOS beta simulator on session start") in RDE template docs — illustrative example data, not a real toggle. | Info |

### E4. Recommendation (content review)

The **secret-scan verdict in Section C is unchanged: no credentials → safe to
publish from a leak standpoint.** This content pass adds governance guidance, it
does not contradict Section C:

1. **Publish `main` only; do not push the feature/working branches**
   (`add-oauth-login-docs`, `gate-non-ga-commands`, `RDE-257-view-logs-feature`,
   `claude/*`). This alone removes the only **Medium** finding (the OAuth design
   doc with the Confluence/security-review link and internal PR refs) and the
   non-GA roadmap exposure, with zero history rewriting. When open-sourcing,
   pushing a single clean `main` to the new public remote is the norm anyway.
2. **Maintainer decision on the `main` references to the internal monolith.**
   The `bitrise-website` repo name (`internal/app/create.go`), the Ruby
   `registration_type` validation note (`internal/user/service.go`), and the
   `CLAUDE.md` Confluence mention are **Low/Info** — internal but not secret, and
   common in OSS client code. If the team prefers to keep the monolith name and
   server internals private, lightly reword those three comments before the
   first public push (a normal edit on `main`, no history rewrite needed since
   the public repo starts fresh from the published tip). If a squash-to-single-
   initial-commit is chosen for the public repo, these comments are the only
   content worth a quick editorial pass first.
3. No action required for E1, E2, or the Info-level E3 rows.

> Note: this report file (`docs/history-review.md`) itself contains pattern
> words ("secret", "password", ".kubeconfig", "do not force-push", the internal
> names quoted above) and will be flagged by re-runs of these scans. It lives on
> the `BACKEND-565/history-review` branch and is an internal audit artifact — it
> is **not** intended to ship in the public `main`, so it does not need scrubbing
> and is not itself a finding.
