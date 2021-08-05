# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.0] - 2021-05-17
### Added
- Added the new `kraken` command to perform project code generation
- Added support for module code generation via the `kraken` command
- Added support for extension code generation via the `kraken` command
- Added support for app entry-point code generation via the `kraken` command
### Changed
- Updated README to document code generation
- Support multiple extension opbjects in one extension package
- Relocated some common tasks for module init into core
### Deprecated
### Removed
- Removed the old, no-longer functional `kraken-builder` command
### Fixed
- Fixed an issue where state lookups for enum v alues would fail with improper URLShift usage

### Security

## [0.1.1] - 2021-04-15
### Added
- Added this changelog (`CHANGELOG.md`)
- Added a submodule for `krakenctl` at [utils/krakenctl](utils/krakenctl)
### Fixed
- Submodules now use relative paths so we don't accidentally

## [0.1.0] - 2021-04-13
### Added
- Semantic versioning started.  Note: this project has been in dev for some time, but never previously versioned.
### Changed
- Migrate from github.com/hpc/kraken to github.com/kraken-hpc/kraken
- Split-out modules & extensions that are not "core" to the kraken framework
