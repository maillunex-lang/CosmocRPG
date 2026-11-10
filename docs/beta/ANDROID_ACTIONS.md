# Ручная сборка APK через GitHub Actions

Workflow `.github/workflows/android-release.yml`, задание `android-apk`, запускается только вручную: Actions → Android release APK → Run workflow, ветка `main`. Собирает подписанный APK и прикладывает artifact на 14 дней. Не публикует GitHub Release, `latest.json` и не делает деплой. Успешный workflow ещё не запускался: прогон [42756907822](https://github.com/maillunex-lang/CosmocRPG/actions/runs/42756907822) остановился на установке SDK (`android-actions/setup-android` искал удалённый пакет `tools`). Этот пакет из workflow убран. Пока новый прогон не станет зелёным, APK не считать собранным.

Задание не добавлено в `.github/workflows/ci.yml`: тот workflow запускается на `pull_request`. Подпись живёт отдельно и на запросы слияния не подписана.

## Версии

Берутся из репозитория, не из общих рекомендаций документации Godot:

- Godot и экспортные шаблоны: `4.7.2-stable` (`client/android/.build_version` = `4.7.2.stable`)
- Java: 17 (`JavaVersion.VERSION_17` в `client/android/build/config.gradle`)
- Android Gradle Plugin: `8.6.1`
- Gradle wrapper: `8.11.1`
- compileSdk / targetSdk: `36`, minSdk: `24`
- Build-Tools: `36.1.0`
- NDK: `29.0.14206865`
- Package: `ru.cosmocrpg.game`, preset `Android`, Gradle build включён

## Ключи

Использовать только существующий `~/keys/cosmoc/release.keystore` и соответствующий `release.env` на VPS. Не генерировать замену. Не путать с manifest_private.pem: это отдельный ключ подписи обновлений, он этой сборке не нужен.

GitHub → репозиторий → Settings → Secrets and variables → Actions → Secrets → New repository secret:

- `ANDROID_KEYSTORE_BASE64`: содержимое `base64 -w0 ~/keys/cosmoc/release.keystore`, выполненного локально на доверенном VPS/компьютере. Base64 не шифрование; не отправлять в чат и не коммитить.
- `ANDROID_KEY_ALIAS`: существующий alias `cosmoc-release` (проверить по release.env).
- `ANDROID_KEYSTORE_PASSWORD`: существующий пароль keystore из release.env. Gradle проекта использует один пароль хранилища и ключа; если они разные — остановиться и отдельно настроить сборку.

На вкладке Variables можно добавить `ANDROID_CERT_SHA256` — тот же публичный SHA-256, что в `docs/SECURITY.md`. Workflow и без переменной сравнивает отпечаток APK с этим файлом.

Если установлен gh CLI на доверенной машине, файл можно загрузить без копирования в чат:

```bash
base64 -w0 ~/keys/cosmoc/release.keystore | gh secret set ANDROID_KEYSTORE_BASE64 --repo maillunex-lang/CosmocRPG
```

Пароль вводить через интерфейс GitHub или интерактивный `gh secret set`, не записывать в командную строку/историю. Перед загрузкой сохранить резервную копию ключа и паролей вне VPS.

Секреты репозитория GitHub не отдаёт workflow из форка. Этот файл не запускается на `pull_request` / `pull_request_target` и дополнительно требует `refs/heads/main`. Это не закрывает другой случай: участник с правом push может изменить workflow в запросе на слияние из ветки того же репозитория, и такой прогон получит секреты репозитория. Чтобы закрыть и его, секреты нужно перенести в Environment с разрешённой веткой только `main`.

## Где лежит APK

После зелёного прогона: вкладка запуска → Artifacts → `zvezdnye-rubezhi-release-apk`. Рядом `cert-sha256.txt`, `SHA256SUMS`, вывод `apksigner` и `aapt`. Хранится 14 дней. Это не страница загрузки игры.

Совместимость с уже установленной копией: SHA-256 сертификата в `cert-sha256.txt` должен совпасть с `docs/SECURITY.md` (`D1:26:00:89:…:24:9F`). Android принимает обновление только с тем же ключом и не меньшим `versionCode`. Текущие исходники остаются `0.3.12` / code `15`, как у уже опубликованного APK: поверх него эта сборка не встанет, пока version code не увеличат отдельно. Подпись при этом можно сверить.

## До публикации

Версия в исходниках сейчас 0.3.12/code15 совпадает с опубликованной. Workflow собирает кандидат этой версии; для обновления перед выпуском согласованно повысить версии в project.godot/export_presets.cfg после сверки свежего latest.json и main. Проверить updater_test, сертификат, размер, установку поверх существующего APK. Подписанный artifact сам по себе не опубликованное обновление. Для публикации нужен отдельный VPS-доступ и прежний ключ подписи манифеста.