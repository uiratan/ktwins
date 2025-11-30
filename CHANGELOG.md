# Changelog

All notable changes to this project will be documented in this file.

## v1.0.0 - 2025-11-30
### Added
- Initial release with `ktwins` CLI dashboard (workloads, network, cluster, metrics pages) and keyboard navigation.
- UI modals for logs/describe, alerts/events popups, and periodic auto-refresh.
- Data layer wrapping `kubectl`/client-go for listings, metrics, events, and summaries.
- Project layout with `cmd/ktwins`, `internal/ui`, `internal/data`, `internal/theme`; Makefile targets and README with screenshots.
- GitHub Actions workflow to build and upload the `ktwins` artifact.
