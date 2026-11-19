# Powiadomienia na żywo — SSE

W SakuraOps wybieramy **Server-Sent Events (SSE)** dla sygnałów backend -> panel.

## Dlaczego SSE, nie WebSocket?

W tym etapie klient musi **otrzymywać** wiadomość, że konfiguracja repozytoriów
się zmieniła. Polecenia w konsoli i zapisy z GUI już korzystają z
uwierzytelnionego API HTTP. SSE jest prostsze i obsługuje automatyczne ponawianie
połączenia. WebSocket rozważymy dopiero dla przyszłych w pełni dwukierunkowych
sesji lub interaktywnego sterowania zadaniami.

## Działanie

- Po otwarciu sekcji Repozytoria przeglądarka tworzy `EventSource` dla
  `/api/repositories/events`.
- Endpoint wymaga aktywnej sesji administratora, nie ujawnia treści repozytoriów.
- Serwer sprawdza odcisk konfiguracji SQLite co około **1 sekundę** i wysyła
  `repositories-changed` przy zmianie (także z innej karty lub procesu).
- Przeglądarka pobiera wtedy listę przez istniejące `/api/repositories`,
  bez przeładowania dokumentu. Duplikaty wyników nie przebudowują listy.
- Po opuszczeniu widoku lub ukryciu karty połączenie jest zamykane.
  Po powrocie lista jest ponownie odczytywana.
- Jeśli SSE zostanie przerwane lub przeglądarka go nie obsługuje,
  awaryjne odpytywanie działa co **30 sekund**.
- Co 15 sekund serwer wysyła heartbeat; konto jest ponownie weryfikowane.

## Zakres

To nie jest jeszcze system dystrybucji zdarzeń dla dużej liczby klientów.
Dla niewielkiej instalacji administratorów aktualne sprawdzanie SQLite jest
wystarczające. Przed dużym wdrożeniem warto przejść na publikowanie zdarzeń
przez broker lub tabelę rewizji, zamiast odpytywania bazy dla każdego klienta.

Nie daje to dostępu do powłoki ani nie uruchamia synchronizacji Git.
Operacje Ansible nadal pozostają poza zakresem.

## Odbiór

1. Zaloguj się do SakuraOps w dwóch kartach.
2. W obu otwórz Repozytoria.
3. Dodaj konfigurację w jednej karcie; druga powinna ją zobaczyć po ok. 1 s
   bez pełnego odświeżania strony.
4. Powtórz test, dodając repozytorium komendą `repository add` w terminalu.
5. Sprawdź powrót do karty po dłuższej nieaktywności.

## Kolejny etap: synchronizacja Git

Przed uruchomieniem automatycznej synchronizacji prywatnego Forgejo trzeba
dodać mechanizm zarządzania poświadczeniami (najlepiej read-only deploy key),
weryfikację SSH host key, ograniczenia dopuszczonych Git hostów,
izolowany katalog checkoutów, blokadę równoległych synchronizacji,
limity czasu i rozmiaru oraz audyt operacji. Konfiguracja musi być dostępna
w GUI i CLI przez tę samą usługę. Nie wolno przyjmować tokenów jako argumentów
komend (trafiłyby do historii terminala).
