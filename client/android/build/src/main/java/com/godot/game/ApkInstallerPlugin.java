package com.godot.game;

import android.app.Activity;
import android.content.Intent;
import android.net.Uri;
import android.os.Build;
import android.provider.Settings;

import androidx.core.content.FileProvider;

import org.godotengine.godot.Godot;
import org.godotengine.godot.plugin.GodotPlugin;
import org.godotengine.godot.plugin.UsedByGodot;

import java.io.File;

/**
 * Установка APK обновления из игры. Файл должен лежать в каталоге files/updates приложения
 * (в игре это user://updates); проверка суммы и подписи манифеста выполняется на стороне GDScript до вызова.
 */
public class ApkInstallerPlugin extends GodotPlugin {
    public ApkInstallerPlugin(Godot godot) {
        super(godot);
    }

    @Override
    public String getPluginName() {
        return "ApkInstaller";
    }

    /** Разрешена ли этому приложению установка пакетов («Установка из этого источника»). */
    @UsedByGodot
    public boolean canInstall() {
        Activity a = getActivity();
        if (a == null) {
            return false;
        }
        return Build.VERSION.SDK_INT < Build.VERSION_CODES.O || a.getPackageManager().canRequestPackageInstalls();
    }

    /** Открывает системную страницу разрешения установки для этого приложения. */
    @UsedByGodot
    public boolean openInstallSettings() {
        Activity a = getActivity();
        if (a == null) {
            return false;
        }
        try {
            Intent i = new Intent(Settings.ACTION_MANAGE_UNKNOWN_APP_SOURCES, Uri.parse("package:" + a.getPackageName()));
            a.startActivity(i);
            return true;
        } catch (Exception e) {
            return false;
        }
    }

    /** Запускает системный установщик. Возвращает "" при успехе или текст ошибки. */
    @UsedByGodot
    public String install(String path) {
        Activity a = getActivity();
        if (a == null) {
            return "нет активности";
        }
        try {
            File updates = new File(a.getFilesDir(), "updates").getCanonicalFile();
            File apk = new File(path).getCanonicalFile();
            if (!apk.getPath().startsWith(updates.getPath() + File.separator) || !apk.getName().endsWith(".apk") || !apk.isFile()) {
                return "файл обновления не найден";
            }
            Uri uri = FileProvider.getUriForFile(a, a.getPackageName() + ".updates", apk);
            Intent i = new Intent(Intent.ACTION_VIEW);
            i.setDataAndType(uri, "application/vnd.android.package-archive");
            i.addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION);
            a.startActivity(i);
            return "";
        } catch (Exception e) {
            return "не удалось запустить установщик: " + e.getClass().getSimpleName();
        }
    }
}
