import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:mobile_scanner/mobile_scanner.dart';

void main() {
  runApp(const KvnSochiApp());
}

enum VpnState { idle, connecting, connected }

extension VpnStateText on VpnState {
  String get label => switch (this) {
    VpnState.idle => 'Idle',
    VpnState.connecting => 'Connecting',
    VpnState.connected => 'Connected',
  };
}

class VpnStatus {
  const VpnStatus({
    required this.state,
    required this.flow,
    required this.skipArti,
    this.lastError,
  });

  factory VpnStatus.fromMap(Map<Object?, Object?> map) {
    final stateText = (map['state'] as String?) ?? 'Idle';
    return VpnStatus(
      state: switch (stateText) {
        'Connected' => VpnState.connected,
        'Connecting' => VpnState.connecting,
        _ => VpnState.idle,
      },
      flow: ((map['flow'] as List<Object?>?) ?? const <Object?>[])
          .whereType<String>()
          .toList(),
      skipArti: (map['skipArti'] as bool?) ?? false,
      lastError: map['lastError'] as String?,
    );
  }

  final VpnState state;
  final List<String> flow;
  final bool skipArti;
  final String? lastError;
}

class VpnController extends ChangeNotifier {
  static const _channel = MethodChannel('kvn_sochi/vpn');

  VpnStatus _status = const VpnStatus(
    state: VpnState.idle,
    flow: <String>['TUN', 'hev-socks5-tunnel', 'Arti', 'xray-core', 'Internet'],
    skipArti: false,
  );
  final List<String> _logs = <String>['Ready'];
  String _configText = '';
  String _configUrl = 'https://incss.ru/vless.conf';
  String _exitCountry = 'Auto';
  bool _skipArti = false;
  Timer? _poller;

  VpnStatus get status => _status;
  List<String> get logs => List.unmodifiable(_logs.reversed);
  String get configText => _configText;
  String get configUrl => _configUrl;
  String get exitCountry => _exitCountry;
  bool get skipArti => _skipArti;

  void startPolling() {
    _poller ??= Timer.periodic(const Duration(seconds: 2), (_) => refresh());
    unawaited(refresh());
  }

  @override
  void dispose() {
    _poller?.cancel();
    super.dispose();
  }

  Future<void> connect() async {
    _setLocalState(VpnState.connecting);
    await _invoke('connect', <String, Object?>{
      'configUrl': _configUrl,
      'configText': _configText,
      'exitCountry': _exitCountry,
      'skipArti': _skipArti,
    });
  }

  Future<void> disconnect() async {
    await _invoke('disconnect', const <String, Object?>{});
  }

  Future<void> refresh() async {
    await _invoke('status', const <String, Object?>{}, quiet: true);
  }

  void setConfigUrl(String value) {
    _configUrl = value.trim().isEmpty
        ? 'https://incss.ru/vless.conf'
        : value.trim();
    notifyListeners();
  }

  void setManualConfig(String value) {
    _configText = value;
    notifyListeners();
  }

  void setExitCountry(String value) {
    _exitCountry = value;
    notifyListeners();
  }

  void setSkipArti(bool value) {
    _skipArti = value;
    notifyListeners();
  }

  Future<void> importQr(String payload) async {
    final trimmed = payload.trim();
    if (trimmed.startsWith('http://') || trimmed.startsWith('https://')) {
      setConfigUrl(trimmed);
    } else {
      setManualConfig(trimmed);
    }
    _addLog('QR configuration imported');
  }

  Future<void> _invoke(
    String action,
    Map<String, Object?> args, {
    bool quiet = false,
  }) async {
    try {
      final response = await _channel.invokeMapMethod<Object?, Object?>(
        action,
        args,
      );
      if (response != null) {
        _status = VpnStatus.fromMap(response);
      }
      if (!quiet) {
        _addLog('$action -> ${_status.state.label}');
      } else {
        notifyListeners();
      }
    } on PlatformException catch (error) {
      _status = VpnStatus(
        state: VpnState.idle,
        flow: _status.flow,
        skipArti: _skipArti,
        lastError: error.message ?? error.code,
      );
      _addLog('$action failed: ${error.message ?? error.code}');
    }
  }

  void _setLocalState(VpnState state) {
    _status = VpnStatus(state: state, flow: _status.flow, skipArti: _skipArti);
    notifyListeners();
  }

  void _addLog(String value) {
    final now = DateTime.now().toIso8601String();
    _logs.add('$now  $value');
    if (_logs.length > 300) {
      _logs.removeRange(0, _logs.length - 300);
    }
    notifyListeners();
  }
}

class KvnSochiApp extends StatefulWidget {
  const KvnSochiApp({super.key});

  @override
  State<KvnSochiApp> createState() => _KvnSochiAppState();
}

class _KvnSochiAppState extends State<KvnSochiApp> {
  late final VpnController controller;

  @override
  void initState() {
    super.initState();
    controller = VpnController()..startPolling();
  }

  @override
  void dispose() {
    controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      animation: controller,
      builder: (context, _) {
        return MaterialApp(
          title: 'KVN Sochi',
          debugShowCheckedModeBanner: false,
          theme: ThemeData(
            colorScheme: ColorScheme.fromSeed(
              seedColor: const Color(0xFFF6821F),
              brightness: Brightness.dark,
            ),
            scaffoldBackgroundColor: const Color(0xFF0B0F14),
            useMaterial3: true,
          ),
          home: HomeScreen(controller: controller),
        );
      },
    );
  }
}

class HomeScreen extends StatefulWidget {
  const HomeScreen({required this.controller, super.key});

  final VpnController controller;

  @override
  State<HomeScreen> createState() => _HomeScreenState();
}

class _HomeScreenState extends State<HomeScreen> {
  int tab = 0;

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: SafeArea(
        child: IndexedStack(
          index: tab,
          children: <Widget>[
            ConnectPage(controller: widget.controller),
            ConfigPage(controller: widget.controller),
            RoutingPage(controller: widget.controller),
            DeveloperPage(controller: widget.controller),
          ],
        ),
      ),
      bottomNavigationBar: NavigationBar(
        selectedIndex: tab,
        onDestinationSelected: (value) => setState(() => tab = value),
        destinations: const <NavigationDestination>[
          NavigationDestination(icon: Icon(Icons.shield), label: 'VPN'),
          NavigationDestination(icon: Icon(Icons.tune), label: 'Config'),
          NavigationDestination(icon: Icon(Icons.route), label: 'Routing'),
          NavigationDestination(icon: Icon(Icons.terminal), label: 'Dev'),
        ],
      ),
    );
  }
}

class ConnectPage extends StatelessWidget {
  const ConnectPage({required this.controller, super.key});

  final VpnController controller;

  @override
  Widget build(BuildContext context) {
    final connected = controller.status.state == VpnState.connected;
    final connecting = controller.status.state == VpnState.connecting;
    return Padding(
      padding: const EdgeInsets.all(24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: <Widget>[
          const Text(
            'KVN Sochi',
            style: TextStyle(fontSize: 34, fontWeight: FontWeight.w800),
          ),
          const SizedBox(height: 8),
          Text(
            'Tor + VLESS Reality over gRPC',
            style: Theme.of(context).textTheme.titleMedium,
          ),
          const Spacer(),
          Center(
            child: SizedBox.square(
              dimension: 220,
              child: FilledButton(
                style: FilledButton.styleFrom(shape: const CircleBorder()),
                onPressed: connecting
                    ? null
                    : connected
                    ? controller.disconnect
                    : controller.connect,
                child: connecting
                    ? const CircularProgressIndicator()
                    : Icon(
                        connected ? Icons.power_settings_new : Icons.lock,
                        size: 74,
                      ),
              ),
            ),
          ),
          const SizedBox(height: 24),
          Center(
            child: Text(
              controller.status.state.label,
              style: const TextStyle(fontSize: 28, fontWeight: FontWeight.w700),
            ),
          ),
          if (controller.status.lastError != null) ...<Widget>[
            const SizedBox(height: 12),
            Text(
              controller.status.lastError!,
              textAlign: TextAlign.center,
              style: TextStyle(color: Theme.of(context).colorScheme.error),
            ),
          ],
          const Spacer(),
          FlowCard(flow: controller.status.flow),
        ],
      ),
    );
  }
}

class FlowCard extends StatelessWidget {
  const FlowCard({required this.flow, super.key});

  final List<String> flow;

  @override
  Widget build(BuildContext context) {
    final items = flow.isEmpty
        ? <String>['TUN', 'hev-socks5-tunnel', 'Arti', 'xray-core', 'Internet']
        : flow;
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Text(
          items.join('  →  '),
          textAlign: TextAlign.center,
          style: const TextStyle(fontWeight: FontWeight.w600),
        ),
      ),
    );
  }
}

class ConfigPage extends StatefulWidget {
  const ConfigPage({required this.controller, super.key});

  final VpnController controller;

  @override
  State<ConfigPage> createState() => _ConfigPageState();
}

class _ConfigPageState extends State<ConfigPage> {
  late final TextEditingController url;
  late final TextEditingController manual;

  @override
  void initState() {
    super.initState();
    url = TextEditingController(text: widget.controller.configUrl);
    manual = TextEditingController(text: widget.controller.configText);
  }

  @override
  void dispose() {
    url.dispose();
    manual.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return ListView(
      padding: const EdgeInsets.all(24),
      children: <Widget>[
        const Text(
          'Configuration',
          style: TextStyle(fontSize: 30, fontWeight: FontWeight.w800),
        ),
        const SizedBox(height: 20),
        TextField(
          controller: url,
          decoration: const InputDecoration(labelText: 'Config URL'),
          onChanged: widget.controller.setConfigUrl,
        ),
        const SizedBox(height: 16),
        TextField(
          controller: manual,
          minLines: 8,
          maxLines: 12,
          decoration: const InputDecoration(
            labelText: 'Manual JSON',
            alignLabelWithHint: true,
            border: OutlineInputBorder(),
          ),
          onChanged: widget.controller.setManualConfig,
        ),
        const SizedBox(height: 16),
        FilledButton.icon(
          onPressed: () async {
            final value = await Navigator.of(context).push<String>(
              MaterialPageRoute(builder: (_) => const QrImportPage()),
            );
            if (value != null) {
              await widget.controller.importQr(value);
            }
          },
          icon: const Icon(Icons.qr_code_scanner),
          label: const Text('Import from QR'),
        ),
        const SizedBox(height: 16),
        SwitchListTile(
          value: widget.controller.skipArti,
          onChanged: widget.controller.setSkipArti,
          title: const Text('skip_arti'),
          subtitle: const Text('TUN → xray-core directly'),
        ),
        const SizedBox(height: 16),
        DropdownButtonFormField<String>(
          initialValue: widget.controller.exitCountry,
          decoration: const InputDecoration(labelText: 'TOR exit country'),
          items: const <String>['Auto', 'DE', 'NL', 'FI', 'US', 'TR', 'JP']
              .map(
                (country) => DropdownMenuItem<String>(
                  value: country,
                  child: Text(country),
                ),
              )
              .toList(),
          onChanged: (value) {
            if (value != null) {
              widget.controller.setExitCountry(value);
            }
          },
        ),
      ],
    );
  }
}

class QrImportPage extends StatelessWidget {
  const QrImportPage({super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Scan QR')),
      body: MobileScanner(
        onDetect: (capture) {
          final value = capture.barcodes.firstOrNull?.rawValue;
          if (value != null) {
            Navigator.of(context).pop(value);
          }
        },
      ),
    );
  }
}

class RoutingPage extends StatelessWidget {
  const RoutingPage({required this.controller, super.key});

  final VpnController controller;

  @override
  Widget build(BuildContext context) {
    return ListView(
      padding: const EdgeInsets.all(24),
      children: const <Widget>[
        Text(
          'Per-app routing',
          style: TextStyle(fontSize: 30, fontWeight: FontWeight.w800),
        ),
        SizedBox(height: 20),
        Card(
          child: ListTile(
            leading: Icon(Icons.android),
            title: Text('Android VpnService routing'),
            subtitle: Text(
              'Native service applies allowed/disallowed app lists.',
            ),
          ),
        ),
        Card(
          child: ListTile(
            leading: Icon(Icons.phone_iphone),
            title: Text('iOS routing'),
            subtitle: Text(
              'Network Extension uses full-device packet tunnel rules.',
            ),
          ),
        ),
      ],
    );
  }
}

class DeveloperPage extends StatelessWidget {
  const DeveloperPage({required this.controller, super.key});

  final VpnController controller;

  @override
  Widget build(BuildContext context) {
    final statusJson = const JsonEncoder.withIndent('  ')
        .convert(<String, Object?>{
          'state': controller.status.state.label,
          'flow': controller.status.flow,
          'skipArti': controller.status.skipArti,
          'lastError': controller.status.lastError,
        });
    return ListView(
      padding: const EdgeInsets.all(24),
      children: <Widget>[
        const Text(
          'Developer',
          style: TextStyle(fontSize: 30, fontWeight: FontWeight.w800),
        ),
        const SizedBox(height: 16),
        FilledButton.icon(
          onPressed: controller.refresh,
          icon: const Icon(Icons.refresh),
          label: const Text('Refresh status'),
        ),
        const SizedBox(height: 16),
        Text(statusJson, style: const TextStyle(fontFamily: 'monospace')),
        const Divider(height: 32),
        for (final line in controller.logs)
          Padding(
            padding: const EdgeInsets.only(bottom: 8),
            child: Text(line, style: const TextStyle(fontFamily: 'monospace')),
          ),
      ],
    );
  }
}
