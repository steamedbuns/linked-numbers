import 'package:linked_numbers_web/src/app_info.dart';
import 'package:test/test.dart';

void main() {
  group('appTitle', () {
    test('is the app name without a page', () {
      expect(appTitle(), 'Linked Numbers');
      expect(appTitle(''), 'Linked Numbers');
    });

    test('prefixes the page name', () {
      expect(appTitle('Values'), 'Values · Linked Numbers');
    });
  });
}
