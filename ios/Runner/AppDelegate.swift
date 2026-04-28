import Flutter
import NetworkExtension
import UIKit

@main
@objc class AppDelegate: FlutterAppDelegate, FlutterImplicitEngineDelegate {
  private let vpn = IOSVpnController()

  override func application(
    _ application: UIApplication,
    didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]?
  ) -> Bool {
    if let controller = window?.rootViewController as? FlutterViewController {
      let channel = FlutterMethodChannel(
        name: "kvn_sochi/vpn",
        binaryMessenger: controller.binaryMessenger
      )
      channel.setMethodCallHandler { [vpn] call, result in
        switch call.method {
        case "connect":
          vpn.connect(arguments: call.arguments as? [String: Any]) { result($0) }
        case "disconnect":
          vpn.disconnect { result($0) }
        case "status":
          result(vpn.status())
        default:
          result(FlutterMethodNotImplemented)
        }
      }
    }
    return super.application(application, didFinishLaunchingWithOptions: launchOptions)
  }

  func didInitializeImplicitFlutterEngine(_ engineBridge: FlutterImplicitEngineBridge) {
    GeneratedPluginRegistrant.register(with: engineBridge.pluginRegistry)
  }
}

final class IOSVpnController {
  private let manager = NETunnelProviderManager()
  private var state = "Idle"
  private var lastError: String?

  func connect(arguments: [String: Any]?, completion: @escaping ([String: Any?]) -> Void) {
    state = "Connecting"
    manager.loadFromPreferences { [weak self] error in
      guard let self else { return }
      if let error {
        self.lastError = error.localizedDescription
        self.state = "Idle"
        completion(self.status())
        return
      }
      let protocolConfiguration = NETunnelProviderProtocol()
      protocolConfiguration.providerBundleIdentifier = "ai.devin.kvnsochi.kvn-sochi.PacketTunnel"
      protocolConfiguration.serverAddress = "KVN Sochi"
      protocolConfiguration.providerConfiguration = arguments ?? [:]
      self.manager.protocolConfiguration = protocolConfiguration
      self.manager.localizedDescription = "KVN Sochi"
      self.manager.isEnabled = true
      self.manager.saveToPreferences { error in
        if let error {
          self.lastError = error.localizedDescription
          self.state = "Idle"
          completion(self.status())
          return
        }
        do {
          try self.manager.connection.startVPNTunnel()
          self.state = "Connected"
        } catch {
          self.lastError = error.localizedDescription
          self.state = "Idle"
        }
        completion(self.status())
      }
    }
  }

  func disconnect(completion: @escaping ([String: Any?]) -> Void) {
    manager.connection.stopVPNTunnel()
    state = "Idle"
    completion(status())
  }

  func status() -> [String: Any?] {
    [
      "state": state,
      "flow": ["TUN", "hev-socks5-tunnel", "Arti", "xray-core", "Internet"],
      "skipArti": false,
      "lastError": lastError,
    ]
  }
}
