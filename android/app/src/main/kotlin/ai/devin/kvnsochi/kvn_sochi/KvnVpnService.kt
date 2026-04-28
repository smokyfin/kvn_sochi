package ai.devin.kvnsochi.kvn_sochi

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Intent
import android.net.VpnService
import android.os.Build
import android.os.ParcelFileDescriptor
import androidx.core.app.NotificationCompat
import java.io.File
import java.net.ServerSocket
import java.security.SecureRandom
import java.util.concurrent.atomic.AtomicReference

class KvnVpnService : VpnService() {
    private val secureRandom = SecureRandom()
    private var tun: ParcelFileDescriptor? = null
    private var coreProcess: Process? = null

    override fun onCreate() {
        super.onCreate()
        createNotificationChannel()
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_CONNECT -> connect(intent)
            ACTION_DISCONNECT -> disconnect()
        }
        return START_STICKY
    }

    override fun onDestroy() {
        disconnect()
        super.onDestroy()
    }

    private fun connect(intent: Intent) {
        status.set("Connecting")
        startForeground(NOTIFICATION_ID, notification("Connecting"))
        try {
            tun = Builder()
                .setSession("KVN Sochi")
                .setMtu(8500)
                .addAddress("10.88.0.2", 32)
                .addRoute("0.0.0.0", 0)
                .addDnsServer("10.88.0.1")
                .apply {
                    val allowed = intent.getStringArrayListExtra("allowedApps").orEmpty()
                    val disallowed = intent.getStringArrayListExtra("disallowedApps").orEmpty()
                    allowed.forEach { addAllowedApplication(it) }
                    disallowed.forEach { addDisallowedApplication(it) }
                }
                .establish()
            startCore(intent)
            status.set("Connected")
            startForeground(NOTIFICATION_ID, notification("Connected"))
        } catch (error: Exception) {
            lastError.set(error.message ?: error.javaClass.simpleName)
            disconnect()
        }
    }

    private fun startCore(intent: Intent) {
        val nativeDir = applicationInfo.nativeLibraryDir ?: return
        val core = File(nativeDir, "libkvn-core.so")
        if (!core.exists()) {
            return
        }
        val privateDir = File(filesDir, "runtime").apply { mkdirs() }
        privateDir.setReadable(false, false)
        privateDir.setWritable(false, false)
        privateDir.setExecutable(false, false)
        privateDir.setReadable(true, true)
        privateDir.setWritable(true, true)
        privateDir.setExecutable(true, true)
        val config = intent.getStringExtra("configText").orEmpty().ifBlank {
            intent.getStringExtra("configUrl") ?: "https://incss.ru/vless.conf"
        }
        coreProcess = ProcessBuilder(
            core.absolutePath,
            "--private-dir", privateDir.absolutePath,
            "--config", config,
            "--tun", "tun0",
            "--tun-gateway", "10.88.0.1",
        )
            .directory(privateDir)
            .redirectErrorStream(true)
            .start()
    }

    private fun disconnect() {
        coreProcess?.destroy()
        coreProcess?.waitFor()
        coreProcess = null
        tun?.close()
        tun = null
        status.set("Idle")
        stopForeground(STOP_FOREGROUND_REMOVE)
        stopSelf()
    }

    private fun createNotificationChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val manager = getSystemService(NotificationManager::class.java)
            manager.createNotificationChannel(
                NotificationChannel(CHANNEL_ID, "KVN Sochi VPN", NotificationManager.IMPORTANCE_LOW),
            )
        }
    }

    private fun notification(text: String): Notification {
        val intent = packageManager.getLaunchIntentForPackage(packageName)
        val pendingIntent = PendingIntent.getActivity(
            this,
            secureRandom.nextInt(),
            intent,
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
        )
        return NotificationCompat.Builder(this, CHANNEL_ID)
            .setContentTitle("KVN Sochi")
            .setContentText(text)
            .setSmallIcon(android.R.drawable.stat_sys_warning)
            .setOngoing(true)
            .setContentIntent(pendingIntent)
            .build()
    }

    private fun randomPort(): Int {
        ServerSocket(0).use { return it.localPort }
    }

    companion object {
        const val ACTION_CONNECT = "ai.devin.kvnsochi.CONNECT"
        const val ACTION_DISCONNECT = "ai.devin.kvnsochi.DISCONNECT"
        private const val CHANNEL_ID = "kvn_sochi_vpn"
        private const val NOTIFICATION_ID = 17520
        private val status = AtomicReference("Idle")
        private val lastError = AtomicReference<String?>(null)

        fun currentStatus(stateOverride: String? = null): Map<String, Any?> = mapOf(
            "state" to (stateOverride ?: status.get()),
            "flow" to listOf("TUN", "hev-socks5-tunnel", "Arti", "xray-core", "Internet"),
            "skipArti" to false,
            "lastError" to lastError.get(),
        )
    }
}
