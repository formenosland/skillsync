# Security policy

## Supported versions

Security fixes are provided for the latest release tag and the `main` branch. Older tags are not supported unless noted in a release advisory.

## Reporting a vulnerability

Please report security issues **privately** — do not open a public GitHub issue for undisclosed vulnerabilities.

Email: **security@formenos.land**

Include a description of the issue, steps to reproduce, and impact if known. We will acknowledge receipt and work with you on coordinated disclosure.

## Scope notes

skillsync clones and reads **user-configured skill sources** (git repositories and local paths declared in your manifest). Treat source URLs and paths as untrusted input in automation. The tool only creates and removes **symlinks** in its store and agent view directories; it does not delete skill files in source trees.
