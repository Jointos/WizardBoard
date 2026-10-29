# AlphaBeta r2 v1 (alphabeta-r2-v1)

## Mi ez?

Az r2 szabályrendszer alfabéta alapmotorja: negamax alpha-béta vágással,
transzpozíciós táblával, iteratív mélyítéssel és jelölt-szűréssel. Ez az
összehasonlítási alap (baseline), amelyhez a szabályrendszer ML motorja mérhető.

## Hogyan választ lépést?

1. A legális lépésekből (alapból) kiszűri a haszontalan `moveturn` jelölteket:
   csak azt a tüzér lépés+fordulás kombinációt tartja meg, amelynek az új
   irányában ellenfél áll a lőtávolságon belül, vagy amelyik az ellenfél térfele
   felé néz. A szűrő `DisableFilter`-rel kikapcsolható; az élő/aréna beállítás
   alapból szűrő nélküli.
2. A megmaradt jelölteket véletlenszerűen rendezi (a `Seed` determinálja).
3. Iteratív mélyítéssel 1..`MaxDepth` mélységig keres; a következő mélységben a
   legjobb lépés kerül előre.
4. A csomópontokon transzpozíciós táblát használ (exact/lower/upper flag), a
   lépéssorrendet taktikai kategóriák szerint számolja, és a tábla legjobb
   lépését előre hozza.
5. `MaxTime` vagy `MaxNodes` elérésekor leáll, és az utolsó befejezett mélység
   legjobb lépését adja vissza.

## Értékelés / heurisztikák

A levélkiértékelés a lépésre következő oldal szemszögéből számol:

- **Anyag:** gyalog 4, tüzér 12.
- **Varázsló:** 2 élő varázslónál 55/db, 1 élő varázslónál 95. Az első
  varázsló elvesztése olcsóbb, az utolsó védelme a prioritás; a végállás
  ±1 000 000 − ply.
- **Buff / nerf:** +3 / −3 a megjelölt bábura.
- **Gyalog előrehaladás:** `rank/2` az ellenfél hátulsó sora felé.
- **Célzó tüzér:** +7, ha a nézési irányban ellenfél áll lőtávolságon.
- **Tüzér mobilitás:** max +4 az üres szomszédos mezők után.
- **Varázsló-veszély:** saját varázslónként tüzér 9, gyalog 5, varázsló 2; a
  saját fenyegetés levonódik, az ellenfélé hozzáadódik.
- **Token:** ±2 tokondonként.
- **Csere (r2):** nincs külön anyagi tagja. A csere pozíciós lépés: a keresés
  közvetlenül látja a csere utáni állást, és a statikus értékelés (varázsló-
  veszély, gyalog-előrehaladás, tüzér célzás) már tartalmazza a következményt,
  ezért külön bónusz duplán számolná.

## Konfiguráció

- `MaxTime` — élő játékban 1,5 s (erős), 2,5 s (max), 0,4 s (fast).
- `MaxDepth` — alapból 8, az arénában és az assistban 6.
- `MaxNodes` — csak hibakereséshez / dataset-generáláshoz.
- `Seed` — a gyökér-lépések keverése; azonos seed = azonos viselkedés.
- `DisableFilter` — a tüzér lépés+fordulás szűrő kikapcsolása.

## Bonyolultság és futásidő

Az elágazás a tüzér lépés+fordulás lépések miatt nagy; a szűrő ezt a
legfontosabb esetben harmadára-negyedére csökkenti. A keresés időkorlátos:
1,5 s alatt jellemzően 5–6 mélységig jut el. A csere legális lépésként jelen
van, de csak akkor, ha a varázsló szomszédjában saját bábu áll.

## Ismert gyengeségek

- A statikus értékelés nem jutalmazza külön a csere lehetőségét; mély taktikai
  csapdákat csak a keresés lát.
- Az első varázsló feláldozása vonzó lehet, ha a kompenzáció nem látszik
  elég mélyen.
- A szűrő nélküli módban a tüzér lépés+fordulás lépések miatt a mélység
  alacsonyabb lehet.

## Változások az előző verzióhoz képest

Első verzió ebben a szabályrendszerben.

## Futtatás és tesztelés

```bash
go test ./internal/ai/alphabeta_r2_v1
go run ./cmd/arena-eval -pairs "alphabeta-r2-v1|alphabeta-r2-v1" -games 10 \
  -time 1500ms -depth 6 -nodes 0 -no-elo -out data/balance/r2_smoke.json
```
