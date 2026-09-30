# web

The browser app: Dart 3 + [OverReact](https://github.com/Workiva/over_react) (function components and hooks; the history panel is a `UiComponent2` class component), OverReact Redux and built_value.

- `web/`: the page (`index.html`) and entrypoint (`index.dart`), which mounts `App` into `#app`.
- `lib/src/components/`: OverReact components. `App` is the root.
- `test/`: browser tests, run in Chrome with [react_testing_library](https://github.com/Workiva/react_testing_library). `test/html_template.html` loads the React JS for every test file.

Run these from the repo root:

```bash
./dev run make serve-web                # http://localhost:8080 (webdev serve)
./dev run make lint-web                 # build_runner build, dart format, dart analyze
./dev run make test-web                 # dart run build_runner test, in Chrome
./dev run scripts/dev/smoke-web.sh      # webdev serve renders App; add --screenshot=out.png
```

Generated `*.g.dart` files are git-ignored and rebuilt by the `make` targets. Run `./dev run bash -c 'cd web && dart run build_runner watch'` while editing, so the IDE can resolve them. Plain `dart test` doesn't work here: use `make test-web`. See [ADR 0003](../docs/adr/0003-web-app-shell.md).
