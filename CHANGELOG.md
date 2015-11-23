# Change Log

## [1.0.0](https://github.com/grammarly/rocker/tree/1.0.0) (2015-11-23)
[Full Changelog](https://github.com/grammarly/rocker/compare/0.2.3...1.0.0)

**Implemented enhancements:**

- Ability to lookup images by fuzzy semver tags [\#46](https://github.com/grammarly/rocker/issues/46)
- rocker/template: `image` helper that can read artifacts [\#45](https://github.com/grammarly/rocker/issues/45)
- Export artifacts as a build result [\#44](https://github.com/grammarly/rocker/issues/44)
- do not create .rockerignore for user [\#27](https://github.com/grammarly/rocker/issues/27)
- Read Rockerfile from STDIN [\#15](https://github.com/grammarly/rocker/issues/15)
- Rewrite MOUNT and $GIT\_SSH\_KEY readme due to the new template engine [\#14](https://github.com/grammarly/rocker/issues/14)
- making rocker requires rocker [\#1](https://github.com/grammarly/rocker/issues/1)
- V1 - rewrite Rocker from scratch, completely client-driven [\#50](https://github.com/grammarly/rocker/pull/50) ([ybogdanov](https://github.com/ybogdanov))

**Fixed bugs:**

- v1: "COPY a\*.js ./" is not working properly [\#48](https://github.com/grammarly/rocker/issues/48)
- Ability to update $PATH env variable \(compatibility with docker\) [\#42](https://github.com/grammarly/rocker/issues/42)
- WARN\[0000\] Tar: Can't archive a file with includes [\#40](https://github.com/grammarly/rocker/issues/40)
- Make in the rocker's Rockerfile does not work with Go 1.5  [\#26](https://github.com/grammarly/rocker/issues/26)

## [0.2.3](https://github.com/grammarly/rocker/tree/0.2.3) (2015-11-23)
[Full Changelog](https://github.com/grammarly/rocker/compare/0.2.2...0.2.3)

**Implemented enhancements:**

- Would be nice to get results of `rocker show` in creation time order [\#25](https://github.com/grammarly/rocker/issues/25)
- \[experiment\] make rocker create a light semver aliases for published tags [\#9](https://github.com/grammarly/rocker/issues/9)
- rocker/template: include [\#37](https://github.com/grammarly/rocker/issues/37)
- Store information about pushed images as artifact files [\#35](https://github.com/grammarly/rocker/issues/35)
- rocker/template: load vars from file [\#34](https://github.com/grammarly/rocker/issues/34)
- rocker/template: call strings helper "indexOf" instead of "index" [\#33](https://github.com/grammarly/rocker/issues/33)
- Hightlight INCLUDE for sublime text language [\#29](https://github.com/grammarly/rocker/issues/29)
- rocker/template: other template string helpers [\#22](https://github.com/grammarly/rocker/issues/22)
- rocker/template: shell helper [\#20](https://github.com/grammarly/rocker/issues/20)
- rocker/template: yaml helper [\#19](https://github.com/grammarly/rocker/issues/19)
- rocker/template: json helper [\#18](https://github.com/grammarly/rocker/issues/18)
- Do not fail on gathering git info, give a warning instead [\#17](https://github.com/grammarly/rocker/issues/17)
- rocker/template: load variable content from a file [\#13](https://github.com/grammarly/rocker/issues/13)

**Fixed bugs:**

- Randomly appearing IMPORT/EXPORT problem in rocker [\#8](https://github.com/grammarly/rocker/issues/8)
- Image fails to parse if registry is an ip with a port [\#24](https://github.com/grammarly/rocker/issues/24)
- Adopt image name splitting logic from docker [\#41](https://github.com/grammarly/rocker/pull/41) ([fxposter](https://github.com/fxposter))

**Closed issues:**

- --attach skips through cached layers [\#39](https://github.com/grammarly/rocker/issues/39)
- how to def default value in template [\#38](https://github.com/grammarly/rocker/issues/38)
- Multiple MOUNTs have stange behaviour [\#31](https://github.com/grammarly/rocker/issues/31)
- \[feature request\] add hooks support [\#3](https://github.com/grammarly/rocker/issues/3)

**Merged pull requests:**

- Fix Windows inability to handle a tilde as the home directory [\#43](https://github.com/grammarly/rocker/pull/43) ([tyrken](https://github.com/tyrken))
- ability to create artifacts without push images to regestry [\#36](https://github.com/grammarly/rocker/pull/36) ([ctrlok](https://github.com/ctrlok))
- \#22 integrate Go's string functions to the template [\#23](https://github.com/grammarly/rocker/pull/23) ([ybogdanov](https://github.com/ybogdanov))
- Merge template functions collected in Dev branch [\#21](https://github.com/grammarly/rocker/pull/21) ([ybogdanov](https://github.com/ybogdanov))

## [0.2.2](https://github.com/grammarly/rocker/tree/0.2.2) (2015-09-17)
[Full Changelog](https://github.com/grammarly/rocker/compare/0.2.1...0.2.2)

**Fixed bugs:**

- ATTACH does not restore the terminal from the raw mode when finished [\#11](https://github.com/grammarly/rocker/issues/11)

**Closed issues:**

- `default` templating function is not present in rocker but present in rocker-compose [\#12](https://github.com/grammarly/rocker/issues/12)
- Add `id` parameter to rocker to override default Rockerfile cache key [\#6](https://github.com/grammarly/rocker/issues/6)

**Merged pull requests:**

- implements \#9 [\#10](https://github.com/grammarly/rocker/pull/10) ([ybogdanov](https://github.com/ybogdanov))

## [0.2.1](https://github.com/grammarly/rocker/tree/0.2.1) (2015-09-14)
[Full Changelog](https://github.com/grammarly/rocker/compare/0.2.0...0.2.1)

**Implemented enhancements:**

- MOUNT is not respecting ~ alias [\#4](https://github.com/grammarly/rocker/issues/4)

**Merged pull requests:**

- \#6 provide `-id` parameter [\#7](https://github.com/grammarly/rocker/pull/7) ([ybogdanov](https://github.com/ybogdanov))
- Issue 4 [\#5](https://github.com/grammarly/rocker/pull/5) ([ybogdanov](https://github.com/ybogdanov))

## [0.2.0](https://github.com/grammarly/rocker/tree/0.2.0) (2015-09-08)


\* *This Change Log was automatically generated by [github_changelog_generator](https://github.com/skywinder/Github-Changelog-Generator)*