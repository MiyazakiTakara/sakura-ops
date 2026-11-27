# 16. Macierz kompatybilności Linux

Plan testowy, nie deklaracja wsparcia. Zarządzane hosty wyłącznie Linux z systemd; przeglądarka administratora niezależna od OS.

| Dystrybucja | Backend pakietów | Priorytet | Stan |
| --- | --- | --- | --- |
| Ubuntu 22.04 | APT/dpkg | P1 | do testu |
| Ubuntu 24.04 | APT/dpkg | P0 lab | do testu |
| Ubuntu 26.04 | APT/dpkg | P1 | do testu |
| Oracle Linux 8 | DNF/RPM | P0 lab | do testu |
| Oracle Linux 9 | DNF/RPM | P1 | do testu |
| Debian 12/13 | APT/dpkg | P2 | do testu |
| Rocky/Alma | DNF/RPM | później | poza pierwszym odbiorem |

P0 to dwie reprezentatywne rodziny w pierwszym laboratorium; P1 przed deklaracją wsparcia pozostałych wersji. amd64 pierwsze, arm64 po osobnym build/test; nie obiecywać wsparcia EOL ani architektury bez matrycy. Wersje OS i dostępność narzędzi zweryfikować podczas spike, nie zakładać zachowania na podstawie nazwy dystrybucji. OS detection przez bezpieczne parsowanie os-release, systemd capabilities, wersję menedżera i adapter; nie source pliku, nie fallback do losowej komendy.

## Wspólny kontrakt

Agent binary: dedykowany UID, systemd, ograniczone katalogi i uprawnienia, brak Python/Node i SSH do automatyzacji. D0 bez roota; helper opt-in osobno. APT/dpkg i DNF/RPM zwracają ujednolicone typowane pola, source/cache timestamp, security_classification known/unknown, adapter/version, status, error_code. D0 nie odświeża cache i nie uruchamia niejawnej sieci; gdy backend nie gwarantuje cache-only, odmowa z przyczyną. D1 refresh jawny. Kod DNF 100 może oznaczać aktualizacje, test adaptera normalizuje; nie klasyfikować security po nazwie. Brak advisory = unknown. Reboot indicator unknown jeśli brak wiarygodnej metody. Pakiety i zależności są typowane, brak shell/argv injection.

systemd: zatwierdzone unit IDs, mapowanie do lokalnych jednostek, odczyt statusu; restart/reload tylko po helper i pre/post health. Certyfikaty: wyłącznie zatwierdzone publiczne pliki, nie key. /proc/sysfs i statfs: jednostki i limity, brak env/proc cudzych danych. Docker profile nie wymaga docker.sock, nie używać członkostwa docker jako obejścia root. Jeśli brak bezpiecznego adaptera, funkcja niedostępna.

## Odbiór per system × arch × akcja

Fresh install, reinstall, upgrade/uninstall, cert rotation/revoke, heartbeat/reboot/reconnect, SELinux enforcing na Oracle, brak inbound listener i sudo dla D0, ścisły TLS, cache stale/missing, locale/encoding, pakiety locked, brak narzędzia, full disk, timeout, wynik unknown. Mutacje wyłącznie disposable snapshot VM po osobnym review, weryfikacja skutków i recovery. Build/release nie deklaruje wspieranej akcji bez wykonanych testów na danej kombinacji; publikować macierz PASS/FAIL/NOT RUN z SHA i datą.
