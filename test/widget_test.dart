import 'package:flutter_test/flutter_test.dart';

import 'package:kvn_sochi/main.dart';

void main() {
  testWidgets('shows VPN home and traffic flow', (WidgetTester tester) async {
    await tester.pumpWidget(const KvnSochiApp());
    await tester.pump();

    expect(find.text('KVN Sochi'), findsOneWidget);
    expect(find.text('Idle'), findsOneWidget);
    expect(find.textContaining('hev-socks5-tunnel'), findsOneWidget);
  });

  testWidgets('opens configuration tab', (WidgetTester tester) async {
    await tester.pumpWidget(const KvnSochiApp());
    await tester.pump();

    await tester.tap(find.text('Config'));
    await tester.pumpAndSettle();

    expect(find.text('Configuration'), findsOneWidget);
    expect(find.text('https://incss.ru/vless.conf'), findsOneWidget);
  });
}
