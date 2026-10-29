# ML r2 v1 (ml-r2-v1)

## Mi ez?

Az r2 szabályrendszer tanult kiértékelésű motorja. A keresés és a lépés-
generálás az r2 alfabéta baseline-nal azonos (negamax, alpha-béta, transzpozíciós
tábla, iteratív mélyítés, jelölt-szűrés); a levélkiértékelés egy kézi heurisztika
és egy kis neurális háló összege.

## Hogyan választ lépést?

1. A gyökér jelölteket a `DisableFilter` kapcsoló szerint szűri (élő/aréna
   alapból szűrő nélkül).
2. Iteratív mélyítéssel keres, a transzpozíciós tábla és a taktikai
   lépéssorrend segítségével.
3. A levélkiértékelés:

   ```text
   eval = HandEval(állás) + clamp(model(features), -100, +100)
   ```

   A `HandEval` a szabályrendszer szerinti kézi értékelés, a modell tanult
   korrekciója legfeljebb ±100 ponttal térhet el tőle.
4. Idő- vagy csomópontkorlát esetén az utolsó teljes mélység legjobb lépését
   adja vissza.

## Értékelés / heurisztikák

A kézi rész ugyanazokat a tagokat használja, mint az r2 alfabéta motor:

- anyag: gyalog 4, tüzér 12, varázsló 100;
- buff/nerf a megjelölt bábura;
- gyalog-előrehaladás, célzó tüzér, tüzér-mobilitás;
- varázsló-veszély (tüzér/gyalog/varázsló);
- tokenek és a függőben lévő szabad lépés.

A tanult rész 24 oldal-relatív jellemzőt lát: egységszámok, buff/nerf számok,
tokenek, fázis, gyalog-előrehaladás, célzások, mobilitás, varázsló-veszély,
bábszám, kézi kiértékelés, ply és a függőben lévő lépés jelzője. A háló egy
24 → 64 → 1 MLP (ReLU), a súlyok a `weights.json`-ban, `go:embed`-del beágyazva.

**Csere (r2):** a swap nem kap külön statikus tagot. A keresés legális
lépésként vizsgálja, a tanult modell pedig a csere utáni állás jellemzőiből
tanulja meg az értékét.

## Konfiguráció

- `MaxTime` — élő játékban 1,5 s (erős), 2,5 s (max), 0,4 s (fast).
- `MaxDepth` — alapból 8, az arénában és az assistban 6.
- `MaxNodes` — dataset-generáláshoz / hibakereséshez.
- `Seed` — determinisztikus gyökér-keverés.
- `DisableFilter` — a tüzér lépés+fordulás szűrő kikapcsolása.

## Bonyolultság és futásidő

A keresés az alfabéta baseline-nal azonos; a levélkiértékelés plusz költsége a
24 jellemző kiszámítása és a kis MLP előre-hívása. Ezért azonos időkorláton
belül jellemzően egy mélységgel kevesebbre jut, cserébe pontosabb a
levélpozíciók megítélése.

## Ismert gyengeségek

- A tanult korrekció a tanító adatok eloszlására érzékeny; a ritka, mély
  taktikai állásokat kevésbé látja.
- A modell a keresés alatt ugyanazt a kézi értékelést használja alapként, ezért
  a kézi rész hibái továbbra is megjelennek.
- A csere értékét nem külön jellemző, hanem a csere utáni állás jellemzői
  hordozzák.

## Változások az előző verzióhoz képest

Első verzió ebben a szabályrendszerben.

## Futtatás és tesztelés

```bash
make dataset-r2      # r2 alfabéta önjátszma -> data/datasets/r2_ml_v1.jsonl
make train-r2        # weights.json újratanítása
make ml-r2-eval      # ml-r2-v1 vs alphabeta-r2-v1 (1,5 s / 6 / szűrő nélkül)
go test ./internal/ai/ml_r2_v1
```
