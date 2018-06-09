# Inmensus Trading
Monorepo for Inmensus Trading.

# Pipelines & Workflows
## Version Control
VC using git. Include source code, exclude generated code and binaries.

## Branches
Individual contributions to the `staging` branch must be committed to an individual branch first, and commits merged into `staging` later. Preferably, prior to merging into `staging`, another member will conduct code reviews of said commits. This avoids hasty commits which break `staging` as well as painful merge conflicts.

Commits in `staging` can then be merged into `production`, the default branch, once all members have reviewed the code changes. `production` should always contain functional code, and reflect the status of the running server instances.

## Directory Organization
Directories are organized as specified in the master pipeline overview available at (WPI).

# Documentation
All documentation will be organized on the Github Wiki.