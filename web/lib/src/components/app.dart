import 'package:over_react/over_react.dart';

import '../app_info.dart';

part 'app.over_react.g.dart';

mixin AppProps on UiProps {}

/// The root component: the page header and the main content area.
UiFactory<AppProps> App = uiFunction(
  (_) => Fragment()(
    Dom.header()(Dom.h1()(appName)),
    Dom.main()(Dom.p()('Reports will appear here.')),
  ),
  _$AppConfig,
);
