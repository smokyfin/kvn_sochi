import NetworkExtension

final class PacketTunnelProvider: NEPacketTunnelProvider {
  private var coreProcess: Process?

  override func startTunnel(
    options: [String: NSObject]?,
    completionHandler: @escaping (Error?) -> Void
  ) {
    let settings = NEPacketTunnelNetworkSettings(tunnelRemoteAddress: "127.0.0.1")
    settings.ipv4Settings = NEIPv4Settings(addresses: ["10.88.0.2"], subnetMasks: ["255.255.255.255"])
    settings.ipv4Settings?.includedRoutes = [NEIPv4Route.default()]
    settings.dnsSettings = NEDNSSettings(servers: ["10.88.0.1"])
    setTunnelNetworkSettings(settings) { [weak self] error in
      guard error == nil else {
        completionHandler(error)
        return
      }
      self?.startCore()
      completionHandler(nil)
    }
  }

  override func stopTunnel(
    with reason: NEProviderStopReason,
    completionHandler: @escaping () -> Void
  ) {
    coreProcess?.terminate()
    coreProcess = nil
    completionHandler()
  }

  private func startCore() {
    guard let url = Bundle.main.url(forResource: "kvn-core", withExtension: nil) else {
      return
    }
    let privateDir = FileManager.default.containerURL(
      forSecurityApplicationGroupIdentifier: "group.ai.devin.kvnsochi"
    ) ?? FileManager.default.temporaryDirectory
    let runtime = privateDir.appendingPathComponent("runtime", isDirectory: true)
    try? FileManager.default.createDirectory(at: runtime, withIntermediateDirectories: true)
    let process = Process()
    process.executableURL = url
    process.arguments = [
      "--private-dir", runtime.path,
      "--config", "https://incss.ru/vless.conf",
      "--tun", "utun",
      "--tun-gateway", "10.88.0.1",
    ]
    try? process.run()
    coreProcess = process
  }
}
