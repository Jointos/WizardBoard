# Wizard Boardgame – Játékszabály

> Ez a fájl a játék hivatalos, ember által olvasható szabálya (magyar).
> A gép által olvasható konfiguráció a `internal/game/rules.go` fájlban van
> (`Rules` struct). Ha a kettő eltér, ez a dokumentum az igazságforrás, és a
> kódot kell hozzáigazítani.

## 1. Alapok

- **Tábla:** 8×8 mező, koordináták: `a`–`h` oszlop, `1`–`8` sor.
- **Játékosok:** 2 (A és B). A az alsó (1–3. sor), B a felső (6–8. sor) oldalon áll.
- **Egységek / játékos:** 6 gyalog, 4 tüzér, 2 varázsló (összesen 12 bábu).
- **HP:** minden bábu 1 HP – ha eltalálják, meghal.
- **Körök:** a két játékos felváltva lép (A, B, A, B, …).

## 2. Kezdőfelállás

Nincs lerakási (placement) fázis: a bábuk fix, szimmetrikus helyen állnak.
A két középső sor (4. és 5.) üres. Minden sorban a bábuk középre igazítva állnak.

```
     a  b  c  d  e  f  g  h
8    .  .  .  V  V  .  .  .     B – varázsló
7    .  .  T  T  T  T  .  .     B – tüzér
6    .  G  G  G  G  G  G  .     B – gyalog
5    .  .  .  .  .  .  .  .     üres
4    .  .  .  .  .  .  .  .     üres
3    .  G  G  G  G  G  G  .     A – gyalog
2    .  .  T  T  T  T  .  .     A – tüzér
1    .  .  .  V  V  .  .  .     A – varázsló
```

- Gyalogok: `b3`–`g3` (A), `b6`–`g6` (B)
- Tüzérek: `c2`–`f2` (A), `c7`–`f7` (B)
- Varázslók: `d1`, `e1` (A), `d8`, `e8` (B)

A tüzérek kezdő nézőiránya a szemben lévő játékos felé mutat
(A: észak, B: dél).

## 3. Körsorrend (round-robin)

Nincs szabad bábuválasztás: minden játékosnak saját, fix körsorrendje van

```
gyalog → tüzér → varázsló → gyalog → …
```

A körök váltakozva mennek, így a tényleges sorrend:

```
A gyalog, B gyalog, A tüzér, B tüzér, A varázsló, B varázsló, A gyalog, …
```

- A soron következő típusból **egy** bábuval kell akciózni.
- Ha egy típusból **már nincs élő bábu**, a sorrend átugorja, és a játékos a
  következő meglévő típusával lép.
- Ha a soron lévő típusnak **nincs legális akciója**, a játékos kényszerűen
  passzol (a kör elmegy).

## 4. Gyalog

- **Mozgás:** mint a sakk királya – 1 mező bármelyik irányba (8 irány),
  de csak **üres** mezőre. Mozgással nem lehet ütni.
- **Ütés** (csak szomszédos mezőn álló ellenséges bábura):
  - **Előre:** minden **3. saját akciója** alkalmával. Az „előre” a 3 első
    irány (bal-elöl, elöl, jobb-elöl). Ez lehetőség, nem kötelező; ha nincs
    célpont, a gyalog simán mozoghat.
  - **Hátra:** bármikor (a 3 hátsó irány).
  - **Oldalra:** csak **buffal** (a 2 oldalsó irány).
- Az akciószámláló gyalogonként külön számol, és minden akció (mozgás vagy
  ütés) növeli.

## 5. Tüzér

A tüzérnek **nézőiránya** van (8 irány). Egy körben egy akciót választ:

1. **Lövés:** pontosan **2 mezőre egyenesen előre** (a nézőirányba).
   - A lövés **átlő a bábukon** (nincs LOS-követelmény).
   - Csak ellenséges bábura lehet lőni.
   - A lövés az akció, a tüzér **nem lép** mellé.
   - **Buffal** 2 **vagy 3** mezőre is lőhet.
2. **Fordulás:** helyben, bármelyik irányba (nem kötelező lépni).
3. **Lépés:** 1 mező bármelyik irányba, **üres** mezőre; utána **opcionálisan**
   fordulhat.

A tüzér ütni kizárólag lövéssel tud, mozgással nem.

## 6. Varázsló

A varázsló körében az alábbiak egyike:

### 6.1 Skip (kör kihagyása)

- A varázsló nem tesz semmit, cserébe a játékos **+1 tokent** kap
  (max **4** token játékosonként).

### 6.2 Varázslás (buff / nerf / cleanse)

- **Hatótáv:** korlátlan, de csak a varázsló **8 vonala** mentén.
- A varázsló **csak olyan bábut tud megcélozni, amely pontosan egyenesen
  (vízszintesen/függőlegesen) vagy pontosan átlósan áll tőle.**
- Abban az irányban a **szomszédos mezőnek üresnek kell lennie**; a varázsló
  oda lép, majd onnan varázsol.
  - Ha a célpont szomszédos (nincs üres mező az irányban), nem lehet varázsolni.
  - Ha a szomszédos mező foglalt, nem lehet varázsolni.
  - **Buff és nerf:** nincs LOS-követelmény, a bábuk nem takarnak.
  - **Cleanse:** ezen felül LOS is kell a lépés utáni mezőről.
- **Önmagára nem varázsolhat.**
- **Buff:** csak saját (szövetséges) bábura.
- **Nerf:** csak ellenséges bádura.
- **Cleanse:** szövetségesről **nerfet**, ellenségről **buffot** töröl.
  Csak akkor legális, ha van mit törölni.
- Egy varázslónak egyszerre **max 1 buffja és max 1 nerfje** lehet.
  Ha új célpontra buffol/nerfel, a korábbi hatás lekerül a régi célpontról.
- Egy célponton lehet egyszerre buff és nerf (két külön varázslótól).
- A hatás a varázsló **haláláig** vagy **cleanse-ig** tart.

### 6.3 Hatások

| Hatás | Célpont | Effekt |
|---|---|---|
| Buff gyalog | saját gyalog | oldalra is tud ütni |
| Buff tüzér | saját tüzér | 2 vagy 3 mezőre lőhet |
| Buff varázsló | saját varázsló | varázslás után bárhová léphet a táblán (egyszer) |
| Nerf gyalog | ellenséges gyalog | ha tud, hátra kell lépnie (üres mezőre) |
| Nerf tüzér | ellenséges tüzér | nem tud fordulni (csak lő vagy lép) |
| Nerf varázsló | – | **nincs ilyen** (kivéve a játékból) |

- **Buffolt varázsló** varázslás után egyszer bárhová léphet egy üres mezőre
  (vagy ott maradhat). Ez a varázslás része, nem külön kör.

### 6.4 Token / teleport

- A token **játékosonként** gyűlik (max 4), bármelyik varázslója használhatja.
- A teleport **4 tokenba kerül** (tehát csak teli tokenkészlettel lehet).
- A teleport után a varázsló **bármelyik üres mezőre** kerülhet.
- A teleport a varázsló akciója (a körét viszi).
- A tokenek a varázsló halála után is megmaradnak (játékos-szintűek).

### 6.5 Csere (swap)

- A varázsló a körét arra is használhatja, hogy helyet cseréljen egy
  **szomszédos saját bábuval** (a 8 irány bármelyike, átlósan is).
- A célpont lehet gyalog, tüzér vagy a másik varázsló.
- A csere a varázsló akciója (a körét viszi), és nem kerül tokenbe.
- A bábuk identitása nem változik: a buff/nerf és a gyalog akciószámláló a
  bábuval együtt mozog.
- A csere nem támadás: nem üt ki bábut és nem ad tokent.

## 7. Játék vége

- Egy játékos akkor veszít, ha **mindkét varázslója meghalt**.
- A győztes az, akinek maradt varázslója.
- Döntetlen: ha a lépésszám eléri a `MaxPly` korlátot (alapértelmezés: 300 kör),
  a játék döntetlen. (Ez az ML/önjátszás miatt van, emberi játékban ritka.)

## 8. Implementációs jegyzetek

- A motor `internal/game` csomagja tiszta, side-effect mentes: `Apply` mindig új
  `State`-et ad vissza, az eredeti nem változik.
- A játék determinisztikus és teljes információs → negamax / MCTS használható.
- A `State.Actions(buf)` adja a legális akciókat, a `State.Hash()` a
  transzpozíciós táblához használható.
- A `Rules` structban állítható a `PawnForwardCaptureEvery`, `CannonRange`,
  `CannonBuffRange`, `MaxTokens`, `MaxPly`.
- **Wizard-nerf nincs** a szabályban: a motor a `Rules.WizardNerf` kapcsolóval
  (alapból `false`) tiltja a nerf generálását és alkalmazását, a kliens pedig
  nem is jelenít meg nerfet varázslón.
