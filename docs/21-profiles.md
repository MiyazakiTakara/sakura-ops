# 21. Grupy, profile i konfiguracja

Cel: po dołączeniu hosta administrator wybiera profil, a nie ręcznie kilkadziesiąt ustawień. To projekt, nie aktywne polityki.

## Model

Host ma immutable UUID, alias, opis, środowisko lab/prod, tagi, zero lub wiele grup organizacyjnych i dokładnie jeden wersjonowany profil bazowy (domyślnie Linux Basic). Grupa jest przede wszystkim filtrem i zakresem RBAC. Profil opisuje zadania, domyślne inputy, progi diagnostyki, sugerowane harmonogramy i ograniczenia. Środowisko jest osobną właściwością bezpieczeństwa; Production/Development to presety polityki środowiska, nie konkurencyjne profile techniczne. Nie zmieniamy środowiska przez wybór profilu. Agent ma oddzielny lokalny profil capabilities/privilege, diagnostic przy enrollment; centralny profil nie nadaje uprawnień root.

## Dziedziczenie

Konfiguracja niesekretna: global defaults -> profil bazowy -> jawny host override. Grupy w MVP nie nadpisują ustawień, co eliminuje kolejność przy wielu grupach. Jeżeli później potrzeba group defaults, tylko dla pól niesecurity i jawna kolejność/konflikt z preview (osobny ADR). Polityka bezpieczeństwa nie jest last-write-wins: efektywne dozwolone akcje to intersection globalnej polityki, środowiska, profilu, ograniczeń wszystkich grup, hosta, RBAC i lokalnych capabilities. Deny/revoke ma pierwszeństwo. Limity przyjmują najbardziej restrykcyjną wartość; obowiązkowe zgody pozostają; okno serwisowe jest przecięciem dopuszczonych okien. Konflikt lub brak wspólnego okna = blocked. Host override może zaostrzyć, nie poszerzyć uprawnień ani zmienić scope bez admin review. Zmiana grupy może zmienić visibility: preview, explicit approval admin, revision i audyt. Nie przypisuj scope automatycznie na podstawie tagu lub hostname.

## Presety

| Profil | Domyślne odczyty | Uwagi |
| --- | --- | --- |
| Linux Basic | ping, facts, resources, updates.audit, agent.version | MVP; tylko D0 po odbiorze |
| Web Server | Basic + services.status, certificates.audit | wymaga jawnego mapowania service/certificate IDs |
| Database Server | Basic + zatwierdzone usługi, dysk | brak DB credentials/query w MVP |
| Docker Host | Basic + przyszła bezpieczna diagnostyka | brak docker.sock, nie aktywować nieobsługiwanej akcji |
| Production policy | wyłącznie zatwierdzony odczyt, zmiany wyłączone domyślnie | explicit okna, druga osoba dla Z2, canary |
| Development policy | D0, lab-only testy | nie omija podpisów, RBAC ani lokalnej polityki |

Profile zawierają wyłącznie referencje do immutable TaskVersion Sakura Task v1; nie są innym formatem zadania. Sugerowane harmonogramy D0 np. codzienny audyt 07:00, ale nie są aktywowane samym przypisaniem profilu; user potwierdza grant i cele w etapie 5. Zmiany produkcyjne nigdy nie są automatycznie włączane. Profile nie zawierają sekretów, adresów produkcyjnych ani dowolnych skryptów.

## Wersje i UI

Profile(id, version, digest, typed defaults, restrictive policy, pinned task refs); immutable published version. Draft/preview/publish, przypisanie jawnej wersji, rollback konfiguracji jako nowa revision (nie przywracanie revoke). Host effective config pokazuje źródło pola i blokadę, różnice przed zapisaniem. Aktualizacja profilu nie nadpisuje przypisanej wersji bez zgody; nowy plan/approval po zmianie. Plan pin host IDs, profile version, policy revision, group membership digest, adapter/version i lokalne capabilities. Recheck przed wykonaniem. Retired/suspended host nie może otrzymać zadań. Zmiana opisu/tagu bez wpływu na policy nie musi unieważniać planu; revision bezpieczeństwa oddzielna od metadata revision.

## Testy

Host w dwóch grupach z konfliktem ograniczeń, deny precedence, próba poszerzenia host override, środowisko prod -> lab bez zgody, zmiana członkostwa podczas planu, stara wersja profilu, brak adaptera, wygaśnięty grant, przypisanie profilu bez włączenia harmonogramu/SSH/root, profile publish/audit i widoczność scope. D0 na nowym hoście bez ręcznego YAML. W MVP Basic; rozszerzenia wraz z odbiorem działań.
