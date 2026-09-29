import 'package:linked_numbers_web/src/components/app.dart';
import 'package:react_testing_library/matchers.dart';
import 'package:react_testing_library/react_testing_library.dart' as rtl;
import 'package:test/test.dart';

void main() {
  group('App', () {
    test('renders the app name as the page heading', () {
      final view = rtl.render(App()());

      expect(
        view.getByRole('heading', name: 'Linked Numbers'),
        isInTheDocument,
      );
    });

    test('renders a main landmark', () {
      final view = rtl.render(App()());

      expect(view.getByRole('main'), isInTheDocument);
    });
  });
}
