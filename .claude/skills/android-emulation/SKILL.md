---
name: android-emulation
description: Use when you need to run/test an Android (Flutter or native) app without a physical device — spin up an emulator, install an APK, and drive + observe the UI entirely via adb (screencap + input). Covers the AVD lifecycle and the non-obvious gotchas (cleartext http, host-loopback, prefs injection, coordinate scaling).
---

<!-- Сторонний навык (MIT): https://github.com/DSmereski/agent-skills, коммит 7eb549e65c1bd6bba546c0250f8a8641459e5292. Подключён без изменений текста; контекст проекта «Уралия» — см. docs/SKILLS.md. Лицензия — .claude/skills/THIRD_PARTY_SKILLS.md -->

<!-- Public trimmed version shared from smereski.com -->

# Android Emulation (headless drive via adb)

Run and verify an Android app with no physical device, driving the UI through
`adb` and *seeing* the screen via `adb exec-out screencap`. This lets an agent
do full UI verification (install → pair → interact → screenshot) unattended.

## Paths (standard SDK locations)

- **SDK root**: `$ANDROID_HOME` (commonly `~/Android/Sdk` on Linux/macOS,
  `%LOCALAPPDATA%\Android\Sdk` on Windows)
- **adb**: `$ANDROID_HOME/platform-tools/adb`
- **emulator**: `$ANDROID_HOME/emulator/emulator`
- **avdmanager / sdkmanager**: `$ANDROID_HOME/cmdline-tools/latest/bin/`

Add to PATH (Linux/macOS):
```bash
export PATH="$PATH:$ANDROID_HOME/platform-tools:$ANDROID_HOME/emulator"
```

Add to PATH (Git Bash on Windows):
```bash
export PATH="$PATH:/c/Users/$USER/AppData/Local/Android/Sdk/platform-tools:/c/Users/$USER/AppData/Local/Android/Sdk/emulator"
```

## Lifecycle

1. **System image** (one-time, ~1 GB):
   ```bash
   yes | sdkmanager "system-images;android-35;google_apis;x86_64"
   ```
2. **Create AVD**:
   ```bash
   echo no | avdmanager create avd -n myavd -k "system-images;android-35;google_apis;x86_64" -d pixel_6
   ```
3. **Boot (background)**:
   ```bash
   emulator -avd myavd -no-snapshot -no-boot-anim -gpu swiftshader_indirect &
   ```
   (`swiftshader_indirect` is the safe software renderer; no GPU required.)
4. **Wait for boot**: poll until the property is `1`:
   ```bash
   until [ "$(adb -s emulator-5554 shell getprop sys.boot_completed 2>/dev/null)" = "1" ]; do sleep 2; done
   ```
5. **Install**:
   ```bash
   adb -s emulator-5554 install -r path/to/app-debug.apk
   ```
6. **Launch**:
   ```bash
   adb -s emulator-5554 shell am start -n com.example.myapp/.MainActivity
   ```

If a physical device is also connected, **always pass `-s emulator-5554`** —
bare `adb` errors with "more than one device/emulator".

---

## TOKEN ECONOMY (read this first)

Reading screenshots is the most expensive thing an agent does here — each
full-res image costs ~1.5 k+ tokens AND stays in context, re-billed every
turn. Verify with TEXT first; screenshot last.

1. **UI tree, not pixels**:
   ```bash
   adb -s emulator-5554 shell uiautomator dump /sdcard/ui.xml
   adb -s emulator-5554 shell cat /sdcard/ui.xml
   ```
   Grep for `text=` / `content-desc=` to assert state without any image.

   **Flutter unlock**: Flutter only emits `AccessibilityNodeInfo` when an
   accessibility service is running — without it the tree is one opaque
   `FlutterView`. Enable TalkBack once per emulator (sound doesn't matter):
   ```bash
   adb -s emulator-5554 shell settings put secure enabled_accessibility_services \
     com.google.android.marvin.talkback/com.google.android.marvin.talkback.TalkBackService
   adb -s emulator-5554 shell settings put secure accessibility_enabled 1
   ```
   After this the dump exposes `text`/`content-desc`/`checked`/`enabled` for
   every Flutter Semantics node. Standard `Text`/`Button`/`TextField` widgets
   emit automatically; custom-painted widgets need an explicit
   `Semantics(label: '…')` wrapper.

   Researched alternatives: Appium and mobile-mcp need the same TalkBack
   prerequisite plus extra sidecar processes. Maestro needs a WSL2 adb bridge
   on Windows (brittle). Patrol adds a 30–90 s build cycle per check. Plain
   adb wins for agent loops.

2. **Assertions in code**: widget/integration tests (`flutter test`) — zero images.

3. **Logs**: add a temporary `print('[TAG] …')`, rebuild, then:
   ```bash
   adb -s emulator-5554 logcat -d | grep TAG
   ```
   Flutter `print` appears under the `flutter` logcat tag. Revert the print after.

4. **Screenshot only at milestones**, then crop + downscale + JPEG before reading:
   ```python
   from PIL import Image
   im = Image.open('shot.png').crop((0, 0, 1080, 260))
   im.thumbnail((800, 800))
   im.save('shot.jpg', quality=70)
   ```

5. If a human needs to see it, send the file to them — their eyeball is free;
   yours is metered.

---

## Observe + drive (no MCP needed)

### Screenshot
```bash
adb -s emulator-5554 exec-out screencap -p > /tmp/shot.png
```
**Always downscale before reading** — full-res phone shots (e.g. 1080×2400) can
exceed the API's multi-image limit once a conversation holds several images,
poisoning the whole conversation with "image could not be processed" errors
(only `/compact` clears it). Downscale first:
```python
from PIL import Image
im = Image.open('shot.png')
im.thumbnail((900, 1950))
im.save('shot_s.png')
```

### Tap
```bash
adb -s emulator-5554 shell input tap X Y
```
Coordinates are **device pixels** (e.g. 1080×2400), NOT the scaled-down image
you viewed. If the screenshot was displayed at 900×2000, multiply your
read-off coordinates by 1.2 (1080/900) before tapping.

### Type
```bash
adb -s emulator-5554 shell input text "literal"
```
**Unreliable** for long or special-character strings — it silently truncates
and can double-insert. Prefer injecting state directly (see Prefs Injection below).

### Key events
| Code | Action |
|------|--------|
| `4` | BACK (also closes soft keyboard cleanly) |
| `3` | HOME |
| `66` | ENTER |
| `67` | DEL |
| `123` | MOVE_END |
| `111` | ESC |

### Clear a text field
Tap the field → `keyevent 123` (move to end) → many `keyevent 67` (delete).
**Close the keyboard with `keyevent 4` BEFORE tapping buttons** — the soft
keyboard shifts the layout, so post-typing taps land on the wrong widget.

### Grant a runtime permission
```bash
adb -s emulator-5554 shell pm grant com.example.myapp android.permission.CAMERA
```

### Trigger onResume
```bash
adb -s emulator-5554 shell input keyevent 3   # HOME
adb -s emulator-5554 shell am start -n com.example.myapp/.MainActivity
```
Useful for poll-on-resume logic.

---

## Gotchas that cost real time

### Cleartext HTTP is blocked
Android 9+ refuses `http://` by default — every request fails silently (the
app appears "offline"). Add this attribute to `<application>` in
`AndroidManifest.xml`:
```xml
android:usesCleartextTraffic="true"
```

### Reaching a host server from the emulator
The emulator is NAT'd. Two options:

- **`10.0.2.2`** — the emulator's built-in alias for the host loopback.
- **`adb reverse`** (more reliable on Windows):
  ```bash
  adb -s emulator-5554 reverse tcp:PORT tcp:PORT
  ```
  Then point the app at `127.0.0.1:PORT`. Note: `adb reverse` is dropped when
  the adb server restarts — re-run it. Removing the reverse cleanly simulates
  the server going unreachable (good for testing offline behavior without
  killing the real server).

A server bound only to loopback (`127.0.0.1`) on the host is reachable from
the emulator via `10.0.2.2` or `adb reverse`. A server bound to a VPN/overlay
IP (e.g. a mesh network interface) is typically **not** reachable from the
emulator, which is not a member of that network.

### Inject app state instead of typing it
For debug builds, write SharedPreferences directly to avoid `input text`
mangling. Read the current prefs:
```bash
adb -s emulator-5554 shell "run-as com.example.myapp \
  cat /data/data/com.example.myapp/shared_prefs/FlutterSharedPreferences.xml"
```

To write, **base64-encode the XML on the host and decode on device** — shell
quoting otherwise strips XML attribute quotes, and Git Bash on Windows rewrites
`/data/…` push paths:
```bash
B64=$(python -c "import base64; print(base64.b64encode(open('f.xml','rb').read()).decode())")
adb -s emulator-5554 shell "run-as com.example.myapp sh -c \
  'echo $B64 | base64 -d > /data/data/com.example.myapp/shared_prefs/FlutterSharedPreferences.xml'"
```
Flutter prefs keys are prefixed `flutter.` and typed:
```xml
<string name="flutter.myKey">value</string>
```

### Debugging silent failures
If the app swallows errors (e.g. a sync loop with `catch (_)`), add a
temporary `print('[TAG] error: $e')`, rebuild, and read logcat:
```bash
adb -s emulator-5554 logcat -d | grep TAG
```
Flutter `print` appears under the `flutter` tag. Revert the temporary print
once the issue is found.

### Camera / QR scanning won't work
The emulator camera renders a synthetic 3D scene and cannot read a real QR
code. Use a manual entry or token-based path in tests instead.

### Reinstall may clear app data
`adb install -r` nominally keeps data but **sometimes resets it** (clearing
session tokens, pairing state, etc.). Re-authenticate or re-inject state after
reinstalling.

### Debug builds are slow to first frame
Flutter debug builds show a splash for several seconds. Re-screenshot after a
`sleep 7` before asserting UI state.

---

## When to use a physical device instead

Reach for a USB device (`adb devices` shows its serial) when you need:
- Real BLE (Bluetooth Low Energy)
- Camera (reading a real QR / taking a real photo)
- GPS
- True network conditions (cellular, real Wi-Fi latency)

Enable **USB debugging** on the device and accept the host RSA key prompt.
Everything else — UI flows, offline behavior, state injection via tokens — is
faster and fully scriptable on the emulator.
