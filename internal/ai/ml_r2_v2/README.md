# ML r2 v2 (ml-r2-v2)

## Mi ez?

Az r2 szabályrendszer második tanult kiértékelésű motorja. Ugyanazt az r2
alfabéta keresést használja, mint az `ml-r2-v1`, de **részletesebb jellemzővektort**
és **dense keresési pontszám-címkéket** tanul, 64 rejtett neuronnal.

## Hogyan választ lépést?

1. A gyökér jelölteket a `DisableFilter` szerint szűri (élő/aréna alapból
   szűrő nélkül).
2. Iteratív mélyítéssel keres (negamax, alpha-béta, transzpozíciós tábla,
   taktikai lépéssorrend).
3. A levélkiértékelés:

   ```text
   eval = HandEval(állás) + clamp(model(features), -100, +100)
   ```

4. Idő- vagy csomópontkorlát esetén az utolsó teljes mélység legjobb lépése.

## Jellemzők (43 input)

A 43 oldal-relatív jellemző 41 általános állásjellemzőből és 2 r2-specifikus
csere-jellemzőből áll:

- anyag-, buff- és nerf-szám típusonként;
- tokenek, fázis (one-hot), gyalog-előrehaladás;
- célzások típusonként, célzott típusok, buffolt célzás;
- tüzér-mobilitás, varázsló-veszély típusonként;
- gyalog forward-kész, teleport-kész, varázsló–ellenfél távolságok;
- bábszám, üres mezők, kézi kiértékelés és komponensei;
- ply / MaxPly, függőben lévő lépés;
- **csere (r2):** hány varázslónak van szomszédos saját bábuja, és összesen
  hány ilyen szomszéd van (varázslónként 4-re vágva).

A háló: **43 → 64 → 1** MLP (ReLU), a súlyok `go:embed`-del beágyazva.

## Edzés

A tanító adat kizárólag `alphabeta-r2-v1` önjátszmából származik (nincs random,
greedy vagy régi szabályrendszerű adat). A címke a tanár keresési pontszáma
(dense score), kiegészítve a játék kimenetelével.

```bash
make dataset-r2-v2   # 300 önjátszma, 300 ms/lépés
make train-r2-v2     # 43→64→1 MLP, score címkék
```

## Konfiguráció

`MaxTime` (élő: 1,5 s / 2,5 s / 0,4 s), `MaxDepth` (alap 8, aréna 6),
`MaxNodes` (dataset/debug), `Seed`, `DisableFilter`.

## Ismert gyengeségek

- A dense score címkék a tanár heurisztikáját is megtanulják, így a tanár
  hibái részben átkerülnek.
- A tanult korrekció a tanító adatok eloszlására érzékeny.
- Több jellemző → nagyobb modell, de a keresés sebessége az `ml-r2-v1`-hez
  hasonló marad (a plusz két csere-jellemző olcsó).

## Változások az előző verzióhoz képest

- 24 → **43 jellemző**: az általános állásjellemzők mellett két r2 csere-
  jellemző (csereképes varázslók, szomszédos saját bábuk).
- Outcome helyett **dense keresési pontszám** címkék.
- 64 rejtett neuron, 43 bemenet.
- **Alapértelmezett motor** (a 100 játszmás párosított meccsen 67–29–4, 0.690).

## Futtatás és tesztelés

```bash
go test ./internal/ai/ml_r2_v2
make ml-r2-v2-selfplay     # 100 önjátszma, stats-only
go run ./cmd/arena-report -in data/balance/balance_mlr2v2_selfplay_<stamp>.json \
  -out data/reports/r2_mlv2_selfplay_report.html -stamp
```
