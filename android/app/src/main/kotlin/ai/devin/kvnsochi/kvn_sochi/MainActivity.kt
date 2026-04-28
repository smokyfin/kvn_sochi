package ai.devin.kvnsochi.kvn_sochi

import android.app.Activity
import android.content.Intent
import android.net.VpnService
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.embedding.android.FlutterActivity
import io.flutter.plugin.common.MethodChannel

class MainActivity : FlutterActivity() {
    private val channelName = "kvn_sochi/vpn"
    private var pendingResult: MethodChannel.Result? = null

    override fun configureFlutterEngine(flutterEngine: FlutterEngine) {
        super.configureFlutterEngine(flutterEngine)
        MethodChannel(flutterEngine.dartExecutor.binaryMessenger, channelName).setMethodCallHandler { call, result ->
            when (call.method) {
                "connect" -> {
                    val permissionIntent = VpnService.prepare(this)
                    if (permissionIntent != null) {
                        pendingResult = result
                        startActivityForResult(permissionIntent, VPN_PERMISSION_REQUEST)
                    } else {
                        startVpn(call.arguments as? Map<String, Any?>)
                        result.success(KvnVpnService.currentStatus())
                    }
                }
                "disconnect" -> {
                    stopService(Intent(this, KvnVpnService::class.java).setAction(KvnVpnService.ACTION_DISCONNECT))
                    result.success(KvnVpnService.currentStatus("Idle"))
                }
                "status" -> result.success(KvnVpnService.currentStatus())
                else -> result.notImplemented()
            }
        }
    }

    override fun onActivityResult(requestCode: Int, resultCode: Int, data: Intent?) {
        super.onActivityResult(requestCode, resultCode, data)
        if (requestCode == VPN_PERMISSION_REQUEST) {
            val result = pendingResult
            pendingResult = null
            if (resultCode == Activity.RESULT_OK) {
                startVpn(null)
                result?.success(KvnVpnService.currentStatus())
            } else {
                result?.error("vpn_permission_denied", "VPN permission denied", null)
            }
        }
    }

    private fun startVpn(args: Map<String, Any?>?) {
        val intent = Intent(this, KvnVpnService::class.java).setAction(KvnVpnService.ACTION_CONNECT)
        args?.forEach { (key, value) ->
            when (value) {
                is String -> intent.putExtra(key, value)
                is Boolean -> intent.putExtra(key, value)
            }
        }
        startForegroundService(intent)
    }

    companion object {
        private const val VPN_PERMISSION_REQUEST = 4817
    }
}
