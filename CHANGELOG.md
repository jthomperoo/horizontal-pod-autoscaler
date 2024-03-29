# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [v0.9.0] - 2022-05-14
### Changed
- Bump `jthomperoo/custom-pod-autoscaler` to `v2.6.0`.
- Extracted all HPA calculations to separate project <https://github.com/jthomperoo/k8shorizmetrics>.
- Updated Go version to `v1.17`.
### Removed
- Removed all public modules from this project, if depending on this instead use <https://github.com/jthomperoo/k8shorizmetrics>.

## [v0.8.0] - 2021-12-27
### Added
- Support for `argoproj.io/v1alpha1` `Rollout` resource.
### Changed
- Bump `jthomperoo/custom-pod-autoscaler` to `v2.3.0`
- Bump `k8s.io/kubernetes` to `v1.21.8`.
### Fixed
- **BREAKING CHANGE**, the Pod metric evaluation now correctly looks at the `averageValue` metric target field rather
than the incorrect `value` target field.

## [v0.7.0] - 2021-08-15
### Changed
- Update Custom Pod Autoscaler version to `v2.2.0`.
- Update Go version to `v1.16`.

## [v0.6.0] - 2020-08-31
### Changed
- Update Custom Pod Autoscaler version to `v1.0.0`.

## [v0.5.0] - 2020-03-10
### Changed
- Update Custom Pod Autoscaler version to `v0.11.0`.
- Changed `cpuInitializationPeriod`, time now set in seconds rather than minutes.
- Set default `interval` to be `15000` (15 seconds) to match K8s HPA.
- Set default `downscaleStabilization` to be `300` (5 minutes) to match K8s HPA.

## [v0.4.0] - 2020-01-25
### Changed
- Update Custom Pod Autoscaler version to v0.10.0.

## [v0.3.0] - 2019-12-17
### Changed
- Update Custom Pod Autoscaler version to 0.8.0.

## [v0.2.0] - 2019-12-10
### Added
- New configuration options can be set in the YAML config.
    - Can now configure `tolerance` as a configuration option - works the same as the `--horizontal-pod-autoscaler-tolerance` flag, [see here](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/). Default value of 0.1.
    - Can now configure `cpuInitializationPeriod` as a configuration option - works the same as the `--horizontal-pod-autoscaler-cpu-initialization-period` flag, [see here](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/). Time set in minutes, default 5 minutes.
    - Can now configure `initialReadinessDelay` as a configuration option - works the same as the `--horizontal-pod-autoscaler-initial-readiness-delay` flag, [see here](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/). Time set in seconds, default 30 seconds.
### Fixed
- Issues with evaluation decision making looking in the wrong specs for target values.

## [v0.1.0] - 2019-12-08
### Added
- Restructured the Horizontal Pod Autoscaler to work within a Custom Pod Autoscaler.

[Unreleased]: https://github.com/jthomperoo/horizontal-pod-autoscaler/compare/v0.9.0...HEAD
[v0.9.0]: https://github.com/jthomperoo/horizontal-pod-autoscaler/compare/v0.8.0...v0.9.0
[v0.8.0]: https://github.com/jthomperoo/horizontal-pod-autoscaler/compare/v0.7.0...v0.8.0
[v0.7.0]: https://github.com/jthomperoo/horizontal-pod-autoscaler/compare/v0.6.0...v0.7.0
[v0.6.0]: https://github.com/jthomperoo/horizontal-pod-autoscaler/compare/v0.5.0...v0.6.0
[v0.5.0]: https://github.com/jthomperoo/horizontal-pod-autoscaler/compare/v0.4.0...v0.5.0
[v0.4.0]: https://github.com/jthomperoo/horizontal-pod-autoscaler/compare/v0.3.0...v0.4.0
[v0.3.0]: https://github.com/jthomperoo/horizontal-pod-autoscaler/compare/v0.2.0...v0.3.0
[v0.2.0]: https://github.com/jthomperoo/horizontal-pod-autoscaler/compare/v0.1.0...v0.2.0
[v0.1.0]: https://github.com/jthomperoo/horizontal-pod-autoscaler/releases/tag/v0.1.0
