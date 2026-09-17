# Security Policy

## Supported versions

Only the latest tagged release receives security fixes.

## Reporting a vulnerability

Please do **not** open a public issue for a security problem.

Use GitHub's private vulnerability reporting on this repository
(Security → Report a vulnerability), which notifies the maintainer
directly and keeps the report confidential until a fix is ready.

## What counts as a security issue here

This tool writes `.github/dependabot.yml` in your repositories. Report:

- Any input (existing config, repository layout, workflow file) that makes
  the tool write content the user did not intend
- Configs the tool rewrites when it promised to stay suggest-only
  (unknown constructs, unparseable YAML, invalid entries)
- Injection from repository content into the generated YAML
- Anything in the `--enable-security-fixes` GitHub API path that leaks the
  token (logs, error messages, structured context)
