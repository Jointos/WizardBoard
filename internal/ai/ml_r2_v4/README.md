# ML r2 v4 (ml-r2-v4)

## Mi ez?

Az `ml-r2-v3` keresésének és kiértékelésének továbbfejlesztése **quiescence
kereséssel**. A fő keresés horizontján túl a motor már nem áll meg a statikus
értékelésnél: az ütéseket és lövéseket tovább vizsgálja, így nem sétál bele egy
horizonton túli visszaütésbe.

## Hogyan választ lépést?

1. A gyökér jelölteket a `DisableFilter` szerint szűri.
2. Iteratív mélyítéssel keres (negamax, alpha-béta, transzpozíciós tábla).
3. Amikor a fő keresés eléri a mélységhatárt (vagy `MaxDepth`-et), a
   `quiesce` lép be:
   - **stand-pat**: a soron lévő fél megtarthatja a statikus értékelést, tehát
     nem kényszerül rossz ütésre;
   - csak `capture` és `shoot` lépéseket vizsgál;
   - a varázslót ütő lépések mennek előre;
   - legfeljebb **4** extra fél-lépés mélyen;
   - ugyanazt az idő- és csomópontkorlátot használja, mint a fő keresés.
4. A levélkiértékelés a v3-mal azonos:

   ```text
   eval = HandEval(állás) + clamp(model(50 jellemző), -100, +100)
   ```

## Jellemzők és modell

A jellemzők és a háló **megegyeznek az `ml-r2-v3`-mal**: 50 jellemző,
50 → 96 → 1 MLP, float32, normalizálás-beépített inferencia. A v4 ugyanazt a
tanított súlyfájlt használja (a tanár és a jellemzők azonosak), a különbség
kizárólag a keresésben van.

## Konfiguráció

`MaxTime` (élő: 1,5 s / 2,5 s / 0,4 s), `MaxDepth` (alap 8, aréna 6),
`MaxNodes`, `Seed`, `DisableFilter`.

## Bonyolultság és futásidő

A quiescence megnöveli a levélközeli csomópontszámot, ezért azonos időkeretben
a fő keresés jellemzően valamivel alacsonyabb mélységet ér el. Cserébe a
taktikai hibák (horizont-effektus) jelentősen csökkennek. Az időkorlát miatt a
összköltség nem nő, csak a csomópontok eloszlása tolódik a taktikai vonalak
felé.

## Ismert gyengeségek

- A quiescence csak ütéseket és lövéseket vizsgál; a csere, a buff és a nerf
  nem része a taktikai kiterjesztésnek.
- A stand-pat ebben a játékban nem pontosan „passzolás”, de a statikus
  értékelés felső korlátként való használata jól működik a gyakorlatban.
- A nagyobb csomópontszám miatt a mélyebb stratégiai vonalak kicsit
  rövidebbek lehetnek.

## Változások az előző verzióhoz képest

- **Quiescence keresés** (capture/shoot, stand-pat, max. 4 extra fél-lépés).
- A varázslót ütő folytatások priorizálása a quiescence-ben.
- A jellemzők, a háló és a súlyok változatlanok a v3-hoz képest.

## Futtatás és tesztelés

```bash
go test ./internal/ai/ml_r2_v4
go run ./cmd/arena-eval -pairs "ml-r2-v4|ml-r2-v3" -games 50 \
  -time 1500ms -depth 6 -nodes 0 -paired
```
