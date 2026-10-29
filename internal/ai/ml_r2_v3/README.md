# ML r2 v3 (ml-r2-v3)

## Mi ez?

Az r2 szabályrendszer harmadik tanult kiértékelésű motorja. A keresés az r2
alfabéta baseline-nal azonos; a hangsúly a **jellemzőkön**, a **tanító adaton**
és a **gyorsabb inferencián** van.

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

## Jellemzők (50 input)

A 43 alapjellemzőn felül 7 új, a csere **védekező értékét** leíró jellemző:

- támadott varázslók, amelyek tudnak cserélni;
- támadott varázslók, amelyek nem tudnak;
- biztonságos (nem támadott) cserecélpontok száma;
- a legtöbb szomszédos saját bábu egy varázsló körül;
- szomszédos varázsló-párok;
- teleport-kész állapot (token >= költség);
- a tokenkészlet távolsága a teleport-költségtől.

A háló: **50 → 96 → 1** MLP (ReLU). A normalizálás (mean/std) az első réteg
súlyaiba van beépítve, az inferencia **float32** tömbökkel, allokáció nélkül fut.

## Edzés

- Tanár: `alphabeta-r2-v1` önjátszma, **500 játszma, 500 ms/lépés**, depth 6.
- Címke: a tanár keresési pontszáma (dense score) + a játék kimenetele.
- Game-szintű train/val/test split, Adam, MSE, L2, early stopping.

```bash
make dataset-r2-v3
make train-r2-v3
```

## Konfiguráció

`MaxTime` (élő: 1,5 s / 2,5 s / 0,4 s), `MaxDepth` (alap 8, aréna 6),
`MaxNodes`, `Seed`, `DisableFilter`.

## Ismert gyengeségek

- A tanár 500 ms-os keresése továbbra is zajos címkéket ad a mély taktikai
  állásokban.
- A nagyobb háló több számítást igényel, de a float32 inferencia ezt kompenzálja.
- A csere védelmi értéke közelítés: a jellemzők nem szimulálják a csere utáni
  teljes állást.

## Változások az előző verzióhoz képest

- 43 → **50 jellemző** (7 csere-védelmi jellemző).
- 64 → **96 rejtett neuron**.
- **Float32, normalizálás-beépített inferencia**; nincs allokáció a levél-
  kiértékelésben.
- Tanító adat: 300 @ 300 ms → **500 @ 500 ms**.

## Futtatás és tesztelés

```bash
go test ./internal/ai/ml_r2_v3
go run ./cmd/arena-eval -pairs "ml-r2-v3|ml-r2-v2" -games 50 \
  -time 1500ms -depth 6 -nodes 0 -paired
```
