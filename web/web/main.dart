// over_react's react_dom.render takes a dart:html Element.
// ignore: deprecated_member_use
import 'dart:html';

import 'package:linked_numbers_web/src/app_info.dart';
import 'package:linked_numbers_web/src/components/app.dart';
import 'package:over_react/react_dom.dart' as react_dom;

void main() {
  document.title = appTitle();
  react_dom.render(App()(), querySelector('#app')!);
}
