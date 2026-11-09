# Ручная сборка APK через GitHub Actions

Workflow `.github/workflows/android-release.yml` запускается только вручную (Actions → Android release APK → Run workflow), собирает подписанный APK и прикладывает artifact. Не публикует latest.json и не делает деплой. Фактическая сборка требует доступной квоты Actions и настроенных секретов; подготовленный workflow ещё не означает успешный APK.

## Ключи

Использовать только существующий `~/keys/cosmoc/release.keystore` и соответствующий `release.env` на VPS. Не генерировать замену. Не путать с manifest_private.pem: это отдельный ключ подписи обновлений, он этой сборке не нужен.

GitHub → репозиторий → Settings → Secrets and variables → Actions → Secrets → New repository secret:

- `ANDROID_KEYSTORE_BASE64`: содержимое `base64 -w0 ~/keys/cosmoc/release.keystore`, выполненного локально на доверенном VPS/компьютере. Base64 не шифрование; не отправлять в чат и не коммитить.
- `ANDROID_KEY_ALIAS`: существующий alias `cosmoc-release` (проверить по release.env).
- `ANDROID_KEYSTORE_PASSWORD`: существующий пароль keystore из release.env. Gradle проекта использует один пароль хранилища и ключа; если они разные — остановиться и отдельно настроить сборку.

На вкладке Variables добавить `ANDROID_CERT_SHA256` — SHA-256 отпечаток сертификата существующего релизного ключа. Получить `keytool -list -v -keystore ~/keys/cosmoc/release.keystore -alias cosmoc-release` с вводом пароля по запросу; использовать строку SHA256. Сверить с ранее выпущенным APK. Это публичный отпечаток, не приватный ключ.

Если установлен gh CLI на доверенной машине, файл можно загрузить без копирования в чат:

```bash
base64 -w0 ~/keys/cosmoc/release.keystore | gh secret set ANDROID_KEYSTORE_BASE64 --repo maillunex-lang/CosmocRPG
```

Пароль вводить через интерфейс GitHub или интерактивный `gh secret set`, не записывать в командную строку/историю. Перед загрузкой сохранить резервную копию ключа и паролей вне VPS.

## До публикации

Версия в исходниках сейчас 0.3.12/code15 совпадает с опубликованной. Workflow пока собирает кандидат этой версии; для обновления перед выпуском согласованно повысить версии в project.godot/export_presets.cfg после сверки свежего latest.json и main. Проверить updater_test, сертификат, размер, установку поверх существующего APK. Подписанный artifact сам по себе не опубликованное обновление. Для публикации нужен отдельный VPS-доступ и прежний ключ подписи манифеста.
