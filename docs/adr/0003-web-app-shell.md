# 0003. Web app shell

- Status: Accepted
- Date: 2026-09-29

## Context

LN-1.3 turns `/web` into an OverReact app that UI stories can build on. `webdev serve` has to render an `App` component, strict lints have to pass, and a component test has to pass under `build_runner test`. That means choosing how React is loaded, how browser tests get their JS, what happens to generated code, and how CI gets a browser.

## Decision

- **React 18 JS.** `web/index.html` and the test template load `packages/react/js/react.dev.js`. react-dart 7.3 deprecates its React 17 files. It has no `createRoot` binding yet, so `main()` mounts with `react_dom.render`.
- **Browser tests run through build_runner.** `web/dart_test.yaml` sets `platforms: [chrome]` and a single `custom_html_template_path` (`test/html_template.html`), which loads React and react_testing_library's JS for every test file. We don't add test_html_builder or per-test `.html` files. `make test-web` runs `dart run build_runner test --no-symlink -- -p chrome`. By default build_runner symlinks its precompiled output, and the test runner's static handler refuses symlinks that point outside that directory, so every test would fail to load.
- **Generated code is git-ignored.** `*.g.dart` (over_react now, built_value later) is in `.gitignore`. `make lint-web` and `make test-web` run build_runner first, so CI always checks freshly generated code. This differs from sqlc output ([CLAUDE.md](../../CLAUDE.md)): build_runner is already required to compile and test the app, so committing its output would only add review noise and a staleness check. `dart format` and `dart analyze` skip `*.g.dart`.
- **Strict lints.** [web/analysis_options.yaml](../../web/analysis_options.yaml) uses `package:lints/recommended.yaml`, the three `strict-*` language modes and a list of extra rules, and `make lint-web` runs `dart analyze --fatal-infos`. `non_constant_identifier_names` is off because OverReact factories are PascalCase. The over_react analyzer plugin is not enabled, because `dart analyze` doesn't run legacy analyzer plugins.
- **A smoke check covers `webdev serve`.** [scripts/dev/smoke-web.sh](../../scripts/dev/smoke-web.sh) starts `webdev serve`, and [scripts/dev/chrome-check.mjs](../../scripts/dev/chrome-check.mjs) waits over the DevTools protocol until `#app h1` reads "Linked Numbers". Two Chrome shortcuts don't work here: `--dump-dom` returns on the load event, before DDC has run `main()`, and `--virtual-time-budget` never finishes under DDC's module loader. The check runs locally, like `smoke-up.sh`, and not in CI, because it binds port 8080. In CI, the component test covers rendering.
- **CI uses the runner's Chrome.** The web job runs `make test-web` with the `google-chrome` preinstalled on `ubuntu-24.04`, not the pinned Chrome for Testing from the Dockerfile.
- **dart_dev is deferred.** The Makefile stays the only task runner until dart_dev earns its place.

## Consequences

- Plain `dart test` no longer works in `/web`, because components need build_runner's output. Use `make test-web`.
- A fresh checkout needs a build_runner build before the IDE can resolve `*.g.dart` parts. `make lint-web` or `dart run build_runner build` does it.
- The Chrome version can differ between CI and the dev container. If browser tests start to flake over this, pin it in CI with `CHROME_VERSION` from the Dockerfile.
- Moving to `createRoot` (and dropping the React 17 files) waits on react-dart 8.
