# Android release APK: безопасная сборка и подпись (SECURITY-ANDROID-001)

Обновлено: 2026-11-10 (Kimi Code). Workflow: `.github/workflows/android-release.yml`, успешный базовый прогон `42758272938`.

## Схема хранения секретов

```
~/keys/cosmoc/release.keystore + release.env (VPS, права 600/700, вне Git)
        │  (источник истины, бэкап у владельца вне VPS — обязателен)
        ▼
GitHub Environment «android-signing» (Environment Secrets)
  ├─ ANDROID_KEYSTORE_BASE64   (PKCS12, base64)
  ├─ ANDROID_KEY_ALIAS         (cosmoc-release)
  └─ ANDROID_KEYSTORE_PASSWORD
        │  доступ: только ветка main + обязательное подтверждение владельцем
        ▼
workflow «Android release APK» (только workflow_dispatch из main)
```

Repository Secrets с теми же именами пока дублируют значения — **удалить после подтверждения владельца**
(пока они на уровне репозитория, любой workflow может к ним обратиться).

## Порядок запуска сборки

1. Push в `main` запускает сборку сам. Ручной запуск тоже остаётся: Actions → «Android release APK» → «Run workflow» → ветка `main`.
2. Подтверждение **«Approve and deploy»** не требуется. Секреты доступны только с ветки `main`, не из запросов на слияние.
3. Если код версии в git уже есть в игре, для этой сборки он увеличивается на один. В репозиторий новый номер не записывается, иначе push зациклился бы.
4. Сборка: Godot 4.7.2 → Gradle → apksigner (проверки package ID `ru.cosmocrpg.game`,
   подписи v2/v3, отпечатка SHA-256) → артефакты (APK + cert-sha256.txt + SHA256SUMS + отчёты).
5. Затем APK заливается на VPS и публикуется через `tools/publish_release.sh`. Игра видит его в `https://rikagro.ru/download/latest.json`. Сервер этим workflow не перезапускается: его выкладывает отдельный workflow на тот же push.

## Восстановление после ошибки

- «не задан секрет …» — проверить `gh secret list --env android-signing -R maillunex-lang/CosmocRPG`.
- Падение экспорта — смотреть лог шага «Экспорт»; временный keystore удаляется всегда (`if: always`).
- «расшифрованный файл не похож на Java keystore» — секрет BASE64 повреждён: перезалить из
  `base64 -w0 ~/keys/cosmoc/release.keystore`.
- Подпись не совпала — сверить `cert-sha256.txt` с отпечатком в `docs/SECURITY.md`.
- «VPS_SSH_KEY не читается» — секрет должен быть файлом приватного ключа OpenSSH целиком, без пароля. Заново: `gh secret set VPS_SSH_KEY --env android-signing --repo maillunex-lang/CosmocRPG < ~/.ssh/apk_publish`.
- SSH не пускает — на VPS в `~/.ssh/authorized_keys` пользователя `gamedev` должна быть открытая пара этого ключа. Хост только `78.17.214.25`.

## Резервное хранение keystore (критично)

Потеря keystore или пароля = невозможность обновлять установленные копии. Обязательно:
1. Копия `release.keystore` и `release.env` в менеджере паролей владельца и/или офлайн-носителе (вне VPS).
2. Проверка восстановления: раз в квартал собрать APK из копии и сверить отпечаток сертификата.
3. Секреты GitHub — не резервная копия: это рабочая копия для CI.

## Правила доступа к релизной подписи

- Keystore никогда не попадает в Git, логи, чаты и артефакты (workflow проверяет сигнатуру JKS/PKCS12
  и не печатает содержимое).
- Сборку запускает только владелец (workflow_dispatch + required reviewers).
- Доступ к Environment имеет только владелец (`maillunex-lang`).
- Локальная сборка на VPS — по `docs/SECURITY.md` (переменные из `release.env`, без вывода значений).

## Безопасная смена GitHub-токена

Токен используется `gh` на VPS (~/.config/gh). Смена: создать новый PAT (scopes: `repo`, `workflow`)
→ `gh auth login` → `gh auth status` проверить → старый токен отозвать на github.com/settings/tokens.
Токен не вводить в командную строку в виде аргументов (история shell); использовать интерактивный ввод.

## Проверка сертификата APK

```bash
APKSIGNER=~/Android/Sdk/build-tools/35.0.1/apksigner
$APKSIGNER verify --print-certs файл.apk | grep SHA-256
# ожидается отпечаток из docs/SECURITY.md (D1:26:00:89:…:9F)
aapt2 dump badging файл.apk | head -1   # package: name='ru.cosmocrpg.game'
```

## Защита ветки main (ruleset «защита-main»)

Запрещены force-push и удаление main (админы могут обходить — владелец не блокируется).
Обязательные status checks и обязательные ревью PR **не включены** (меняют процесс разработки):
включаются только по решению владельца.
